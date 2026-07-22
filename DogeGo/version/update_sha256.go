// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ParseChecksumSidecar extracts a lowercase hex SHA256 from GNU sha256sum-style text.
func ParseChecksumSidecar(raw []byte) (string, error) {
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return "", fmt.Errorf("empty checksum file")
	}
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", fmt.Errorf("invalid checksum file")
	}
	hash := strings.ToLower(strings.TrimSpace(fields[0]))
	if len(hash) != 64 {
		return "", fmt.Errorf("checksum length %d, want 64 hex chars", len(hash))
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", fmt.Errorf("checksum not hex: %w", err)
	}
	return hash, nil
}

// FileSHA256Hex returns the lowercase hex SHA256 digest of path.
func FileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fetchExpectedChecksum(client *http.Client, url string) (string, error) {
	if strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("no checksum URL")
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum fetch: HTTP %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	return ParseChecksumSidecar(raw)
}

func verifyFileSHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return fmt.Errorf("expected checksum missing")
	}
	got, err := FileSHA256Hex(path)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("checksum mismatch (got %s, want %s)", got, expected)
	}
	return nil
}
