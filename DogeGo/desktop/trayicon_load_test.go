// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import "testing"

func TestTrayIconPNGEmbedded(t *testing.T) {
	if len(trayIconPNG) < 256 {
		t.Fatalf("trayicon.png embed too small: %d bytes", len(trayIconPNG))
	}
	img, ok := decodeTrayPNG(trayIconPNG)
	if !ok {
		t.Fatal("decode trayicon.png failed")
	}
	b := img.Bounds()
	if b.Dx() < 16 || b.Dy() < 16 {
		t.Fatalf("unexpected tray icon size %dx%d", b.Dx(), b.Dy())
	}
}

func TestTrayIconTestnetPNGEmbedded(t *testing.T) {
	if len(trayIconTestnetPNG) < 256 {
		t.Fatalf("trayicon_testnet.png embed too small: %d bytes", len(trayIconTestnetPNG))
	}
	img, ok := decodeTrayPNG(trayIconTestnetPNG)
	if !ok {
		t.Fatal("decode trayicon_testnet.png failed")
	}
	b := img.Bounds()
	if b.Dx() < 16 || b.Dy() < 16 {
		t.Fatalf("unexpected testnet tray icon size %dx%d", b.Dx(), b.Dy())
	}
	if !isTestnetNetwork("testnet") || isTestnetNetwork("mainnet") {
		t.Fatal("isTestnetNetwork classification")
	}
}
