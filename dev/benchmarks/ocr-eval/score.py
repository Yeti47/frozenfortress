import re
import statistics
from collections import Counter

UMLAUT = re.compile(r"[äöüÄÖÜß]")
EDGE = re.compile(r"^[\W_]+|[\W_]+$")


def words(text):
    out = []
    for w in text.replace("·", " ").split():
        w = EDGE.sub("", w)
        if w:
            out.append(w)
    return out


def lcs(a, b):
    if not a or not b:
        return 0
    prev = [0] * (len(b) + 1)
    for x in a:
        cur = [0]
        for j, y in enumerate(b, 1):
            cur.append(prev[j - 1] + 1 if x == y else max(prev[j], cur[j - 1]))
        prev = cur
    return prev[-1]


def recall(wanted, got):
    """Share of the wanted words found in got, regardless of order (each found word is used once)."""
    if not wanted:
        return None
    left = Counter(got)
    found = 0
    for w in wanted:
        if left[w] > 0:
            left[w] -= 1
            found += 1
    return found / len(wanted)


def score(truth_lines, ocr_text):
    """word_acc: in-order accuracy (LCS); word_recall: order-free; umlaut_recall: words with äöüß only."""
    gt = words(" ".join(truth_lines))
    got = words(ocr_text)
    return {"word_acc": lcs(gt, got) / len(gt) if gt else 0.0,
            "word_recall": recall(gt, got) or 0.0,
            "umlaut_recall": recall([w for w in gt if UMLAUT.search(w)], got),
            "gt_words": len(gt), "ocr_words": len(got)}


def mean_scores(scores):
    """Averages the metrics of several score() results; a metric that is None everywhere stays None."""
    out = {}
    for key in ("word_acc", "word_recall", "umlaut_recall"):
        values = [s[key] for s in scores if s[key] is not None]
        out[key] = sum(values) / len(values) if values else None
    return out


def summarize(times):
    s = sorted(times)
    p95 = s[min(len(s) - 1, int(round(0.95 * (len(s) - 1))))]
    return {"median": statistics.median(s), "min": s[0], "p95": p95, "n": len(s)}


def peak_rss_mib():
    with open("/proc/self/status") as f:
        for line in f:
            if line.startswith("VmHWM:"):
                return int(line.split()[1]) / 1024
    return None
