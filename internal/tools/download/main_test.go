package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadVerifiesBeforePublishing(t *testing.T) {
	const content = "runtime"
	pin := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "runtime.wasm")
	if err := download(server.URL, output, "wrong"); err == nil {
		t.Fatal("accepted a mismatched download")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("unverified download published: %v", err)
	}
	if err := download(server.URL, output, pin); err != nil {
		t.Fatal(err)
	}
	if err := download("", output, pin); err != nil {
		t.Fatalf("verified cache was rejected: %v", err)
	}
	if err := download(server.URL, output, "wrong"); err == nil {
		t.Fatal("accepted a mismatched cached asset")
	}
}
