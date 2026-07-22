// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"errors"

	"dogego/chain"
)

// ErrMainnetUnencrypted is returned when mainnet spend/export is attempted on a plaintext wallet.
var ErrMainnetUnencrypted = errors.New("mainnet wallet must be encrypted before spending (use encryptwallet)")

// MainnetRequiresEncryption reports whether chain policy blocks spend/export without encryptwallet.
func MainnetRequiresEncryption(chainName string) bool {
	net, err := chain.ParseNetwork(chainName)
	if err != nil {
		return false
	}
	return net == chain.MainnetDogecoin
}

// RequireMainnetEncryption returns ErrMainnetUnencrypted on mainnet when the wallet is plaintext.
func (w *Disk) RequireMainnetEncryption(chainName string) error {
	if !MainnetRequiresEncryption(chainName) {
		return nil
	}
	if w.IsEncrypted() {
		return nil
	}
	return ErrMainnetUnencrypted
}
