// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package zmqnotify

import "strings"

// coreZMQHighWaterMark matches Bitcoin/Dogecoin Core default ZMQ_SNDHWM.
const coreZMQHighWaterMark = 1000

// ActiveNotifications returns Core-shaped getzmqnotifications rows for configured PUB endpoints.
func (c Config) ActiveNotifications() []map[string]interface{} {
	var out []map[string]interface{}
	add := func(typ, addr string) {
		a := strings.TrimSpace(addr)
		if a == "" {
			return
		}
		ep, err := normalizeEndpoint(a)
		if err != nil {
			ep = a
		}
		out = append(out, map[string]interface{}{
			"type":    typ,
			"address": ep,
			"hwm":     coreZMQHighWaterMark,
		})
	}
	add("pubhashblock", c.PubHashBlock)
	add("pubhashtx", c.PubHashTx)
	add("pubrawblock", c.PubRawBlock)
	add("pubrawtx", c.PubRawTx)
	return out
}
