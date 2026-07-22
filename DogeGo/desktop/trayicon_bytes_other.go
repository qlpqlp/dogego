//go:build !windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"bytes"
	"image"
	"image/png"
)

func trayIconBytes() []byte {
	return trayIconBytesFromImage(trayIconImage())
}

func trayIconBytesFromImage(img *image.RGBA) []byte {
	if img == nil {
		return trayIconPNGBytes()
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
