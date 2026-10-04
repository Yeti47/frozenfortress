package main

import (
	"os"
	"testing"
)

// TestSnapshotIsUpToDate fails when api/openapi.yaml differs from the generated spec.
// Regenerate with: go run ./api/cmd/openapi > api/openapi.yaml
func TestSnapshotIsUpToDate(t *testing.T) {
	want, err := Spec()
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
