"""Benchmarks one PaddleOCR configuration in-process. Run once per container so the peak RSS is per config."""
import argparse
import json
import os
import tempfile
import time

import payloads
from score import mean_scores, peak_rss_mib, score, summarize

CONFIGS = {
    "v5-mobile-latin": dict(text_detection_model_name="PP-OCRv5_mobile_det",
                            text_recognition_model_name="latin_PP-OCRv5_mobile_rec"),
    "v6-tiny": dict(text_detection_model_name="PP-OCRv6_tiny_det", text_recognition_model_name="PP-OCRv6_tiny_rec"),
    "v6-small": dict(text_detection_model_name="PP-OCRv6_small_det", text_recognition_model_name="PP-OCRv6_small_rec"),
    "v6-medium": dict(text_detection_model_name="PP-OCRv6_medium_det",
                      text_recognition_model_name="PP-OCRv6_medium_rec"),
}


def make(config, threads, extra):
    from paddleocr import PaddleOCR
    kw = dict(CONFIGS[config], use_doc_orientation_classify=False, use_doc_unwarping=False,
              use_textline_orientation=False, cpu_threads=threads)
    kw.update(extra)
    return PaddleOCR(**kw)


def to_text(results):
    """Groups recognised boxes into lines (top to bottom, then left to right) and returns the page texts."""
    pages = []
    for res in results:
        r = res.json["res"] if "res" in res.json else res.json
        items = []
        for txt, box in zip(r["rec_texts"], r["rec_boxes"]):
            x0, y0, x1, y1 = box
            items.append((y0, y1, x0, txt))
        items.sort()
        lines, cur, cur_bottom = [], [], None
        for y0, y1, x0, txt in items:
            mid = (y0 + y1) / 2
            if cur and mid > cur_bottom:
                lines.append(cur)
                cur = []
            if not cur:
                cur_bottom = y1
            cur.append((x0, txt))
        if cur:
            lines.append(cur)
        pages.append("\n".join(" ".join(t for _, t in sorted(l)) for l in lines))
    return "\n\n".join(pages)


def run_once(ocr, name, seed):
    data, kind, truth, _ = payloads.build(name, seed)
    suffix = ".pdf" if kind == "pdf" else (".jpg" if name.startswith("phone") else ".png")
    with tempfile.NamedTemporaryFile(suffix=suffix) as f:
        f.write(data)
        f.flush()
        t0 = time.perf_counter()
        results = list(ocr.predict(f.name))
        dt = time.perf_counter() - t0
    text = to_text(results)
    return dt, text, truth, len(results)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--config", required=True)
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--payloads", default=",".join(payloads.PAYLOADS))
    ap.add_argument("--extra", default="{}", help="JSON kwargs for PaddleOCR()")
    ap.add_argument("--label", default="")
    ap.add_argument("--save-text", default="")
    a = ap.parse_args()

    t0 = time.perf_counter()
    ocr = make(a.config, a.threads, json.loads(a.extra))
    init_s = time.perf_counter() - t0
    rss_after_init = peak_rss_mib()

    out = {"config": a.config, "label": a.label, "threads": a.threads, "extra": json.loads(a.extra),
           "init_s": init_s, "rss_after_init_mib": rss_after_init, "payloads": {}}
    texts = {}
    first = True
    for name in a.payloads.split(","):
        times, scores, pages = [], [], 0
        cold = None
        for i in range(a.runs + (1 if first else 0)):
            dt, text, truth, pages = run_once(ocr, name, 1000 + i)
            if first:
                cold, first = dt, False
                continue
            times.append(dt)
            scores.append(score(truth, text))
            texts.setdefault(name, []).append({"seed": 1000 + i, "text": text})
        out["payloads"][name] = {"time": summarize(times), **mean_scores(scores), "pages": pages}
        if cold is not None:
            out["payloads"][name]["cold_s"] = cold
        print(name, json.dumps(out["payloads"][name]), flush=True)
    out["peak_rss_mib"] = peak_rss_mib()
    print("RESULT " + json.dumps(out), flush=True)
    if a.save_text:
        json.dump(texts, open(a.save_text, "w"), ensure_ascii=False)


if __name__ == "__main__":
    os.environ.setdefault("OMP_NUM_THREADS", "1")
    main()
