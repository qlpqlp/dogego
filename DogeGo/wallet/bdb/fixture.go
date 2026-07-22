// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bdb

import (
	"encoding/binary"
	"fmt"
	"os"
)

// WriteFixtureWallet writes a minimal Core-style Berkeley DB wallet.dat containing kv pairs.
// Used by tests and offline migration certification fixtures (not for production wallets).
func WriteFixtureWallet(path string, kv map[string][]byte) error {
	const pageSize = 512
	const (
		outerMetaPageNum = 0
		outerRootPage    = 1
		innerMetaPageNum = 2
		dataLeafPage     = 3
		lastPage         = 3
	)
	pages := make([][]byte, lastPage+1)
	for i := range pages {
		pages[i] = make([]byte, pageSize)
	}
	writeMetaPage(pages[outerMetaPageNum], outerRootPage, lastPage)
	writeMetaPage(pages[innerMetaPageNum], dataLeafPage, lastPage)

	mainRef := make([]byte, 4)
	binary.BigEndian.PutUint32(mainRef, innerMetaPageNum)
	pages[outerRootPage] = writeLeafPage([][2][]byte{
		{[]byte(subDBName), mainRef},
	})

	var pairs [][2][]byte
	for k, v := range kv {
		pairs = append(pairs, [2][]byte{[]byte(k), v})
	}
	pages[dataLeafPage] = writeLeafPage(pairs)

	var file []byte
	for _, p := range pages {
		file = append(file, p...)
	}
	if err := os.WriteFile(path, file, 0o600); err != nil {
		return err
	}
	return nil
}

func writeMetaPage(page []byte, rootPage, lastPage int) {
	binary.LittleEndian.PutUint32(page[12:16], btreeMagic)
	binary.LittleEndian.PutUint32(page[16:20], dbVersion)
	binary.LittleEndian.PutUint32(page[20:24], 512)
	page[28] = btreeMeta
	binary.LittleEndian.PutUint32(page[36:40], uint32(lastPage))
	binary.LittleEndian.PutUint32(page[88:92], uint32(rootPage))
}

func writeLeafPage(pairs [][2][]byte) []byte {
	page := make([]byte, 512)
	page[25] = btreeLeaf

	var blobs [][]byte
	for _, pair := range pairs {
		blobs = append(blobs, pair[0], pair[1])
	}
	pos := 512
	offsets := make([]uint16, len(blobs))
	for i := len(blobs) - 1; i >= 0; i-- {
		data := blobs[i]
		size := 3 + len(data)
		pos -= size
		if pos < pageHeaderSize+len(blobs)*2 {
			panic("bdb fixture: page overflow")
		}
		binary.LittleEndian.PutUint16(page[pos:pos+2], uint16(len(data)))
		page[pos+2] = recordKeyData
		copy(page[pos+3:], data)
		offsets[i] = uint16(pos)
	}
	binary.LittleEndian.PutUint16(page[16:18], uint16(len(blobs)))
	for i, off := range offsets {
		binary.LittleEndian.PutUint16(page[pageHeaderSize+i*2:], off)
	}
	return page
}

// FixtureRoundTrip checks that OpenKV returns the same pairs written by WriteFixtureWallet.
func FixtureRoundTrip(path string, kv map[string][]byte) error {
	if err := WriteFixtureWallet(path, kv); err != nil {
		return err
	}
	got, err := OpenKV(path)
	if err != nil {
		return err
	}
	if len(got) != len(kv) {
		return fmt.Errorf("kv count got=%d want=%d", len(got), len(kv))
	}
	for k, want := range kv {
		g, ok := got[k]
		if !ok {
			return fmt.Errorf("missing key %q", k)
		}
		if string(g) != string(want) {
			return fmt.Errorf("value mismatch for %q", k)
		}
	}
	return nil
}
