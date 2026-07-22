// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// execBackupWallet copies wallet.json to destination (built-in wallet only).
func execBackupWallet(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var dest string
	if err := json.Unmarshal(params[0], &dest); err != nil {
		return nil, -8, "backupwallet: destination must be a string"
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return nil, -8, "backupwallet: invalid destination"
	}
	if paths == nil || paths.WalletPath == nil {
		return nil, -1, "backupwallet: wallet is not implemented in DogeGo"
	}
	src := strings.TrimSpace(paths.WalletPath())
	if src == "" {
		return nil, -1, "backupwallet: wallet is not implemented in DogeGo"
	}
	if _, err := os.Stat(src); err != nil {
		return nil, -1, "backupwallet: wallet file not found"
	}
	dest = filepath.Clean(dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil && filepath.Dir(dest) != "." {
		return nil, -1, "backupwallet: " + err.Error()
	}
	in, err := os.Open(src)
	if err != nil {
		return nil, -1, "backupwallet: " + err.Error()
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, -1, "backupwallet: " + err.Error()
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return nil, -1, "backupwallet: " + err.Error()
	}
	if closeErr != nil {
		return nil, -1, "backupwallet: " + closeErr.Error()
	}
	return nil, 0, ""
}
