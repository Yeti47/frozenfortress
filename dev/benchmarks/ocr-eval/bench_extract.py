"""Field extraction with NuExtract served by llama-server, on ground-truth text or saved OCR output."""
import argparse
import json
import re
import time

import requests

import payloads
from score import summarize

TEMPLATES = {
    "invoice": {
        "v1": {"absender": "", "rechnungsnummer": "", "rechnungsdatum": "", "faellig_bis": "", "kundennummer": "",
               "iban": "", "nettobetrag": "", "gesamtbetrag": ""},
        "v2": {"absender": "verbatim-string", "rechnungsnummer": "verbatim-string", "rechnungsdatum": "date-time",
               "faellig_bis": "date-time", "kundennummer": "verbatim-string", "iban": "verbatim-string",
               "nettobetrag": "number", "gesamtbetrag": "number"},
    },
    "letter": {
        "v1": {"absender": "", "empfaenger": "", "datum": "", "aktenzeichen": "", "betreff": ""},
        "v2": {"absender": "verbatim-string", "empfaenger": "verbatim-string", "datum": "date-time",
               "aktenzeichen": "verbatim-string", "betreff": "verbatim-string"},
    },
}
EN_KEYS = {"absender": "sender", "empfaenger": "recipient", "rechnungsnummer": "invoice_number",
           "rechnungsdatum": "invoice_date", "faellig_bis": "due_date", "kundennummer": "customer_number",
           "iban": "iban", "nettobetrag": "net_amount", "gesamtbetrag": "total_amount", "datum": "date",
           "aktenzeichen": "reference_number", "betreff": "subject"}
DOCS = {"invoice-a4-200dpi": "invoice", "letter-a4-300dpi": "letter", "phone-photo-12mp": "invoice"}


def prompt(fmt, template, text):
    t = json.dumps(template, ensure_ascii=False, indent=4)
    if fmt == "v1":
        return f"<|input|>\n### Template:\n{t}\n### Text:\n{text}\n\n<|output|>"
    return ("<|im_start|>system\nYou are NuExtract, an information extraction tool created by NuMind.<|im_end|>\n"
            f"<|im_start|>user\n# Template:\n{t}\n# Context:\n{text}<|im_end|>\n<|im_start|>assistant\n")


def norm_str(v):
    return re.sub(r"\s+", " ", str(v or "")).strip(" .,:;").casefold()


def norm_date(v):
    s = str(v or "")
    m = re.search(r"(\d{1,2})\.(\d{1,2})\.(\d{4})", s)
    if m:
        return (int(m[3]), int(m[2]), int(m[1]))
    m = re.search(r"(\d{4})-(\d{2})-(\d{2})", s)
    return (int(m[1]), int(m[2]), int(m[3])) if m else None


def norm_amount(v):
    if isinstance(v, (int, float)):
        return round(float(v), 2)
    s = re.sub(r"[^\d,.\-]", "", str(v or ""))
    if "," in s:
        s = s.replace(".", "").replace(",", ".")
    try:
        return round(float(s), 2)
    except ValueError:
        return None


def field_ok(key, want, got):
    if key in ("rechnungsdatum", "faellig_bis", "datum"):
        return norm_date(got) is not None and norm_date(got) == norm_date(want)
    if key in ("nettobetrag", "gesamtbetrag"):
        return norm_amount(got) == norm_amount(want)
    if key == "iban":
        return re.sub(r"\s", "", str(got or "")).upper() == want
    return norm_str(got) == norm_str(want)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", default="http://ff-eval-llm:8080")
    ap.add_argument("--fmt", choices=["v1", "v2"], required=True)
    ap.add_argument("--source", default="gt", help="gt or a JSON file written by bench_paddle --save-text")
    ap.add_argument("--label", default="")
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--en-keys", action="store_true", help="English template keys")
    a = ap.parse_args()

    saved = json.load(open(a.source)) if a.source != "gt" else None
    out = {"label": a.label, "fmt": a.fmt, "source": a.source, "docs": {}}
    for doc, kind in DOCS.items():
        walls, pms, gms, ptoks, ok, total, bad_json, misses = [], [], [], [], 0, 0, 0, {}
        # OCR runs skip the cold-start document, so their saved seeds don't always start at 1000.
        if saved is not None:
            inputs = [(e["seed"], e["text"]) for e in saved.get(doc, [])][:a.runs]
        else:
            inputs = [(1000 + i, None) for i in range(a.runs)]
        if not inputs:
            print(doc, "skipped: no saved OCR text", flush=True)
            continue
        for seed, text in inputs:
            _, _, truth, fields = payloads.build(doc, seed)
            if text is None:
                text = "\n".join(truth)
            tmpl = TEMPLATES[kind][a.fmt]
            if a.en_keys:
                tmpl = {EN_KEYS[k]: v for k, v in tmpl.items()}
            t0 = time.perf_counter()
            r = requests.post(a.url + "/completion", json={
                "prompt": prompt(a.fmt, tmpl, text), "temperature": 0, "n_predict": 400,
                "cache_prompt": False, "stop": ["<|end-output|>", "<|im_end|>", "<|endoftext|>"]}, timeout=900)
            wall = time.perf_counter() - t0
            r.raise_for_status()
            body = r.json()
            tm = body.get("timings", {})
            walls.append(wall); pms.append(tm.get("prompt_ms", 0) / 1000); gms.append(tm.get("predicted_ms", 0) / 1000)
            ptoks.append(tm.get("prompt_n", 0))
            raw = body["content"].strip()
            try:
                got = json.loads(raw[raw.index("{"): raw.rindex("}") + 1])
            except Exception:
                bad_json += 1
                got = {}
            for k, want in fields.items():
                total += 1
                if a.en_keys:
                    got[k] = got.get(EN_KEYS[k])
                if field_ok(k, want, got.get(k)):
                    ok += 1
                else:
                    misses.setdefault(k, []).append({"want": want, "got": got.get(k)})
        out["docs"][doc] = {"wall": summarize(walls), "prompt_s": summarize(pms), "gen_s": summarize(gms),
                            "prompt_tokens": sum(ptoks) / len(ptoks), "field_acc": ok / total if total else None,
                            "bad_json": bad_json, "misses": misses}
        print(doc, json.dumps(out["docs"][doc], ensure_ascii=False), flush=True)
    print("RESULT " + json.dumps(out, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    main()
