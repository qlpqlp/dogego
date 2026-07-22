// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"dogego/httptls"
)

func registerTLSRoutes(mux *http.ServeMux, cfg StartConfig) {
	mux.HandleFunc("/api/tls/status", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) || r.Method != http.MethodGet {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		opts, web, rpc, local := tlsStatusInputs(cfg)
		_ = json.NewEncoder(w).Encode(httptls.Status(opts, web, rpc, local))
	})
	mux.HandleFunc("/api/tls/trust-ca", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) || r.Method != http.MethodPost {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _, _, local := tlsStatusInputs(cfg)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if local == nil || strings.TrimSpace(local.CACertPath) == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "local TLS not enabled; turn on webui_tls_local or rpc_tls_local, restart, then try again",
			})
			return
		}
		res := httptls.TrustLocalCA(local.CACertPath)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func tlsStatusInputs(cfg StartConfig) (httptls.LocalTLSOptions, httptls.Pair, httptls.Pair, *httptls.LocalMaterial) {
	opts := httptls.LocalTLSOptions{
		BaseDataDir:    cfg.BaseDataDir,
		WebUITLSLocal:  cfg.EffectiveFile.WebUITLSLocal,
		RpcTLSLocal:    cfg.EffectiveFile.RpcTLSLocal,
		WebUIListen:    cfg.ListenAddr,
		RPCListen:      cfg.EffectiveFile.RPCAddr,
		TrustCAOnStart: cfg.EffectiveFile.LocalTLSTrustCA,
		WebUITLSCert:   cfg.EffectiveFile.WebUITLSCert,
		WebUITLSKey:    cfg.EffectiveFile.WebUITLSKey,
		RpcTLSCert:     cfg.EffectiveFile.RpcTLSCert,
		RpcTLSKey:      cfg.EffectiveFile.RpcTLSKey,
	}
	web := cfg.TLS
	rpc := httptls.Pair{
		CertFile: cfg.EffectiveFile.RpcTLSCert,
		KeyFile:  cfg.EffectiveFile.RpcTLSKey,
	}
	var local *httptls.LocalMaterial
	if cfg.BaseDataDir != "" && (opts.WebUITLSLocal || opts.RpcTLSLocal) {
		if mat, err := httptls.EnsureLocalMaterial(cfg.BaseDataDir); err == nil {
			local = mat
		}
	}
	return opts, web, rpc, local
}
