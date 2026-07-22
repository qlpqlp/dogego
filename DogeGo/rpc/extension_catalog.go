// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package rpc

// extensionRPCCatalog is set by the node when the extension manager is wired.
// Returns extension-owned RPC names (dogego_ext_<id>_<method>).
var extensionRPCCatalog func() []string

// SetExtensionRPCCatalog registers a callback that lists extension RPC methods for help/OpenRPC.
func SetExtensionRPCCatalog(fn func() []string) {
	extensionRPCCatalog = fn
}

// extensionHost is wired from the node when the extension manager starts.
var extensionHost ExtensionsManager

// SetExtensionsHost registers the in-process extension manager for RPC dispatch.
func SetExtensionsHost(m ExtensionsManager) {
	extensionHost = m
}

func activeExtensionsManager(paths *DataPaths) ExtensionsManager {
	if paths != nil && paths.Extensions != nil {
		return paths.Extensions
	}
	return extensionHost
}

// ExtensionRPCHelp returns help for extension-owned RPC when the manager is wired on DataPaths.
func ExtensionRPCHelp(paths *DataPaths, method string) (string, bool) {
	m := activeExtensionsManager(paths)
	if m == nil {
		return "", false
	}
	return m.RPCHelp(method)
}
