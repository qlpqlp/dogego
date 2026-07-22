// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"path/filepath"
	"strings"

	"dogego/config"
)

func buildDualInstanceConfigs(base config.File, dataDir string) (mainnet, testnet config.File, instances config.InstancesFile, mainConfPath, testConfPath string, err error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return mainnet, testnet, instances, "", "", err
	}
	if abs, resolveErr := config.ResolveDataDir(dataDir); resolveErr == nil && abs != "" {
		dataDir = abs
	}
	mainnet = base
	mainnet.DataDir = dataDir
	mainnet.Network = "mainnet"
	mainnet.WebUI = config.DualMainnetWebUI
	mainnet.Mine = false
	mainnet.RPCAddr = config.DefaultRPCListenAddr("mainnet")

	testnet = base
	testnet.DataDir = dataDir
	testnet.Network = "testnet"
	testnet.WebUI = config.DualTestnetWebUI
	config.ApplyTestnetAutoMine(&testnet)
	testnet.RPCAddr = config.DefaultRPCListenAddr("testnet")

	mainConfPath = filepath.Join(dataDir, config.DualMainnetConfName)
	testConfPath = filepath.Join(dataDir, config.DualTestnetConfName)

	instances = config.InstancesFile{
		Instances: []config.InstanceEntry{
			{Network: "mainnet", WebUI: mainnet.WebUI, ConfPath: mainConfPath, Label: "Mainnet"},
			{Network: "testnet", WebUI: testnet.WebUI, ConfPath: testConfPath, Label: "Testnet"},
		},
	}
	return mainnet, testnet, instances, mainConfPath, testConfPath, nil
}

func instancesForAPI(dataDir, currentNetwork string) []map[string]any {
	inst, err := config.LoadInstances(dataDir)
	if err != nil || len(inst.Instances) == 0 {
		return nil
	}
	cur := strings.ToLower(strings.TrimSpace(currentNetwork))
	out := make([]map[string]any, 0, len(inst.Instances))
	for _, e := range inst.Instances {
		url := dashboardURLFromWebUI(e.WebUI, false)
		netSlug := strings.ToLower(strings.TrimSpace(e.Network))
		row := map[string]any{
			"network":  e.Network,
			"webui":    e.WebUI,
			"url":      url,
			"label":    e.Label,
			"current":  netSlug == cur,
			"conf_path": e.ConfPath,
		}
		out = append(out, row)
	}
	return out
}

func dashboardURLFromWebUI(webui string, https bool) string {
	webui = strings.TrimSpace(webui)
	scheme := "http"
	if https {
		scheme = "https"
	}
	if webui == "" {
		return scheme + "://" + config.DefaultWebUIListen + "/"
	}
	if strings.HasPrefix(webui, "http://") || strings.HasPrefix(webui, "https://") {
		if strings.HasSuffix(webui, "/") {
			return webui
		}
		return webui + "/"
	}
	return scheme + "://" + webui + "/"
}
