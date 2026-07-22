// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/fieldevidence"
	"dogego/offlinegate"
	"dogego/operatorworkflow"
	"dogego/pqcert"
	"dogego/walletimport"
	"dogego/walletmigration"
)

// TestMilestoneScriptsExist guards operator cert / soak scripts referenced from ROADMAP.
func TestMilestoneScriptsExist(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"scripts/mempool_stateful_parity_reboottestnet.ps1",
		"scripts/corruption_timed_loop.ps1",
		"scripts/corruption_timed_loop_mini.ps1",
		"scripts/corruption_extended_cert_mini.ps1",
		"scripts/corruption_inject_soak.ps1",
		"scripts/corruption_soak_cert.ps1",
		"scripts/core_operator_runbook_full.ps1",
		"scripts/core_e2e_reboottestnet_runbook.ps1",
		"scripts/core_e2e_full_runbook.ps1",
		"scripts/core_e2e_mainnet_runbook.ps1",
		"scripts/core_recovery_workflow.ps1",
		"scripts/corruption_long_soak_gate.ps1",
		"scripts/mempool_stateful_core_gate.ps1",
		"scripts/core_reboottestnet_core_aligned_gate.ps1",
		"scripts/core_reboottestnet_reindex_compare.ps1",
		"scripts/core_reindex_prune_disruptive_workflow.ps1",
		"scripts/ci_scheduled_corruption_soak.ps1",
		"scripts/ci_live_reboottestnet_gate.ps1",
		"scripts/ci_offline_gate.sh",
		"scripts/ci_offline_gate.ps1",
		"scripts/core_mainnet_disruptive_reindex_gate.ps1",
		"scripts/ci_runner_preflight.ps1",
		"scripts/setup_reboottestnet_core_parity.ps1",
		"scripts/ci_runner_provision_checklist.ps1",
		"scripts/ci_scheduled_weekly_live.ps1",
		"scripts/gh_enable_scheduled_live.ps1",
		"scripts/extended_operator_soak.ps1",
		"scripts/core_mempool_corpus_probe.ps1",
		"scripts/core_mempool_bip125_offline_probe.ps1",
		"scripts/wallet_migration_cert.ps1",
		"scripts/wallet_import_cert.ps1",
		"scripts/wallet_import_cert.sh",
		"scripts/wallet_migration_cert.sh",
		"scripts/pq_cert.ps1",
		"scripts/pq_cert.sh",
		"scripts/operator_workflow_cert.ps1",
		"scripts/operator_workflow_cert.sh",
		"scripts/field_evidence_cert.sh",
		"scripts/cert_offline_prerequisites.ps1",
		"scripts/cert_offline_prerequisites.sh",
		"scripts/provision_wallet_dat_fixture.ps1",
		"scripts/_wallet_dat_env.ps1",
		"scripts/field_evidence_cert.ps1",
		"fieldevidence/suites.go",
		"fieldevidence/run.go",
		"offlinegate/suites.go",
		"offlinegate/bootstrap.go",
		"walletmigration/verify.go",
		"walletimport/verify.go",
		"operatorworkflow/verify.go",
		"pqcert/suites.go",
		"runner/doc.go",
		"runner/provision.go",
		"runner/preflight.go",
		"runner/setup_parity.go",
		"runner/weekly_live.go",
		"runner/live_soak.go",
		"runner/enable_weekly.go",
	} {
		st, err := os.Stat(filepath.Join(root, rel))
		if err != nil || st.IsDir() {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestCertOfflinePrerequisitesShape(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"cert_offline_prerequisites.ps1", "cert_offline_prerequisites.sh"} {
		raw, err := os.ReadFile(filepath.Join(root, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, needle := range []string{
			"dogego cert offline",
			"go run ./cmd/dogego cert offline",
			"dogego cert wallet-import",
			"go run ./cmd/dogego cert wallet-import",
			"dogego cert pq",
			"dogego cert operator",
		} {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing %q", name, needle)
			}
		}
	}
}

func TestWalletImportCertScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "wallet_import_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert wallet-import",
		"go run ./cmd/dogego cert wallet-import",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("wallet_import_cert.ps1 missing %q", needle)
		}
	}
}

