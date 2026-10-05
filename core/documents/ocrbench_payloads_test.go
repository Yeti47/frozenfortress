//go:build ocrbench

package documents

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math/rand"
	"strings"
	"unicode"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// benchPayload is a generated document image together with the words that are
// printed on it, so the benchmark can report how much of the text came back.
type benchPayload struct {
	Name   string
	Data   []byte // PNG or JPEG, as an upload would be
	Format string
	Width  int
	Height int
	Words  []string
}

var benchVocabulary = strings.Fields(`invoice payment account statement contract insurance policy
	tenant landlord electricity internet subscription reference number customer
	amount balance transfer delivery address warranty receipt service agreement
	monthly annual notice reminder deadline signature department insurance claim
	tax return income employer salary pension health doctor prescription appointment
	vehicle registration licence renewal municipality council application permit`)

// benchPayloads builds the set of payloads. The output is deterministic. Each
// variant has the same layout but different text, so the benchmark can send a
// new document on every run: Ollama caches the encoded image of a repeated
// request, which would make later runs unrealistically fast.
func benchPayloads(variant int) ([]benchPayload, error) {
	offset := int64(variant) * 100
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}

	dense150 := denseBenchPage(regular, 1240, 1754, 150, 1+offset)
	dense300 := denseBenchPage(regular, 2480, 3508, 300, 1+offset)
	sparse := sparseBenchPage(regular, bold, 1240, 1754, 150, 2+offset)
	invoice := invoiceBenchPage(regular, bold, 1654, 2339, 200, 3+offset)

	phone := phoneBenchPhoto(regular, 4+offset)

	specs := []struct {
		name   string
		page   benchPage
		format string
	}{
		{"dense-a4-150dpi", dense150, "png"},
		{"dense-a4-300dpi", dense300, "png"},
		{"sparse-a4-150dpi", sparse, "png"},
		{"invoice-a4-200dpi", invoice, "png"},
		{"phone-photo-12mp", phone, "jpeg"},
	}

	payloads := make([]benchPayload, 0, len(specs))
	for _, s := range specs {
		var buf bytes.Buffer
		switch s.format {
		case "jpeg":
			err = jpeg.Encode(&buf, s.page.img, &jpeg.Options{Quality: 85})
		default:
			err = png.Encode(&buf, s.page.img)
		}
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", s.name, err)
		}
		b := s.page.img.Bounds()
		payloads = append(payloads, benchPayload{
			Name:   s.name,
			Data:   buf.Bytes(),
			Format: s.format,
			Width:  b.Dx(),
			Height: b.Dy(),
			Words:  s.page.words,
		})
	}
	return payloads, nil
}

type benchPage struct {
	img   *image.RGBA
	words []string
}

func newBenchCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

func benchFace(f *opentype.Font, pt, dpi float64) font.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: pt, DPI: dpi, Hinting: font.HintingFull})
	if err != nil {
		panic(err) // fixed inputs, cannot fail at runtime
	}
	return face
}

func benchWords(rng *rand.Rand, n int) []string {
	words := make([]string, n)
	for i := range words {
		words[i] = benchVocabulary[rng.Intn(len(benchVocabulary))]
	}
	return words
}

// benchProse returns n words grouped into sentences of 6 to 14 words, each
// starting with a capital letter and ending with a period. Running text ends
// differently than a list of unrelated words, which matters because the model
// decides by itself when to stop generating.
func benchProse(rng *rand.Rand, n int) []string {
	words := benchWords(rng, n)
	for start := 0; start < len(words); {
		length := min(6+rng.Intn(9), len(words)-start)
		words[start] = strings.ToUpper(words[start][:1]) + words[start][1:]
		words[start+length-1] += "."
		start += length
	}
	return words
}

// drawBenchParagraph wraps words into lines and draws them until the bottom
// margin is reached. It returns the words that were actually drawn.
func drawBenchParagraph(img *image.RGBA, face font.Face, x, y, maxX, maxY int, words []string) ([]string, int) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: face}
	lineHeight := face.Metrics().Height.Ceil() * 5 / 4
	space := d.MeasureString(" ")

	var drawn []string
	cx := fixed.I(x)
	cy := y + face.Metrics().Ascent.Ceil()
	for _, w := range words {
		width := d.MeasureString(w)
		if cx+width > fixed.I(maxX) {
			cx = fixed.I(x)
			cy += lineHeight
		}
		if cy > maxY {
			break
		}
		d.Dot = fixed.Point26_6{X: cx, Y: fixed.I(cy)}
		d.DrawString(w)
		cx += width + space
		drawn = append(drawn, w)
	}
	return drawn, cy - face.Metrics().Ascent.Ceil() + lineHeight
}

func denseBenchPage(f *opentype.Font, w, h int, dpi float64, seed int64) benchPage {
	rng := rand.New(rand.NewSource(seed))
	img := newBenchCanvas(w, h)
	margin := int(dpi * 0.8)
	drawn, _ := drawBenchParagraph(img, benchFace(f, 10, dpi), margin, margin, w-margin, h-margin, benchProse(rng, 4000))
	return benchPage{img: img, words: drawn}
}

