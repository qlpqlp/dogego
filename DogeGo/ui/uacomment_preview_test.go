// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
)

func TestBuildUACommentPreviewCommentOnly(t *testing.T) {
	out, err := buildUACommentPreview(uacommentPreviewRequest{
		UAComment: "much node",
		Network:   "testnet",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["effective_uacomment"] != "much node" {
		t.Fatalf("comment %#v", out["effective_uacomment"])
	}
	sub, _ := out["subversion"].(string)
	if !strings.Contains(sub, "much node") {
		t.Fatalf("subversion %q", sub)
	}
}

func TestBuildUACommentPreviewNodeTip(t *testing.T) {
	dir := t.TempDir()
	if _, err := ensureSetupWallet(dir, "testnet"); err != nil {
		t.Fatal(err)
	}
	use := true
	out, err := buildUACommentPreview(uacommentPreviewRequest{
		UAComment:           "wow",
		PublishTip:          true,
		UACommentUseNodeTip: &use,
		DataDir:             dir,
		Network:             "testnet",
	})
	if err != nil {
		t.Fatal(err)
	}
	tip, _ := out["tip_address"].(string)
	if tip == "" {
		t.Fatal("expected tip address")
	}
	eff, _ := out["effective_uacomment"].(string)
	if !strings.Contains(eff, tip) {
		t.Fatalf("effective %q tip %q", eff, tip)
	}
}

func TestUACommentPreviewHTTPEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	registerUACommentPreview(mux)
	dir := t.TempDir()
	body, _ := json.Marshal(map[string]any{
		"uacomment":   "lab",
		"publish_tip": false,
		"datadir":     dir,
		"network":     "testnet",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/config/uacomment-preview", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	want := chain.BuildSubVersion("lab")
	if out["subversion"] != want {
		t.Fatalf("got %v want %q", out["subversion"], want)
	}
}

func TestBuildUACommentPreviewRequiresWalletForNodeTip(t *testing.T) {
	use := true
	_, err := buildUACommentPreview(uacommentPreviewRequest{
		PublishTip:          true,
		UACommentUseNodeTip: &use,
		DataDir:             filepath.Join(t.TempDir()),
		Network:             "testnet",
		NoWallet:            true,
	})
	if err == nil || !strings.Contains(err.Error(), "wallet") {
		t.Fatalf("err %v", err)
	}
}
