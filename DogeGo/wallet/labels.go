// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"fmt"
	"sort"
	"strings"
)

// SetLabel assigns a display label to an address tracked by this wallet (empty removes).
func (w *Disk) SetLabel(addr, label string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("empty address")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.labels == nil {
		w.labels = make(map[string]string)
	}
	label = strings.TrimSpace(label)
	if label == "" {
		delete(w.labels, addr)
		return w.saveLocked()
	}
	w.labels[addr] = label
	return w.saveLocked()
}

// Label returns the label for addr, or "" if unset.
func (w *Disk) Label(addr string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.labels == nil {
		return ""
	}
	return w.labels[strings.TrimSpace(addr)]
}

// ListLabels returns sorted unique non-empty labels.
func (w *Disk) ListLabels() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.labels) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	for _, lbl := range w.labels {
		lbl = strings.TrimSpace(lbl)
		if lbl == "" {
			continue
		}
		seen[lbl] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for lbl := range seen {
		out = append(out, lbl)
	}
	sort.Strings(out)
	return out
}

func (w *Disk) loadLabels(m map[string]string) {
	w.labels = nil
	if len(m) == 0 {
		return
	}
	w.labels = make(map[string]string, len(m))
	for addr, lbl := range m {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		w.labels[addr] = strings.TrimSpace(lbl)
	}
}
