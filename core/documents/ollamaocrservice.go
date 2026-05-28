package documents

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"net/http"
	"strings"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/nfnt/resize"
)

// ollamaOCRConfidence is the fixed confidence score reported for all successful Ollama OCR results.
// It is derived from GLM-OCR's published benchmark: 94.62 on OmniDocBench V1.5 (ranked #1 overall).
// Source: https://ollama.com/library/glm-ocr / https://github.com/zai-org/GLM-OCR
// Note: our integration calls the model directly via Ollama without the full SDK pipeline
// (PP-DocLayout-V3 layout detection), so this is a conservative approximation.
const ollamaOCRConfidence float32 = 0.95

type OllamaOCRService struct {
	config     ccc.OCRConfig
	logger     ccc.Logger
	httpClient *http.Client
}

func NewOllamaOCRService(config ccc.OCRConfig, logger ccc.Logger) *OllamaOCRService {
	if logger == nil {
		logger = ccc.NopLogger
	}
	if config.OllamaURL == "" {
		config.OllamaURL = "http://ollama:11434"
	}
	config.OllamaURL = strings.TrimRight(config.OllamaURL, "/")
	if config.OllamaModel == "" {
		config.OllamaModel = "glm-ocr:q8_0"
	}
	if config.OllamaTimeoutSeconds <= 0 {
		config.OllamaTimeoutSeconds = 300
	}
	if config.ImageMaxDimension <= 0 {
		config.ImageMaxDimension = 640
	}

	return &OllamaOCRService{
		config:     config,
		logger:     logger,
		httpClient: &http.Client{Timeout: time.Duration(config.OllamaTimeoutSeconds) * time.Second},
	}
}

func (s *OllamaOCRService) IsOcrEnabled() bool {
	return s.config.Enabled && s.config.OllamaURL != "" && s.config.OllamaModel != ""
}

func (s *OllamaOCRService) ExtractText(ctx context.Context, imageData []byte) (string, float32, error) {
	if !s.IsOcrEnabled() {
		return "", 0, fmt.Errorf("Ollama OCR is not enabled")
	}

	processedImage, err := prepareImageForOllama(imageData, s.config.ImageMaxDimension)
	if err != nil {
		return "", 0, err
	}

	request := ollamaGenerateRequest{
		Model:     s.config.OllamaModel,
		Prompt:    "Text Recognition: extract all visible text from this document image. Preserve structure, tables, lists, and line breaks where possible. Return only the extracted text.",
		Images:    []string{base64.StdEncoding.EncodeToString(processedImage)},
		Stream:    false,
		KeepAlive: s.config.OllamaKeepAlive,
		Options: map[string]any{
			"temperature": 0,
		},
	}

	var response ollamaGenerateResponse
	if err := s.postJSON(ctx, "/api/generate", request, &response); err != nil {
		return "", 0, err
	}

	return strings.TrimSpace(response.Response), ollamaOCRConfidence, nil
}

func (s *OllamaOCRService) postJSON(ctx context.Context, path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.OllamaURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Ollama request failed with status %s", resp.Status)
	}

	if target == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func prepareImageForOllama(imageData []byte, maxDimension int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image for Ollama OCR: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}

	resized := img
	if width > maxDimension || height > maxDimension {
		if width >= height {
			resized = resize.Resize(uint(maxDimension), 0, img, resize.Lanczos3)
		} else {
			resized = resize.Resize(0, uint(maxDimension), img, resize.Lanczos3)
		}
	}

	rgb := image.NewRGBA(resized.Bounds())
	draw.Draw(rgb, rgb.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(rgb, rgb.Bounds(), resized, resized.Bounds().Min, draw.Over)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgb, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode image for Ollama OCR: %w", err)
	}
	return buf.Bytes(), nil
}

type ollamaGenerateRequest struct {
	Model     string         `json:"model"`
	Prompt    string         `json:"prompt"`
	Images    []string       `json:"images,omitempty"`
	Stream    bool           `json:"stream"`
	KeepAlive string         `json:"keep_alive,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}
