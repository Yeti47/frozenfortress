"""Prints Markdown tables for all result files in a directory (OCR runs and extraction runs).

Usage: python3 report.py <results-dir>
"""
import glob
import json
import os
import sys

PAYLOADS = ["invoice-a4-200dpi", "letter-a4-300dpi", "dense-a4-150dpi", "phone-photo-12mp", "scanned-pdf-3p-200dpi"]


def load(directory):
    ocr, extraction = [], []
    for path in sorted(glob.glob(os.path.join(directory, "*.json"))):
        if os.path.basename(path).startswith("text-"):
            continue
        try:
            d = json.load(open(path))
        except (json.JSONDecodeError, OSError):
            continue  # an empty file is left behind by a failed run
        if "payloads" in d:
            ocr.append(d)
        elif "docs" in d:
            extraction.append(d)
    return ocr, extraction


def ocr_cell(v):
    if not v or not v.get("time"):
        return "–"
    if v.get("word_recall") is not None:
        return "%.2fs / %.1f%%" % (v["time"]["median"], 100 * v["word_recall"])
    return "%.2fs / %.1f%%*" % (v["time"]["median"], 100 * v["word_acc"])


def main():
    ocr, extraction = load(sys.argv[1] if len(sys.argv) > 1 else "results")
    if ocr:
        print("| run | init | peak RSS | " + " | ".join(PAYLOADS) + " |")
        print("|" + "--|" * (3 + len(PAYLOADS)))
        for d in ocr:
            init = "%.1fs" % d["init_s"] if "init_s" in d else "–"
            rss = "%d MiB" % d["peak_rss_mib"] if d.get("peak_rss_mib") else "–"
            cells = [ocr_cell(d["payloads"].get(p)) for p in PAYLOADS]
            print(f"| {d['label']} | {init} | {rss} | " + " | ".join(cells) + " |")
        print("\ncells: median wall time / word recall (order-free); * = in-order accuracy\n")
    if extraction:
        print("| extraction run | doc | field acc | prompt tok | prompt s | gen s | wall med | wall p95 | bad json |")
        print("|--|--|--|--|--|--|--|--|--|")
        for d in extraction:
            for doc, v in d["docs"].items():
                print("| %s | %s | %.0f%% | %d | %.2f | %.2f | %.2fs | %.2fs | %d |" % (
                    d["label"], doc, 100 * (v["field_acc"] or 0), v["prompt_tokens"], v["prompt_s"]["median"],
                    v["gen_s"]["median"], v["wall"]["median"], v["wall"]["p95"], v["bad_json"]))


if __name__ == "__main__":
    main()
