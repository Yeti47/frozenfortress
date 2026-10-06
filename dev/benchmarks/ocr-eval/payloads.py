"""Synthetic German document payloads with ground truth text and fields.

Every call with a different seed yields a different document with the same
layout, so no OCR backend can profit from a cached encoding.
"""
import io
import json
import math
import random

from PIL import Image, ImageDraw, ImageFilter, ImageFont

FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
FONT_BOLD = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"
FONT_SERIF = "/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf"

A4_MM = (210, 297)

COMPANIES = ["Stadtwerke Kühlungsborn GmbH", "Müller & Söhne Heizungsbau KG", "Bäckerei Großmann e.K.",
             "Allgemeine Ortskrankenkasse Nordost", "Zahnarztpraxis Dr. Weißgerber", "Gärtnerei Fröhlich OHG",
             "Hausverwaltung Schäfer GmbH", "Versicherungsbüro Köhler & Partner"]
STREETS = ["Hauptstraße", "Lindenallee", "Am Mühlenteich", "Goethestraße", "Schloßplatz", "Kirchgasse"]
CITIES = [("18225", "Kühlungsborn"), ("10115", "Berlin"), ("80331", "München"), ("50667", "Köln"),
          ("01067", "Dresden"), ("20095", "Hamburg")]
PERSONS = ["Jürgen Weiß", "Anna Bäumer", "Sören Schröder", "Grete Hößl", "Lukas Fuß", "Maria Özdemir"]
ITEMS = ["Wartung Gasbrennwerttherme", "Austausch Thermostatventil", "Anfahrtspauschale", "Kleinmaterial",
         "Arbeitszeit Monteur (Std.)", "Druckprüfung Heizkreis", "Entlüftung Heizkörper", "Abgasmessung"]
WORDS = ("der die das und nicht mit für über unter Bescheid Antrag Zahlung Frist Monat Jahr Vertrag "
         "Kündigung Versicherung Beitrag Erstattung Rechnung Mahnung Gebühr Kosten Leistung Nachweis "
         "Unterlagen Änderung Prüfung Grundstück Nebenkosten Abrechnung Heizöl Wärmepumpe Größe Maßnahme "
         "zuständig gemäß außerdem rückwirkend fällig übermitteln berücksichtigen Bundesländer "
         "Straßenreinigung Müllabfuhr Gebäude Wohnfläche Schlüssel Rückfragen Ansprechpartnerin "
         "Bankverbindung Überweisung Verwendungszweck Kontoauszug Steuerbescheid Lohnsteuer Einkünfte "
         "Krankenkasse Zuzahlung Heilmittel Verordnung Bestätigung Anlage beigefügt erhalten bitten "
         "innerhalb Wochen Tagen spätestens jeweils insgesamt bereits weiterhin ausschließlich").split()


def _font(path, pt, dpi):
    return ImageFont.truetype(path, int(round(pt * dpi / 72)))


def _px(mm, dpi):
    return int(round(mm / 25.4 * dpi))


def _sentence(rng, n):
    words = [rng.choice(WORDS) for _ in range(n)]
    words[0] = words[0][0].upper() + words[0][1:]
    return " ".join(words) + "."


def _iban(rng):
    return "DE" + "".join(str(rng.randint(0, 9)) for _ in range(20))


def _fmt_iban(iban):
    return " ".join(iban[i:i + 4] for i in range(0, len(iban), 4))


def _eur(v):
    s = f"{v:,.2f}"
    return s.replace(",", "X").replace(".", ",").replace("X", ".") + " €"


