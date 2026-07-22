// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadResult is the outcome of saving a verified release asset.
type DownloadResult struct {
	Path           string
	SHA256         string
	ChecksumURL    string
	ChecksumVerify bool
}

// DownloadReleaseAsset saves the configured GitHub release asset under baseDataDir/updates/.
func (c *UpdateChecker) DownloadReleaseAsset(baseDataDir string) (path string, err error) {
	res, err := c.DownloadReleaseAssetVerified(baseDataDir)
	if err != nil {
		return "", err
	}
	return res.Path, nil
}

// DownloadReleaseAssetVerified downloads the release asset and verifies SHA256 when a checksum is published.
func (c *UpdateChecker) DownloadReleaseAssetVerified(baseDataDir string) (DownloadResult, error) {
	if c == nil {
		return DownloadResult{}, fmt.Errorf("update checker not available")
	}
	st := c.Status()
	if st.DownloadURL == "" {
		return DownloadResult{}, fmt.Errorf("no direct download asset for this platform")
	}
	expected := strings.ToLower(strings.TrimSpace(st.ChecksumSHA256))
	checksumURL := st.ChecksumURL
	if expected == "" && checksumURL != "" {
		got, err := fetchExpectedChecksum(c.client, checksumURL)
		if err != nil {
			return DownloadResult{}, fmt.Errorf("fetch checksum: %w", err)
		}
		expected = got
	}
	path, sha, err := downloadReleaseAsset(c.client, baseDataDir, st.DownloadURL, st.LatestVersion, expected)
	if err != nil {
		return DownloadResult{}, err
	}
	return DownloadResult{
		Path:           path,
		SHA256:         sha,
		ChecksumURL:    checksumURL,
		ChecksumVerify: expected != "",
	}, nil
}

func downloadReleaseAsset(client *http.Client, baseDataDir, url, version, expectedSHA256 string) (path, sha256Hex string, err error) {
	if strings.TrimSpace(baseDataDir) == "" {
		return "", "", fmt.Errorf("datadir required for update download")
	}
	if client == nil {
		client = http.DefaultClient
	}
	dir := filepath.Join(baseDataDir, "updates")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	name := "dogego-" + strings.TrimSpace(version)
	if name == "dogego-" {
		name = "dogego-latest"
	}
	if u := strings.Split(url, "?")[0]; strings.Contains(u, ".") {
		if ext := filepath.Ext(u); ext != "" && ext != "." {
			name += ext
		}
	}
	dest := filepath.Join(dir, name)
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download failed: HTTP %s", resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", "", err
	}
	if err := f.Close(); err != nil {
		return "", "", err
	}
	sha256Hex, err = FileSHA256Hex(dest)
	if err != nil {
		return "", "", err
	}
	if expectedSHA256 != "" {
		if err := verifyFileSHA256(dest, expectedSHA256); err != nil {
			_ = os.Remove(dest)
			return "", "", err
		}
	}
	return dest, sha256Hex, nil
}

// VerifyDownloadedAsset re-checks SHA256 for a previously downloaded update binary.
func (c *UpdateChecker) VerifyDownloadedAsset(path string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("update checker not available")
	}
	st := c.Status()
	expected := strings.ToLower(strings.TrimSpace(st.ChecksumSHA256))
	if expected == "" && st.ChecksumURL != "" {
		got, err := fetchExpectedChecksum(c.client, st.ChecksumURL)
		if err != nil {
			return "", err
		}
		expected = got
	}
	if expected == "" {
		return FileSHA256Hex(path)
	}
	if err := verifyFileSHA256(path, expected); err != nil {
		return "", err
	}
	return expected, nil
}
