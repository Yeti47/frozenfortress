"""PP-OCR via RapidOCR (ONNX Runtime / OpenVINO). PDFs are rendered with pypdfium2 at 144 dpi, like PaddleX."""
import argparse
import io
import json
import time

import numpy as np
import pypdfium2
from PIL import Image

import payloads
from score import mean_scores, peak_rss_mib, score, summarize


def make(engine, version, tier, rec_lang, threads, max_side):
    from rapidocr import EngineType, LangRec, ModelType, OCRVersion, RapidOCR
    eng = EngineType(engine)
    params = {
        "Global.use_cls": False, "Global.log_level": "warning", "Global.max_side_len": max_side,
        "Det.engine_type": eng, "Det.ocr_version": OCRVersion(version), "Det.model_type": ModelType(tier),
        "Rec.engine_type": eng, "Rec.ocr_version": OCRVersion(version), "Rec.model_type": ModelType(tier),
        "EngineConfig.onnxruntime.intra_op_num_threads": threads,
        "EngineConfig.onnxruntime.inter_op_num_threads": 1,
        "EngineConfig.openvino.inference_num_threads": threads,
    }
    if version == "PP-OCRv6":
        # One multilingual v6 model pair; RapidOCR selects it by language name, not by the LangDet/LangRec enums.
        params["Det.lang_type"] = rec_lang or "german"
        params["Rec.lang_type"] = rec_lang or "german"
    elif rec_lang:
        params["Rec.lang_type"] = LangRec(rec_lang)
    return RapidOCR(params=params)


def page_arrays(data, kind):
    if kind == "pdf":
        pdf = pypdfium2.PdfDocument(data)
        return [np.array(pdf[i].render(scale=2.0).to_pil().convert("RGB")) for i in range(len(pdf))]
    return [np.array(Image.open(io.BytesIO(data)).convert("RGB"))]


def to_text(res):
    if res.txts is None:
        return ""
    items = []
    for box, txt in zip(res.boxes, res.txts):
        ys, xs = [p[1] for p in box], [p[0] for p in box]
        items.append((min(ys), max(ys), min(xs), txt))
    items.sort()
    lines, cur, bottom = [], [], None
    for y0, y1, x0, txt in items:
        if cur and (y0 + y1) / 2 > bottom:
            lines.append(cur)
            cur = []
        if not cur:
            bottom = y1
        cur.append((x0, txt))
    if cur:
        lines.append(cur)
    return "\n".join(" ".join(t for _, t in sorted(l)) for l in lines)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--engine", default="onnxruntime")
    ap.add_argument("--version", default="PP-OCRv6")
    ap.add_argument("--tier", default="tiny")
    ap.add_argument("--rec-lang", default="")
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--max-side", type=int, default=2000)
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--payloads", default=",".join(payloads.PAYLOADS))
    ap.add_argument("--label", default="")
    ap.add_argument("--save-text", default="")
    a = ap.parse_args()

    t0 = time.perf_counter()
    ocr = make(a.engine, a.version, a.tier, a.rec_lang, a.threads, a.max_side)
    out = {"config": f"rapid-{a.engine}-{a.version}-{a.tier}{'-' + a.rec_lang if a.rec_lang else ''}",
           "label": a.label, "threads": a.threads, "max_side": a.max_side,
           "init_s": time.perf_counter() - t0, "rss_after_init_mib": peak_rss_mib(), "payloads": {}}
    texts, first = {}, True
    for name in a.payloads.split(","):
        times, scores, cold, pages = [], [], None, 0
        for i in range(a.runs + (1 if first else 0)):
            data, kind, truth, _ = payloads.build(name, 1000 + i)
            t1 = time.perf_counter()
            arrays = page_arrays(data, kind)
            text = "\n\n".join(to_text(ocr(p)) for p in arrays)
            dt = time.perf_counter() - t1
            pages = len(arrays)
            if first:
                cold, first = dt, False
                continue
            texts.setdefault(name, []).append({"seed": 1000 + i, "text": text})
            times.append(dt)
            scores.append(score(truth, text))
        out["payloads"][name] = {"time": summarize(times), **mean_scores(scores), "pages": pages}
        if cold is not None:
            out["payloads"][name]["cold_s"] = cold
        print(name, json.dumps(out["payloads"][name]), flush=True)
    out["peak_rss_mib"] = peak_rss_mib()
    print("RESULT " + json.dumps(out), flush=True)
    if a.save_text:
        json.dump(texts, open(a.save_text, "w"), ensure_ascii=False)


if __name__ == "__main__":
    main()
