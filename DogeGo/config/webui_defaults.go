// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

// Default Web UI listen addresses use the hostname "localhost" (not 127.0.0.1) so
// browser WebAuthn / device biometrics work on Windows, Linux, and macOS. Operators
// may still set webui to 127.0.0.1:2013 (HTTPS local certs include both names as SANs).
const (
	DefaultWebUIListen = "localhost:2013"
	DualMainnetWebUI   = "localhost:2013"
	DualTestnetWebUI   = "localhost:2014"
)
