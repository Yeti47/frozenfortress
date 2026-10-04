//go:build !notesseract

package main

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
)

// createOCRService creates a TesseractOCRService when the notesseract build tag is not present
func createOCRService(config ccc.AppConfig, logger ccc.Logger) documents.OCRService {
	tesseract := documents.NewTesseractOCRService(config.OCR, logger)
	switch config.OCR.Provider {
	case "tesseract":
		return tesseract
	case "ollama":
		return documents.NewOllamaOCRService(config.OCR, logger)
	case "nop":
		return documents.NewNopOCRService(config.OCR, logger)
	default:
		return documents.NewFallbackOCRService(documents.NewOllamaOCRService(config.OCR, logger), tesseract, logger)
	}
}