func TestWalletImportCertScriptSuitesDrift(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "wallet_import_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "go run ./cmd/dogego cert wallet-import") {
		t.Fatal("wallet_import_cert.ps1 must delegate to cert wallet-import")
	}
	for _, s := range walletimport.DefaultOfflineSuites() {
		for _, needle := range walletimport.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("wallet_import_cert.ps1 should not duplicate suite %q after delegating to cert wallet-import", s.Name)
			}
		}
	}
}

func TestWalletImportCertShellShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "wallet_import_cert.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "go run ./cmd/dogego cert wallet-import") {
		t.Fatal("wallet_import_cert.sh must delegate to cert wallet-import")
	}
	for _, s := range walletimport.DefaultOfflineSuites() {
		for _, needle := range walletimport.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("wallet_import_cert.sh should not duplicate suite %q", s.Name)
			}
		}
	}
}

func TestOperatorWorkflowCertShellShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "operator_workflow_cert.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "go run ./cmd/dogego cert operator") {
		t.Fatal("operator_workflow_cert.sh must delegate to cert operator")
	}
	if !strings.Contains(text, "field_disk_connect_cert.ps1") {
		t.Fatal("operator_workflow_cert.sh must document Windows-only field disk connect")
	}
	for _, s := range operatorworkflow.DefaultCoreSuites() {
		for _, needle := range operatorworkflow.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("operator_workflow_cert.sh should not duplicate suite %q", s.Name)
			}
		}
	}
}

func TestFieldEvidenceCertShellShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "field_evidence_cert.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert field-evidence",
		"go run ./cmd/dogego cert field-evidence",
		"field_evidence_cert.ps1",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("field_evidence_cert.sh missing %q", needle)
		}
	}
	for _, s := range fieldevidence.DefaultSuites() {
		for _, needle := range fieldevidence.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("field_evidence_cert.sh should not duplicate suite %q", s.Name)
			}
		}
	}
}

func TestWalletMigrationCertShellShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "wallet_migration_cert.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"go run ./cmd/dogego cert wallet-migration -offline-only",
		"-live-import",
		"DOGEGO_WALLET_DAT",
		"SKIP_OFFLINE",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("wallet_migration_cert.sh missing %q", needle)
		}
	}
	for _, s := range walletmigration.DefaultOfflineSuites() {
		for _, needle := range walletmigration.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("wallet_migration_cert.sh should not duplicate suite %q", s.Name)
			}
		}
	}
}

func TestOperatorWorkflowCertScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "operator_workflow_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert operator",
		"go run ./cmd/dogego cert operator",
		"DOGEGO_FIELD_DISK_CONNECT",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("operator_workflow_cert.ps1 missing %q", needle)
		}
	}
}

func TestCorruptionTimedLoopMiniScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "corruption_timed_loop_mini.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"corruption_timed_loop.ps1",
		"reboottestnet-only",
		"DurationMin",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("corruption_timed_loop_mini.ps1 missing %q", needle)
		}
	}
}

func TestE2EReboottestnetRunbookShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_e2e_reboottestnet_runbook.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"node_health.ps1",
		"core_restart_resume_check.ps1",
		"core_bip152_probe.ps1",
		`Step "bip152_hb"`,
		"mempool_stateful_parity_reboottestnet.ps1",
		"Scenario      = \"all\"",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_e2e_reboottestnet_runbook.ps1 missing %q", needle)
		}
	}
}

func TestE2EFullRunbookShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_e2e_full_runbook.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"operator_workflow_cert.ps1",
		"ibd_convergence_check.ps1",
		"core_recovery_workflow.ps1",
		"core_reindex_prune_workflow.ps1",
		"core_bip152_probe.ps1",
		`Step "bip152_hb"`,
		"mempool_stateful_parity_reboottestnet.ps1",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_e2e_full_runbook.ps1 missing %q", needle)
		}
	}
}

func TestCorruptionLongSoakGateShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "corruption_long_soak_gate.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"corruption_timed_loop.ps1",
		"ibd_timed_soak.ps1",
		"DOGEGO_CORRUPTION_LONG_MIN",
		"reboottestnet-only",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("corruption_long_soak_gate.ps1 missing %q", needle)
		}
	}
}

func TestCorruptionTimedLoopScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "corruption_timed_loop.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"corruption_inject_soak.ps1",
		"verifychain",
		"DurationMin",
		"CorruptionCycles",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("corruption_timed_loop.ps1 missing %q", needle)
		}
	}
}

func TestMempoolStatefulScriptExpandedScenarios(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "mempool_stateful_parity_reboottestnet.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, scenario := range []string{
		"min_relay_fee", "mempool_double_spend", "rbf_not_replaceable", "rbf_fullrbf", "package_descendant_limit",
		"p2pkh_roundtrip", "rbf_too_many_descendants",
		"p2sh_nested_p2pkh", "p2sh_multisig", "bare_multisig", "p2sh_cltv_p2pk", "p2sh_csv_p2pk",
		"p2pk_non_standard_input", "package_ancestor_size", "package_descendant_size",
	} {
		if !strings.Contains(text, `"`+scenario+`"`) {
			t.Fatalf("mempool_stateful script missing -Scenario %s", scenario)
		}
	}
	if !strings.Contains(text, "statefulprobe") {
		t.Fatal("mempool_stateful script missing wallet-anchored statefulprobe helper")
	}
	if !strings.Contains(text, "coreCompareRequired") {
		t.Fatal("mempool_stateful script missing Core compare gate")
	}
	if !strings.Contains(text, "coreCompareStrict") {
		t.Fatal("mempool_stateful script missing Core compare required gate")
	}
	if !strings.Contains(text, "coreCompareMin") {
		t.Fatal("mempool_stateful script missing Core compare min rows gate")
	}
}

func TestReindexPruneDisruptiveWorkflowShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_reindex_prune_disruptive_workflow.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"reindextx",
		"verifychain",
		"ConfirmDisruptive",
		"pruneblockchain",
		"reindexblockfilters",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_reindex_prune_disruptive_workflow.ps1 missing %q", needle)
		}
	}
}

func TestCIRunnerPreflightShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "ci_runner_preflight.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego-live",
		"RequireCore",
		"getblockchaininfo",
		"getwalletinfo",
		"dogecoin-cli",
		"DOGEGO_WALLET_DAT",
		"RequireWalletDat",
		"Initialize-WalletDatEnv",
		"-live-probe",
		"-require-wallet-dat",
		"dogego cert preflight",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ci_runner_preflight.ps1 missing %q", needle)
		}
	}
}

func TestProvisionWalletDatFixtureShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "provision_wallet_dat_fixture.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"DOGEGO_WALLET_DAT",
		"wallet-migration",
		"-skip-offline",
		"DOGEGO_WALLET_DAT_REQUIRED",
		"_wallet_dat_env.ps1",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("provision_wallet_dat_fixture.ps1 missing %q", needle)
		}
	}
}

func TestSetupReboottestnetCoreParityShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "setup_reboottestnet_core_parity.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"ci_runner_preflight.ps1",
		"core_reboottestnet_core_aligned_gate.ps1",
		"DOGEGO_CORE_COMPARE_MIN",
		"MineBootstrap",
		"dogego cert setup-parity",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("setup_reboottestnet_core_parity.ps1 missing %q", needle)
		}
	}
}

func TestCILiveReboottestnetGateShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "ci_live_reboottestnet_gate.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"ci_runner_preflight.ps1",
		"core_e2e_reboottestnet_runbook.ps1",
		"core_reboottestnet_core_aligned_gate.ps1",
		"node_health.ps1",
		"reboottestnet-only",
		"dogego cert weekly-live",
		"skip-scripts",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ci_live_reboottestnet_gate.ps1 missing %q", needle)
		}
	}
}

