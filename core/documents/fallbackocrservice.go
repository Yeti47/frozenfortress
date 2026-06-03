package documents

import (
	"context"
	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// FallbackOCRService tries a primary OCR service and falls back to another service if the primary fails.
type FallbackOCRService struct {
	primary  OCRService
	fallback OCRService
	logger   ccc.Logger
}

func NewFallbackOCRService(primary OCRService, fallback OCRService, logger ccc.Logger) *FallbackOCRService {
	if logger == nil {
		logger = ccc.NopLogger
	}
	return &FallbackOCRService{primary: primary, fallback: fallback, logger: logger}
}

func (s *FallbackOCRService) IsOcrEnabled() bool {
	return (s.primary != nil && s.primary.IsOcrEnabled()) || (s.fallback != nil && s.fallback.IsOcrEnabled())
}

func (s *FallbackOCRService) ExtractText(ctx context.Context, imageData []byte) (string, float32, error) {
	if s.primary != nil && s.primary.IsOcrEnabled() {
		text, confidence, err := s.primary.ExtractText(ctx, imageData)
		if err == nil {
			return text, confidence, nil
		}
		s.logger.Warn("Primary OCR provider failed; trying fallback", "error", err)
	}

	if s.fallback != nil && s.fallback.IsOcrEnabled() {
		return s.fallback.ExtractText(ctx, imageData)
	}

	return "", 0, fmt.Errorf("no OCR provider available")
}
