"""Downloads the RapidOCR models used by the benchmark at image build time, so runs need no network."""
from bench_rapid import make

for engine, version, tier, lang in [
    ("onnxruntime", "PP-OCRv6", "tiny", ""),
    ("onnxruntime", "PP-OCRv6", "small", ""),
    ("onnxruntime", "PP-OCRv6", "medium", ""),
    ("onnxruntime", "PP-OCRv5", "mobile", "latin"),
    ("openvino", "PP-OCRv6", "tiny", ""),
]:
    make(engine, version, tier, lang, 2, 2000)
    print("prefetched", engine, version, tier, lang, flush=True)
