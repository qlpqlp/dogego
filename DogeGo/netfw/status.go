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
	Active           bool     `json:"active"`
	Severity         string   `json:"severity"` // warn | ok
	Title            string   `json:"title"`
	Message          string   `json:"message"`
	CopyHint         string   `json:"copy_hint,omitempty"`
	ManualCommands   []string `json:"manual_commands,omitempty"`
	ManualNotes      []string `json:"manual_notes,omitempty"`
	RulesPresent     bool     `json:"rules_present"`
	Platform         string   `json:"platform"`
	Port             int      `json:"port"`
	ExePath          string   `json:"exe_path,omitempty"`
	ElevationOffered bool     `json:"elevation_offered"`
	Dismissed        bool     `json:"dismissed"`
}

var alertMu sync.RWMutex
var lastAlert UserAlert
var alertDismissed bool
var alertDismissPort int

// DismissUserAlert hides the dashboard firewall banner until rules change or the process restarts
// with a different P2P port. Operators can still open Settings → OS firewall later.
func DismissUserAlert() {
	alertMu.Lock()
	defer alertMu.Unlock()
	alertDismissed = true
	alertDismissPort = lastAlert.Port
	lastAlert.Dismissed = true
	lastAlert.Active = false
}

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
		CopyHint:     CopyHint(cfg),
	}
	if res.OK || res.AlreadyOK {
		a.Message = "P2P firewall rules are in place."
		if res.UserMessage != "" {
			a.Message = res.UserMessage
		}
		alertMu.Lock()
		alertDismissed = false
		lastAlert = a
		alertMu.Unlock()
		return
	}
	if cfg.Mode != ModeNever {
		a.Active = true
		a.Severity = "warn"
		a.Title = "Firewall rules required for P2P"
		a.ManualCommands, a.ManualNotes = ManualCommandsAndNotes(cfg)
		a.Message = buildAlertMessage(cfg, res)
	}
	alertMu.Lock()
	if alertDismissed && alertDismissPort == cfg.Port && a.Active {
		a.Dismissed = true
		a.Active = false
	}
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
		b.WriteString("\nIf you dismissed an admin prompt, or run under a service/container account, add the rules manually using the commands below for this OS.")
	}
	if cfg.Mode == ModeAuto {
		b.WriteString("\n\nUntil rules exist, peers may disconnect (e.g. “connection aborted by the software on your host machine”).")
	}
	if thirdPartyNote := ThirdPartyFirewallNote(); thirdPartyNote != "" {
		b.WriteString("\n\n")
		b.WriteString(thirdPartyNote)
	}
	b.WriteString("\n\nYou can Dismiss this banner if a gateway or host firewall already allows P2P (common on DogeBox).")
	return strings.TrimSpace(b.String())
}

// UserAlertSnapshot returns the latest firewall alert for /api/summary.
func UserAlertSnapshot() UserAlert {
	alertMu.RLock()
	defer alertMu.RUnlock()
	return lastAlert
}

// CopyHint is the OS-specific line above the command list in the dashboard.
func CopyHint(cfg Config) string {
	switch platformName() {
	case "windows":
		return "Copy into an elevated Command Prompt or PowerShell (Run as administrator):"
	case "darwin":
		return "Copy into Terminal (enter your Mac admin password when sudo asks):"
	case "linux":
		return "Copy into a terminal (use sudo when prompted). Pick ufw or firewalld:"
	default:
		return "Copy the commands below into a terminal with admin rights:"
	}
}

// ManualCommandsAndNotes returns executable lines plus short operator notes for the UI.
func ManualCommandsAndNotes(cfg Config) (cmds []string, notes []string) {
	text := ManualInstructions(cfg)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			notes = append(notes, strings.TrimSpace(strings.TrimPrefix(line, "#")))
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "run in") || strings.HasPrefix(lower, "linux") || strings.HasPrefix(lower, "macos") || strings.HasPrefix(lower, "windows") {
			notes = append(notes, line)
			continue
		}
		cmds = append(cmds, line)
	}
	if len(cmds) == 0 && text != "" {
		cmds = append(cmds, strings.TrimSpace(text))
	}
	return cmds, notes
}

// ManualCommands returns one command per line for UI copy-paste.
func ManualCommands(cfg Config) []string {
	cmds, _ := ManualCommandsAndNotes(cfg)
	return cmds
}

// ThirdPartyFirewallNote is platform-specific extra guidance (AV suites, etc.).
func ThirdPartyFirewallNote() string {
	return thirdPartyFirewallNotePlatform()
}
