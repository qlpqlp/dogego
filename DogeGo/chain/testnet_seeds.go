// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

// TestnetFixedSeedAddrs are host:port entries from src/chainparamsseeds.h pnSeed6_test (Dogecoin Core rebooted testnet).
// DNS discovery for reboot testnet is separate: ParamsFor sets seed.dogego.org first (DogeGo helper seed), then these fixed peers.
var TestnetFixedSeedAddrs = []string{
	"[2600:3c00::f03c:91ff:fe5b:4cf3]:44556",
	"167.179.147.155:44556",
	"[2001:19f0:ac01:105e:5400:3ff:fee8:e995]:44556",
	"[2a01:4f9:c012:5ab0::1]:44556",
	"[2600:3c00::f03c:91ff:fe9e:7f03]:44556",
	"[2a01:4f9:3081:35da::2]:44556",
	"15.235.55.110:44556",
	"37.27.37.73:44556",
	"198.58.102.18:44556",
	"45.63.86.162:44556",
	"194.110.169.133:44556",
	"65.109.229.85:44556",
	"185.232.70.226:44556",
	"80.82.21.77:44556",
	"104.237.131.138:44556",
	"37.27.68.95:44556",
	"65.21.95.163:44556",
	"38.50.247.160:44556",
	"20.48.60.13:44556",
	"165.227.227.139:44556",
	"35.84.106.212:44556",
	"89.58.9.219:44556",
}
