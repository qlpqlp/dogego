// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import (
	"fmt"
	"strings"
)

// LocalTLSOptions configures auto-generated local HTTPS certificates.
type LocalTLSOptions struct {
	BaseDataDir string
	WebUITLSLocal bool
	RpcTLSLocal   bool
	WebUIListen   string
	RPCListen     string
	TrustCAOnStart bool
	// Explicit PEM paths override local generation when both cert and key are set.
	WebUITLSCert string
	WebUITLSKey  string
	RpcTLSCert   string
	RpcTLSKey    string
}

// ResolvedTLS holds final listener pairs after merging explicit and local TLS.
type ResolvedTLS struct {
	WebUI Pair
	RPC   Pair
	Local *LocalMaterial
}

// ResolveLocalTLS builds RPC and web UI TLS pairs from config and datadir.
func ResolveLocalTLS(opts LocalTLSOptions) (ResolvedTLS, error) {
	out := ResolvedTLS{}
	if pair, ok := explicitPair(opts.WebUITLSCert, opts.WebUITLSKey); ok {
		out.WebUI = pair
	} else if opts.WebUITLSLocal {
		mat, err := EnsureLocalMaterial(opts.BaseDataDir)
		if err != nil {
			return out, fmt.Errorf("webui local tls: %w", err)
		}
		out.Local = mat
		hosts := HostsForListenAddrs(opts.WebUIListen)
		pair, err := EnsureLeafPair(mat, "webui", hosts)
		if err != nil {
			return out, fmt.Errorf("webui local tls: %w", err)
		}
		out.WebUI = pair
	}
	if pair, ok := explicitPair(opts.RpcTLSCert, opts.RpcTLSKey); ok {
		out.RPC = pair
	} else if opts.RpcTLSLocal {
		mat := out.Local
		if mat == nil {
			var err error
			mat, err = EnsureLocalMaterial(opts.BaseDataDir)
			if err != nil {
				return out, fmt.Errorf("rpc local tls: %w", err)
			}
			out.Local = mat
		}
		hosts := HostsForListenAddrs(opts.RPCListen, opts.WebUIListen)
		pair, err := EnsureLeafPair(mat, "rpc", hosts)
		if err != nil {
			return out, fmt.Errorf("rpc local tls: %w", err)
		}
		out.RPC = pair
	}
	return out, nil
}

func explicitPair(cert, key string) (Pair, bool) {
	cert = strings.TrimSpace(cert)
	key = strings.TrimSpace(key)
	if cert == "" && key == "" {
		return Pair{}, false
	}
	if cert == "" || key == "" {
		return Pair{}, false
	}
	return Pair{CertFile: cert, KeyFile: key}, true
}

// Status reports TLS configuration for the web UI API.
func Status(opts LocalTLSOptions, web Pair, rpc Pair, local *LocalMaterial) map[string]any {
	trusted := false
	trustMsg := ""
	caPath := ""
	if local != nil {
		caPath = local.CACertPath
		tr := CATrustStatus(local.CACertPath)
		trusted = tr.Trusted
		trustMsg = tr.Detail
	}
	return map[string]any{
		"webui_https":        web.Enabled(),
		"rpc_https":          rpc.Enabled(),
		"webui_tls_local":    opts.WebUITLSLocal && !explicitPairSet(opts.WebUITLSCert, opts.WebUITLSKey),
		"rpc_tls_local":      opts.RpcTLSLocal && !explicitPairSet(opts.RpcTLSCert, opts.RpcTLSKey),
		"webui_cert_path":    web.CertFile,
		"rpc_cert_path":      rpc.CertFile,
		"local_ca_path":      caPath,
		"local_ca_trusted":   trusted,
		"local_ca_trust_detail": trustMsg,
		"trust_ca_on_start":  opts.TrustCAOnStart,
	}
}

func explicitPairSet(cert, key string) bool {
	_, ok := explicitPair(cert, key)
	return ok
}
