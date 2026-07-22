// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/wallet"
)

func encryptSetupWallet(dataDir, network, passphrase string) error {
	passphrase = strings.TrimSpace(passphrase)
	if passphrase == "" {
		return fmt.Errorf("wallet passphrase required")
	}
	wpath, err := ensureSetupWallet(dataDir, network)
	if err != nil {
		return err
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		return err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return err
	}
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		return err
	}
	if w.IsEncrypted() {
		return nil
	}
	if _, err := w.Encrypt(passphrase); err != nil {
		return err
	}
	return nil
}
