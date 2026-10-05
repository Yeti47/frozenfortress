#!/usr/bin/env bash
# Benchmark the Ollama OCR service on limited CPU profiles. See doc/ocr-benchmark.md.
#
# For each profile this starts a fresh container of the repo's CPU Ollama image
# with pinned cores (cpuset plus a matching CPU quota) and a memory limit, runs the Go benchmark against it
# (core/documents, build tag ocrbench) and finally writes a Markdown report.
#
# Usage: dev/ocr-bench.sh [profile...]
#   A profile is a core count ("4", pins cores 0-3) or "name:cpuset" with a
#   cpuset in Docker syntax ("big:0-7", "small:0,2"). Default: 2 4 8
#   Append "@isa" to limit Ollama to the CPU backend of an older instruction
#   set: x64, sse42, sandybridge, haswell, skylakex, icelake or alderlake
#   ("4@sse42", "small:0,2@sse42"). Without it, Ollama uses the best backend
#   the host CPU supports.
#
# Environment:
#   FF_BENCH_MEMORY    memory limit per container (default 6g)
#   FF_BENCH_RUNS      warm runs per payload (default 3)
#   FF_BENCH_MAX_DIMS  comma-separated max image dimensions (default 640)
#   FF_BENCH_MODEL     model (default glm-ocr:q8_0)
#   FF_BENCH_OUT_DIR   output directory (default dev/ocr-bench-results/<timestamp>)
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

IMAGE=frozenfortress-ollama-bench
VOLUME=frozenfortress-ollama-bench-models
MEMORY=${FF_BENCH_MEMORY:-6g}
MODEL=${FF_BENCH_MODEL:-glm-ocr:q8_0}
OUT_DIR=${FF_BENCH_OUT_DIR:-dev/ocr-bench-results/$(date +%Y%m%d-%H%M%S)}
CONTAINER=
PROFILES=("$@")
[[ ${#PROFILES[@]} -gt 0 ]] || PROFILES=(2 4 8)

cleanup() {
    [[ -z "$CONTAINER" ]] || docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Ollama's CPU backends, oldest first. A profile with @isa hides every backend
# that is newer than the chosen one, so Ollama has to fall back to it.
ISA_LEVELS=(x64 sse42 sandybridge haswell skylakex icelake alderlake)

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

HOST_CORES=$(nproc --all)
mkdir -p "$OUT_DIR"

# Same image as the compose stack (digest-pinned base images in the Dockerfile).
docker build --quiet --target cpu -t "$IMAGE" docker/ollama >/dev/null

for profile in "${PROFILES[@]}"; do
    isa=
    if [[ "$profile" == *@* ]]; then
        isa=${profile##*@}
        profile=${profile%@*}
        isa_index=-1
        for i in "${!ISA_LEVELS[@]}"; do
            [[ "${ISA_LEVELS[$i]}" == "$isa" ]] && isa_index=$i
        done
        if (( isa_index < 0 )); then
            echo "invalid instruction set '$isa' (use ${ISA_LEVELS[*]})" >&2
            exit 1
        fi
    fi

    if [[ "$profile" == *:* ]]; then
        name=${profile%%:*}
        cpuset=${profile#*:}
    elif [[ "$profile" =~ ^[0-9]+$ && "$profile" -ge 1 ]]; then
        if (( profile > HOST_CORES )); then
            echo "skipping profile $profile: host has only $HOST_CORES logical cores" >&2
            continue
        fi
        name="${profile}c${isa:+-$isa}"
        cpuset="0-$((profile - 1))"
    else
        echo "invalid profile '$profile' (use N or name:cpuset)" >&2
        exit 1
    fi

    echo "== profile $name (cpuset $cpuset, memory $MEMORY${isa:+, $isa backend})"
    cpus=$(count_cpus "$cpuset")

    # Hide the newer CPU backends by mounting /dev/null over them; Ollama skips
    # a backend it cannot load.
    isa_args=()
    if [[ -n "$isa" ]]; then
        for i in "${!ISA_LEVELS[@]}"; do
            if (( i > isa_index )); then
                isa_args+=(-v "/dev/null:/usr/lib/ollama/libggml-cpu-${ISA_LEVELS[$i]}.so:ro")
            fi
        done
    fi
    CONTAINER="frozenfortress-ollama-bench-$name"
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    docker run -d --name "$CONTAINER" \
        --cpuset-cpus "$cpuset" --cpus "$cpus" --memory "$MEMORY" --memory-swap "$MEMORY" \
        -e "OLLAMA_MODEL=$MODEL" \
        -v "$VOLUME:/models" \
        "${isa_args[@]}" \
        -p 127.0.0.1::11434 \
        "$IMAGE" >/dev/null

    # The entrypoint pulls the model on the first run (about 1.6 GB, kept in the
    # volume afterwards). Wait until it is listed.
    deadline=$((SECONDS + 1800))
    until docker exec "$CONTAINER" ollama list 2>/dev/null | grep -qF "${MODEL%%:*}"; do
        if (( SECONDS > deadline )); then
            echo "model $MODEL did not become available" >&2
            docker logs --tail 20 "$CONTAINER" >&2
            exit 1
        fi
        sleep 3
    done

    port=$(docker port "$CONTAINER" 11434/tcp | head -n1 | sed 's/.*://')

    FF_BENCH_OLLAMA_URL="http://127.0.0.1:$port" \
    FF_BENCH_PROFILE="$name" \
    FF_BENCH_CPUSET="$cpuset${isa:+, $isa backend}" \
    FF_BENCH_MEMORY="$MEMORY" \
    FF_BENCH_OUT="$PWD/$OUT_DIR/$name.json" \
        go test -tags ocrbench -run '^TestOCRBenchmark$' -count=1 -timeout 0 -v ./core/documents

    # Ollama sizes its thread pool from the CPU quota (--cpus), not from the
    # cpuset. A mismatch would oversubscribe the cores and skew the numbers.
    threads=$(docker logs "$CONTAINER" 2>&1 | grep -o 'NumThreads:[0-9]*' | tail -n1 | cut -d: -f2)
    if [[ "$threads" != "$cpus" ]]; then
        echo "WARNING: profile $name: Ollama used ${threads:-?} threads on $cpus CPUs; results are not comparable" >&2
    fi

    backend=$(docker logs "$CONTAINER" 2>&1 | grep -o 'libggml-cpu-[a-z0-9]*' | tail -n1)
    echo "CPU backend used: ${backend:-unknown}"
    if [[ -n "$isa" && "$backend" != "libggml-cpu-$isa" ]]; then
        echo "WARNING: profile $name: expected $isa backend but Ollama loaded ${backend:-none}" >&2
    fi

    cleanup
    CONTAINER=
done

FF_BENCH_REPORT_DIR="$PWD/$OUT_DIR" \
    go test -tags ocrbench -run '^TestOCRBenchmarkReport$' -count=1 ./core/documents >/dev/null

echo
echo "Report: $OUT_DIR/report.md"
