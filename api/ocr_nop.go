//go:build notesseract

package main

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
)

// createOCRService creates an OCRService when the notesseract build tag is present.
// Without Tesseract, Ollama is the only real provider; NOP is used when OCR is
// explicitly disabled (provider "nop" or unrecognised value).
func createOCRService(config ccc.AppConfig, logger ccc.Logger) documents.OCRService {
	switch config.OCR.Provider {
	case "ollama", "ollama-tesseract":
		return documents.NewOllamaOCRService(config.OCR, logger)
	default:
		return documents.NewNopOCRService(config.OCR, logger)
	}
}
