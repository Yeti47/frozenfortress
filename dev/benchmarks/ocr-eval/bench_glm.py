"""GLM-OCR via Ollama, with the same preprocessing as OllamaOCRService (Lanczos resize, white JPEG q90)."""
import argparse
import base64
import io
import json
import time

import pypdfium2
import requests
from PIL import Image

import payloads
from score import mean_scores, score, summarize

PROMPT = ("Text Recognition: extract all visible text from this document image. Preserve structure, tables, "
          "lists, and line breaks where possible. Return only the extracted text.")


def prep(img, max_dim):
    img = img.convert("RGB")
    if max(img.size) > max_dim:
        img.thumbnail((max_dim, max_dim), Image.LANCZOS)
    buf = io.BytesIO()
    img.save(buf, "JPEG", quality=90)
    return base64.b64encode(buf.getvalue()).decode()


def page_images(data, kind):
    if kind != "pdf":
        return [Image.open(io.BytesIO(data))]
    pdf = pypdfium2.PdfDocument(data)
    return [pdf[i].render(scale=200 / 72).to_pil() for i in range(len(pdf))]


def ocr(url, model, img, max_dim):
    r = requests.post(url + "/api/generate", json={
        "model": model, "prompt": PROMPT, "images": [prep(img, max_dim)], "stream": False,
        "keep_alive": "30m", "options": {"temperature": 0}}, timeout=1800)
    r.raise_for_status()
    return r.json()["response"].strip()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", default="http://ff-eval-glm:11434")
    ap.add_argument("--model", default="glm-ocr:q8_0")
    ap.add_argument("--max-dim", type=int, default=640)
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--payloads", default=",".join(payloads.PAYLOADS))
    ap.add_argument("--label", default="")
    ap.add_argument("--save-text", default="")
    a = ap.parse_args()

    out = {"config": f"glm-ocr@{a.max_dim}", "label": a.label, "payloads": {}}
    first = True
    texts = {}
    for name in a.payloads.split(","):
        times, scores, cold, fails = [], [], None, 0
        for i in range(a.runs + (1 if first else 0)):
            data, kind, truth, _ = payloads.build(name, 1000 + i)
            pages = page_images(data, kind)
            t0 = time.perf_counter()
            try:
                text = "\n\n".join(ocr(a.url, a.model, p, a.max_dim) for p in pages)
            except Exception as e:
                print("fail", name, repr(e), flush=True)
                fails += 1
                continue
            dt = time.perf_counter() - t0
            if first:
                cold, first = dt, False
                continue
            texts.setdefault(name, []).append({"seed": 1000 + i, "text": text})
            times.append(dt)
            scores.append(score(truth, text))
        out["payloads"][name] = {"time": summarize(times) if times else None, **mean_scores(scores),
                                 "pages": len(pages), "fails": fails}
        if cold is not None:
            out["payloads"][name]["cold_s"] = cold
        print(name, json.dumps(out["payloads"][name]), flush=True)
    print("RESULT " + json.dumps(out), flush=True)
    if a.save_text:
        json.dump(texts, open(a.save_text, "w"), ensure_ascii=False)


if __name__ == "__main__":
    main()
