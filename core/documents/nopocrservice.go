//go:build notesseract

package documents

import (
	"context"
	"fmt"
)

// NopOCRService is a no-op implementation of OCRService for when Tesseract is not available
// This allows the application to compile and run without Tesseract installed
type NopOCRService struct{}

// NewNopOCRService creates a new NopOCRService
func NewNopOCRService() *NopOCRService {
	return &NopOCRService{}
}

// ExtractText returns a placeholder message indicating OCR is not available
func (s *NopOCRService) ExtractText(ctx context.Context, imageData []byte) (text string, confidence float32, err error) {
	return fmt.Sprintf("OCR not available - Tesseract not installed (image size: %d bytes)", len(imageData)), 0.0, nil
}
