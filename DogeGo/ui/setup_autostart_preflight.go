// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"dogego/autostart"
)

type setupAutostartPreflightRequest struct {
	Autostart string `json:"autostart"`
}

func registerSetupAutostartPreflight(mux *http.ServeMux) {
	mux.HandleFunc("/api/setup/autostart-preflight", handleSetupAutostartPreflight)
}

func handleSetupAutostartPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req setupAutostartPreflightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp := buildSetupAutostartPreflight(req.Autostart)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func buildSetupAutostartPreflight(autostartVal string) setupPreflightResponse {
	want := strings.TrimSpace(autostartVal) == "login"
	if !want {
		return setupPreflightResponse{
			OK: true,
			Checks: []setupPreflightCheck{{
				ID: "autostart_skip", Status: "ok", Title: "Login autostart",
				Message: "Disabled - node starts only when you launch it manually",
			}},
		}
	}

	st := autostart.Current()
	checks := make([]setupPreflightCheck, 0, 3)

	if !st.Supported {
		checks = append(checks, setupPreflightCheck{
			ID: "platform", Status: "warn", Title: "OS support",
			Message: "Login autostart is not supported on this platform",
			Fix:     "Uncheck sign-in autostart or run the node manually after reboot",
		})
	} else {
		checks = append(checks, setupPreflightCheck{
			ID: "platform", Status: "ok", Title: "OS support",
			Message: "Platform supports login autostart (" + strings.TrimSpace(st.Platform) + ")",
		})
	}

	if st.Installed {
		checks = append(checks, setupPreflightCheck{
			ID: "registration", Status: "ok", Title: "OS registration",
			Message: "Autostart entry already registered - will be updated on save",
		})
	} else {
		checks = append(checks, setupPreflightCheck{
			ID: "registration", Status: "warn", Title: "OS registration",
			Message: "Not registered yet - Save & start will create the OS login entry",
			Fix:     "After save, verify with dogego cert autostart or Features → OS login autostart probe",
		})
	}

	checks = append(checks, setupPreflightCheck{
		ID: "headless_hint", Status: "ok", Title: "Headless Linux",
		Message: "On headless Linux you may need: loginctl enable-linger $USER",
	})

	return setupPreflightResponse{OK: true, Checks: checks}
}