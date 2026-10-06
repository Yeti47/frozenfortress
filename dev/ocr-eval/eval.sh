#!/usr/bin/env bash
# OCR and metadata-extraction evaluation on limited CPU profiles. See dev/ocr-eval/README.md.
#
# Every run starts a fresh container pinned to a cpuset, with a matching CPU quota and a memory limit,
# and writes <label>.log, <label>.json and (for OCR runs) text-<label>.json to the output directory.
#
# Usage: dev/ocr-eval/eval.sh <command> [args...]
#   build                                   build the eval images (and the repo's CPU Ollama image for glm)
#   models [file...]                        download the pinned NuExtract GGUF files (default: all) and verify their hashes
#   paddle  <label> <cpuset> <config> [extra-json]
#                                           PaddleOCR on PaddlePaddle; config: v6-tiny|v6-small|v6-medium|v5-mobile-latin
#   rapid   <label> <cpuset> [bench_rapid.py args...]
#                                           PP-OCR via RapidOCR, e.g. --tier tiny, --engine openvino
#   glm     <label> <cpuset> [bench_glm.py args...]
#                                           GLM-OCR via the repo's Ollama image (docker/ollama, target cpu)
#   extract <label> <cpuset> <model.gguf> [bench_extract.py args...]
#                                           NuExtract via llama.cpp server, e.g. --fmt v2 --source /out/text-<ocr-label>.json
#   report                                  print Markdown tables of all results
#   rescore                                 re-score all saved OCR texts (in-order accuracy, recall, umlauts)
#   clean                                   remove every container, image, network and volume this script created
#
# A cpuset uses Docker syntax ("0-3", "0,2,4,6"). On CPUs with SMT, check `lscpu -e` and pick distinct
# physical cores.
#
# Environment:
#   FF_EVAL_MEMORY      memory limit per container (default 6g)
#   FF_EVAL_RUNS        warm runs per payload (default 3)
#   FF_EVAL_OUT_DIR     output directory (default dev/ocr-eval/results)
#   FF_EVAL_MODELS_DIR  GGUF directory (default dev/ocr-eval/models)
#   FF_EVAL_CLIENT_CPUS cpuset for the client container of glm/extract runs (default: all CPUs)
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

MEMORY=${FF_EVAL_MEMORY:-6g}
RUNS=${FF_EVAL_RUNS:-3}
OUT_DIR=${FF_EVAL_OUT_DIR:-$HERE/results}
MODELS_DIR=${FF_EVAL_MODELS_DIR:-$HERE/models}
CLIENT_CPUS=${FF_EVAL_CLIENT_CPUS:-}

NETWORK=ff-eval
GLM_VOLUME=ff-eval-glm-models
# ghcr.io/ggml-org/llama.cpp:server
LLAMA_IMAGE=ghcr.io/ggml-org/llama.cpp@sha256:559ac229adefe0f7e2d4e32f5222f26927b6bb8ba44b9db08e2544afebf41984

# repo, revision, file, sha256
GGUF_MODELS=(
    "mradermacher/NuExtract-1.5-tiny-GGUF 2fa2d2acc159ca3cfe5ca8c774476b8195438744 NuExtract-1.5-tiny.Q8_0.gguf 4644640167723b4370710de0bff1bb473a0b5403b2ce2e0d0e37a1e544d269fc"
    "QuantFactory/NuExtract-1.5-smol-GGUF e618286df0c67739cc900128ee87ceb3a934d665 NuExtract-1.5-smol.Q8_0.gguf 1ef90ef2a1d530a9525dd0b58ff608fd710a26b12a51ecb73a6d77dedadfc382"
    "mradermacher/NuExtract-2.0-2B-GGUF 8a19f61615a6249ced4c6e9dddbb7f105699d0e4 NuExtract-2.0-2B.Q8_0.gguf c0a5c410ddfd3931e8d8fa02035d827058a937eaa9fcbd7235d7c8397fc84260"
    "mradermacher/NuExtract-2.0-2B-GGUF 8a19f61615a6249ced4c6e9dddbb7f105699d0e4 NuExtract-2.0-2B.Q4_K_M.gguf 4f1cf125f1e493314c479d971457bb860be67febed1c6017ba3226a4175f2257"
)

SERVICE=
cleanup() {
    [[ -z "$SERVICE" ]] || docker rm -f "$SERVICE" >/dev/null 2>&1 || true
}
trap cleanup EXIT

die() {
    echo "error: $*" >&2
    exit 1
}

