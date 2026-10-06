//go:build ocrbench

package documents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestOCRBenchmarkReport merges the per-profile JSON files written by
// TestOCRBenchmark into one Markdown report. FF_BENCH_REPORT_DIR is the
// directory that holds them; the report is written to report.md in it.
func TestOCRBenchmarkReport(t *testing.T) {
	dir := os.Getenv("FF_BENCH_REPORT_DIR")
	if dir == "" {
		t.Skip("FF_BENCH_REPORT_DIR not set; run dev/benchmarks/ocr-bench.sh")
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no result files in %s", dir)
	}
	var reports []benchReport
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var r benchReport
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		reports = append(reports, r)
	}
	slices.SortFunc(reports, func(a, b benchReport) int { return strings.Compare(a.Profile, b.Profile) })

	var b strings.Builder
	first := reports[0]
	fmt.Fprintf(&b, "# OCR benchmark\n\n")
	fmt.Fprintf(&b, "- Model: `%s`\n- Host CPU: %s\n- Date: %s\n\n", first.Model, first.HostCPU, first.Date)
	fmt.Fprintf(&b, "Times are wall-clock seconds for one `ExtractText` call (resize, JPEG encode, Ollama request). ")
	fmt.Fprintf(&b, "CPU profiles limit the number of cores and the memory, not clock speed or memory bandwidth, so compare profiles with each other rather than reading them as a hardware guarantee.\n\n")

	fmt.Fprintf(&b, "## Profiles\n\n| Profile | CPU limit | Memory limit | Cold start (first request) |\n| -- | -- | -- | -- |\n")
	for _, r := range reports {
		fmt.Fprintf(&b, "| %s | %s | %s | %.1f s (%s) |\n", r.Profile, r.CPUSet, r.Memory, r.ColdMs/1000, r.ColdPayload)
	}

	dims := map[int]bool{}
	for _, r := range reports {
		for _, res := range r.Results {
			dims[res.MaxDimension] = true
		}
	}
	var dimList []int
	for d := range dims {
		dimList = append(dimList, d)
	}
	slices.Sort(dimList)

	for _, dim := range dimList {
		fmt.Fprintf(&b, "\n## Median warm time at max dimension %d\n\n| Payload | Sent as |", dim)
		for _, r := range reports {
			fmt.Fprintf(&b, " %s |", r.Profile)
		}
		fmt.Fprintf(&b, " Word accuracy |\n| -- | -- |")
		for range reports {
			fmt.Fprintf(&b, " -- |")
		}
		fmt.Fprintf(&b, " -- |\n")

		for _, row := range reports[0].Results {
			if row.MaxDimension != dim {
				continue
			}
			fmt.Fprintf(&b, "| %s (%s) | %s, %d KB |", row.Payload, row.Source, row.SentSize, row.SentKB)
			var recall float64
			for _, r := range reports {
				cell := "n/a"
				for _, res := range r.Results {
					if res.Payload == row.Payload && res.MaxDimension == dim && len(res.RunsMs) > 0 {
						cell = fmt.Sprintf("%.1f s (p95 %.1f s)", res.MedianMs/1000, res.P95Ms/1000)
						recall = res.WordAccuracy
						if len(res.Failures) > 0 {
							cell += fmt.Sprintf(", %d failed", len(res.Failures))
						}
					}
				}
				fmt.Fprintf(&b, " %s |", cell)
			}
			fmt.Fprintf(&b, " %.0f%% |\n", recall*100)
		}
	}

	path := filepath.Join(dir, "report.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}
