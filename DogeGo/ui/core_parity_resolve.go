// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"os"
	"strings"

	"dogego/config"
)

// CoreParityEndpoints is the resolved Dogecoin Core JSON-RPC target for live parity probes.
type CoreParityEndpoints struct {
	Addr string
	User string
	Pass string
}

// ResolveCoreParityEndpoints picks Core RPC addr/auth: env overrides config, then network defaults.
func ResolveCoreParityEndpoints(network string, f config.File) CoreParityEndpoints {
	addr := strings.TrimSpace(f.CoreRPCAddr)
	user := strings.TrimSpace(f.CoreRPCUser)
	pass := strings.TrimSpace(f.CoreRPCPassword)
	if user != "" && pass == "" {
		pass = strings.TrimSpace(f.RpcPassword)
	}
	if user == "" && strings.TrimSpace(f.RpcUser) != "" {
		user = strings.TrimSpace(f.RpcUser)
		pass = strings.TrimSpace(f.RpcPassword)
	}
	if v := strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_ADDR")); v != "" {
		addr = v
	} else if p := strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_PORT")); p != "" {
		addr = "127.0.0.1:" + p
	}
	if addr == "" {
		addr = defaultCoreRPCAddrForNetwork(network)
	}
	if u := strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_USER")); u != "" {
		user = u
		pass = strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_PASS"))
	} else if user == "" {
		user, pass = coreRPCAuthFromEnv()
	}
	return CoreParityEndpoints{Addr: addr, User: user, Pass: pass}
}

func defaultCoreRPCAddrForNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "testnet", "reboottestnet":
		return "127.0.0.1:44556"
	default:
		return "127.0.0.1:22555"
	}
}

func coreRPCAuthFromEnv() (user, pass string) {
	user = strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_USER"))
	pass = strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_PASS"))
	if user == "" {
		user = strings.TrimSpace(os.Getenv("DOGEGO_RPC_USER"))
	}
	if pass == "" {
		pass = strings.TrimSpace(os.Getenv("DOGEGO_RPC_PASS"))
	}
	return user, pass
}

// probeConfigFromStart loads the freshest on-disk dogecoinconf for live probes (no restart required).
func probeConfigFromStart(cfg StartConfig) config.File {
	out := cfg.EffectiveFile
	if cfg.ConfSavePath == "" {
		return out
	}
	b, err := os.ReadFile(cfg.ConfSavePath)
	if err != nil {
		return out
	}
	var disk config.File
	if json.Unmarshal(b, &disk) != nil {
		return out
	}
	if strings.TrimSpace(disk.CoreRPCAddr) != "" {
		out.CoreRPCAddr = disk.CoreRPCAddr
	}
	if strings.TrimSpace(disk.CoreRPCUser) != "" {
		out.CoreRPCUser = disk.CoreRPCUser
	}
	if disk.CoreRPCPassword != "" {
		out.CoreRPCPassword = disk.CoreRPCPassword
	}
	if strings.TrimSpace(disk.SignerCmd) != "" {
		out.SignerCmd = disk.SignerCmd
	}
	if strings.TrimSpace(disk.RpcUser) != "" && strings.TrimSpace(out.CoreRPCUser) == "" {
		out.RpcUser = disk.RpcUser
		out.RpcPassword = disk.RpcPassword
	}
	return out
}
