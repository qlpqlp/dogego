// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

func rpcAllowBareMultisig(paths *DataPaths) bool {
	if paths == nil || paths.Standard == nil {
		return true
	}
	return paths.Standard().AllowBareMultisig
}

func parseImportDescriptorAllowed(paths *DataPaths, desc string) (parsedDescriptor, bool) {
	parsed, ok := parseImportDescriptor(desc)
	if !ok {
		return parsedDescriptor{}, false
	}
	if parsed.scriptType == "bare-multi" && !rpcAllowBareMultisig(paths) {
		return parsedDescriptor{}, false
	}
	return parsed, true
}
