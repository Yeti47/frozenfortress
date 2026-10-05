package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

func ollamaTestImage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 200, 100))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newOllamaTestService starts a fake Ollama that answers every /api/generate request with the given
// response and returns the service plus a pointer to the last request body it received.
func newOllamaTestService(t *testing.T, maxDimension int, response ollamaGenerateResponse) (*OllamaOCRService, *ollamaGenerateRequest) {
	t.Helper()

	var got ollamaGenerateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)

	service := NewOllamaOCRService(ccc.OCRConfig{
		Enabled:           true,
		OllamaURL:         server.URL,
		OllamaModel:       "test-model",
		ImageMaxDimension: maxDimension,
	}, nil)
	return service, &got
}

func TestOllamaOCRServiceLimitsOutputAndContext(t *testing.T) {
	service, got := newOllamaTestService(t, 640, ollamaGenerateResponse{Response: "hello", DoneReason: "stop"})

	if _, _, err := service.ExtractText(context.Background(), ollamaTestImage(t)); err != nil {
		t.Fatal(err)
	}

	if numPredict, ok := got.Options["num_predict"].(float64); !ok || int(numPredict) != ollamaMaxOutputTokens {
		t.Errorf("num_predict = %v, want %d", got.Options["num_predict"], ollamaMaxOutputTokens)
	}
	if numCtx, ok := got.Options["num_ctx"].(float64); !ok || int(numCtx) != 4096 {
		t.Errorf("num_ctx = %v, want 4096", got.Options["num_ctx"])
	}
	if got.Options["temperature"] != float64(0) {
		t.Errorf("temperature = %v, want 0", got.Options["temperature"])
	}
}

func TestOllamaOCRServiceStoppedResultKeepsFullConfidence(t *testing.T) {
	service, _ := newOllamaTestService(t, 640, ollamaGenerateResponse{Response: "  some text \n", DoneReason: "stop"})

	text, confidence, err := service.ExtractText(context.Background(), ollamaTestImage(t))
	if err != nil {
		t.Fatal(err)
	}
	if text != "some text" {
		t.Errorf("text = %q, want %q", text, "some text")
	}
	if confidence != ollamaOCRConfidence {
		t.Errorf("confidence = %v, want %v", confidence, ollamaOCRConfidence)
	}
}

func TestOllamaOCRServiceTruncatedResultIsKeptWithLowerConfidence(t *testing.T) {
	service, _ := newOllamaTestService(t, 640, ollamaGenerateResponse{Response: "partial text", DoneReason: "length"})

	text, confidence, err := service.ExtractText(context.Background(), ollamaTestImage(t))
	if err != nil {
		t.Fatal(err)
	}
	if text != "partial text" {
		t.Errorf("text = %q, want %q", text, "partial text")
	}
	if confidence != ollamaOCRTruncatedConfidence {
		t.Errorf("confidence = %v, want %v", confidence, ollamaOCRTruncatedConfidence)
	}
}

func TestOllamaOCRServiceReturnsErrorOnServerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"runner crashed"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	service := NewOllamaOCRService(ccc.OCRConfig{Enabled: true, OllamaURL: server.URL, OllamaModel: "test-model"}, nil)
	if _, _, err := service.ExtractText(context.Background(), ollamaTestImage(t)); err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
}

func TestOllamaContextSize(t *testing.T) {
	tests := []struct {
		maxDimension int
		want         int
	}{
		{256, 4096},
		{640, 4096},  // default: 23*23 image tokens + prompt + output fit into Ollama's default
		{1024, 5120}, // 37*37 = 1369 image tokens
		{1536, 7168}, // 55*55 = 3025 image tokens
	}
	for _, tt := range tests {
		got := ollamaContextSize(tt.maxDimension)
		if got != tt.want {
			t.Errorf("ollamaContextSize(%d) = %d, want %d", tt.maxDimension, got, tt.want)
		}
		// The point of the context size: prompt, largest image and the full output always fit.
		blocks := (tt.maxDimension + ollamaImageTokenBlock - 1) / ollamaImageTokenBlock
		if need := blocks*blocks + ollamaPromptReserve + ollamaMaxOutputTokens; got < need {
			t.Errorf("ollamaContextSize(%d) = %d, less than the %d tokens a request can need", tt.maxDimension, got, need)
		}
	}
}
