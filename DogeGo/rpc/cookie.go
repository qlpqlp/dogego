// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// CookieUserName is the fixed RPC username written to .cookie (Core uses a similar fixed prefix pattern).
const CookieUserName = "__dogego__"

// WriteCookieAuth writes chainDataDir/.cookie (0600) as a single line "user:password" and returns
// credentials for HTTP Basic auth. Each call generates a new random password (Core regenerates per start).
func WriteCookieAuth(chainDataDir string) (*RPCAuth, string, error) {
	if chainDataDir == "" {
		return nil, "", fmt.Errorf("rpc cookie: empty chain data directory")
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, "", err
	}
	pass := hex.EncodeToString(b[:])
	line := CookieUserName + ":" + pass
	full := filepath.Join(chainDataDir, ".cookie")
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, []byte(line), 0o600); err != nil {
		return nil, "", err
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return nil, "", err
	}
	_ = os.Chmod(full, 0o600)
	return &RPCAuth{User: CookieUserName, Password: pass}, full, nil
}
