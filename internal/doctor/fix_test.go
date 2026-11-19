package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateOnlyPreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := createOnly(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := createOnly(path, []byte("overwrite"), 0644); err == nil {
		t.Fatal("overwrote existing file")
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "original" {
		t.Fatalf("existing file changed: %q %v", b, err)
	}
}
