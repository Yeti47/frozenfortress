package documents

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/ledongthuc/pdf"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var disablePDFCPUConfigDirOnce sync.Once

// PDFFileProcessor handles PDF file processing
type PDFFileProcessor struct {
	ocrService OCRService
}

// NewPDFFileProcessor creates a new PDFFileProcessor
func NewPDFFileProcessor(ocrService OCRService) *PDFFileProcessor {
	disablePDFCPUConfigDirOnce.Do(pdfcpuapi.DisableConfigDir)
	return &PDFFileProcessor{ocrService: ocrService}
}

// SupportsContentType checks if this processor can handle PDF content types
func (p *PDFFileProcessor) SupportsContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return contentType == "application/pdf"
}

// ExtractText extracts text from PDF files using the ledongthuc/pdf library
func (p *PDFFileProcessor) ExtractText(ctx context.Context, fileData []byte) (text string, confidence float32, pageCount int, err error) {
	if err := ctx.Err(); err != nil {
		return "", 0.0, 0, err
	}
	// Create a reader from the byte data
	reader := bytes.NewReader(fileData)

	// Create PDF reader
	pdfReader, err := pdf.NewReader(reader, int64(len(fileData)))
	if err != nil {
		return "", 0.0, 0, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	// Get the number of pages
	pageCount = pdfReader.NumPage()

	// Extract text from all pages
	textReader, err := pdfReader.GetPlainText()
	if err != nil {
		return "", 0.0, pageCount, fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	// Read all text into a buffer
	var buf bytes.Buffer
	_, err = buf.ReadFrom(textReader)
	if err != nil {
		return "", 0.0, pageCount, fmt.Errorf("failed to read extracted text: %w", err)
	}

	text = buf.String()

	if p.ocrService == nil || !p.ocrService.IsOcrEnabled() {
		if strings.TrimSpace(text) == "" {
			return "", 0.0, pageCount, ErrOCRSkipped
		}
		return text, 1.0, pageCount, nil
	}

	type imageOCRResult struct {
		pageNumber   int
		objectNumber int
		name         string
		text         string
		confidence   float32
	}
	var imageResults []imageOCRResult
	err = pdfcpuapi.ExtractImages(reader, nil, func(img pdfcpumodel.Image, _ bool, _ int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		imageData, err := io.ReadAll(img)
		if err != nil {
			return fmt.Errorf("failed to read image on PDF page %d: %w", img.PageNr, err)
		}
		imageText, imageConfidence, err := p.ocrService.ExtractText(ctx, imageData)
		if err != nil {
			return fmt.Errorf("failed to OCR image on PDF page %d: %w", img.PageNr, err)
		}
		imageResults = append(imageResults, imageOCRResult{
			pageNumber:   img.PageNr,
			objectNumber: img.ObjNr,
			name:         img.Name,
			text:         strings.TrimSpace(imageText),
			confidence:   imageConfidence,
		})
		return nil
	}, pdfcpumodel.NewDefaultConfiguration())
	if err != nil {
		return "", 0.0, pageCount, fmt.Errorf("failed to extract images from PDF: %w", err)
	}

	sort.SliceStable(imageResults, func(i, j int) bool {
		if imageResults[i].pageNumber != imageResults[j].pageNumber {
			return imageResults[i].pageNumber < imageResults[j].pageNumber
		}
		if imageResults[i].objectNumber != imageResults[j].objectNumber {
			return imageResults[i].objectNumber < imageResults[j].objectNumber
		}
		return imageResults[i].name < imageResults[j].name
	})

	imageTexts := make([]string, 0, len(imageResults))
	var confidenceTotal float64
	for _, imageResult := range imageResults {
		confidenceTotal += float64(imageResult.confidence)
		if imageResult.text != "" {
			imageTexts = append(imageTexts, imageResult.text)
		}
	}
	if len(imageResults) > 0 {
		confidence = float32(confidenceTotal / float64(len(imageResults)))
	} else if strings.TrimSpace(text) != "" {
		confidence = 1.0
	}
	text = combinePDFTextAndOCR(text, imageTexts)
	return text, confidence, pageCount, nil
}

func combinePDFTextAndOCR(text string, imageTexts []string) string {
	hasPDFText := strings.TrimSpace(text) != ""
	if !hasPDFText && len(imageTexts) == 0 {
		return text
	}
	if len(imageTexts) == 0 {
		return text
	}

	var sections []string
	if hasPDFText {
		sections = append(sections, "[Text]\n"+strings.TrimSpace(text))
	}
	sections = append(sections, "[Images]\n"+strings.Join(imageTexts, "\n\n"))
	return strings.Join(sections, "\n\n")
}

// GeneratePreview creates a preview for PDF files
// For PDFs, we don't generate actual preview images, just return the content type
// The frontend will show a generic PDF icon based on the PreviewType
func (p *PDFFileProcessor) GeneratePreview(ctx context.Context, fileData []byte) (*PreviewGenerationResult, error) {
	return &PreviewGenerationResult{
		PreviewData: nil, // No actual preview data
		PreviewType: "application/pdf",
		Width:       0, // No dimensions for PDF previews
		Height:      0,
	}, nil
}
