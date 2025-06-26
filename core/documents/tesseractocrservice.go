//go:build !notesseract

package documents

import (
	"context"
	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/otiai10/gosseract/v2"
)

// TesseractOCRService implements OCRService using Tesseract OCR engine
// This service is stateless but uses configurable languages for OCR processing
type TesseractOCRService struct {
	config ccc.OCRConfig // OCR configuration including languages
	logger ccc.Logger    // Logger for logging errors and information
}

// NewTesseractOCRService creates a new TesseractOCRService with specified OCR configuration
func NewTesseractOCRService(config ccc.OCRConfig, logger ccc.Logger) *TesseractOCRService {

	if logger == nil {
		logger = ccc.NopLogger
	}

	// Default to English if no languages specified
	if len(config.Languages) == 0 {
		config.Languages = []string{"eng"}
	}
	return &TesseractOCRService{
		config: config,
		logger: logger,
	}
}

// ExtractText extracts text from image data using Tesseract OCR with sensible defaults
func (s *TesseractOCRService) ExtractText(ctx context.Context, imageData []byte) (text string, confidence float32, err error) {

	s.logger.Debug("Extracting text from image data using Tesseract OCR")

	// Create a new Tesseract client for this operation
	client := gosseract.NewClient()
	defer client.Close()

	// Use configured languages for OCR processing
	err = client.SetLanguage(s.config.Languages...)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to set OCR languages %v: %w", s.config.Languages, err)
	}

	// Set automatic page segmentation mode
	err = client.SetPageSegMode(gosseract.PSM_AUTO_OSD)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to set OCR page segmentation mode: %w", err)
	}

	// Set image data
	err = client.SetImageFromBytes(imageData)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to set image data for OCR: %w", err)
	}

	// Extract text
	text, err = client.Text()
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to extract text from image: %w", err)
	}

	// Try to get confidence by using bounding boxes
	// If we can't get confidence, we'll return a default value
	confidence = 0.5 // Default confidence

	// Attempt to get more accurate confidence from bounding boxes
	boxes, boxErr := client.GetBoundingBoxes(gosseract.RIL_WORD)
	if boxErr == nil && len(boxes) > 0 {
		// Calculate average confidence from all words
		totalConfidence := 0.0
		for _, box := range boxes {
			totalConfidence += box.Confidence
		}
		averageConfidence := totalConfidence / float64(len(boxes))
		// Convert from 0-100 to 0.0-1.0 range
		confidence = float32(averageConfidence / 100.0)
	}

	return text, confidence, nil
}

// IsOcrEnabled returns true if OCR is enabled in the configuration
func (s *TesseractOCRService) IsOcrEnabled() bool {
	return s.config.Enabled
}
