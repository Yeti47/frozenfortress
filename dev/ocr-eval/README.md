# OCR and metadata-extraction evaluation

This harness compares OCR engines and small extraction models on CPU-only hardware. It was built for the v2 OCR decision (YETI-109) and is the starting point for the follow-up spike YETI-112. It is a research tool: nothing here is part of the product, and CI never runs it.

For the Go benchmark of the v1 Ollama OCR service, see [`doc/ocr-benchmark.md`](../../doc/ocr-benchmark.md).

## What it compares

| Command | Engine | Notes |
| -- | -- | -- |
| `rapid` | PP-OCR models via [RapidOCR](https://github.com/RapidAI/RapidOCR) on ONNX Runtime or OpenVINO | PP-OCRv6 tiny/small/medium (language `german`), PP-OCRv5 mobile + `latin` recognizer |
| `paddle` | PaddleOCR 3.7 on the native PaddlePaddle runtime | pinned to PaddlePaddle 3.2.2: 3.3.1 and 3.4.0 crash on the oneDNN path |
| `glm` | GLM-OCR via Ollama, using the repo's `docker/ollama` CPU image | same preprocessing as `OllamaOCRService` (resize to 640 px, JPEG q90) |
| `extract` | NuExtract via the llama.cpp server | 1.5-tiny, 1.5-smol, 2.0-2B (Q8_0, Q4_K_M); runs on ground truth or on saved OCR text |

## Payloads

`payloads.py` generates German documents in code, so no documents are stored here and every run is reproducible:

| Payload | Imitates |
| -- | -- |
| `invoice-a4-200dpi` | an invoice with a ruled table, amounts, IBAN and dates |
| `letter-a4-300dpi` | a letter with sender, reference number, subject and body text |
| `dense-a4-150dpi` | a full page of text |
| `phone-photo-12mp` | the invoice photographed with a phone: tilt, keystone, shadow band, noise, JPEG |
| `scanned-pdf-3p-200dpi` | an image-only PDF with three pages (invoice, letter, dense page) |

The text contains umlauts and ß on purpose. Each repetition uses a different seed, so no engine can profit from a cached request.

## Metrics

* **Word recall:** share of the printed words found in the OCR output, regardless of order. This is what search and extraction need.
* **In-order accuracy:** the same, but in reading order (longest common subsequence). It shows reading-order problems, e.g. table rows on tilted photos.
* **Umlaut recall:** word recall for words with ä, ö, ü or ß only.
* **Field accuracy** (extraction): exact match per field after normalisation (dates as day/month/year, amounts as numbers, IBAN without spaces).
* **Time:** cold start (first request after the container starts), median and p95 of the warm runs. **Peak RSS** of the OCR process.

## Usage

Needs Docker, Bash and Python 3 (only for `report`). Run it on an otherwise idle machine.

```bash
dev/ocr-eval/eval.sh build                 # images: ff-eval-rapid, ff-eval-paddle, ff-eval-ollama
dev/ocr-eval/eval.sh models                # NuExtract GGUFs (~5 GB), pinned revisions, SHA-256 checked

# OCR, pinned to physical cores 0-3
dev/ocr-eval/eval.sh rapid  r4-v6-tiny 0-3 --tier tiny
dev/ocr-eval/eval.sh rapid  r4-ov-tiny 0-3 --tier tiny --engine openvino
dev/ocr-eval/eval.sh paddle p4-v6-tiny 0-3 v6-tiny
dev/ocr-eval/eval.sh glm    g4-glm     0-3

# Extraction on the text RapidOCR produced above
dev/ocr-eval/eval.sh extract x4-2b-q4 0-3 NuExtract-2.0-2B.Q4_K_M.gguf --fmt v2 --source /out/text-r4-v6-tiny.json
dev/ocr-eval/eval.sh extract x4-2b-gt 0-3 NuExtract-2.0-2B.Q4_K_M.gguf --fmt v2     # on ground truth

dev/ocr-eval/eval.sh report                # Markdown tables
dev/ocr-eval/eval.sh rescore               # re-score saved OCR texts
dev/ocr-eval/eval.sh clean                 # remove containers, images, volume and network
```

`--fmt v1` is the NuExtract 1.5 prompt format (tiny, smol), `--fmt v2` the NuExtract 2.0 format with typed templates. `--en-keys` uses English template keys instead of German ones. The options of each runner are defined at the end of its `bench_*.py`.

Results go to `dev/ocr-eval/results/` and models to `dev/ocr-eval/models/`; both are git-ignored. See the header of `eval.sh` for the environment variables (memory limit, run count, directories).

### Pinning cores

Each service runs with `--cpuset-cpus` and a matching `--cpus` quota. The quota matters: Ollama, llama.cpp and ONNX Runtime size their thread pools from it. On CPUs with SMT, check `lscpu -e` and pick distinct physical cores (e.g. `0-3` on a host where 12-23 are the hyper-thread siblings of 0-11). The client container of `glm` and `extract` runs only sends requests; pin it elsewhere with `FF_EVAL_CLIENT_CPUS` if you want to keep it off the measured cores.

## Results from YETI-109 (2026-10-05)

Host: Xeon E5-2650 v4 (Broadwell, AVX2, no AVX-512), 4 physical cores per profile, 6 GB memory limit. Median per document / word recall:

| Run | Invoice | Letter 300 dpi | Phone photo | 3-page PDF | Peak RSS |
| -- | -- | -- | -- | -- | -- |
| RapidOCR v6-tiny, ONNX Runtime | 0.78 s / 100 % | 1.24 s / 99.8 % | 0.93 s / 99.1 % | 2.91 s / 99.2 % | 1.1 GB |
| RapidOCR v6-tiny, 2 cores | 1.13 s | 1.79 s | 1.26 s | 4.40 s | 1.1 GB |
| PaddleOCR v6-tiny, PaddlePaddle 3.2.2 | 1.29 s | 3.26 s | 3.65 s | 5.37 s | 2.5 GB |
| PaddleOCR v6-medium | 15.1 s | 35.0 s | 27.7 s | 70.6 s | 5.7 GB |
| GLM-OCR q8_0 at 640 px | 51.5 s / 89 %* | 69.3 s / 58 %* | 55.6 s / 77 %* | failed | – |

\* in-order accuracy. GLM-OCR looped on the dense page until the runner crashed; this was fixed in YETI-108.

Extraction from RapidOCR text with NuExtract-2.0-2B Q4_K_M: 100 % of the fields correct on invoices and letters; 14–16 s per document on 4 cores, 26–30 s on 2, 9 s on 8. NuExtract-1.5-tiny took about 7.5 s but got only 47–60 % of the letter fields right.

The full numbers, including the variants that lost, are in YETI-109.

## Limits

* The payloads are synthetic and cleanly typeset. Real scans (handwriting, fax quality, rotated pages, poor phone photos) still need to be checked; that is YETI-112. **Never commit real documents here.**
* Pinning cores reproduces core count and memory, not clock speed, cache size or memory bandwidth. Compare profiles on the same host.
* Extraction runs use only a few documents per type, with simple layouts.

## Updating the pins

The Python dependencies are pinned with hashes, as AGENTS.md requires. After editing `docker/requirements-*.in`:

```bash
cd dev/ocr-eval/docker
uv pip compile --generate-hashes --python-version 3.12 --python-platform x86_64-manylinux_2_28 \
    requirements-rapid.in -o requirements-rapid.txt
```

The base image, the llama.cpp server image and the GGUF files (revision and SHA-256) are pinned in the Dockerfiles and in `eval.sh`.
