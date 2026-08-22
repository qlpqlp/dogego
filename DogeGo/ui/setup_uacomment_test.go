// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestApplySetupUACommentTipBeforeEncrypt(t *testing.T) {
	dir := t.TempDir()
	if _, err := ensureSetupWallet(dir, "testnet"); err != nil {
		t.Fatal(err)
	}
	on := true
	f := config.File{
		DataDir:  dir,
		Network:  "testnet",
		UACommentUseNodeTip: &on,
	}
	if err := applySetupUACommentTip(&f, ""); err != nil {
		t.Fatalf("apply before encrypt: %v", err)
	}
	if f.UACommentTipAddress == "" {
		t.Fatal("expected tip address in config")
	}
	pass := "wizard-uacomment-pass"
	if err := encryptSetupWallet(dir, "testnet", pass); err != nil {
		t.Fatal(err)
	}
	// Retry path: encryption is idempotent after a partial wizard failure.
	if err := encryptSetupWallet(dir, "testnet", pass); err != nil {
		t.Fatalf("encrypt retry: %v", err)
	}
}

func TestApplySetupUACommentTipAfterEncryptFails(t *testing.T) {
	dir := t.TempDir()
	if _, err := ensureSetupWallet(dir, "testnet"); err != nil {
		t.Fatal(err)
	}
	if err := encryptSetupWallet(dir, "testnet", "test-pass"); err != nil {
		t.Fatal(err)
	}
	on := true
	f := config.File{
		DataDir:             dir,
		Network:             "testnet",
		UACommentUseNodeTip: &on,
	}
	if err := applySetupUACommentTip(&f, ""); err == nil {
		t.Fatal("expected error applying node tip after wallet encryption without unlock")
	}
	if err := applySetupUACommentTip(&f, "test-pass"); err != nil {
		t.Fatalf("apply with passphrase: %v", err)
	}
	if f.UACommentTipAddress == "" {
		t.Fatal("expected tip after unlock")
	}
}

func TestResolveUACommentTipForConfig_encryptedWallet(t *testing.T) {
	dir := t.TempDir()
	if _, err := ensureSetupWallet(dir, "testnet"); err != nil {
		t.Fatal(err)
	}
	if err := encryptSetupWallet(dir, "testnet", "test-pass"); err != nil {
		t.Fatal(err)
	}
	on := true
	existing := config.File{UACommentTipAddress: "DTipExisting"}
	f := config.File{
		DataDir:             dir,
		Network:             "testnet",
		UACommentUseNodeTip: &on,
	}
	warn, err := resolveUACommentTipForConfig(&f, existing)
	if err != nil {
		t.Fatalf("save should not fail: %v", err)
	}
	if f.UACommentTipAddress != "DTipExisting" {
		t.Fatalf("tip address: %q", f.UACommentTipAddress)
	}
	if warn == "" {
		t.Fatal("expected warning when wallet locked")
	}
}
