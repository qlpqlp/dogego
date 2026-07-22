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
	"os/exec"
	"strings"

	"dogego/chain"
	"dogego/pow"
)

// tryReadMainnetCheckpointHeaderFromCoreCLI fetches header80 via dogecoin-cli when
// DOGEGO_ENRICH_CHECKPOINT_RPC=1 (export_mainnet_field_headers.ps1 -CoreRpc).
func tryReadMainnetCheckpointHeaderFromCoreCLI(height int64) (string, bool) {
	if os.Getenv("DOGEGO_ENRICH_CHECKPOINT_RPC") != "1" || height <= 0 {
		return "", false
	}
	want, ok := chain.CheckpointHashAt(chain.MainnetDogecoin, height)
	if !ok {
		return "", false
	}
	cli := strings.TrimSpace(os.Getenv("DOGEGO_CORE_CLI"))
	if cli == "" {
		cli = "dogecoin-cli"
	}
	hashOut, err := exec.Command(cli, "getblockhash", fmt.Sprint(height)).Output()
	if err != nil {
		return "", false
	}
	blockHash := strings.Trim(strings.TrimSpace(string(hashOut)), `"`)
	hdrOut, err := exec.Command(cli, "getblockheader", blockHash, "false").Output()
	if err != nil {
		return "", false
	}
	hx := strings.Trim(strings.TrimSpace(string(hdrOut)), `"`)
	b, err := hex.DecodeString(hx)
	if err != nil || len(b) != 80 {
		return "", false
	}
	got := strings.ToLower(pow.BlockHashHex(b))
	want = strings.ToLower(strings.TrimPrefix(want, "0x"))
	if got != want {
		return "", false
	}
	return strings.ToUpper(hx), true
}
