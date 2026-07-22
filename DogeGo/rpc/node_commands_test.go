// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMapDisconnectError(t *testing.T) {
	code, msg := mapDisconnectError(errors.New("Node not found in connected nodes"))
	if code != CodeRPCNodeNotConnected || msg != ErrNodeNotConnected {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	code, msg = mapDisconnectError(errors.New("cannot disconnect the primary sync peer"))
	if code != -8 {
		t.Fatalf("primary code=%d", code)
	}
}

func TestExecDisconnectNodeNotConnected(t *testing.T) {
	paths := &DataPaths{
		DisconnectNode: func(addr string) error {
			return errors.New("Node not found in connected nodes")
		},
	}
	_, code, msg := execDisconnectNode(paths, []json.RawMessage{json.RawMessage(`"10.0.0.2:22556"`)})
	if code != CodeRPCNodeNotConnected || msg != ErrNodeNotConnected {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestMapSetBanError(t *testing.T) {
	code, msg := mapSetBanError(errors.New(ErrInvalidIPOrSubnet))
	if code != CodeRPCInvalidIPOrSubnet {
		t.Fatalf("invalid %d %q", code, msg)
	}
	code, msg = mapSetBanError(errors.New(ErrIPAlreadyBanned))
	if code != CodeRPCNodeAlreadyAdded {
		t.Fatalf("dup %d %q", code, msg)
	}
}
