// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net/http"
)

// requireWalletRead restricts wallet balance/history reads to loopback clients.
// Remote dashboard PIN does not grant financial privacy over the LAN.
func requireWalletRead(w http.ResponseWriter, r *http.Request) bool {
	if isLoopback(r) {
		return true
	}
	http.Error(w, "forbidden", http.StatusForbidden)
	return false
}
