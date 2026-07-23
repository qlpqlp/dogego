// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"os"
	"testing"
)

func TestDisableLocalTLS(t *testing.T) {
	f := File{
		WebUITLSLocal:   true,
		RpcTLSLocal:     true,
		LocalTLSTrustCA: true,
		WebUITLSCert:    "a.crt",
		WebUITLSKey:     "a.key",
	}
	DisableLocalTLS(&f)
	if !f.NoTLS || f.WebUITLSLocal || f.RpcTLSLocal || f.LocalTLSTrustCA {
		t.Fatalf("DisableLocalTLS incomplete: %+v", f)
	}
	if f.WebUITLSCert != "" || f.WebUITLSKey != "" {
		t.Fatal("want PEM paths cleared")
	}
}

func TestSetupWizardSeed_noTLS(t *testing.T) {
	f := SetupWizardSeed(File{NodeMode: "full", Network: "testnet", NoTLS: true})
	if f.WebUITLSLocal || f.LocalTLSTrustCA {
		t.Fatal("NoTLS seed must not enable local HTTPS defaults")
	}
	if !f.NoTLS {
		t.Fatal("want NoTLS preserved")
	}
}

func TestEnvNoTLS(t *testing.T) {
	t.Setenv("DOGEGO_NO_TLS", "")
	t.Setenv("DOGEGO_NOTLS", "")
	if EnvNoTLS() {
		t.Fatal("empty env should be false")
	}
	t.Setenv("DOGEGO_NO_TLS", "1")
	if !EnvNoTLS() {
		t.Fatal("want DOGEGO_NO_TLS=1")
	}
	t.Setenv("DOGEGO_NO_TLS", "")
	t.Setenv("DOGEGO_NOTLS", "true")
	if !EnvNoTLS() {
		t.Fatal("want DOGEGO_NOTLS=true")
	}
	_ = os.Unsetenv("DOGEGO_NOTLS")
}
