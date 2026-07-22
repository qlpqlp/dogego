// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
)

// ResolveOpenConfig picks the config file for dogego open / dogecoin:// handling.
// When rawURL is an http(s) dashboard URL, dual-mode instances.json can map the port
// to dogecoinconf.mainnet.json vs dogecoinconf.testnet.json.
func ResolveOpenConfig(rawURL string) (File, string) {
	f, path := LoadFirst()
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return f, path
	}
	lower := strings.ToLower(rawURL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return f, path
	}
	port := portFromListenURL(rawURL)
	if port == "" {
		return f, path
	}
	dataDir := strings.TrimSpace(f.DataDir)
	if dataDir == "" {
		return f, path
	}
	inst, err := LoadInstances(dataDir)
	if err != nil || len(inst.Instances) == 0 {
		return f, path
	}
	for _, e := range inst.Instances {
		if !portMatchesWebUI(e.WebUI, port) {
			continue
		}
		confPath := strings.TrimSpace(e.ConfPath)
		if confPath == "" {
			continue
		}
		if cf, err := loadConfigFile(confPath); err == nil {
			return cf, confPath
		}
	}
	return f, path
}

func loadConfigFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var c File
	if err := json.Unmarshal(b, &c); err != nil {
		return File{}, err
	}
	normalizeLegacyUserAgent(&c, b)
	return c, nil
}

func portFromListenURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Port())
}

func portMatchesWebUI(webui, port string) bool {
	webui = strings.TrimSpace(webui)
	port = strings.TrimSpace(port)
	if port == "" {
		return false
	}
	if p := portFromListenURL("http://" + webui); p != "" {
		return p == port
	}
	if strings.Contains(webui, ":") {
		_, after, ok := strings.Cut(webui, ":")
		if ok {
			return after == port
		}
	}
	return webui == port
}