class Page:
    def __init__(self, dpi):
        self.dpi = dpi
        self.img = Image.new("RGB", (_px(A4_MM[0], dpi), _px(A4_MM[1], dpi)), "white")
        self.draw = ImageDraw.Draw(self.img)
        self.lines = []

    def text(self, x_mm, y_mm, s, pt=10, bold=False, serif=False):
        path = FONT_BOLD if bold else (FONT_SERIF if serif else FONT)
        self.draw.text((_px(x_mm, self.dpi), _px(y_mm, self.dpi)), s, fill="black", font=_font(path, pt, self.dpi))
        self.lines.append(s)

    def hline(self, y_mm, x0=20, x1=190):
        y = _px(y_mm, self.dpi)
        self.draw.line([(_px(x0, self.dpi), y), (_px(x1, self.dpi), y)], fill="black", width=max(1, self.dpi // 100))

    def wrap(self, x_mm, y_mm, para, pt, width_mm, serif=False, leading=1.35):
        font = _font(FONT_SERIF if serif else FONT, pt, self.dpi)
        max_w = _px(width_mm, self.dpi)
        line = ""
        for w in para.split():
            cand = (line + " " + w).strip()
            if self.draw.textlength(cand, font=font) > max_w and line:
                self.text(x_mm, y_mm, line, pt, serif=serif)
                y_mm += pt * 0.3528 * leading
                line = w
            else:
                line = cand
        if line:
            self.text(x_mm, y_mm, line, pt, serif=serif)
            y_mm += pt * 0.3528 * leading
        return y_mm


def invoice(seed, dpi=200):
    rng = random.Random(seed)
    p = Page(dpi)
    company = rng.choice(COMPANIES)
    street = f"{rng.choice(STREETS)} {rng.randint(1, 120)}"
    plz, city = rng.choice(CITIES)
    person = rng.choice(PERSONS)
    inv_no = f"RE-{rng.randint(2024, 2026)}-{rng.randint(10000, 99999)}"
    day, month, year = rng.randint(1, 28), rng.randint(1, 12), rng.randint(2024, 2026)
    date = f"{day:02d}.{month:02d}.{year}"
    due = f"{min(day + 14, 28):02d}.{month:02d}.{year}"
    iban = _iban(rng)
    cust = f"KD-{rng.randint(100000, 999999)}"

    p.text(20, 15, company, 14, bold=True)
    p.text(20, 22, f"{street} · {plz} {city}", 8)
    p.text(20, 45, person, 10)
    p.text(20, 50, f"{rng.choice(STREETS)} {rng.randint(1, 80)}", 10)
    p.text(20, 55, " ".join(rng.choice(CITIES)), 10)
    p.text(130, 45, f"Rechnungsnummer: {inv_no}", 9)
    p.text(130, 50, f"Kundennummer: {cust}", 9)
    p.text(130, 55, f"Rechnungsdatum: {date}", 9)
    p.text(20, 75, f"Rechnung {inv_no}", 13, bold=True)
    p.text(20, 85, f"Sehr geehrte/r {person},", 10)
    p.text(20, 91, "für die erbrachten Leistungen erlauben wir uns, wie folgt zu berechnen:", 10)
    y = 102
    p.text(20, y, "Pos.", 9, bold=True); p.text(32, y, "Bezeichnung", 9, bold=True)
    p.text(120, y, "Menge", 9, bold=True); p.text(140, y, "Einzelpreis", 9, bold=True); p.text(170, y, "Gesamt", 9, bold=True)
    p.hline(y + 5.5)
    y += 8
    net = 0.0
    for i, item in enumerate(rng.sample(ITEMS, 5), 1):
        qty = rng.randint(1, 4)
        price = round(rng.uniform(8, 240), 2)
        total = qty * price
        net += total
        p.text(20, y, str(i), 9); p.text(32, y, item, 9); p.text(122, y, str(qty), 9)
        p.text(140, y, _eur(price), 9); p.text(170, y, _eur(total), 9)
        p.hline(y + 5.5)
        y += 8
    vat = round(net * 0.19, 2)
    gross = round(net + vat, 2)
    y += 4
    p.text(130, y, "Nettobetrag:", 9); p.text(170, y, _eur(net), 9); y += 6
    p.text(130, y, "zzgl. 19 % MwSt.:", 9); p.text(170, y, _eur(vat), 9); y += 6
    p.text(130, y, "Rechnungsbetrag:", 10, bold=True); p.text(170, y, _eur(gross), 10, bold=True); y += 14
    y = p.wrap(20, y, f"Bitte überweisen Sie den Rechnungsbetrag bis spätestens {due} unter Angabe der "
                      f"Rechnungsnummer auf das unten genannte Konto. Vielen Dank für Ihren Auftrag.", 10, 170)
    p.text(20, 270, f"Bankverbindung: Sparkasse {city} · IBAN {_fmt_iban(iban)} · BIC NOLADE21XXX", 7)
    p.text(20, 274, f"Geschäftsführer: {rng.choice(PERSONS)} · USt-IdNr. DE{rng.randint(100000000, 999999999)}", 7)
    fields = {"absender": company, "rechnungsnummer": inv_no, "rechnungsdatum": date, "faellig_bis": due,
              "kundennummer": cust, "iban": iban, "nettobetrag": round(net, 2), "gesamtbetrag": gross}
    return p, fields


def letter(seed, dpi=300, paragraphs=6):
    rng = random.Random(seed)
    p = Page(dpi)
    sender = rng.choice(COMPANIES)
    person = rng.choice(PERSONS)
    day, month, year = rng.randint(1, 28), rng.randint(1, 12), rng.randint(2024, 2026)
    date = f"{day:02d}.{month:02d}.{year}"
    ref = f"Az. {rng.randint(100, 999)}/{rng.randint(10000, 99999)}-{rng.choice('ABCDEFGH')}"
    subject = rng.choice(["Änderung Ihres Beitragssatzes", "Bescheid über die Nebenkostenabrechnung",
                          "Erstattung von Zuzahlungen", "Kündigung Ihres Vertrags", "Mahnung – Zahlungserinnerung"])
    p.text(20, 15, sender, 13, bold=True)
    p.text(20, 45, person, 10, serif=True)
    p.text(20, 50, f"{rng.choice(STREETS)} {rng.randint(1, 80)}", 10, serif=True)
    p.text(20, 55, " ".join(rng.choice(CITIES)), 10, serif=True)
    p.text(140, 45, f"Datum: {date}", 9)
    p.text(140, 50, f"Unser Zeichen: {ref}", 9)
    p.text(20, 75, subject, 11, bold=True)
    y = p.wrap(20, 85, f"Sehr geehrte/r {person},", 10, 170, serif=True) + 2
    for _ in range(paragraphs):
        para = " ".join(_sentence(rng, rng.randint(8, 16)) for _ in range(rng.randint(3, 5)))
        y = p.wrap(20, y, para, 10, 170, serif=True) + 3
        if y > 260:
            break
    p.wrap(20, min(y, 268), "Mit freundlichen Grüßen", 10, 170, serif=True)
    fields = {"absender": sender, "datum": date, "aktenzeichen": ref, "betreff": subject, "empfaenger": person}
    return p, fields


def dense(seed, dpi=150):
    rng = random.Random(seed)
    p = Page(dpi)
    y = 15.0
    while y < 280:
        para = " ".join(_sentence(rng, rng.randint(8, 16)) for _ in range(rng.randint(3, 6)))
        y = p.wrap(15, y, para, 10, 180, leading=1.25) + 2
    return p, {}


def phone_photo(seed):
    """The invoice photographed with a 12 MP phone: tilt, perspective, uneven light, noise."""
    rng = random.Random(seed)
    page, fields = invoice(seed, dpi=300)
    w, h = 4000, 3000
    doc = page.img.resize((int(page.img.width * 0.95), int(page.img.height * 0.95)))
    doc = doc.rotate(rng.uniform(-4, 4), resample=Image.BICUBIC, expand=True, fillcolor=(90, 70, 50))
    # portrait page on a portrait photo: 3000x4000
    canvas = Image.new("RGB", (h, w), (95, 75, 55))
    doc.thumbnail((int(h * 0.92), int(w * 0.92)))
    canvas.paste(doc, ((h - doc.width) // 2, (w - doc.height) // 2))
    # keystone
    dx = int(h * rng.uniform(0.02, 0.05))
    coeffs = _perspective_coeffs([(0, 0), (h, 0), (h, w), (0, w)], [(dx, 0), (h - dx, 0), (h, w), (0, w)])
    canvas = canvas.transform(canvas.size, Image.PERSPECTIVE, coeffs, Image.BICUBIC, fillcolor=(95, 75, 55))
    # light gradient
    grad = Image.linear_gradient("L").resize(canvas.size).rotate(rng.uniform(0, 360))
    shade = Image.merge("RGB", [grad.point(lambda v: 150 + v * 105 // 255)] * 3)
    canvas = Image.composite(canvas, Image.blend(canvas, Image.new("RGB", canvas.size, "black"), 0.35),
                             grad.point(lambda v: 255 if v > 90 else int(v * 2.8)))
    canvas = Image.blend(canvas, shade, 0.12).filter(ImageFilter.GaussianBlur(1.1))
    noise = Image.effect_noise(canvas.size, 18).convert("RGB")
    canvas = Image.blend(canvas, noise, 0.06)
    buf = io.BytesIO()
    canvas.save(buf, "JPEG", quality=88)
    return buf.getvalue(), page.lines, fields


def _perspective_coeffs(dst, src):
    import numpy as np
    m = []
    for (x, y), (u, v) in zip(dst, src):
        m.append([x, y, 1, 0, 0, 0, -u * x, -u * y])
        m.append([0, 0, 0, x, y, 1, -v * x, -v * y])
    a = np.array(m, dtype=float)
    b = np.array([c for pt in src for c in pt], dtype=float)
    return np.linalg.solve(a, b).tolist()


def png(page):
    buf = io.BytesIO()
    page.img.save(buf, "PNG", optimize=False)
    return buf.getvalue()


def scanned_pdf(seed, pages=3, dpi=200):
    """Image-only PDF, as produced by a scanner without OCR."""
    built = [invoice(seed, dpi)[0], letter(seed + 1, dpi)[0], dense(seed + 2, dpi)[0]][:pages]
    buf = io.BytesIO()
    imgs = [b.img.convert("L").convert("RGB") for b in built]
    imgs[0].save(buf, "PDF", resolution=dpi, save_all=True, append_images=imgs[1:])
    lines = [l for b in built for l in b.lines]
    return buf.getvalue(), lines


PAYLOADS = ["invoice-a4-200dpi", "letter-a4-300dpi", "dense-a4-150dpi", "phone-photo-12mp", "scanned-pdf-3p-200dpi"]


def build(name, seed):
    """Returns (bytes, kind, ground_truth_lines, fields, page_images) for a payload name."""
    if name == "invoice-a4-200dpi":
        p, f = invoice(seed, 200); return png(p), "image", p.lines, f
    if name == "letter-a4-300dpi":
        p, f = letter(seed, 300); return png(p), "image", p.lines, f
    if name == "dense-a4-150dpi":
        p, f = dense(seed, 150); return png(p), "image", p.lines, f
    if name == "phone-photo-12mp":
        b, lines, f = phone_photo(seed); return b, "image", lines, f
    if name == "scanned-pdf-3p-200dpi":
        b, lines = scanned_pdf(seed); return b, "pdf", lines, {}
    raise KeyError(name)


if __name__ == "__main__":
    import os, sys
    out = sys.argv[1] if len(sys.argv) > 1 else "samples"
    os.makedirs(out, exist_ok=True)
    for n in PAYLOADS:
        data, kind, lines, fields = build(n, 1)
        ext = {"pdf": "pdf"}.get(kind, "jpg" if n.startswith("phone") else "png")
        open(f"{out}/{n}.{ext}", "wb").write(data)
        json.dump({"lines": lines, "fields": fields}, open(f"{out}/{n}.json", "w"), ensure_ascii=False, indent=1)
        print(n, len(data) // 1024, "KiB", len(" ".join(lines).split()), "words")
