// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWriteCookieAuthAndBasic(t *testing.T) {
	dir := t.TempDir()
	auth, p, err := WriteCookieAuth(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, ".cookie") {
		t.Fatalf("path %q", p)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != auth.User+":"+auth.Password {
		t.Fatalf("file mismatch")
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, auth))
	defer srv.Close()
	body := []byte(`{"method":"ping","id":1}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth.User+":"+auth.Password)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
}
