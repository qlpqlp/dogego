// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// ServiceRow is one runtime subsystem for GET /api/services.
type ServiceRow struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Running     bool     `json:"running"`
	Detail      string   `json:"detail,omitempty"`
	CanStart    bool     `json:"can_start"`
	CanStop     bool     `json:"can_stop"`
	Actions     []string `json:"actions,omitempty"`
	RestartNote string   `json:"restart_note,omitempty"`
}

// ServiceController exposes in-process start/stop from the dashboard (loopback only).
type ServiceController interface {
	ServiceRows() []ServiceRow
	ApplyServiceAction(id, action string) error
	AnalyticsRunning() bool
}
