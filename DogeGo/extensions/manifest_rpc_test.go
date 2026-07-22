// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import "testing"

func TestManifestDeclaredRPC(t *testing.T) {
	m := Manifest{
		ManifestVersion: 1,
		ID:              "example.go",
		Name:            "Hello",
		Version:         "0.2.0",
		Entry:           Entry{Type: EntrySubprocess, Binary: "hello-ext"},
		RPC: []RPCMethod{
			{Name: "greet", Help: "say hi"},
		},
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	methods := m.AdvertisedRPCMethods()
	if len(methods) != 1 || methods[0].Name != "greet" {
		t.Fatalf("advertised %#v", methods)
	}
	if got := FullRPCName(m.ID, "greet"); got != "dogego_ext_example_go_greet" {
		t.Fatalf("full name %q", got)
	}
}

func TestManifestRejectsReservedRPCName(t *testing.T) {
	m := Manifest{
		ManifestVersion: 1,
		ID:              "example.bad",
		Name:            "Bad",
		Version:         "0.0.1",
		Entry:           Entry{Type: EntrySubprocess, Binary: "x"},
		RPC:             []RPCMethod{{Name: "dogego_on_enable"}},
	}
	if err := ValidateManifest(m); err == nil {
		t.Fatal("expected reserved rpc name reject")
	}
}
