package documents

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFFileProcessor handles PDF file processing
type PDFFileProcessor struct {
	// No dependencies needed for this simple implementation
}

// NewPDFFileProcessor creates a new PDFFileProcessor
func NewPDFFileProcessor() *PDFFileProcessor {
	return &PDFFileProcessor{}
}

// SupportsContentType checks if this processor can handle PDF content types
func (p *PDFFileProcessor) SupportsContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return contentType == "application/pdf"
}

// ExtractText extracts text from PDF files using the ledongthuc/pdf library
func (p *PDFFileProcessor) ExtractText(ctx context.Context, fileData []byte) (text string, confidence float32, pageCount int, err error) {
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

	// For PDF text extraction, we don't have a confidence score like OCR
	// We'll return 1.0 (100%) if we successfully extracted text, 0.0 if no text was found
	confidence = 1.0
	if strings.TrimSpace(text) == "" {
		confidence = 0.0
	}

	return text, confidence, pageCount, nil
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
