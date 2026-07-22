// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"image"
	"image/color"
)

func generateTestnetTrayImage() *image.RGBA {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	outer := color.RGBA{0x3d, 0x8b, 0xfd, 0xFF}
	inner := color.RGBA{0xFF, 0xD7, 0x00, 0xFF}
	cx, cy := size/2, size/2
	outerR, innerR := size/2-1, size/2-5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-cx, y-cy
			d2 := dx*dx + dy*dy
			switch {
			case d2 <= innerR*innerR:
				img.Set(x, y, inner)
			case d2 <= outerR*outerR:
				img.Set(x, y, outer)
			default:
				img.Set(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}
	return img
}

func generateTrayImage() *image.RGBA {
	// Fallback only when trayicon.png is missing from the embed.
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	outer := color.RGBA{0xC2, 0xA0, 0x33, 0xFF}
	inner := color.RGBA{0xFF, 0xD7, 0x00, 0xFF}
	cx, cy := size/2, size/2
	outerR, innerR := size/2-1, size/2-5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := x-cx, y-cy
			d2 := dx*dx + dy*dy
			switch {
			case d2 <= innerR*innerR:
				img.Set(x, y, inner)
			case d2 <= outerR*outerR:
				img.Set(x, y, outer)
			default:
				img.Set(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}
	return img
}
