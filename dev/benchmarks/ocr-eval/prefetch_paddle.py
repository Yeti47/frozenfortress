"""Downloads the models of every PaddleOCR config at image build time, so benchmark runs need no network."""
from bench_paddle import CONFIGS, make

for name in CONFIGS:
    make(name, 2, {})
    print("prefetched", name, flush=True)
