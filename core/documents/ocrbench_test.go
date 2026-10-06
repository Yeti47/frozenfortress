//go:build ocrbench

// OCR speed benchmark for the Ollama OCR service. It is excluded from normal
// test runs by the ocrbench build tag and is meant to be driven by
// dev/benchmarks/ocr-bench.sh, which starts Ollama on a limited set of CPU cores.
//
// Environment:
//
//	FF_BENCH_OLLAMA_URL  Ollama base URL (required, the test skips without it)
//	FF_BENCH_MODEL       model name (default glm-ocr:q8_0)
//	FF_BENCH_RUNS        warm runs per payload and dimension (default 3)
//	FF_BENCH_MAX_DIMS    comma-separated FF_OCR_IMAGE_MAX_DIMENSION values (default 640)
//	FF_BENCH_TIMEOUT     per-request timeout in seconds (default 900)
//	FF_BENCH_PROFILE     label for the report (default "default")
//	FF_BENCH_CPUSET      description of the CPU limit, for the report only
//	FF_BENCH_MEMORY      description of the memory limit, for the report only
//	FF_BENCH_OUT         path of the JSON result (default ocr-bench.json)
package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

type benchResult struct {
	Payload      string    `json:"payload"`
	Source       string    `json:"source"`
	MaxDimension int       `json:"maxDimension"`
	SentSize     string    `json:"sentSize"`
	SentKB       int       `json:"sentKB"`
	RunsMs       []float64 `json:"runsMs"`
	MinMs        float64   `json:"minMs"`
	MedianMs     float64   `json:"medianMs"`
	P95Ms        float64   `json:"p95Ms"`
	WordAccuracy float64   `json:"wordAccuracy"`
	Failures     []string  `json:"failures,omitempty"`
}

type benchReport struct {
	Profile     string        `json:"profile"`
	CPUSet      string        `json:"cpuset"`
	Memory      string        `json:"memory"`
	HostCPU     string        `json:"hostCpu"`
	Model       string        `json:"model"`
	Date        string        `json:"date"`
	ColdPayload string        `json:"coldPayload"`
	ColdMs      float64       `json:"coldMs"`
	Results     []benchResult `json:"results"`
}

func TestOCRBenchmark(t *testing.T) {
	url := os.Getenv("FF_BENCH_OLLAMA_URL")
	if url == "" {
		t.Skip("FF_BENCH_OLLAMA_URL not set; run dev/benchmarks/ocr-bench.sh")
	}
	model := benchEnv("FF_BENCH_MODEL", "glm-ocr:q8_0")
	runs := benchEnvInt(t, "FF_BENCH_RUNS", 3)
	timeout := benchEnvInt(t, "FF_BENCH_TIMEOUT", 900)
	var dims []int
	for _, s := range strings.Split(benchEnv("FF_BENCH_MAX_DIMS", "640"), ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n <= 0 {
			t.Fatalf("FF_BENCH_MAX_DIMS: invalid value %q", s)
		}
		dims = append(dims, n)
	}

	// Variant i is used by warm run i, variant "runs" by the cold request, so
	// no request repeats an earlier one.
	variants := make([][]benchPayload, runs+1)
	for v := range variants {
		var err error
		if variants[v], err = benchPayloads(v); err != nil {
			t.Fatalf("building payloads: %v", err)
		}
	}
	payloads := variants[0]

	newService := func(maxDim int) *OllamaOCRService {
		return NewOllamaOCRService(ccc.OCRConfig{
			Enabled:              true,
			OllamaURL:            url,
			OllamaModel:          model,
			OllamaKeepAlive:      "30m",
			OllamaTimeoutSeconds: timeout,
			ImageMaxDimension:    maxDim,
		}, nil)
	}

	report := benchReport{
		Profile: benchEnv("FF_BENCH_PROFILE", "default"),
		CPUSet:  os.Getenv("FF_BENCH_CPUSET"),
		Memory:  os.Getenv("FF_BENCH_MEMORY"),
		HostCPU: hostCPUModel(),
		Model:   model,
		Date:    time.Now().UTC().Format(time.RFC3339),
	}

	// The first request after the container start includes loading the model
	// into memory, so it is reported on its own and not mixed into the runs.
	// It uses the small sparse page, which is cheap and does not fail.
	coldIndex := slices.IndexFunc(payloads, func(p benchPayload) bool { return p.Name == "sparse-a4-150dpi" })
	cold := variants[runs][coldIndex]
	start := time.Now()
	if _, _, err := newService(dims[0]).ExtractText(context.Background(), cold.Data); err != nil {
		t.Fatalf("cold request failed: %v", err)
	}
	report.ColdPayload = cold.Name
	report.ColdMs = ms(time.Since(start))
	t.Logf("cold start (%s): %.0f ms", cold.Name, report.ColdMs)

	for _, dim := range dims {
		svc := newService(dim)
		for pi, p := range payloads {
			res := benchResult{
				Payload:      p.Name,
				Source:       strings.ToUpper(p.Format) + " " + strconv.Itoa(p.Width) + "x" + strconv.Itoa(p.Height) + ", " + strconv.Itoa(len(p.Data)/1024) + " KB",
				MaxDimension: dim,
			}
			if sent, err := prepareImageForOllama(p.Data, dim); err == nil {
				res.SentKB = len(sent) / 1024
				if cfg, _, err := image.DecodeConfig(bytes.NewReader(sent)); err == nil {
					res.SentSize = strconv.Itoa(cfg.Width) + "x" + strconv.Itoa(cfg.Height)
				}
			}

			var accuracy float64
			for i := 0; i < runs; i++ {
				start := time.Now()
				doc := variants[i][pi]
				text, _, err := svc.ExtractText(context.Background(), doc.Data)
				elapsed := time.Since(start)
				if err != nil {
					res.Failures = append(res.Failures, err.Error())
					t.Logf("%s @%d run %d failed: %v", p.Name, dim, i+1, err)
					continue
				}
				res.RunsMs = append(res.RunsMs, ms(elapsed))
				accuracy += benchWordAccuracy(doc.Words, text)
			}
			if n := len(res.RunsMs); n > 0 {
				res.WordAccuracy = accuracy / float64(n)
				sorted := slices.Clone(res.RunsMs)
				slices.Sort(sorted)
				res.MinMs = sorted[0]
				res.MedianMs = sorted[n/2]
				if n%2 == 0 {
					res.MedianMs = (sorted[n/2-1] + sorted[n/2]) / 2
				}
				res.P95Ms = sorted[min(n-1, (n*95+99)/100-1)]
			}
			t.Logf("%-18s @%-4d median %7.0f ms  accuracy %3.0f%%  (sent %s, %d KB)",
				p.Name, dim, res.MedianMs, res.WordAccuracy*100, res.SentSize, res.SentKB)
			report.Results = append(report.Results, res)
		}
	}

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(benchEnv("FF_BENCH_OUT", "ocr-bench.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func benchEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func benchEnvInt(t *testing.T, key string, fallback int) int {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		t.Fatalf("%s: invalid value %q", key, v)
	}
	return n
}

func hostCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			if _, v, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return "unknown"
}
