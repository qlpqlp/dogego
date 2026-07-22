// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"testing"
	"time"

	"dogego/analytics"
	"dogego/mempool"
)

func TestRuntimeServicesMempoolPause(t *testing.T) {
	pool := mempool.New(5)
	svc := NewRuntimeServices(RuntimeServicesConfig{Parent: context.Background(), Pool: pool})
	if err := svc.ApplyServiceAction("mempool", "pause"); err != nil {
		t.Fatal(err)
	}
	if !pool.Paused() {
		t.Fatal("expected paused")
	}
	if err := svc.ApplyServiceAction("mempool", "resume"); err != nil {
		t.Fatal(err)
	}
	if pool.Paused() {
		t.Fatal("expected resumed")
	}
}

func TestRuntimeServicesAnalyticsStartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewRuntimeServices(RuntimeServicesConfig{
		Parent: ctx,
		AnalyticsCfg: func() analytics.SidecarConfig {
			return analytics.SidecarConfig{ChainRoot: t.TempDir(), Tick: time.Hour}
		},
	})
	if err := svc.StartAnalytics(); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartAnalytics(); err == nil {
		t.Fatal("expected already running error")
	}
	if err := svc.StopAnalytics(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if svc.AnalyticsRunning() {
		t.Fatal("expected stopped")
	}
}

func TestRuntimeServicesRPCStatus(t *testing.T) {
	svc := NewRuntimeServices(RuntimeServicesConfig{Parent: context.Background()})
	svc.SetRPCConfigured(true)
	st := svc.Status()
	var rpcRow ServiceStatus
	for _, row := range st {
		if row.ID == "rpc" {
			rpcRow = row
			break
		}
	}
	if rpcRow.Detail != "starting (bind pending)" {
		t.Fatalf("detail %q", rpcRow.Detail)
	}
	svc.SetRPCListening(true)
	st = svc.Status()
	for _, row := range st {
		if row.ID == "rpc" {
			rpcRow = row
			break
		}
	}
	if rpcRow.Detail != "warming up (port open; methods after chain init)" {
		t.Fatalf("detail %q", rpcRow.Detail)
	}
	svc.SetRPCDispatchReady(true)
	st = svc.Status()
	for _, row := range st {
		if row.ID == "rpc" {
			rpcRow = row
			break
		}
	}
	if rpcRow.Detail != "listening" {
		t.Fatalf("detail %q", rpcRow.Detail)
	}
}
