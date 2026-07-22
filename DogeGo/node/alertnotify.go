// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"unicode"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
)

// alertNotifyState deduplicates chain-warning notifications (Core -alertnotify).
type alertNotifyState struct {
	mu   sync.Mutex
	last string
}

func (s *alertNotifyState) maybeNotify(cmd string, skipIBD bool, warnings []string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" || skipIBD || len(warnings) == 0 {
		return
	}
	msg := strings.Join(warnings, "; ")
	s.mu.Lock()
	if msg == s.last {
		s.mu.Unlock()
		return
	}
	s.last = msg
	s.mu.Unlock()
	go func() {
		if err := RunAlertNotify(cmd, msg); err != nil {
			applog.Line("alert", "alertnotify: "+err.Error())
		}
	}()
}

// RunAlertNotify executes a Core-style -alertnotify command with %s replaced by a sanitized message.
func RunAlertNotify(cmd, message string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	safe := sanitizeAlertMessage(message)
	quoted := "'" + strings.ReplaceAll(safe, "'", `'"'"'`) + "'"
	cmd = strings.ReplaceAll(cmd, "%s", quoted)
	// #nosec G204 - operator-configured shell hook (same as Core -alertnotify).
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/C", cmd)
	} else {
		c = exec.Command("sh", "-c", cmd)
	}
	return c.Run()
}

func sanitizeAlertMessage(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' {
			b.WriteByte(' ')
			continue
		}
		if r < 32 || r == 127 || !unicode.IsPrint(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func pollChainAlertNotify(cmd string, skipIBD bool, j consensus.HeaderChain, net chain.Network, st *alertNotifyState) {
	if st == nil || strings.TrimSpace(cmd) == "" || j == nil {
		return
	}
	st.maybeNotify(cmd, skipIBD, consensus.ChainWarnings(j, net))
}
