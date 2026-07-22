// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	walletTxDefaultLimit = 40
	walletTxMaxLimit     = 200
)

func parseWalletTxListQuery(r *http.Request) (offset, limit int, q, kind string) {
	q = strings.TrimSpace(r.URL.Query().Get("q"))
	kind = strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind == "" {
		kind = strings.TrimSpace(r.URL.Query().Get("type"))
	}
	if kind == "" {
		kind = "all"
	}
	limit = walletTxDefaultLimit
	if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	if limit < 0 {
		limit = 0
	}
	if limit > walletTxMaxLimit {
		limit = walletTxMaxLimit
	}
	if s := strings.TrimSpace(r.URL.Query().Get("offset")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}
	return offset, limit, q, kind
}
