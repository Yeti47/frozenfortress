//go:build notesseract

package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// NopOCRService is a no-op implementation of OCRService for when Tesseract is not available
// This allows the application to compile and run without Tesseract installed
type NopOCRService struct {
	logger ccc.Logger
	config ccc.OCRConfig
}

// NewNopOCRService creates a new NopOCRService
func NewNopOCRService(config ccc.OCRConfig, logger ccc.Logger) *NopOCRService {

	if logger == nil {
		logger = ccc.NopLogger
	}

	return &NopOCRService{
		logger: logger,
		config: config,
	}
}

// ExtractText is a no-op implementation that returns an empty string and zero confidence
// It logs a warning that OCR functionality is not available
func (s *NopOCRService) ExtractText(ctx context.Context, imageData []byte) (text string, confidence float32, err error) {

	logger.Warn("OCR service is not available. Frozen Fortress was built without Tesseract. Text extraction will not work.")
	return "", 0.0, nil
}

// IsOcrEnabled checks if OCR is enabled in the configuration
func (s *NopOCRService) IsOcrEnabled() bool {
	return s.config.Enabled
}
