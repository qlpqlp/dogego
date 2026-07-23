// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"os"
	"strings"
)

// ApplyRecommendedSecurityDefaults sets HTTPS and local CA trust for new wizard installs.
// Skipped when NoTLS is set (e.g. -notls / DOGEGO_NO_TLS for DogeBox and plain-HTTP hosts).
func ApplyRecommendedSecurityDefaults(f *File) {
	if f == nil || f.NoTLS {
		return
	}
	f.WebUITLSLocal = true
	f.LocalTLSTrustCA = true
}

// DisableLocalTLS turns off auto-generated HTTPS and OS CA trust for the web UI and RPC.
// Explicit PEM paths are also cleared so listeners stay plain HTTP.
func DisableLocalTLS(f *File) {
	if f == nil {
		return
	}
	f.NoTLS = true
	f.WebUITLSLocal = false
	f.RpcTLSLocal = false
	f.LocalTLSTrustCA = false
	f.WebUITLSCert = ""
	f.WebUITLSKey = ""
	f.RpcTLSCert = ""
	f.RpcTLSKey = ""
}

// ApplyNoTLSMerged clears TLS fields on a Merged node config.
func ApplyNoTLSMerged(m *Merged) {
	if m == nil {
		return
	}
	m.WebUITLSLocal = false
	m.RpcTLSLocal = false
	m.LocalTLSTrustCA = false
	m.WebUITLSCert = ""
	m.WebUITLSKey = ""
	m.RpcTLSCert = ""
	m.RpcTLSKey = ""
}

// EnvNoTLS reports DOGEGO_NO_TLS / DOGEGO_NOTLS truthy (1, true, yes, on).
func EnvNoTLS() bool {
	for _, k := range []string{"DOGEGO_NO_TLS", "DOGEGO_NOTLS"} {
		v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
		switch v {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}
