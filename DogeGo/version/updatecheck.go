// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	updateCheckInterval = 24 * time.Hour
	updateHTTPTimeout   = 15 * time.Second
	updateStateFile     = "update_check.json"
)

// UpdateStatus is the cached result of checking GitHub releases.
type UpdateStatus struct {
	CheckedAt         time.Time `json:"checked_at"`
	Available         bool      `json:"available"`
	Installable       bool      `json:"installable"` // platform asset present (newer or same for force reinstall)
	CurrentVersion    string    `json:"current_version"`
	LatestVersion     string    `json:"latest_version,omitempty"`
	LatestTag         string    `json:"latest_tag,omitempty"`
	Prerelease        bool      `json:"prerelease,omitempty"`
	Source            string    `json:"source,omitempty"`
	ReleaseURL        string    `json:"release_url,omitempty"`
	DownloadURL       string    `json:"download_url,omitempty"`
	ChecksumURL       string    `json:"checksum_url,omitempty"`
	ChecksumSHA256    string    `json:"checksum_sha256,omitempty"`
	DirectUpdate      bool      `json:"direct_update_available"`
	BuildCommand      string    `json:"build_command"`
	Instructions      string    `json:"instructions,omitempty"`
	Dismissed         bool      `json:"dismissed"`
	DismissedVersion  string    `json:"dismissed_version,omitempty"`
	CheckError        string    `json:"check_error,omitempty"`
	SourcesChecked    []string  `json:"sources_checked,omitempty"`
}

