// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestUICoreRunnerLocaleKeys(t *testing.T) {
	root := moduleRoot(t)
	locDir := filepath.Join(root, "ui", "static", "locales")
	required := []string{
		"pages.features.coreRunnerTitle",
		"pages.features.coreRunnerHint",
		"pages.features.coreRunnerRefresh",
		"pages.features.coreRunnerOk",
		"pages.features.coreRunnerWarn",
		"pages.features.coreRunnerFail",
		"pages.features.coreProbeBip152",
		"pages.features.coreBip152Title",
		"pages.features.coreBip152Hint",
		"pages.features.coreBip152Refresh",
		"pages.features.coreBip152Ok",
		"pages.features.coreBip152Warn",
		"pages.features.coreBip152Fail",
		"pages.features.coreBip152Skipped",
		"pages.receive.abTypeKeypool",
		"nav.extensions",
		"pages.extensions.title",
		"pages.extensions.backCatalog",
		"settings.extensionsTitle",
		"settings.extensionsHint",
		"settings.extensionsDetails",
	}
	enKeys, err := uiLocaleKeyPaths(filepath.Join(locDir, "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range required {
		if _, ok := enKeys[key]; !ok {
			t.Fatalf("en.json missing %q", key)
		}
	}
	for _, lang := range []string{"fr", "pt-PT", "de", "zh", "ja"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			got, err := uiLocaleKeyPaths(filepath.Join(locDir, lang+".json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range required {
				if _, ok := got[key]; !ok {
					t.Fatalf("%s missing %q", lang, key)
				}
			}
		})
	}
}

func uiLocaleKeyPaths(path string) (map[string]struct{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	collectUILocaleKeys("", root, out)
	return out, nil
}

func collectUILocaleKeys(prefix string, v any, out map[string]struct{}) {
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			collectUILocaleKeys(key, child, out)
		}
	case []any:
		for i, child := range node {
			collectUILocaleKeys(fmt.Sprintf("%s[%d]", prefix, i), child, out)
		}
	default:
		if prefix != "" {
			out[prefix] = struct{}{}
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
