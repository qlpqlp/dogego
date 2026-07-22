// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"

	"dogego/applog"
	"dogego/config"
	"dogego/httptls"
)

func resolveNodeTLS(cfg *Config, baseDataDir string) error {
	if cfg == nil {
		return nil
	}
	opts := httptls.LocalTLSOptions{
		BaseDataDir:    baseDataDir,
		WebUITLSLocal:  cfg.EffectiveFile.WebUITLSLocal,
		RpcTLSLocal:    cfg.EffectiveFile.RpcTLSLocal,
		WebUIListen:    cfg.WebUIAddr,
		RPCListen:      cfg.RPCAddr,
		TrustCAOnStart: cfg.EffectiveFile.LocalTLSTrustCA,
		WebUITLSCert:   cfg.EffectiveFile.WebUITLSCert,
		WebUITLSKey:    cfg.EffectiveFile.WebUITLSKey,
		RpcTLSCert:     cfg.EffectiveFile.RpcTLSCert,
		RpcTLSKey:      cfg.EffectiveFile.RpcTLSKey,
	}
	resolved, err := httptls.ResolveLocalTLS(opts)
	if err != nil {
		return err
	}
	if resolved.WebUI.Enabled() {
		cfg.WebUITLS = resolved.WebUI
	}
	if resolved.RPC.Enabled() {
		cfg.RpcTLS = resolved.RPC
	}
	cfg.localTLSMaterial = resolved.Local
	if cfg.EffectiveFile.LocalTLSTrustCA && resolved.Local != nil {
		var tr httptls.TrustResult
		if resolved.Local.CAGenerated {
			tr = httptls.TrustLocalCAForce(resolved.Local.CACertPath)
		} else {
			tr = httptls.TrustLocalCA(resolved.Local.CACertPath)
		}
		if tr.Trusted {
			applog.Line("tls", "local CA trusted: "+tr.Detail)
		} else if tr.Detail != "" {
			applog.Line("tls", "local CA trust: "+tr.Detail)
			if tr.Hint != "" {
				applog.Line("tls", tr.Hint)
			}
		}
	}
	if resolved.WebUI.Enabled() {
		applog.Line("tls", "Web UI HTTPS enabled ("+resolved.WebUI.CertFile+")")
	}
	if resolved.RPC.Enabled() {
		applog.Line("tls", "JSON-RPC HTTPS enabled ("+resolved.RPC.CertFile+")")
	}
	return nil
}

func localTLSOpts(cfg Config, baseDataDir string) httptls.LocalTLSOptions {
	return httptls.LocalTLSOptions{
		BaseDataDir:    baseDataDir,
		WebUITLSLocal:  cfg.EffectiveFile.WebUITLSLocal,
		RpcTLSLocal:    cfg.EffectiveFile.RpcTLSLocal,
		WebUIListen:    cfg.WebUIAddr,
		RPCListen:      cfg.RPCAddr,
		TrustCAOnStart: cfg.EffectiveFile.LocalTLSTrustCA,
		WebUITLSCert:   cfg.EffectiveFile.WebUITLSCert,
		WebUITLSKey:    cfg.EffectiveFile.WebUITLSKey,
		RpcTLSCert:     cfg.EffectiveFile.RpcTLSCert,
		RpcTLSKey:      cfg.EffectiveFile.RpcTLSKey,
	}
}

func webUIScheme(cfg Config) string {
	if cfg.WebUITLS.Enabled() {
		return "https"
	}
	return "http"
}

func configUsesLocalTLS(f config.File) bool {
	return f.WebUITLSLocal || f.RpcTLSLocal || strings.TrimSpace(f.WebUITLSCert) != ""
}
