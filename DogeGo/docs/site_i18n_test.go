// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestMarketingSiteLocaleKeyParity ensures docs/locales/*.json match en.json key paths (dogego.org i18n).
func TestMarketingSiteLocaleKeyParity(t *testing.T) {
	root := gitRepoRoot(t)
	locDir := filepath.Join(root, "docs", "locales")
	enKeys, err := localeKeyPaths(filepath.Join(locDir, "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"fr", "pt-PT", "de", "zh", "ja"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			got, err := localeKeyPaths(filepath.Join(locDir, lang+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if missing := diffKeySets(enKeys, got); len(missing) > 0 {
				sort.Strings(missing)
				t.Fatalf("missing %d keys vs en.json: %v", len(missing), missing[:min(10, len(missing))])
			}
			if extra := diffKeySets(got, enKeys); len(extra) > 0 {
				sort.Strings(extra)
				t.Fatalf("extra %d keys vs en.json: %v", len(extra), extra[:min(10, len(extra))])
			}
		})
	}
}

func TestMarketingSiteProtocolLockCopy(t *testing.T) {
	root := gitRepoRoot(t)
	locDir := filepath.Join(root, "docs", "locales")
	needles := map[string][]string{
		"en":    {"no protocol fork", "mainnet consensus follows"},
		"de":    {"keine protokoll-forks", "mainnet-konsens folgt"},
		"fr":    {"pas de forks de protocole", "consensus mainnet suit"},
		"pt-PT": {"sem forks de protocolo", "consenso mainnet segue"},
		"ja":    {"プロトコルフォークなし", "メインネットのコンセンサス"},
		"zh":    {"无协议分叉", "主网共识遵循"},
	}
	for lang, want := range needles {
		lang, want := lang, want
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join(locDir, lang+".json"))
			if err != nil {
				t.Fatal(err)
			}
			text := strings.ToLower(string(raw))
			for _, needle := range want {
				if !strings.Contains(text, strings.ToLower(needle)) {
					t.Fatalf("docs/locales/%s.json missing %q", lang, needle)
				}
			}
		})
	}
}

func localeKeyPaths(path string) (map[string]struct{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	collectLocaleKeys("", root, out)
	return out, nil
}

func collectLocaleKeys(prefix string, v interface{}, out map[string]struct{}) {
	switch node := v.(type) {
	case map[string]interface{}:
		for k, child := range node {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			collectLocaleKeys(key, child, out)
		}
	case []interface{}:
		for i, child := range node {
			key := fmt.Sprintf("%s[%d]", prefix, i)
			collectLocaleKeys(key, child, out)
		}
	default:
		if prefix != "" {
			out[prefix] = struct{}{}
		}
	}
}

func diffKeySets(a, b map[string]struct{}) []string {
	var missing []string
	for k := range a {
		if _, ok := b[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func gitRepoRoot(t *testing.T) string {
	t.Helper()
	dir := repoRoot(t)
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs", "locales", "en.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docs/locales/en.json not found above module root")
		}
		dir = parent
	}
}
