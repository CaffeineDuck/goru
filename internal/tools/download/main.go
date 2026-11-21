package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const maxAssetBytes = 64 << 20

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: download <url> <output> <sha256>")
		os.Exit(1)
	}
	if err := download(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func checkDigest(src io.Reader, expected string) error {
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(src, maxAssetBytes+1))
	if err != nil {
		return err
	}
	if size > maxAssetBytes || hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("runtime asset does not match SHA-256 %s", expected)
	}
	return nil
}

func download(url, output, expected string) error {
	if f, err := os.Open(output); err == nil {
		defer f.Close()
		return checkDigest(f, expected)
	} else if !os.IsNotExist(err) {
		return err
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.CreateTemp(filepath.Dir(output), ".runtime-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = checkDigest(io.TeeReader(resp.Body, f), expected)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), output)
}
