// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// ReadUtxoSnapshotDiskTip reads only the chainActive height from utxo.cache (no coin map load).
// Returns -1, nil when the file is absent.
func ReadUtxoSnapshotDiskTip(path string) (int64, error) {
	tip, _, err := ReadUtxoSnapshotDiskMeta(path)
	return tip, err
}

// ReadUtxoSnapshotDiskMeta returns tip height and file mod time from utxo.cache.
// tip is -1 when the file is absent.
func ReadUtxoSnapshotDiskMeta(path string) (tip int64, modUnix int64, err error) {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, 0, nil
		}
		return 0, 0, err
	}
	modUnix = st.ModTime().Unix()
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	tip, err = readUtxoSnapshotTipOnly(f)
	if err != nil {
		return 0, 0, err
	}
	return tip, modUnix, nil
}

// QuarantineUtxoSnapshot renames a misaligned utxo.cache so startup will not reload it.
func QuarantineUtxoSnapshot(path, reason string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := path + ".stale"
	if reason != "" {
		dst = path + ".stale." + reason
	}
	if _, err := os.Stat(dst); err == nil {
		dst = path + ".stale." + strconv.FormatInt(time.Now().Unix(), 10)
	}
	return os.Rename(path, dst)
}

func readUtxoSnapshotTipOnly(r io.Reader) (int64, error) {
	tip, _, err := readUtxoSnapshotTipAndCount(r)
	return tip, err
}

// ReadUtxoSnapshotTipAndCount reads tip height and coin count without loading the coin map.
func ReadUtxoSnapshotTipAndCount(path string) (tip int64, coinCount uint64, err error) {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, 0, nil
		}
		return 0, 0, err
	}
	_ = st
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	return readUtxoSnapshotTipAndCount(f)
}

func readUtxoSnapshotTipAndCount(r io.Reader) (tip int64, coinCount uint64, err error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return 0, 0, err
	}
	if string(magic[:]) != utxoSnapshotMagic {
		return 0, 0, fmt.Errorf("utxo snapshot: bad magic")
	}
	var ver uint32
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return 0, 0, err
	}
	if ver != utxoSnapshotVersion {
		return 0, 0, fmt.Errorf("utxo snapshot: unsupported version %d", ver)
	}
	if err := binary.Read(r, binary.LittleEndian, &tip); err != nil {
		return 0, 0, err
	}
	if err := binary.Read(r, binary.LittleEndian, &coinCount); err != nil {
		return 0, 0, err
	}
	return tip, coinCount, nil
}
