// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"strings"
)

// trayIconImage returns the embedded official Dogecoin tray bitmap (32×32 PNG),
// or a simple generated fallback when the asset is missing.
func trayIconImage() *image.RGBA {
	if img, ok := decodeTrayPNG(trayIconPNG); ok {
		return img
	}
	return generateTrayImage()
}

func decodeTrayPNG(data []byte) (*image.RGBA, bool) {
	if len(data) < 8 {
		return nil, false
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	draw.Draw(out, bounds, img, bounds.Min, draw.Src)
	return out, true
}

func trayIconPNGBytes() []byte {
	if len(trayIconPNG) >= 8 {
		return trayIconPNG
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, generateTrayImage())
	return buf.Bytes()
}

func trayIconImageForNetwork(network string) *image.RGBA {
	if isTestnetNetwork(network) {
		if img, ok := decodeTrayPNG(trayIconTestnetPNG); ok {
			return img
		}
		return generateTestnetTrayImage()
	}
	return trayIconImage()
}

func trayIconBytesForNetwork(network string) []byte {
	return trayIconBytesFromImage(trayIconImageForNetwork(network))
}

func isTestnetNetwork(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "testnet", "reboottestnet":
		return true
	default:
		return false
	}
}