func TestDogegoCIWorkflowShape(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "..", ".github", "workflows", "dogego.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("dogego.yml: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"ci_offline_gate.sh",
		"dogego-live",
		"core_reboottestnet_core_aligned_gate.ps1",
		"ci_runner_preflight.ps1",
		"dogego-live",
		"ci_live_reboottestnet_gate.ps1",
		"DOGEGO_SCHEDULED_LIVE_SOAK",
		"DOGEGO_SCHEDULED_CORE_GATE",
		"live_core_gate",
		"live-weekly",
		"ci_scheduled_weekly_live.ps1",
		"DOGEGO_SCHEDULED_WEEKLY_LIVE",
		"require_wallet_dat",
		"DOGEGO_WALLET_DAT_REQUIRED",
		"ci_milestone_b_full_gate.ps1",
		"dogego cert weekly-live",
		"dogego cert live-soak",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("dogego.yml missing %q", needle)
		}
	}
}

func TestGhEnableScheduledLiveShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "gh_enable_scheduled_live.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"DOGEGO_SCHEDULED_WEEKLY_LIVE",
		"RequireWalletDat",
		"DOGEGO_WALLET_DAT_REQUIRED",
		"require_wallet_dat=true",
		"dogego cert enable-weekly",
		"dogego cert weekly-live",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("gh_enable_scheduled_live.ps1 missing %q", needle)
		}
	}
}

func TestMilestoneBCertShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "milestone_b_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"corruption_soak_cert.ps1",
		"corruption_extended_cert_mini.ps1",
		"ci_live_reboottestnet_gate.ps1",
		"corruption_long_soak_gate.ps1",
		"ibd_live_soak_gate.ps1",
		"ci_milestone_b_full_gate.ps1",
		"mainnet-ibd",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("milestone_b_cert.ps1 missing %q", needle)
		}
	}
}

func TestCIMilestoneBFullGateShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "ci_milestone_b_full_gate.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"ci_runner_preflight.ps1",
		"corruption_long_soak_gate.ps1",
		"verifychain",
		"reboottestnet-only",
		"dogego cert live-soak",
		"skip-scripts",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ci_milestone_b_full_gate.ps1 missing %q", needle)
		}
	}
}

func TestCiRunnerProvisionChecklistShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "ci_runner_provision_checklist.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"setup_reboottestnet_core_parity.ps1",
		"dogego cert setup-parity",
		"dogego cert provision",
		"DOGEGO_SCHEDULED_LIVE_SOAK",
		"DOGEGO_SCHEDULED_WEEKLY_LIVE",
		"provision_wallet_dat_fixture.ps1",
		"RunPreflight",
		"RunSetup",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ci_runner_provision_checklist.ps1 missing %q", needle)
		}
	}
}

func TestWalletMigrationCertShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "wallet_migration_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"SkipOffline",
		"RequireWalletDat",
		"_wallet_dat_env.ps1",
		"Initialize-WalletDatEnv",
		"dogego_importwalletdat",
		"dogego cert wallet-migration",
		"go run ./cmd/dogego cert wallet-migration -offline-only",
		"pool_count",
		"pool_index_min",
		"pool_pubkeys",
		"pool_keys_matched",
		"keypool_hint",
		"pool_indices_replayed",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("wallet_migration_cert.ps1 missing %q", needle)
		}
	}
}

func TestScheduledCorruptionSoakShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "ci_scheduled_corruption_soak.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"ci_milestone_b_full_gate.ps1",
		"dogego cert live-soak",
		"reboottestnet-only",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ci_scheduled_corruption_soak.ps1 missing %q", needle)
		}
	}
}

func TestScheduledWeeklyLiveShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "ci_scheduled_weekly_live.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"core_reboottestnet_core_aligned_gate.ps1",
		"corruption_extended_cert_mini.ps1",
		"ci_runner_preflight.ps1",
		"DOGEGO_CORE_COMPARE_MIN",
		"RequireWalletDat",
		"wallet_migration_cert.ps1",
		"SkipOffline",
		"Live wallet.dat migration (RPC import)",
		"dogego cert weekly-live",
		"skip-scripts",
		"workflow 10",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ci_scheduled_weekly_live.ps1 missing %q", needle)
		}
	}
}

func TestCorruptionExtendedCertMiniShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "corruption_extended_cert_mini.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"ibd_timed_soak.ps1",
		"corruption_timed_loop.ps1",
		"filter",
		"txindex",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("corruption_extended_cert_mini.ps1 missing %q", needle)
		}
	}
}

func TestCIOfflineGateIncludesWalletFastPath(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"ci_offline_gate.sh", "ci_offline_gate.ps1"} {
		raw, err := os.ReadFile(filepath.Join(root, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "dogego cert offline") {
			t.Fatalf("%s missing dogego cert offline delegation", name)
		}
		if strings.Contains(text, "go run ./cmd/dogego cert offline") {
			continue
		}
		if !strings.Contains(text, offlinegate.BootstrapCommandLine()) {
			t.Fatalf("%s missing canonical bootstrap %q", name, offlinegate.BootstrapCommandLine())
		}
		for _, needle := range offlinegate.Needles() {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing %q", name, needle)
			}
		}
		for _, s := range offlinegate.DefaultSuites() {
			found := false
			for _, needle := range offlinegate.SuiteScriptNeedles(s) {
				if strings.Contains(text, needle) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s missing suite %q (need one of %v)", name, s.Name, offlinegate.SuiteScriptNeedles(s))
			}
		}
	}
}

func TestPQCertScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "pq_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert pq",
		"go run ./cmd/dogego cert pq",
		"no production PQ safety",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("pq_cert.ps1 missing %q", needle)
		}
	}
}

func TestPQCertScriptSuitesDrift(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "pq_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "go run ./cmd/dogego cert pq") {
		t.Fatal("pq_cert.ps1 must delegate to cert pq")
	}
	for _, s := range pqcert.DefaultSuites() {
		for _, needle := range pqcert.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("pq_cert.ps1 should not duplicate suite %q after delegating to cert pq", s.Name)
			}
		}
	}
}

func TestPQCertShellShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "pq_cert.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert pq",
		"go run ./cmd/dogego cert pq",
		"no production PQ safety",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("pq_cert.sh missing %q", needle)
		}
	}
	for _, s := range pqcert.DefaultSuites() {
		for _, needle := range pqcert.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("pq_cert.sh should not duplicate suite %q after delegating to cert pq", s.Name)
			}
		}
	}
}

func TestWalletMigrationCertOfflineDelegationDrift(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "wallet_migration_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "go run ./cmd/dogego cert wallet-migration -offline-only") {
		t.Fatal("wallet_migration_cert.ps1 must delegate offline block to cert wallet-migration -offline-only")
	}
	for _, s := range walletmigration.DefaultOfflineSuites() {
		for _, needle := range walletmigration.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				t.Fatalf("wallet_migration_cert.ps1 should not duplicate suite %q after delegating offline block", s.Name)
			}
		}
	}
}

func TestOperatorWorkflowCertScriptSuitesDrift(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "operator_workflow_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "go run ./cmd/dogego cert operator") {
		t.Fatal("operator_workflow_cert.ps1 must delegate to cert operator")
	}
	for _, s := range operatorworkflow.DefaultCoreSuites() {
		found := false
		for _, needle := range operatorworkflow.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		t.Fatalf("operator_workflow_cert.ps1 should not duplicate suite %q after delegating to cert operator", s.Name)
	}
}

func TestFieldEvidenceCertShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "field_evidence_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"UPDATE_CORE_TESTDATA",
		"dogego cert field-evidence",
		"go run ./cmd/dogego cert field-evidence",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("field_evidence_cert.ps1 missing %q", needle)
		}
	}
	// Default path delegates to dogego cert field-evidence; suites live in fieldevidence/suites.go.
	for _, s := range fieldevidence.DefaultSuites() {
		found := false
		for _, needle := range fieldevidence.SuiteScriptNeedles(s) {
			if strings.Contains(text, needle) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if strings.Contains(text, "go run ./cmd/dogego cert field-evidence") {
			continue
		}
		t.Fatalf("field_evidence_cert.ps1 missing suite %q (need cert field-evidence or one of %v)", s.Name, fieldevidence.SuiteScriptNeedles(s))
	}
}

func TestCoreWalletWorkflowShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_wallet_workflow.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego_probewalletdat",
		"wallet.dat probe:",
		".pool_count",
		"pool_index_min",
		"pool_pubkeys",
		"pool_keys_matched",
		"pool_keys_unmatched",
		"walletdat_probe_pool_keys_unmatched",
		"keypool_hint",
		"pool_indices_replayed",
		"address_book_keypool_count",
		"iskeypool",
		"hd_keypool_core_index",
		"getaddressinfo",
		"keypool_validateaddress_ok",
		"keypool_getaddressinfo_ok",
		"keypool_validateaddress_mismatch",
		"pool_core_indices_count_mismatch",
		"pool_unmatched_hint",
		"wallet_pq_send_ok",
		"wallet_pq_send_pending",
		"Get-PqCommitmentTagFromTxHex",
		"wallet_listtransactions_utxo_walk",
		"wallet_history_fast_path",
		"wallet_scan_index_ok",
		"dogego_wallet_listtransactions_utxo_walk",
		"wallet_scan_building_index",
		"wallet_listtransactions_scan_pending",
		"wallet_history_defer_reason",
		"wallet_history_deferred",
		"listtransactions_skipped: wallet_history_deferred",
		"getblockchaininfo",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_wallet_workflow.ps1 missing %q", needle)
		}
	}
}

func TestExportMainnetFieldScriptsBootstrap(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{
		"export_mainnet_field_blocks.ps1",
		"export_mainnet_field_headers.ps1",
		"export_mainnet_field_auxpow.ps1",
	} {
		raw, err := os.ReadFile(filepath.Join(root, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, offlinegate.BootstrapCommandLine()) {
			t.Fatalf("%s missing canonical bootstrap %q", name, offlinegate.BootstrapCommandLine())
		}
	}
}

func TestCoreBip152ProbeScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_bip152_probe.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"getpeerinfo",
		"bip152_hb_to",
		"bip152_hb_from",
		"no_bip152_hb_negotiated",
		"/api/core-bip152-probe",
		"cmpct_relay_schema_ok",
		"cmpct_relay_counters_missing",
		"dogego_cmpct_reconstruct_fallback_getdata",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_bip152_probe.ps1 missing %q", needle)
		}
	}
}

