// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"dogego/chain"
	"dogego/ui/websecurity"
)

func validateSetupDashboardPIN(pin string) error {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return nil
	}
	if len(pin) != 6 {
		return fmt.Errorf("dashboard PIN must be exactly 6 digits")
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return fmt.Errorf("dashboard PIN must be exactly 6 digits")
		}
	}
	return nil
}

func applySetupDashboardPIN(dataDir, network, pin string) error {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return nil
	}
	if err := validateSetupDashboardPIN(pin); err != nil {
		return err
	}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return fmt.Errorf("datadir required for dashboard PIN")
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		return err
	}
	sub, err := chain.ChainDataDirName(net)
	if err != nil {
		return err
	}
	chainRoot := filepath.Join(dataDir, sub)
	g, err := websecurity.NewGate(chainRoot)
	if err != nil {
		return err
	}
	return g.SetupPIN("", pin)
}

func applySetupDashboardPINNetworks(dataDir string, networks []string, pin string) error {
	for _, net := range networks {
		if err := applySetupDashboardPIN(dataDir, net, pin); err != nil {
			return fmt.Errorf("%s: %w", net, err)
		}
	}
	return nil
}
