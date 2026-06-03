package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// NopOCRService is a no-op implementation of OCRService for disabled OCR or fallback builds.
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

// ExtractText returns an empty OCR result.
func (s *NopOCRService) ExtractText(ctx context.Context, imageData []byte) (text string, confidence float32, err error) {

	s.logger.Warn("OCR service is disabled or unavailable. Text extraction will not run for this image.")
	return "", 0.0, nil
}

// IsOcrEnabled returns false — NopOCRService never performs OCR.
func (s *NopOCRService) IsOcrEnabled() bool {
	return false
}
