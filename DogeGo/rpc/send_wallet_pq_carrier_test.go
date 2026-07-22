// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestWalletBroadcastPQCarrierPaymentNilFundOptsNoPanic(t *testing.T) {
	paths := &DataPaths{
		WalletPqCarrierEnabled: func() bool { return true },
		WalletPQCarrierKeyMaterial: func(string) (string, []byte, []byte, error) {
			return "", nil, nil, errPQCarrierTestAbort
		},
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic with nil extraFundOpts: %v", r)
		}
	}()
	_, code, _ := walletBroadcastPQCarrierPayment(
		"testnet", paths, nil, nil, nil, nil,
		map[string]float64{"D9TestAddress": 1.0},
		func([]byte) error { return nil },
		false, 0, "sendtoaddress", nil,
	)
	if code == 0 {
		t.Fatal("expected error before broadcast")
	}
}

var errPQCarrierTestAbort = errString("pq carrier test abort")

type errString string

func (e errString) Error() string { return string(e) }
