// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestSoloMiningRuntimeServiceStatus(t *testing.T) {
	var active atomic.Bool
	rt := NewSoloMiningRuntime(SoloMiningRuntimeConfig{
		Parent:        context.Background(),
		Active:        &active,
		MineRequested: func() bool { return true },
		PayoutAddress: "nTestAddr",
		Eligible: func() (bool, string) {
			return true, "reboot testnet solo mining available"
		},
	})
	st := rt.ServiceStatus()
	if st.Running || len(st.Actions) != 1 || st.Actions[0] != "start" {
		t.Fatalf("stopped status %#v", st)
	}
	active.Store(true)
	st = rt.ServiceStatus()
	if !st.Running || len(st.Actions) != 2 || st.Actions[0] != "stop" {
		t.Fatalf("running status %#v", st)
	}
}

func TestSoloMiningRuntimeIneligible(t *testing.T) {
	rt := NewSoloMiningRuntime(SoloMiningRuntimeConfig{
		Parent: context.Background(),
		Eligible: func() (bool, string) {
			return false, "background solo mining is reboot testnet only"
		},
	})
	st := rt.ServiceStatus()
	if len(st.Actions) != 0 {
		t.Fatalf("ineligible actions %#v", st.Actions)
	}
	if err := rt.Start(); err == nil {
		t.Fatal("expected start error when ineligible")
	}
}

func TestRuntimeServicesMiningActionUnknown(t *testing.T) {
	svc := NewRuntimeServices(RuntimeServicesConfig{Parent: context.Background()})
	svc.SetMining(NewSoloMiningRuntime(SoloMiningRuntimeConfig{Parent: context.Background()}))
	if err := svc.ApplyServiceAction("mining", "pause"); err == nil {
		t.Fatal("expected unknown action error")
	}
}

func TestRuntimeServicesAnalyticsActionsContextual(t *testing.T) {
	svc := NewRuntimeServices(RuntimeServicesConfig{Parent: context.Background()})
	st := svc.Status()
	var an ServiceStatus
	for _, row := range st {
		if row.ID == "analytics" {
			an = row
			break
		}
	}
	if len(an.Actions) != 1 || an.Actions[0] != "start" {
		t.Fatalf("stopped analytics actions %#v", an.Actions)
	}
}
