"""Re-scores OCR texts saved with --save-text against the regenerated ground truth.

Usage: python rescore.py results/text-*.json
"""
import json
import sys

import payloads
from score import mean_scores, score


def fmt(v):
    return "  –  " if v is None else "%.3f" % v


for path in sys.argv[1:]:
    print(path)
    for name, entries in json.load(open(path)).items():
        m = mean_scores([score(payloads.build(name, e["seed"])[2], e["text"]) for e in entries])
        print("   %-24s in-order %s  recall %s  umlauts %s"
              % (name, fmt(m["word_acc"]), fmt(m["word_recall"]), fmt(m["umlaut_recall"])))
