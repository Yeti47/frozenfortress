package documents

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func TestPDFFileProcessorExtractTextFromSelectableTextPDF(t *testing.T) {
	processor := newTestPDFFileProcessor(&fakePDFOCRService{enabled: true})
	text, confidence, pageCount, err := processor.ExtractText(context.Background(), textOnlyPDF())
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if !strings.Contains(text, "Selectable text") {
		t.Fatalf("ExtractText() = %q, want selectable PDF text", text)
	}
	if strings.Contains(text, "[Text]") || strings.Contains(text, "[Images]") {
		t.Fatalf("text-only PDF should keep its original unlabelled output, got %q", text)
	}
	if confidence != 1.0 || pageCount != 1 {
		t.Fatalf("ExtractText() confidence/pageCount = %v/%d, want 1/1", confidence, pageCount)
	}
}

func TestPDFFileProcessorOCRsScannedPagesInPageOrder(t *testing.T) {
	ocr := &fakePDFOCRService{enabled: true}
	processor := newTestPDFFileProcessor(ocr)
	text, confidence, pageCount, err := processor.ExtractText(context.Background(), imagePDF(t, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}))
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	want := "[Images]\nRed page\n\nBlue page"
	if text != want {
		t.Fatalf("ExtractText() = %q, want %q", text, want)
	}
	if confidence < 0.69 || confidence > 0.71 || pageCount != 2 {
		t.Fatalf("ExtractText() confidence/pageCount = %v/%d, want about 0.7/2", confidence, pageCount)
	}
	if ocr.calls != 2 {
		t.Fatalf("OCR calls = %d, want 2", ocr.calls)
	}
}

func TestPDFFileProcessorSeparatesNativeTextAndOCRText(t *testing.T) {
	redImage := pngImage(t, color.RGBA{R: 255, A: 255})
	var hybridPDF bytes.Buffer
	if err := pdfcpuapi.ImportImages(bytes.NewReader(textOnlyPDF()), &hybridPDF, []io.Reader{bytes.NewReader(redImage)}, pdfcpu.DefaultImportConfig(), pdfcpumodel.NewDefaultConfiguration()); err != nil {
		t.Fatalf("ImportImages() error = %v", err)
	}

	processor := newTestPDFFileProcessor(&fakePDFOCRService{enabled: true})
	text, _, _, err := processor.ExtractText(context.Background(), hybridPDF.Bytes())
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	want := "[Text]\nSelectable text\n\n[Images]\nRed page"
	if text != want {
		t.Fatalf("ExtractText() = %q, want %q", text, want)
	}
}

func TestPDFFileProcessorGeneratePreviewUsesFirstImageInPageOrder(t *testing.T) {
	imageProcessor := NewImageFileProcessorWithOptions(nil, 4, 4, 85)
	processor := NewPDFFileProcessor(nil, imageProcessor)
	pdfData := imagePDF(t, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255})
	preview, err := processor.GeneratePreview(context.Background(), pdfData)
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if preview.PreviewType != "image/png" || preview.Width != 4 || preview.Height != 4 {
		t.Fatalf("GeneratePreview() metadata = %q %dx%d, want injected image preview settings 4x4", preview.PreviewType, preview.Width, preview.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(preview.PreviewData))
	if err != nil {
		t.Fatalf("image.Decode() error = %v", err)
	}
	r, _, b, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	if r <= b {
		t.Fatalf("preview first pixel is not from the first (red) PDF image")
	}
}

func TestPDFFileProcessorGeneratePreviewFallsBackWhenPDFHasNoImages(t *testing.T) {
	processor := newTestPDFFileProcessor(nil)
	preview, err := processor.GeneratePreview(context.Background(), textOnlyPDF())
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if preview.PreviewData != nil || preview.PreviewType != "application/pdf" || preview.Width != 0 || preview.Height != 0 {
		t.Fatalf("GeneratePreview() = %#v, want generic PDF preview fallback", preview)
	}
}

func TestPDFFileProcessorGeneratePreviewFallsBackWhenImageExtractionFails(t *testing.T) {
	processor := newTestPDFFileProcessor(nil)
	preview, err := processor.GeneratePreview(context.Background(), []byte("not a PDF"))
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v, want fallback without error", err)
	}
	if preview.PreviewData != nil || preview.PreviewType != "application/pdf" {
		t.Fatalf("GeneratePreview() = %#v, want generic PDF preview fallback", preview)
	}
}

func newTestPDFFileProcessor(ocrService OCRService) *PDFFileProcessor {
	return NewPDFFileProcessor(ocrService, NewImageFileProcessor(ocrService))
}

type fakePDFOCRService struct {
	enabled bool
	calls   int
}

func (s *fakePDFOCRService) IsOcrEnabled() bool {
	return s.enabled
}

func (s *fakePDFOCRService) ExtractText(_ context.Context, imageData []byte) (string, float32, error) {
	s.calls++
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", 0, fmt.Errorf("decode OCR image: %w", err)
	}
	r, _, b, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	if r > b {
		return "Red page", 0.8, nil
	}
	return "Blue page", 0.6, nil
}

func imagePDF(t *testing.T, colors ...color.RGBA) []byte {
	t.Helper()
	imageReaders := make([]io.Reader, 0, len(colors))
	for _, fill := range colors {
		imageReaders = append(imageReaders, bytes.NewReader(pngImage(t, fill)))
	}
	var pdfData bytes.Buffer
	if err := pdfcpuapi.ImportImages(nil, &pdfData, imageReaders, pdfcpu.DefaultImportConfig(), pdfcpumodel.NewDefaultConfiguration()); err != nil {
		t.Fatalf("ImportImages() error = %v", err)
	}
	return pdfData.Bytes()
}

func pngImage(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return imageData.Bytes()
}

func textOnlyPDF() []byte {
	var output bytes.Buffer
	output.WriteString("%PDF-1.4\n")
	offsets := make([]int64, 6)
	writeObject := func(number int, content string) {
		offsets[number] = int64(output.Len())
		fmt.Fprintf(&output, "%d 0 obj\n%s\nendobj\n", number, content)
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>")
	writeObject(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	stream := "BT /F1 12 Tf 50 700 Td (Selectable text) Tj ET"
	writeObject(5, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	xrefOffset := output.Len()
	output.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&output, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&output, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
	return output.Bytes()
}
