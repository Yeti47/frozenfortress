//go:build notesseract

package main

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
)

// createOCRService creates a NopOCRService when the notesseract build tag is present
func createOCRService(config ccc.AppConfig, logger ccc.Logger) documents.OCRService {
	nop := documents.NewNopOCRService(config.OCR, logger)
	switch config.OCR.Provider {
	case "ollama", "ollama-tesseract":
		return documents.NewFallbackOCRService(documents.NewOllamaOCRService(config.OCR, logger), nop, logger)
	default:
		return nop
	}
}
