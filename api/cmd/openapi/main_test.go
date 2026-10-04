package main

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

// TestSnapshotIsUpToDate fails when api/openapi.yaml differs from the generated spec.
// Regenerate with: go run ./api/cmd/openapi > api/openapi.yaml
func TestSnapshotIsUpToDate(t *testing.T) {
	want, err := generateSpec()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatalf("cannot read the snapshot: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("api/openapi.yaml is out of date; run: go run ./api/cmd/openapi > api/openapi.yaml")
	}
}

func TestRunWritesSpec(t *testing.T) {
	var buf bytes.Buffer
	if err := run(&buf); err != nil {
		t.Fatal(err)
	}
	want, _ := generateSpec()
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatal("run output differs from generateSpec")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestRunReturnsWriteError(t *testing.T) {
	if err := run(failingWriter{}); err == nil {
		t.Fatal("expected the write error")
	}
}