func sparseBenchPage(regular, bold *opentype.Font, w, h int, dpi float64, seed int64) benchPage {
	rng := rand.New(rand.NewSource(seed))
	img := newBenchCanvas(w, h)
	margin := int(dpi * 0.8)

	title, y := drawBenchParagraph(img, benchFace(bold, 22, dpi), margin, margin, w-margin, h-margin, benchWords(rng, 3))
	body, _ := drawBenchParagraph(img, benchFace(regular, 12, dpi), margin, y+int(dpi*0.3), w-margin, h-margin, benchProse(rng, 60))
	return benchPage{img: img, words: append(title, body...)}
}

func invoiceBenchPage(regular, bold *opentype.Font, w, h int, dpi float64, seed int64) benchPage {
	rng := rand.New(rand.NewSource(seed))
	img := newBenchCanvas(w, h)
	margin := int(dpi * 0.8)
	var words []string

	head, y := drawBenchParagraph(img, benchFace(bold, 20, dpi), margin, margin, w-margin, h-margin, []string{"Invoice", fmt.Sprintf("%d", 20000+rng.Intn(9999))})
	words = append(words, head...)

	face := benchFace(regular, 10, dpi)
	rowHeight := int(dpi * 0.4)
	colX := []int{margin, margin + (w-2*margin)*5/10, margin + (w-2*margin)*65/100, margin + (w-2*margin)*82/100}
	line := image.NewUniform(color.Gray{Y: 90})

	top := y + int(dpi*0.4)
	for row := 0; top+row*rowHeight+rowHeight < h-margin; row++ {
		ry := top + row*rowHeight
		draw.Draw(img, image.Rect(margin, ry, w-margin, ry+2), line, image.Point{}, draw.Src)

		var cells []string
		if row == 0 {
			cells = []string{"Description", "Quantity", "Unit price", "Total"}
		} else {
			cells = []string{
				strings.Join(benchWords(rng, 2+rng.Intn(3)), " "),
				fmt.Sprintf("%d", 1+rng.Intn(12)),
				fmt.Sprintf("%d.%02d", 5+rng.Intn(400), rng.Intn(100)),
				fmt.Sprintf("%d.%02d", 5+rng.Intn(4000), rng.Intn(100)),
			}
		}
		for i, cell := range cells {
			drawn, _ := drawBenchParagraph(img, face, colX[i]+8, ry+int(dpi*0.08), w-margin, ry+rowHeight, strings.Fields(cell))
			words = append(words, drawn...)
		}
	}
	return benchPage{img: img, words: words}
}

// phoneBenchPhoto imitates a phone photo of a page: a 12 MP JPEG with uneven
// lighting and sensor noise. The text is rendered at page resolution and
// scaled up, like a camera frame that is larger than the content needs.
func phoneBenchPhoto(f *opentype.Font, seed int64) benchPage {
	page := denseBenchPage(f, 1240, 1754, 150, seed)

	const w, h = 3024, 4032
	photo := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.BiLinear.Scale(photo, photo.Bounds(), page.img, page.img.Bounds(), xdraw.Src, nil)

	rng := rand.New(rand.NewSource(seed))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Vignette plus a diagonal lighting gradient, then noise.
			light := 1.0 - 0.25*float64(x+y)/float64(w+h)
			noise := float64(rng.Intn(17) - 8)
			i := photo.PixOffset(x, y)
			for c := 0; c < 3; c++ {
				v := float64(photo.Pix[i+c])*light*0.92 + 8 + noise
				photo.Pix[i+c] = uint8(max(0, min(255, v)))
			}
		}
	}
	return benchPage{img: photo, words: page.words}
}

// normalizeBenchWord reduces a token to lower-case letters and digits.
func normalizeBenchWord(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// benchWordAccuracy is the length of the longest common subsequence of the
// expected and the recognised words, divided by the number of expected words.
// It rewards words that are found in the right order, so a model that only
// guesses plausible words from the vocabulary scores low.
func benchWordAccuracy(expected []string, output string) float64 {
	var want, got []string
	for _, w := range expected {
		if n := normalizeBenchWord(w); n != "" {
			want = append(want, n)
		}
	}
	for _, tok := range strings.Fields(output) {
		if n := normalizeBenchWord(tok); n != "" {
			got = append(got, n)
		}
	}
	if len(want) == 0 {
		return 0
	}

	prev := make([]int, len(got)+1)
	cur := make([]int, len(got)+1)
	for _, w := range want {
		for j, g := range got {
			if w == g {
				cur[j+1] = prev[j] + 1
			} else {
				cur[j+1] = max(prev[j+1], cur[j])
			}
		}
		prev, cur = cur, prev
	}
	return float64(prev[len(got)]) / float64(len(want))
}
