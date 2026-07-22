// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"os"

	"dogego/applog"
	"dogego/netfw"
)

func ensureOSFirewall(mode string, listen bool, port int) {
	fwMode := netfw.ParseMode(mode)
	if fwMode == netfw.ModeNever {
		return
	}
	fwCfg := netfw.DefaultConfig(port, listen, true, fwMode)
	res := netfw.Ensure(fwCfg)
	switch {
	case res.AlreadyOK:
		applog.Line("net", fmt.Sprintf("firewall (%s): rules already allow P2P on port %d", res.Platform, port))
	case res.OK:
		applog.Line("net", fmt.Sprintf("firewall (%s): %s", res.Platform, res.UserMessage))
		for _, r := range res.Applied {
			applog.Line("net", "firewall rule: "+r)
		}
	case res.NeedsAdmin:
		applog.Line("net", fmt.Sprintf("firewall (%s): %s", res.Platform, res.UserMessage))
		if manual := netfw.ManualInstructions(fwCfg); manual != "" {
			applog.Line("net", "firewall manual setup:\n"+manual)
		}
		fmt.Fprintf(os.Stderr, "DogeGo: P2P needs firewall rules - see the Windows warning dialog or Overview tab in http://127.0.0.1:2013/\n")
	default:
		if res.Err != nil {
			applog.Line("net", "firewall: "+res.Err.Error())
		}
	}
}
