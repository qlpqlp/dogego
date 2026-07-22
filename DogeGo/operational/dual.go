// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package operational

import (
	"path/filepath"
	"strings"

	"dogego/config"
)

// InstanceResult is operational verify for one dual-run instance.
type InstanceResult struct {
	Label    string       `json:"label,omitempty"`
	Network  string       `json:"network"`
	ConfPath string       `json:"conf_path,omitempty"`
	WebUI    string       `json:"webui,omitempty"`
	Verify   VerifyResult `json:"verify"`
}

// DualVerifyResult reports mainnet + reboot testnet side-by-side readiness.
type DualVerifyResult struct {
	OK        bool             `json:"ok"`
	DataDir   string           `json:"datadir,omitempty"`
	Instances []InstanceResult `json:"instances"`
	Issues    []string         `json:"issues,omitempty"`
	Warnings  []string         `json:"warnings,omitempty"`
	NextSteps []string         `json:"next_steps,omitempty"`
	Doc       string           `json:"doc,omitempty"`
}

// VerifyDual loads instances.json (or default dual conf paths) and verifies each network.
func VerifyDual(dataDir string) DualVerifyResult {
	out := DualVerifyResult{
		Doc: "docs/MAINNET_TESTNET_OPERATIONAL.md § Dual run",
	}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		if d, err := config.PreferredSaveDir(); err == nil {
			dataDir = d
		}
	}
	if abs, err := config.ResolveDataDir(dataDir); err == nil && abs != "" {
		dataDir = abs
	}
	out.DataDir = dataDir

	inst, err := config.LoadInstances(dataDir)
	if err != nil {
		out.Issues = append(out.Issues, "instances_load: "+err.Error())
		return out
	}

	type pair struct {
		label, network, confName string
		webui                    string
	}
	want := []pair{
		{"Mainnet", "mainnet", config.DualMainnetConfName, config.DualMainnetWebUI},
		{"Reboot testnet", "testnet", config.DualTestnetConfName, config.DualTestnetWebUI},
	}

	found := map[string]bool{}
	for _, e := range inst.Instances {
		confPath := strings.TrimSpace(e.ConfPath)
		if confPath == "" {
			continue
		}
		f, err := config.LoadFile(confPath)
		if err != nil {
			out.Issues = append(out.Issues, "read_conf: "+confPath+": "+err.Error())
			continue
		}
		if strings.TrimSpace(f.DataDir) == "" {
			f.DataDir = dataDir
		}
		vr := Verify(f)
		net := strings.TrimSpace(e.Network)
		if net == "" {
			net = strings.TrimSpace(f.Network)
		}
		found[net] = true
		out.Instances = append(out.Instances, InstanceResult{
			Label:    strings.TrimSpace(e.Label),
			Network:  net,
			ConfPath: confPath,
			WebUI:    strings.TrimSpace(e.WebUI),
			Verify:   vr,
		})
		if !vr.OK {
			out.Issues = append(out.Issues, net+": operational checks failed")
		}
		out.Warnings = append(out.Warnings, vr.Warnings...)
	}

	if len(out.Instances) == 0 {
		for _, w := range want {
			confPath := filepath.Join(dataDir, w.confName)
			f, err := config.LoadFile(confPath)
			if err != nil {
				out.Issues = append(out.Issues, "missing_conf: "+confPath)
				continue
			}
			f.Network = w.network
			if strings.TrimSpace(f.DataDir) == "" {
				f.DataDir = dataDir
			}
			if strings.TrimSpace(f.WebUI) == "" {
				f.WebUI = w.webui
			}
			vr := Verify(f)
			out.Instances = append(out.Instances, InstanceResult{
				Label:    w.label,
				Network:  w.network,
				ConfPath: confPath,
				WebUI:    f.WebUI,
				Verify:   vr,
			})
			found[w.network] = true
			if !vr.OK {
				out.Issues = append(out.Issues, w.network+": operational checks failed")
			}
		}
		if len(out.Instances) == 0 {
			out.Issues = append(out.Issues, "dual_not_configured")
			out.NextSteps = []string{
				"Setup wizard: profile Dual mainnet + reboot testnet",
				"Or create dogecoinconf.mainnet.json + dogecoinconf.testnet.json under datadir",
			}
			return out
		}
	}

	for _, w := range want {
		if !found[w.network] {
			out.Warnings = append(out.Warnings, "missing_instance: "+w.network)
		}
	}

	out.OK = len(out.Issues) == 0
	if out.OK {
		out.NextSteps = []string{
			"Mainnet dashboard: http://127.0.0.1:2013/",
			"Testnet dashboard: http://127.0.0.1:2014/",
			"Start both: tray Open Dashboard or dogego node per conf file",
			"Testnet founder: dogego cert founder -conf dogecoinconf.testnet.json",
			"Mainnet IBD: dogego cert ibd-convergence -conf dogecoinconf.mainnet.json",
		}
	}
	return out
}
