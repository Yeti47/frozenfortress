//go:build !notesseract

package main

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
)

// createOCRService creates a TesseractOCRService when the notesseract build tag is not present
func createOCRService(config ccc.AppConfig, logger ccc.Logger) documents.OCRService {
	return documents.NewTesseractOCRService(
		config.OCR,
		logger,
	)
}
