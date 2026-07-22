// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"

	"dogego/applog"
	"dogego/rpc"
)

func buildNodeRPCAuth(cfg Config, chainDataDir string) *rpc.RPCAuth {
	allow, allowErr := rpc.ParseRPCAllowList(cfg.RpcAllowIP)
	if allowErr != nil {
		applog.Line("rpc", "rpcallowip: "+allowErr.Error())
	}
	auth := &rpc.RPCAuth{
		Allow: allow,
		Limits: rpc.RPCLimits{
			RequestsPerMinute:  cfg.RpclimitPerMin,
			AuthFailsPerMinute: cfg.RpcAuthMaxFail,
		},
	}
	if cfg.RpcCookie {
		if strings.TrimSpace(cfg.RpcUser) != "" {
			applog.Line("rpc", "rpc_cookie enabled: ignoring rpc_user/rpc_password for this process (using fresh .cookie)")
		}
		applyRPCCookieAuth(auth, chainDataDir)
	} else if strings.TrimSpace(cfg.RpcUser) != "" {
		auth.User = strings.TrimSpace(cfg.RpcUser)
		auth.Password = cfg.RpcPassword
	}
	if rpc.ConfigRequiresAuth(cfg.RPCAddr, allow) && !authEnabled(auth) {
		applog.Line("rpc", "SECURITY: JSON-RPC is reachable beyond loopback (rpcallowip or bind) without credentials; enabling rpc_cookie")
		if !cfg.RpcCookie {
			applyRPCCookieAuth(auth, chainDataDir)
		}
	}
	return auth
}

func authEnabled(auth *rpc.RPCAuth) bool {
	return auth != nil && strings.TrimSpace(auth.User) != ""
}

func applyRPCCookieAuth(auth *rpc.RPCAuth, chainDataDir string) {
	cookieAuth, cookiePath, err := rpc.WriteCookieAuth(chainDataDir)
	if err != nil {
		applog.Line("rpc", "rpc_cookie: "+err.Error()+" - RPC starting without auth")
		return
	}
	auth.User = cookieAuth.User
	auth.Password = cookieAuth.Password
	applog.Line("rpc", "JSON-RPC HTTP Basic from "+cookiePath+" (user "+rpc.CookieUserName+")")
}
