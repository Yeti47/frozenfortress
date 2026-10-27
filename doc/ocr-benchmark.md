# OCR benchmark

This guide is for contributors. It measures how long the Ollama OCR service (`OllamaOCRService`, model `glm-ocr:q8_0`) needs per document on CPUs of different sizes, using realistic payloads.

To compare other OCR engines (PP-OCR via RapidOCR or PaddleOCR) and metadata-extraction models (NuExtract) with GLM-OCR, use [`dev/benchmarks/ocr-eval`](../dev/benchmarks/ocr-eval/README.md).

## Run it

Needs Docker and Go.

```bash
./dev/benchmarks/ocr-bench.sh            # profiles 2, 4 and 8 cores
./dev/benchmarks/ocr-bench.sh 4 16       # chosen core counts
./dev/benchmarks/ocr-bench.sh small:0,2 big:0-7   # name:cpuset, Docker cpuset syntax
./dev/benchmarks/ocr-bench.sh 4 4@sse42  # the same 4 cores with and without AVX2/FMA
```

Append `@isa` to a profile to restrict Ollama to the CPU backend of an older instruction set: `x64`, `sse42`, `sandybridge`, `haswell`, `skylakex`, `icelake` or `alderlake`. Ollama ships one backend per instruction set and normally picks the newest one the CPU supports. Many older NAS and mini-PC processors only have SSE4.2, which makes `4@sse42` a closer stand-in for them than `4`. The script hides the newer backends and checks the Ollama log to confirm which one was loaded.

The first run builds the CPU Ollama image from `docker/ollama` and downloads the model (about 1.6 GB) into the Docker volume `frozenfortress-ollama-bench-models`. Later runs reuse it. A full run takes a while on small profiles, because CPU inference is slow; that is what it measures.

| Variable | Default | Meaning |
| -- | -- | -- |
| `FF_BENCH_MEMORY` | `6g` | Memory limit per container (swap is disabled) |
| `FF_BENCH_RUNS` | `3` | Warm runs per payload and image dimension |
| `FF_BENCH_MAX_DIMS` | `640` | Comma-separated `FF_OCR_IMAGE_MAX_DIMENSION` values to sweep, e.g. `640,1024,1536` |
| `FF_BENCH_MODEL` | `glm-ocr:q8_0` | Model to test |
| `FF_BENCH_OUT_DIR` | `dev/benchmarks/ocr-bench-results/<timestamp>` | Output directory (git-ignored) |

The output directory holds one JSON file per profile and `report.md` with the comparison.

## What is measured

For each profile the script starts a fresh Ollama container with pinned cores (`--cpuset-cpus`), a matching CPU quota (`--cpus`) and a memory limit. The Go benchmark in `core/documents/ocrbench_test.go` (build tag `ocrbench`, so `go test ./...` and CI never run it) then calls the real `ExtractText`, including the resize and JPEG encoding step:

- **Cold start:** the first request after the container start, which includes loading the model.
- **Warm time:** median, minimum and p95 over several runs per payload.
- **Word accuracy:** the share of the words printed on the page that the OCR output contains in the right order (longest common subsequence). It shows what a smaller `FF_OCR_IMAGE_MAX_DIMENSION` costs in accuracy.
- **Failures:** requests that return an error are counted per payload and do not abort the run.

The payloads are generated in code, so no documents are stored in the repository and the input is the same on every benchmark run. Within a run, each repetition uses a different generated document with the same layout. This matters: Ollama caches the encoded image of a repeated request, so sending the same image twice makes the second request 5 to 15 times faster, which no real upload gets.

| Payload | Imitates |
| -- | -- |
| `dense-a4-150dpi`, `dense-a4-300dpi` | A full page of 10 pt text, scanned at two resolutions |
| `sparse-a4-150dpi` | A letter with a heading and a short paragraph |
| `invoice-a4-200dpi` | A table with ruled rows and numbers |
| `phone-photo-12mp` | A 12 MP JPEG with uneven lighting and noise, as from a phone camera |

## Limits of the emulation

Pinning cores and memory reproduces the **core count** and **RAM** of a smaller machine, and `@isa` reproduces the **instruction set** (no AVX2, no FMA). It does not reproduce clock speed, instructions per cycle, cache size or memory bandwidth. A 4-core profile on a fast desktop CPU is faster than 4 cores of a low-power NAS processor, even with `@sse42`. Decoding a 1.6 GB model is largely limited by memory bandwidth, and a desktop CPU with a large cache and fast RAM is a flattering host. Treat the numbers as a relative scale and compare them across profiles on the same host.

Other things to keep in mind:

- The CPU quota is required, not optional. Ollama sizes its thread pool from the quota and ignores the cpuset, so with a cpuset alone it starts one thread per physical core of the host. On a 4-core cpuset of an 8-core host that oversubscribed the cores and made the same request about 11 times slower (185 s instead of 17 s). The script prints a warning if the thread count in the Ollama log does not match the profile.
- On CPUs with SMT, cores `0-3` may be two physical cores and their hyper-thread siblings. Check `lscpu -e` and pass an explicit cpuset (`name:0,2,4,6`) to pin to distinct physical cores.
- Run on an otherwise idle machine. Other load changes the result.
- The model file is in the page cache after the first run, so the cold start does not include a disk read from a slow drive.
- Page text is synthetic English. Handwriting, other scripts and degraded scans can be slower or less accurate.