func TestBip152LiveSoakGateScriptShape(t *testing.T) {
	root := repoRoot(t)
	rawTimed, err := os.ReadFile(filepath.Join(root, "scripts", "bip152_timed_soak.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	timed := string(rawTimed)
	for _, needle := range []string{
		"core_bip152_probe.ps1",
		"cmpct_relay_schema_ok",
		"RequireRelayActivity",
	} {
		if !strings.Contains(timed, needle) {
			t.Fatalf("bip152_timed_soak.ps1 missing %q", needle)
		}
	}
	rawGate, err := os.ReadFile(filepath.Join(root, "scripts", "bip152_live_soak_gate.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	gate := string(rawGate)
	for _, needle := range []string{
		"bip152_timed_soak.ps1",
		"DOGEGO_BIP152_LIVE_SOAK",
	} {
		if !strings.Contains(gate, needle) {
			t.Fatalf("bip152_live_soak_gate.ps1 missing %q", needle)
		}
	}
	rawRunner, err := os.ReadFile(filepath.Join(root, "runner", "bip152_soak.go"))
	if err != nil {
		t.Fatal(err)
	}
	runner := string(rawRunner)
	for _, needle := range []string{
		"RunBip152Soak",
		"bip152_live_soak_gate.ps1",
		"offline_auxpow_cmpct_edges_ok",
		"TestReconstructBlockFromCmpct_rejectsAuxpow",
	} {
		if !strings.Contains(runner, needle) {
			t.Fatalf("runner/bip152_soak.go missing %q", needle)
		}
	}
	rawCert, err := os.ReadFile(filepath.Join(root, "cmd", "dogego", "cert_bip152_soak.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawCert), "runCertBip152Soak") {
		t.Fatal("cert_bip152_soak.go missing runCertBip152Soak")
	}
}

func TestCoreEndToEndWorkflowScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_end_to_end_workflow.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		`Step "protocol_lock"`,
		"core_protocol_lock_probe.ps1",
		`Step "offline_corpus"`,
		"core_mempool_corpus_probe.ps1",
		`Step "bip125_offline"`,
		"core_mempool_bip125_offline_probe.ps1",
		"BIP125 rule 2/5 rows",
		`Step "mempool_parity"`,
		"core_mempool_parity_probe.ps1",
		"-WebProbe",
		`Step "setup_parity"`,
		"setup_reboottestnet_core_parity.ps1",
		`Step "bip152_hb"`,
		"core_bip152_probe.ps1",
		`Step "addrman"`,
		"core_addrman_workflow.ps1",
		`Step "reindex_check"`,
		`Step "node_health"`,
		`wallet_basics"`,
		"wallet_pq_send_ok",
		"listtransactions_40=",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_end_to_end_workflow.ps1 missing %q", needle)
		}
	}
}

func TestCoreOperatorWorkflowCertProtocolLock(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_operator_workflow_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"core_protocol_lock_probe.ps1",
		"[protocol-lock]",
		"core_mempool_corpus_probe.ps1",
		"core_mempool_bip125_offline_probe.ps1",
		"Milestone D mempool corpus",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_operator_workflow_cert.ps1 missing %q", needle)
		}
	}
}

func TestCoreOperatorWorkflowCertAddrman(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_operator_workflow_cert.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"DOGEGO_ADDRMAN_PROBE",
		"core_addrman_workflow.ps1",
		"[addrman]",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_operator_workflow_cert.ps1 missing %q", needle)
		}
	}
}

func TestCoreMempoolBIP125OfflineProbeScriptShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_mempool_bip125_offline_probe.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"rbf_too_many_conflicts",
		"rbf_new_unconfirmed_input",
		"TestCoreMempoolDifferentialVectors/",
		"-Json",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_mempool_bip125_offline_probe.ps1 missing %q", needle)
		}
	}
}

func TestCertGoRunnerSubcommands(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "cmd", "dogego", "cert.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, sub := range []string{
		`case "provision":`,
		`case "preflight":`,
		`case "weekly":`,
		`case "weekly-live":`,
		`case "live-soak":`,
		`case "bip152-soak"`,
		`case "enable-weekly":`,
		`case "workflow10"`,
		`case "setup-parity":`,
		`case "mining":`,
		"seventeen live operator-cert gates",
		"workflow 10",
	} {
		if !strings.Contains(text, sub) {
			t.Fatalf("cmd/dogego/cert.go missing %q", sub)
		}
	}
}

func TestCoreAddrmanWorkflowShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_addrman_workflow.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"getaddrmaninfo",
		"getblockchaininfo",
		"dogego_buckets",
		"bucket_schema_ok",
		"addrman_n_key_persisted",
		"addrman_partial",
		"GET /api/core-addrman-probe",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_addrman_workflow.ps1 missing %q", needle)
		}
	}
}

func TestCoreMiningWorkflowShape(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "core_mining_workflow.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"getmininginfo",
		"getblocktemplate",
		"createauxblock",
		"longpollid",
		"GET /api/core-mining-probe",
		"-WebProbe",
		"-DogeGoOnly",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("core_mining_workflow.ps1 missing %q", needle)
		}
	}
}