type updateCheckState struct {
	LastCheckUnix    int64  `json:"last_check_unix"`
	LatestVersion    string `json:"latest_version"`
	LatestTag        string `json:"latest_tag"`
	Source           string `json:"source"`
	ReleaseURL       string `json:"release_url"`
	DownloadURL      string `json:"download_url"`
	ChecksumURL      string `json:"checksum_url"`
	ChecksumSHA256   string `json:"checksum_sha256"`
	DirectUpdate     bool   `json:"direct_update_available"`
	DismissedVersion string `json:"dismissed_version"`
	DismissedAtUnix  int64  `json:"dismissed_at_unix"`
	CheckError       string `json:"check_error,omitempty"`
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// UpdateChecker polls GitHub for newer DogeGo releases (daily) and tracks dismissals.
type UpdateChecker struct {
	dataDir string
	client  *http.Client
	mu      sync.RWMutex
	status  UpdateStatus
	started sync.Once
	onAvailable         func(UpdateStatus)
	lastNotifiedVersion string
}

// SetOnAvailable registers a callback when a non-dismissed update is newly detected.
func (c *UpdateChecker) SetOnAvailable(fn func(UpdateStatus)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAvailable = fn
}

func (c *UpdateChecker) takeNotifyCallbackLocked() (UpdateStatus, func(UpdateStatus), bool) {
	if c.onAvailable == nil || !c.status.Available || c.status.Dismissed || c.status.LatestVersion == "" {
		return UpdateStatus{}, nil, false
	}
	if c.lastNotifiedVersion == c.status.LatestVersion {
		return UpdateStatus{}, nil, false
	}
	c.lastNotifiedVersion = c.status.LatestVersion
	return c.status, c.onAvailable, true
}

// NewUpdateChecker stores dismissal state under dataDir/update_check.json.
func NewUpdateChecker(dataDir string) *UpdateChecker {
	return &UpdateChecker{
		dataDir: dataDir,
		client:  &http.Client{Timeout: updateHTTPTimeout},
		status: UpdateStatus{
			CurrentVersion: Display(),
			BuildCommand:   defaultBuildCommand(),
		},
	}
}

func defaultBuildCommand() string {
	if runtime.GOOS == "windows" {
		return "cd DogeGo & go build -o dogego.exe .\\cmd\\dogego"
	}
	return "cd DogeGo && go build -o dogego ./cmd/dogego"
}

// UpdateCheckDisabled reports whether release checks are turned off (DOGEGO_NO_UPDATE_CHECK=1).
func UpdateCheckDisabled() bool {
	v := strings.TrimSpace(os.Getenv("DOGEGO_NO_UPDATE_CHECK"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// Start loads cached state, checks once, then rechecks every 24h until ctx ends.
func (c *UpdateChecker) Start(ctx context.Context) {
	if c == nil || UpdateCheckDisabled() {
		return
	}
	c.started.Do(func() {
		c.loadState()
		c.refresh(context.Background())
		go func() {
			t := time.NewTicker(updateCheckInterval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					c.refresh(context.Background())
				}
			}
		}()
	})
}

// RefreshNow forces an immediate online release check.
func (c *UpdateChecker) RefreshNow(ctx context.Context) {
	if c == nil || UpdateCheckDisabled() {
		return
	}
	c.refresh(ctx)
}

// Status returns a copy of the latest update check snapshot.
func (c *UpdateChecker) Status() UpdateStatus {
	if c == nil {
		return UpdateStatus{CurrentVersion: Display(), BuildCommand: defaultBuildCommand()}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Dismiss hides the notification for the current latest release until a newer one appears.
func (c *UpdateChecker) Dismiss() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status.LatestVersion == "" {
		return fmt.Errorf("no release version to dismiss")
	}
	c.status.Dismissed = true
	c.status.DismissedVersion = c.status.LatestVersion
	return c.saveStateLocked()
}

// PrintNotice writes a CLI hint when a non-dismissed update is available.
func (c *UpdateChecker) PrintNotice(w io.Writer) {
	if c == nil || w == nil {
		return
	}
	st := c.Status()
	if !st.Available || st.Dismissed {
		return
	}
	fmt.Fprintf(w, "\n*** Update available: DogeGo %s (running %s)\n", st.LatestVersion, st.CurrentVersion)
	if st.ReleaseURL != "" {
		fmt.Fprintf(w, "    Release: %s\n", st.ReleaseURL)
	}
	if st.DownloadURL != "" {
		fmt.Fprintf(w, "    Download binary: %s\n", st.DownloadURL)
	} else if st.ReleaseURL != "" {
		fmt.Fprintf(w, "    Download: open Releases on the page above\n")
	}
	fmt.Fprintf(w, "    Build from source: %s\n", st.BuildCommand)
	if st.DirectUpdate {
		fmt.Fprintf(w, "    Web UI: Settings or Overview banner can download the release asset for manual install\n")
	}
	fmt.Fprintf(w, "    Dismiss: use the Web UI banner or POST /api/update/dismiss (rechecks daily)\n")
}

// SummaryFields returns JSON-friendly fields for /api/summary.
func (c *UpdateChecker) SummaryFields() map[string]any {
	st := c.Status()
	out := map[string]any{
		"dogego_update_available":        st.Available,
		"dogego_update_installable":      st.Installable,
		"dogego_update_prerelease":       st.Prerelease,
		"dogego_update_current":          st.CurrentVersion,
		"dogego_update_build_cmd":        st.BuildCommand,
		"dogego_update_dismissed":        st.Dismissed,
		"dogego_update_direct_available": st.DirectUpdate,
		"dogego_update_check_interval_h": int(updateCheckInterval / time.Hour),
	}
	if st.LatestVersion != "" {
		out["dogego_update_latest"] = st.LatestVersion
	}
	if st.LatestTag != "" {
		out["dogego_update_latest_tag"] = st.LatestTag
	}
	if st.Source != "" {
		out["dogego_update_source"] = st.Source
	}
	if st.ReleaseURL != "" {
		out["dogego_update_release_url"] = st.ReleaseURL
	}
	if st.DownloadURL != "" {
		out["dogego_update_download_url"] = st.DownloadURL
	}
	if st.ChecksumURL != "" {
		out["dogego_update_checksum_url"] = st.ChecksumURL
	}
	if st.ChecksumSHA256 != "" {
		out["dogego_update_checksum_sha256"] = st.ChecksumSHA256
	}
	if st.Instructions != "" {
		out["dogego_update_instructions"] = st.Instructions
	}
	if !st.CheckedAt.IsZero() {
		out["dogego_update_checked_at"] = st.CheckedAt.UTC().Format(time.RFC3339)
	}
	if st.CheckError != "" {
		out["dogego_update_check_error"] = st.CheckError
	}
	if len(st.SourcesChecked) > 0 {
		out["dogego_update_sources_checked"] = st.SourcesChecked
	}
	return out
}

func (c *UpdateChecker) statePath() string {
	if c.dataDir == "" {
		return ""
	}
	return filepath.Join(c.dataDir, updateStateFile)
}

func (c *UpdateChecker) loadState() {
	path := c.statePath()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var st updateCheckState
	if json.Unmarshal(raw, &st) != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyStoredStateLocked(st)
}

func (c *UpdateChecker) applyStoredStateLocked(st updateCheckState) {
	if st.LatestVersion != "" {
		c.status.LatestVersion = st.LatestVersion
		c.status.LatestTag = st.LatestTag
		c.status.Source = st.Source
		c.status.ReleaseURL = st.ReleaseURL
		c.status.DownloadURL = st.DownloadURL
		c.status.ChecksumURL = st.ChecksumURL
		c.status.ChecksumSHA256 = st.ChecksumSHA256
		c.status.DirectUpdate = st.DirectUpdate
	}
	if st.LastCheckUnix > 0 {
		c.status.CheckedAt = time.Unix(st.LastCheckUnix, 0)
	}
	c.status.CheckError = st.CheckError
	c.status.DismissedVersion = st.DismissedVersion
	c.status.Dismissed = st.DismissedVersion != "" && st.DismissedVersion == st.LatestVersion &&
		SemverCompare(st.LatestVersion, normalizeSemver(ClientVersion)) > 0
	c.recomputeAvailableLocked()
}

func (c *UpdateChecker) saveStateLocked() error {
	path := c.statePath()
	if path == "" {
		return nil
	}
	st := updateCheckState{
		LastCheckUnix:    c.status.CheckedAt.Unix(),
		LatestVersion:    c.status.LatestVersion,
		LatestTag:        c.status.LatestTag,
		Source:           c.status.Source,
		ReleaseURL:       c.status.ReleaseURL,
		DownloadURL:      c.status.DownloadURL,
		ChecksumURL:      c.status.ChecksumURL,
		ChecksumSHA256:   c.status.ChecksumSHA256,
		DirectUpdate:     c.status.DirectUpdate,
		DismissedVersion: c.status.DismissedVersion,
		CheckError:       c.status.CheckError,
	}
	if c.status.Dismissed {
		st.DismissedAtUnix = time.Now().Unix()
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *UpdateChecker) refresh(ctx context.Context) {
	if c == nil || UpdateCheckDisabled() {
		return
	}
	best, sources, err := fetchLatestRelease(ctx, c.client)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.CurrentVersion = Display()
	c.status.BuildCommand = defaultBuildCommand()
	c.status.CheckedAt = time.Now()
	c.status.SourcesChecked = sources
	if err != nil {
		c.status.CheckError = err.Error()
		c.recomputeAvailableLocked()
		_ = c.saveStateLocked()
		return
	}
	c.status.CheckError = ""
	c.status.LatestVersion = normalizeSemver(best.TagName)
	c.status.LatestTag = strings.TrimSpace(best.TagName)
	c.status.Prerelease = best.Prerelease
	c.status.Source = best.Source
	c.status.ReleaseURL = best.ReleaseURL
	c.status.DownloadURL = best.AssetURL
	c.status.ChecksumURL = best.ChecksumURL
	c.status.ChecksumSHA256 = best.ChecksumSHA256
	c.status.DirectUpdate = best.AssetURL != ""
	c.status.Instructions = buildUpdateInstructions(best)
	if c.status.DismissedVersion != "" && c.status.DismissedVersion != c.status.LatestVersion {
		c.status.Dismissed = false
	}
	c.recomputeAvailableLocked()
	_ = c.saveStateLocked()
	st, fn, ok := c.takeNotifyCallbackLocked()
	if ok && fn != nil {
		go fn(st)
	}
}

func (c *UpdateChecker) recomputeAvailableLocked() {
	cur := normalizeSemver(ClientVersion)
	c.status.Installable = c.status.LatestVersion != "" && c.status.DownloadURL != ""
	c.status.Available = c.status.LatestVersion != "" && SemverCompare(c.status.LatestVersion, cur) > 0
	if !c.status.Available {
		c.status.Dismissed = false
		return
	}
	if c.status.DismissedVersion == c.status.LatestVersion {
		c.status.Dismissed = true
	}
}

type releaseCandidate struct {
	TagName        string
	Source         string
	ReleaseURL     string
	AssetURL       string
	AssetName      string
	ChecksumURL    string
	ChecksumSHA256 string
	Prerelease     bool
}

func fetchLatestRelease(ctx context.Context, client *http.Client) (releaseCandidate, []string, error) {
	var best releaseCandidate
	var sources []string
	var lastErr error
	for _, src := range enabledUpdateSources() {
		label := src.Owner + "/" + src.Repo
		sources = append(sources, label)
		rel, err := fetchGitHubBestRelease(ctx, client, src.Owner, src.Repo)
		if err != nil {
			lastErr = err
			continue
		}
		ver := normalizeSemver(rel.TagName)
		if ver == "" {
			continue
		}
		candidate := releaseCandidate{
			TagName:    rel.TagName,
			Source:     "https://github.com/" + label,
			ReleaseURL: rel.HTMLURL,
			Prerelease: rel.Prerelease,
		}
		assetName, assetURL := pickReleaseAsset(rel)
		candidate.AssetName = assetName
		candidate.AssetURL = assetURL
		candidate.ChecksumURL = pickReleaseChecksumURL(rel, assetName)
		if best.TagName == "" || SemverCompare(ver, normalizeSemver(best.TagName)) > 0 {
			best = candidate
		}
	}
	if best.TagName == "" {
		if lastErr != nil {
			return releaseCandidate{}, sources, lastErr
		}
		return releaseCandidate{}, sources, fmt.Errorf("no releases found on configured GitHub sources")
	}
	return best, sources, nil
}

func fetchGitHubBestRelease(ctx context.Context, client *http.Client, owner, repo string) (ghRelease, error) {
	listURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=30", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return ghRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", HTTPUserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var list []ghRelease
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			return ghRelease{}, err
		}
		var best ghRelease
		for _, rel := range list {
			if rel.Draft || strings.TrimSpace(rel.TagName) == "" {
				continue
			}
			ver := normalizeSemver(rel.TagName)
			if ver == "" {
				continue
			}
			if best.TagName == "" || SemverCompare(ver, normalizeSemver(best.TagName)) > 0 {
				best = rel
			}
		}
		if best.TagName != "" {
			return best, nil
		}
	} else if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ghRelease{}, fmt.Errorf("%s/%s: HTTP %d %s", owner, repo, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return fetchGitHubLatestRelease(ctx, client, owner, repo)
}

func fetchGitHubLatestRelease(ctx context.Context, client *http.Client, owner, repo string) (ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ghRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", HTTPUserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ghRelease{}, fmt.Errorf("%s/%s: no releases", owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ghRelease{}, fmt.Errorf("%s/%s: HTTP %d %s", owner, repo, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghRelease{}, err
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return ghRelease{}, fmt.Errorf("%s/%s: empty tag", owner, repo)
	}
	return rel, nil
}

func pickReleaseAsset(rel ghRelease) (name, url string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	archAliases := []string{goarch}
	if goarch == "amd64" {
		archAliases = append(archAliases, "x86_64", "x64")
	}
	if goarch == "arm64" {
		archAliases = append(archAliases, "aarch64")
	}
	osAliases := []string{goos}
	if goos == "darwin" {
		osAliases = append(osAliases, "macos", "osx")
	}
	for _, a := range rel.Assets {
		aname := strings.TrimSpace(a.Name)
		lname := strings.ToLower(aname)
		if a.BrowserDownloadURL == "" || !strings.Contains(lname, "dogego") {
			continue
		}
		if strings.Contains(lname, ".sha256") || strings.HasSuffix(lname, ".txt") {
			continue
		}
		hasOS := false
		for _, alias := range osAliases {
			if strings.Contains(lname, alias) {
				hasOS = true
				break
			}
		}
		hasArch := false
		for _, alias := range archAliases {
			if strings.Contains(lname, alias) {
				hasArch = true
				break
			}
		}
		if hasOS && hasArch {
			return aname, a.BrowserDownloadURL
		}
	}
	return "", ""
}

func pickReleaseChecksumURL(rel ghRelease, assetName string) string {
	if strings.TrimSpace(assetName) == "" {
		return ""
	}
	want := strings.ToLower(assetName + ".sha256")
	for _, a := range rel.Assets {
		if a.BrowserDownloadURL == "" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(a.Name)) == want {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func buildUpdateInstructions(best releaseCandidate) string {
	var b strings.Builder
	b.WriteString("Download a prebuilt binary from GitHub Releases")
	if best.Source != "" {
		b.WriteString(" (")
		b.WriteString(best.Source)
		b.WriteString(")")
	}
	b.WriteString(", or compile from source with: ")
	b.WriteString(defaultBuildCommand())
	b.WriteString(". Stop the node, replace the binary, and restart.")
	if best.AssetURL != "" {
		b.WriteString(" The Web UI and system tray can download, verify SHA256, and install the update with an automatic restart.")
	}
	return b.String()
}

// normalizeSemver strips v-prefix and pre-release suffix for comparison.
func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	return v
}

// SemverCompare returns -1 if a<b, 0 if equal, 1 if a>b (numeric x.y.z only).
func SemverCompare(a, b string) int {
	av := parseSemverParts(normalizeSemver(a))
	bv := parseSemverParts(normalizeSemver(b))
	for i := 0; i < 3; i++ {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	return 0
}

func parseSemverParts(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		out[i] = n
	}
	return out
}

// CheckUpdateOnce performs a single online check (for `dogego version` without a datadir).
func CheckUpdateOnce(ctx context.Context) UpdateStatus {
	if UpdateCheckDisabled() {
		return UpdateStatus{CurrentVersion: Display(), BuildCommand: defaultBuildCommand()}
	}
	client := &http.Client{Timeout: updateHTTPTimeout}
	best, sources, err := fetchLatestRelease(ctx, client)
	st := UpdateStatus{
		CurrentVersion: Display(),
		BuildCommand:   defaultBuildCommand(),
		CheckedAt:      time.Now(),
		SourcesChecked: sources,
	}
	if err != nil {
		st.CheckError = err.Error()
		return st
	}
	st.LatestVersion = normalizeSemver(best.TagName)
	st.LatestTag = best.TagName
	st.Prerelease = best.Prerelease
	st.Source = best.Source
	st.ReleaseURL = best.ReleaseURL
	st.DownloadURL = best.AssetURL
	st.ChecksumURL = best.ChecksumURL
	st.ChecksumSHA256 = best.ChecksumSHA256
	st.DirectUpdate = best.AssetURL != ""
	st.Installable = st.LatestVersion != "" && st.DownloadURL != ""
	st.Instructions = buildUpdateInstructions(best)
	st.Available = st.LatestVersion != "" && SemverCompare(st.LatestVersion, normalizeSemver(ClientVersion)) > 0
	return st
}
