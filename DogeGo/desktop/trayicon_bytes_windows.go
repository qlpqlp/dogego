//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"bytes"
	"encoding/binary"
	"image"
)

// fyne.io/systray on Windows uses LoadImage(IMAGE_ICON), which requires ICO bytes (not PNG).
func trayIconBytes() []byte {
	return trayIconBytesFromImage(trayIconImage())
}

func trayIconBytesFromImage(img *image.RGBA) []byte {
	if img == nil {
		return encodeICO(trayIconImage())
	}
	return encodeICO(img)
}

func encodeICO(img *image.RGBA) []byte {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 || w > 256 || h > 256 {
		return nil
	}
	bmpHeaderSize := 40
	pixelSize := w * h * 4
	andMaskRow := ((w + 31) / 32) * 4
	andMaskSize := andMaskRow * h
	imageSize := uint32(bmpHeaderSize + pixelSize + andMaskSize)
	offset := uint32(6 + 16)

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))

	widthByte := byte(w)
	heightByte := byte(h)
	if w == 256 {
		widthByte = 0
	}
	if h == 256 {
		heightByte = 0
	}
	buf.WriteByte(widthByte)
	buf.WriteByte(heightByte)
	buf.WriteByte(0)
	buf.WriteByte(0)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(&buf, binary.LittleEndian, imageSize)
	_ = binary.Write(&buf, binary.LittleEndian, offset)

	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, int32(w))
	_ = binary.Write(&buf, binary.LittleEndian, int32(h*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(uint32(pixelSize)))
	for i := 0; i < 6; i++ {
		_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	}

	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			buf.WriteByte(byte(b >> 8))
			buf.WriteByte(byte(g >> 8))
			buf.WriteByte(byte(r >> 8))
			buf.WriteByte(byte(a >> 8))
		}
	}
	andMask := make([]byte, andMaskSize)
	_, _ = buf.Write(andMask)
	return buf.Bytes()
}