# Number of CPUs in a Docker cpuset string such as "0-3,6".
count_cpus() {
    local n=0 part
    IFS=, read -ra parts <<<"$1"
    for part in "${parts[@]}"; do
        if [[ "$part" == *-* ]]; then
            n=$((n + ${part#*-} - ${part%-*} + 1))
        else
            n=$((n + 1))
        fi
    done
    echo "$n"
}

# Pinned cores plus a matching quota: Ollama, llama.cpp and ONNX Runtime size their thread pools from it.
limits() {
    echo "--cpuset-cpus $1 --cpus $(count_cpus "$1") -m $MEMORY --memory-swap $MEMORY"
}

ensure_network() {
    docker network inspect "$NETWORK" >/dev/null 2>&1 || docker network create "$NETWORK" >/dev/null
}

# Runs a bench script in an image as the calling user, with the output directory mounted at /out.
run_bench() {
    local label=$1 image=$2 net=$3 cpu_args=$4
    shift 4
    mkdir -p "$OUT_DIR"
    # shellcheck disable=SC2086
    docker run --rm --name "ff-eval-$label" --network "$net" $cpu_args --user "$(id -u):$(id -g)" \
        -e HOME=/home/bench -v "$OUT_DIR:/out" "$image" python "$@" --label "$label" >"$OUT_DIR/$label.log" 2>&1 \
        || { tail -5 "$OUT_DIR/$label.log" >&2; die "run $label failed, see $OUT_DIR/$label.log"; }
    grep '^RESULT' "$OUT_DIR/$label.log" | cut -c8- >"$OUT_DIR/$label.json"
    echo "done $label"
}

client_cpu_args() {
    [[ -z "$CLIENT_CPUS" ]] || echo "--cpuset-cpus $CLIENT_CPUS"
}

# Polls a readiness command inside a service container for up to 20 minutes (model downloads included).
wait_for() {
    local container=$1
    shift
    for _ in $(seq 1 600); do
        docker exec "$container" "$@" >/dev/null 2>&1 && return 0
        docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null | grep -q true \
            || die "$container exited: $(docker logs --tail 5 "$container" 2>&1)"
        sleep 2
    done
    die "$container did not become ready"
}

cmd=${1:-}
[[ -n "$cmd" ]] || die "missing command, see the header of $0"
shift

case "$cmd" in
build)
    docker build -f "$HERE/docker/Dockerfile.rapid" -t ff-eval-rapid "$HERE"
    docker build -f "$HERE/docker/Dockerfile.paddle" -t ff-eval-paddle "$HERE"
    docker build --target cpu -t ff-eval-ollama "$REPO/docker/ollama"
    ;;
models)
    mkdir -p "$MODELS_DIR"
    for spec in "${GGUF_MODELS[@]}"; do
        read -r repo rev file sha <<<"$spec"
        [[ $# -eq 0 ]] || [[ " $* " == *" $file "* ]] || continue
        if [[ ! -f "$MODELS_DIR/$file" ]]; then
            echo "downloading $file"
            curl -fL --progress-bar -o "$MODELS_DIR/$file.part" "https://huggingface.co/$repo/resolve/$rev/$file"
            mv "$MODELS_DIR/$file.part" "$MODELS_DIR/$file"
        fi
        echo "$sha  $MODELS_DIR/$file" | sha256sum -c --quiet || die "hash mismatch for $file"
    done
    echo "models ok"
    ;;
paddle)
    [[ $# -ge 3 ]] || die "usage: paddle <label> <cpuset> <config> [extra-json]"
    label=$1 cpuset=$2 config=$3 extra=${4:-'{}'}
    run_bench "$label" ff-eval-paddle none "$(limits "$cpuset")" bench_paddle.py --config "$config" \
        --threads "$(count_cpus "$cpuset")" --runs "$RUNS" --extra "$extra" --save-text "/out/text-$label.json"
    ;;
rapid)
    [[ $# -ge 2 ]] || die "usage: rapid <label> <cpuset> [bench_rapid.py args...]"
    label=$1 cpuset=$2
    shift 2
    run_bench "$label" ff-eval-rapid none "$(limits "$cpuset")" bench_rapid.py \
        --threads "$(count_cpus "$cpuset")" --runs "$RUNS" --save-text "/out/text-$label.json" "$@"
    ;;
glm)
    [[ $# -ge 2 ]] || die "usage: glm <label> <cpuset> [bench_glm.py args...]"
    label=$1 cpuset=$2
    shift 2
    ensure_network
    SERVICE=ff-eval-glm
    docker rm -f "$SERVICE" >/dev/null 2>&1 || true
    # shellcheck disable=SC2046
    docker run -d --name "$SERVICE" --network "$NETWORK" $(limits "$cpuset") \
        -v "$GLM_VOLUME:/models" ff-eval-ollama >/dev/null
    # The entrypoint pulls the model before serving it; wait until it is available.
    wait_for "$SERVICE" ollama show glm-ocr:q8_0
    run_bench "$label" ff-eval-rapid "$NETWORK" "$(client_cpu_args)" bench_glm.py \
        --runs "$RUNS" --save-text "/out/text-$label.json" "$@"
    ;;
extract)
    [[ $# -ge 3 ]] || die "usage: extract <label> <cpuset> <model.gguf> [bench_extract.py args...]"
    label=$1 cpuset=$2 model=$3
    shift 3
    [[ -f "$MODELS_DIR/$model" ]] || die "$MODELS_DIR/$model not found, run: $0 models"
    ensure_network
    SERVICE=ff-eval-llm
    docker rm -f "$SERVICE" >/dev/null 2>&1 || true
    # shellcheck disable=SC2046
    docker run -d --name "$SERVICE" --network "$NETWORK" $(limits "$cpuset") \
        -v "$MODELS_DIR:/models:ro" "$LLAMA_IMAGE" -m "/models/$model" --host 0.0.0.0 --port 8080 \
        -t "$(count_cpus "$cpuset")" -c 8192 -np 1 >/dev/null
    wait_for "$SERVICE" curl -sf localhost:8080/health
    run_bench "$label" ff-eval-rapid "$NETWORK" "$(client_cpu_args)" bench_extract.py --runs "$RUNS" "$@"
    ;;
report)
    python3 "$HERE/report.py" "$OUT_DIR"
    ;;
rescore)
    # The payloads are regenerated for scoring, which needs the fonts inside the image.
    docker run --rm --network none --user "$(id -u):$(id -g)" -v "$OUT_DIR:/out:ro" ff-eval-rapid \
        sh -c 'python rescore.py /out/text-*.json'
    ;;
clean)
    docker ps -aq --filter name=ff-eval- | xargs -r docker rm -f >/dev/null
    docker rmi -f ff-eval-rapid ff-eval-paddle ff-eval-ollama >/dev/null 2>&1 || true
    docker volume rm "$GLM_VOLUME" >/dev/null 2>&1 || true
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
    echo "removed eval containers, images, volume and network (results and models are kept)"
    ;;
*)
    die "unknown command $cmd"
    ;;
esac
