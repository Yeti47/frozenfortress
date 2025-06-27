package documents

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/nfnt/resize"
)

// ImageFileProcessor handles image file processing (PNG, JPEG, etc.)
type ImageFileProcessor struct {
	ocrService       OCRService
	maxPreviewWidth  uint
	maxPreviewHeight uint
	previewQuality   int
}

// NewImageFileProcessor creates a new ImageFileProcessor with default settings
func NewImageFileProcessor(ocrService OCRService) *ImageFileProcessor {
	return &ImageFileProcessor{
		ocrService:       ocrService,
		maxPreviewWidth:  256,
		maxPreviewHeight: 256,
		previewQuality:   85, // JPEG quality for preview generation
	}
}

// NewImageFileProcessorWithOptions creates a new ImageFileProcessor with custom settings
func NewImageFileProcessorWithOptions(ocrService OCRService, maxWidth, maxHeight uint, quality int) *ImageFileProcessor {
	return &ImageFileProcessor{
		ocrService:       ocrService,
		maxPreviewWidth:  maxWidth,
		maxPreviewHeight: maxHeight,
		previewQuality:   quality,
	}
}

// SupportsContentType checks if this processor can handle image content types
func (p *ImageFileProcessor) SupportsContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	switch contentType {
	case "image/jpeg", "image/jpg", "image/png":
		return true
	default:
		return false
	}
}

// ExtractText extracts text from image files using OCR
func (p *ImageFileProcessor) ExtractText(ctx context.Context, fileData []byte) (text string, confidence float32, pageCount int, err error) {

	// Images always have 1 page
	const imgPageCount = 1

	if p.ocrService == nil {
		return "", 0.0, imgPageCount, fmt.Errorf("OCR service not configured")
	}

	// Check if OCR is enabled
	if !p.ocrService.IsOcrEnabled() {
		return "", 0.0, imgPageCount, fmt.Errorf("OCR is not enabled")
	}

	// Use the OCR service to extract text from the image data
	text, confidence, err = p.ocrService.ExtractText(ctx, fileData)
	if err != nil {
		return "", 0.0, imgPageCount, err
	}

	return text, confidence, imgPageCount, nil
}

// GeneratePreview creates a thumbnail/preview image from the original image
func (p *ImageFileProcessor) GeneratePreview(ctx context.Context, fileData []byte) (*PreviewGenerationResult, error) {
	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(fileData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	originalWidth := uint(bounds.Dx())
	originalHeight := uint(bounds.Dy())

	// Calculate new dimensions while maintaining aspect ratio
	newWidth, newHeight := p.calculatePreviewDimensions(originalWidth, originalHeight)

	// Resize the image
	resizedImg := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	// Encode the preview image
	var buf bytes.Buffer
	var outputFormat string

	switch format {
	case "png":
		err = png.Encode(&buf, resizedImg)
		outputFormat = "image/png"
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: p.previewQuality})
		outputFormat = "image/jpeg"
	default:
		// Default to JPEG for unknown formats
		err = jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: p.previewQuality})
		outputFormat = "image/jpeg"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode preview image: %w", err)
	}

	return &PreviewGenerationResult{
		PreviewData: buf.Bytes(),
		PreviewType: outputFormat,
		Width:       int(newWidth),
		Height:      int(newHeight),
	}, nil
}

// calculatePreviewDimensions calculates the preview dimensions while maintaining aspect ratio
func (p *ImageFileProcessor) calculatePreviewDimensions(originalWidth, originalHeight uint) (uint, uint) {
	// If image is already smaller than max dimensions, return original size
	if originalWidth <= p.maxPreviewWidth && originalHeight <= p.maxPreviewHeight {
		return originalWidth, originalHeight
	}

	// Calculate aspect ratio
	aspectRatio := float64(originalWidth) / float64(originalHeight)

	var newWidth, newHeight uint

	if originalWidth > originalHeight {
		// Landscape orientation - limit by width
		newWidth = p.maxPreviewWidth
		newHeight = uint(float64(newWidth) / aspectRatio)

		// Check if height exceeds limit
		if newHeight > p.maxPreviewHeight {
			newHeight = p.maxPreviewHeight
			newWidth = uint(float64(newHeight) * aspectRatio)
		}
	} else {
		// Portrait orientation - limit by height
		newHeight = p.maxPreviewHeight
		newWidth = uint(float64(newHeight) * aspectRatio)

		// Check if width exceeds limit
		if newWidth > p.maxPreviewWidth {
			newWidth = p.maxPreviewWidth
			newHeight = uint(float64(newWidth) / aspectRatio)
		}
	}

	// Ensure dimensions are at least 1
	if newWidth == 0 {
		newWidth = 1
	}
	if newHeight == 0 {
		newHeight = 1
	}

	return newWidth, newHeight
}
