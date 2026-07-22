// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestQRCodeJSEmbedded(t *testing.T) {
	data, err := fs.ReadFile(static, "static/qrcode.min.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 1000 {
		t.Fatal("qrcode.min.js missing or too small")
	}
	if !strings.Contains(string(data), "toCanvas") {
		t.Fatal("qrcode.min.js missing toCanvas export")
	}
}

func TestBlockstepJSEmbedded(t *testing.T) {
	data, err := fs.ReadFile(static, "static/blockstep.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 100 {
		t.Fatal("blockstep.js missing or too small")
	}
	if !strings.Contains(string(data), "DogeGoBlockStep") {
		t.Fatal("blockstep.js does not export DogeGoBlockStep")
	}
	if !strings.Contains(string(data), "getElementById") {
		t.Fatal("blockstep.js missing DOM $ helper")
	}
}

func TestI18nEmbedded(t *testing.T) {
	data, err := fs.ReadFile(static, "static/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "DogeGoI18n") {
		t.Fatal("i18n.js missing DogeGoI18n export")
	}
	for _, lang := range []string{"en", "fr", "pt-PT", "de", "zh", "ja"} {
		path := "static/locales/" + lang + ".json"
		loc, err := fs.ReadFile(static, path)
		if err != nil {
			t.Fatalf("locale %s: %v", lang, err)
		}
		if !strings.Contains(string(loc), `"meta"`) {
			t.Fatalf("locale %s missing meta", lang)
		}
	}
}

func TestNumFormatJSEmbedded(t *testing.T) {
	data, err := fs.ReadFile(static, "static/num-format.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "DogeGoFormat") {
		t.Fatal("num-format.js missing DogeGoFormat export")
	}
	if !strings.Contains(string(data), "formatCompactNumber") {
		t.Fatal("num-format.js missing formatCompactNumber")
	}
	index, err := fs.ReadFile(static, "static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "/num-format.js") {
		t.Fatal("index.html does not load num-format.js")
	}
}
