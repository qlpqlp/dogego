// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

import (
	"strings"
	"sync"
)

// UserAlert is exposed to the web UI and native dialogs when firewall rules are missing.
type UserAlert struct {
	Active          bool     `json:"active"`
	Severity        string   `json:"severity"` // warn | ok
	Title           string   `json:"title"`
	Message         string   `json:"message"`
	ManualCommands  []string `json:"manual_commands,omitempty"`
	RulesPresent    bool     `json:"rules_present"`
	Platform        string   `json:"platform"`
	Port            int      `json:"port"`
	ExePath         string   `json:"exe_path,omitempty"`
	ElevationOffered bool    `json:"elevation_offered"`
}

var alertMu sync.RWMutex
var lastAlert UserAlert

// PublishResult records the outcome of Ensure for UI and optional native notify.
func PublishResult(cfg Config, res Result) {
	a := UserAlert{
		Active:       false,
		Severity:     "ok",
		Title:        "Firewall",
		Platform:     res.Platform,
		Port:         cfg.Port,
		ExePath:      cfg.ExePath,
		RulesPresent: Present(cfg),
	}
	if res.OK || res.AlreadyOK {
		a.Message = "P2P firewall rules are in place."
		if res.UserMessage != "" {
			a.Message = res.UserMessage
		}
	} else if cfg.Mode != ModeNever {
		a.Active = true
		a.Severity = "warn"
		a.Title = "Firewall rules required for P2P"
		a.ManualCommands = ManualCommands(cfg)
		a.Message = buildAlertMessage(cfg, res)
	}
	alertMu.Lock()
	lastAlert = a
	alertMu.Unlock()
}

func buildAlertMessage(cfg Config, res Result) string {
	var b strings.Builder
	if res.UserMessage != "" {
		b.WriteString(res.UserMessage)
	} else if res.Err != nil {
		b.WriteString(res.Err.Error())
	}
	if !Present(cfg) {
		b.WriteString("\n\nDogeGo could not add OS firewall rules automatically.")
		b.WriteString("\nOther programs show an Administrator prompt for this; if you dismissed it or run from a service account, add the rules manually (commands below).")
	}
	if cfg.Mode == ModeAuto {
		b.WriteString("\n\nUntil rules exist, peers may disconnect (e.g. “connection aborted by the software on your host machine”).")
	}
	if thirdPartyNote := ThirdPartyFirewallNote(); thirdPartyNote != "" {
		b.WriteString("\n\n")
		b.WriteString(thirdPartyNote)
	}
	return strings.TrimSpace(b.String())
}

// UserAlertSnapshot returns the latest firewall alert for /api/summary.
func UserAlertSnapshot() UserAlert {
	alertMu.RLock()
	defer alertMu.RUnlock()
	return lastAlert
}

// ManualCommands returns one command per line for UI copy-paste.
func ManualCommands(cfg Config) []string {
	text := ManualInstructions(cfg)
	if text == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Run in") || strings.HasPrefix(line, "Linux") || strings.HasPrefix(line, "macOS") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// ThirdPartyFirewallNote is platform-specific extra guidance (AV suites, etc.).
func ThirdPartyFirewallNote() string {
	return thirdPartyFirewallNotePlatform()
}
