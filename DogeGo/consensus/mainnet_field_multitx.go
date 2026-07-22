// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dogego/pow"
)

const (
	mainnetFieldMultiTxBlockHeight   int64 = 15504
	mainnetFieldMultiTxBlockWantHash       = "8a32f333c1d48ff790b34a7e1e0b64924a2fd77abc25c1c112746e53e16e1ed7"
)

func mainnetFieldMultiTxBlock15504HexPath() string {
	return filepath.Join("testdata", "mainnet_field_block_15504.hex")
}

// loadMainnetFieldMultiTxBlock15504Hex returns committed mainnet block 15504 raw hex
// (Milestone A multi-tx field evidence; exported from Core chain via Blockchair).
func loadMainnetFieldMultiTxBlock15504Hex() (string, error) {
	raw, err := os.ReadFile(mainnetFieldMultiTxBlock15504HexPath())
	if err != nil {
		return "", err
	}
	hexU := strings.ToUpper(strings.TrimSpace(string(raw)))
	if hexU == "" {
		return "", fmt.Errorf("empty %s", mainnetFieldMultiTxBlock15504HexPath())
	}
	decoded, err := hex.DecodeString(hexU)
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", mainnetFieldMultiTxBlock15504HexPath(), err)
	}
	if len(decoded) < 80 {
		return "", fmt.Errorf("%s too short: %d bytes", mainnetFieldMultiTxBlock15504HexPath(), len(decoded))
	}
	got := pow.BlockHashHex(decoded[:80])
	if got != mainnetFieldMultiTxBlockWantHash {
		return "", fmt.Errorf("height %d hash %s want %s", mainnetFieldMultiTxBlockHeight, got, mainnetFieldMultiTxBlockWantHash)
	}
	return hexU, nil
}

func mainnetFieldMultiTxBlock15504Entry() (mainnetFieldBlockEntry, error) {
	hexU, err := loadMainnetFieldMultiTxBlock15504Hex()
	if err != nil {
		return mainnetFieldBlockEntry{}, err
	}
	return mainnetFieldBlockEntry{
		Height: mainnetFieldMultiTxBlockHeight,
		Hex:    hexU,
	}, nil
}
