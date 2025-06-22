//go:build notesseract

package main

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
)

// createOCRService creates a NopOCRService when the notesseract build tag is present
func createOCRService(config ccc.AppConfig) documents.OCRService {
	return documents.NewNopOCRService()
}
