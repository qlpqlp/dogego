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

const (
	pageHeaderSize = 26
	outerMetaPage  = 0

	btreeInternal = 3
	btreeLeaf     = 5
	overflowData  = 7
	btreeMeta     = 9

	recordKeyData       = 1
	recordOverflowData  = 3
	btreeMagic   uint32 = 0x00053162
	dbVersion    uint32 = 9
	subDBName           = "main"
)

// OpenKV reads all key-value pairs from a Core-style Berkeley DB btree wallet.dat (read-only).
func OpenKV(path string) (map[string][]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < pageHeaderSize {
		return nil, fmt.Errorf("bdb: file too small")
	}
	pageSize := binary.LittleEndian.Uint32(raw[20:24])
	if pageSize < 512 {
		return nil, fmt.Errorf("bdb: invalid page size %d", pageSize)
	}
	if len(raw)%int(pageSize) != 0 {
		return nil, fmt.Errorf("bdb: file size not multiple of page size")
	}
	pages := make([][]byte, 0, len(raw)/int(pageSize))
	for off := 0; off < len(raw); off += int(pageSize) {
		pages = append(pages, raw[off:off+int(pageSize)])
	}
	outerMeta, err := parseMetaPage(pages[outerMetaPage])
	if err != nil {
		return nil, err
	}
	root := dumpPage(pages[outerMeta.root])
	if root.pgType != btreeLeaf || len(root.entries) != 2 {
		return nil, fmt.Errorf("bdb: unexpected outer root")
	}
	if string(root.entries[0].data) != subDBName {
		return nil, fmt.Errorf("bdb: missing main subdatabase")
	}
	if len(root.entries[1].data) != 4 {
		return nil, fmt.Errorf("bdb: bad inner meta page ref")
	}
	innerMetaPage := int(binary.BigEndian.Uint32(root.entries[1].data))
	if innerMetaPage >= len(pages) {
		return nil, fmt.Errorf("bdb: inner meta page out of range")
	}
	innerMeta, err := parseMetaPage(pages[innerMetaPage])
	if err != nil {
		return nil, err
	}
	kv := make(map[string][]byte)
	queue := []int{innerMeta.root}
	for len(queue) > 0 {
		curr := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if curr < 0 || curr >= len(pages) {
			return nil, fmt.Errorf("bdb: page %d out of range", curr)
		}
		info := dumpPage(pages[curr])
		switch info.pgType {
		case btreeInternal:
			for _, e := range info.entries {
				queue = append(queue, e.pageNum)
			}
		case btreeLeaf:
			pairs, err := extractKVPairs(info, pages)
			if err != nil {
				return nil, err
			}
			for k, v := range pairs {
				kv[k] = v
			}
		default:
			return nil, fmt.Errorf("bdb: unexpected page type %d at %d", info.pgType, curr)
		}
	}
	return kv, nil
}

type pageEntry struct {
	recordType uint8
	data       []byte
	pageNum    int
}

type pageInfo struct {
	pgType  uint8
	entries []pageEntry
}

type metaInfo struct {
	root     int
	lastPgno int
}

func parseMetaPage(page []byte) (*metaInfo, error) {
	if len(page) < 512 {
		return nil, fmt.Errorf("bdb: meta page too short")
	}
	magic := binary.LittleEndian.Uint32(page[12:16])
	version := binary.LittleEndian.Uint32(page[16:20])
	pgType := page[28]
	if magic != btreeMagic {
		return nil, fmt.Errorf("bdb: bad magic 0x%x", magic)
	}
	if pgType != btreeMeta {
		return nil, fmt.Errorf("bdb: not a btree meta page")
	}
	if version != dbVersion {
		return nil, fmt.Errorf("bdb: unsupported db version %d", version)
	}
	lastPgno := int(binary.LittleEndian.Uint32(page[36:40]))
	root := int(binary.LittleEndian.Uint32(page[88:92]))
	return &metaInfo{root: root, lastPgno: lastPgno}, nil
}

func dumpPage(data []byte) pageInfo {
	hdr := data[0:pageHeaderSize]
	entries := binary.LittleEndian.Uint16(hdr[16:18])
	hfOffset := binary.LittleEndian.Uint16(hdr[20:22])
	pgType := hdr[25]
	out := pageInfo{pgType: pgType}
	if pgType == overflowData {
		out.entries = append(out.entries, pageEntry{data: data[pageHeaderSize : pageHeaderSize+int(hfOffset)]})
		return out
	}
	offsets := make([]uint16, entries)
	for i := 0; i < int(entries); i++ {
		off := pageHeaderSize + i*2
		offsets[i] = binary.LittleEndian.Uint16(data[off : off+2])
	}
	for i := 0; i < int(entries); i++ {
		off := int(offsets[i])
		eLen := binary.LittleEndian.Uint16(data[off : off+2])
		recType := data[off+2]
		off += 3
		var e pageEntry
		e.recordType = recType
		switch pgType {
		case btreeInternal:
			if recType != recordKeyData {
				continue
			}
			e.pageNum = int(binary.BigEndian.Uint32(data[off+1 : off+5]))
		case btreeLeaf:
			switch recType {
			case recordKeyData:
				e.data = append([]byte(nil), data[off:off+int(eLen)]...)
			case recordOverflowData:
				e.pageNum = int(binary.BigEndian.Uint32(data[off+1 : off+5]))
			}
		}
		out.entries = append(out.entries, e)
	}
	return out
}

func extractKVPairs(leaf pageInfo, pages [][]byte) (map[string][]byte, error) {
	out := make(map[string][]byte)
	var lastKey string
	for i, entry := range leaf.entries {
		data, err := entryPayload(entry, pages)
		if err != nil {
			return nil, err
		}
		if i%2 == 0 {
			lastKey = string(data)
			out[lastKey] = nil
		} else {
			out[lastKey] = data
		}
	}
	return out, nil
}

func entryPayload(entry pageEntry, pages [][]byte) ([]byte, error) {
	if entry.recordType == recordKeyData {
		return entry.data, nil
	}
	if entry.recordType != recordOverflowData {
		return nil, fmt.Errorf("bdb: unknown record type %d", entry.recordType)
	}
	var buf []byte
	next := entry.pageNum
	for next != 0 {
		if next >= len(pages) {
			return nil, fmt.Errorf("bdb: overflow page %d out of range", next)
		}
		op := dumpPage(pages[next])
		if len(op.entries) != 1 {
			return nil, fmt.Errorf("bdb: bad overflow page")
		}
		buf = append(buf, op.entries[0].data...)
		next = int(binary.LittleEndian.Uint32(pages[next][12:16]))
	}
	return buf, nil
}

// IsBDBFile reports whether path looks like a Berkeley DB btree wallet file (Core wallet.dat).
func IsBDBFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 16)
	n, _ := f.Read(buf)
	if n < 16 {
		return false
	}
	return binary.LittleEndian.Uint32(buf[12:16]) == btreeMagic
}
