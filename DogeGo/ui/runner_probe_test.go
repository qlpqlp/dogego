package ui

import (
	"testing"

	"dogego/runner"
)

func TestProbeRunnerOfflineTools(t *testing.T) {
	r := ProbeRunner(RunnerProbeOptions{})
	if r.CheckedAt == "" {
		t.Fatal("missing checked_at")
	}
	if r.Provision.Total != 9 {
		t.Fatalf("provision total %d", r.Provision.Total)
	}
}

func TestProbeRunnerRequireWalletDatCLIHint(t *testing.T) {
	r := ProbeRunner(RunnerProbeOptions{RequireWalletDat: true})
	if r.CLIPreflight != "dogego cert preflight -require-core -require-wallet-dat" {
		t.Fatalf("cli hint %q", r.CLIPreflight)
	}
	if r.CLIWeekly != "dogego cert weekly -require-wallet-dat" {
		t.Fatalf("weekly cli %q", r.CLIWeekly)
	}
	if r.CLIWeeklyLive != "dogego cert weekly-live -mine-bootstrap -require-wallet-dat" {
		t.Fatalf("weekly_live cli %q", r.CLIWeeklyLive)
	}
	if r.CLILiveSoak != "dogego cert live-soak -require-soak-env" {
		t.Fatalf("live_soak cli %q", r.CLILiveSoak)
	}
	if r.Doc != runner.DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", r.Doc)
	}
}

func TestProbeRunnerWalletDatImportParity(t *testing.T) {
	r := ProbeRunner(RunnerProbeOptions{RequireCore: true, RequireWalletDat: true})
	if r.Preflight.WalletDatImport != nil {
		return
	}
	r2 := ProbeRunner(RunnerProbeOptions{})
	if r2.CLIWeeklyLive != "dogego cert weekly-live -mine-bootstrap" {
		t.Fatalf("default weekly_live cli %q", r2.CLIWeeklyLive)
	}
}

func TestProbeRunnerForNetworkMainnetSkipped(t *testing.T) {
	r := ProbeRunnerForNetwork("mainnet", RunnerProbeOptions{RequireCore: true})
	if !r.OK || !r.Skipped {
		t.Fatalf("%+v", r)
	}
}
