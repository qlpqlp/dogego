// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"fmt"
	"os"
	"strings"

	"dogego/offlinegate"
)

// MilestonesBDEResult reports offline-closeable portions of milestones B, D, and E.
type MilestonesBDEResult struct {
	OK       bool     `json:"ok"`
	Doc      string   `json:"doc,omitempty"`
	Milestones map[string]MilestoneBDESlice `json:"milestones"`
	Issues   []string `json:"issues,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

// MilestoneBDESlice is one certification milestone slice result.
type MilestoneBDESlice struct {
	ID          string   `json:"id"`
	OfflineOK   bool     `json:"offline_ok"`
	LivePending []string `json:"live_pending,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

// MilestonesBDEOptions configures RunMilestonesBDEOffline.
type MilestonesBDEOptions struct {
	ModuleRoot string
}

// RunMilestonesBDEOffline runs all code-closeable gates for milestones B, D, and E.
// Full milestone sign-off still requires dogego-live weekly-live + live-soak green runs.
func RunMilestonesBDEOffline(opts MilestonesBDEOptions) MilestonesBDEResult {
	r := MilestonesBDEResult{
		Doc: DogegoLiveWorkflow10Doc,
		Milestones: map[string]MilestoneBDESlice{
			"B": {ID: "milestone_b_full", LivePending: []string{
				"dogego cert live-soak on dogego-live (DOGEGO_SCHEDULED_LIVE_SOAK=1)",
				"multi-hour corruption_long_soak_gate green for consecutive weekly runs",
			}},
			"D": {ID: "milestone_d_full", LivePending: []string{
				"dogego cert weekly-live with Core reboottestnet (24/24 stateful compare)",
				"DOGEGO_SCHEDULED_WEEKLY_LIVE=1 on dogego-live runner",
			}},
			"E": {ID: "milestone_e_full", LivePending: []string{
				"seventeen live web operator-cert gates green with Core side-by-side",
				"dogego cert workflow10 green on dogego-live (or stepwise enable-weekly + provision + weekly-live)",
				"optional disruptive mainnet reindex/prune operator sign-off",
			}},
		},
	}
	root := strings.TrimSpace(opts.ModuleRoot)
	if root == "" {
		var err error
		root, err = FindModuleRoot()
		if err != nil {
			r.Issues = append(r.Issues, "module_root_missing")
			r.Notes = append(r.Notes, err.Error())
			return r
		}
	}

	if err := offlinegate.RunBootstrap(root, os.Stdout, os.Stderr); err != nil {
		r.Issues = append(r.Issues, "bootstrap_failed")
		r.Notes = append(r.Notes, err.Error())
		return r
	}

	bOK := runMilestoneSuites(root, milestoneBSuites())
	dOK := runMilestoneSuites(root, milestoneDSuites())
	eOK := runMilestoneSuites(root, milestoneESuites())

	r.Milestones["B"] = mergeSlice(r.Milestones["B"], bOK, "offline corruption + recovery gates")
	r.Milestones["D"] = mergeSlice(r.Milestones["D"], dOK, "offline mempool corpus + stateful map")
	r.Milestones["E"] = mergeSlice(r.Milestones["E"], eOK, "operator workflow + runner provision offline gates")

	r.OK = bOK && dOK && eOK
	if r.OK {
		r.Notes = append(r.Notes, "offline milestones B/D/E code gates passed; schedule dogego cert weekly-live + live-soak on dogego-live for full close")
	}
	return r
}

func mergeSlice(base MilestoneBDESlice, ok bool, note string) MilestoneBDESlice {
	base.OfflineOK = ok
	if note != "" {
		base.Notes = append(base.Notes, note)
	}
	return base
}

func runMilestoneSuites(root string, suites []offlinegate.Suite) bool {
	allOK := true
	for _, s := range suites {
		if err := RunGoTest(root, s.Args...); err != nil {
			allOK = false
		}
	}
	return allOK
}

func milestoneBSuites() []offlinegate.Suite {
	return []offlinegate.Suite{
		{Name: "store corruption recovery", Args: []string{"./store", "-run", "TestBundledTornTail|TestProbeBundled|TestRepairTxIndex|TestCrash|TestSubprocessKill", "-count=1", "-timeout", "5m"}},
		{Name: "node crash recovery", Args: []string{"./node", "-run", "TestAutoRecover|TestCrash|TestSubprocessKill|TestOperatorWorkflowStandaloneCertification", "-count=1", "-timeout", "5m"}},
		{Name: "runner live-soak wiring", Args: []string{"./runner", "-run", "TestRunLiveSoak|TestRunWeeklyLive|TestMilestonesBDE", "-count=1"}},
	}
}

func milestoneDSuites() []offlinegate.Suite {
	return []offlinegate.Suite{
		{Name: "mempool corpus", Args: []string{"./consensus", "-run", "TestEvalMempoolCorpus|TestStatefulMempool|TestCoreMempool", "-count=1", "-timeout", "5m"}},
		{Name: "setup parity runner", Args: []string{"./runner", "-run", "TestVerifySetupParity|TestApplySetupParity", "-count=1"}},
		{Name: "mempool UI probes", Args: []string{"./ui", "-run", "TestRunMempoolParity|TestRunMempoolStateful|TestProbeSetupParity", "-count=1", "-timeout", "120s"}},
	}
}

func milestoneESuites() []offlinegate.Suite {
	return []offlinegate.Suite{
		{Name: "operator workflow", Args: []string{"./node", "-run", "TestOperatorWorkflowStandaloneCertification", "-count=1", "-timeout", "5m"}},
		{Name: "operator cert UI", Args: []string{"./ui", "-run", "TestRunCoreOperatorCert|TestOperatorCert|TestEndToEndFromProbes|TestApplyCoreOperatorCert", "-count=1", "-timeout", "120s"}},
		{Name: "runner weekly wiring", Args: []string{"./runner", "-run", "TestRunPreflight|TestVerifyProvision|TestRunWeeklyLive|TestWorkflow10|TestRunWorkflow10", "-count=1"}},
		{Name: "founder + autostart", Args: []string{"./founder/...", "./autostart/...", "-count=1"}},
	}
}

// MilestonesBDESummaryLine returns a one-line status for CLI output.
func MilestonesBDESummaryLine(r MilestonesBDEResult) string {
	if !r.OK {
		return "milestones B/D/E offline: FAIL (see issues)"
	}
	return fmt.Sprintf("milestones B/D/E offline: PASS (live sign-off still required on dogego-live)")
}
