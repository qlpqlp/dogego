// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"dogego/chain"
	"dogego/netfw"
	"dogego/netfw/cgnat"
)

type setupPreflightRequest struct {
	Network         string `json:"network"`
	NodeMode        string `json:"node_mode"`
	P2PConnectivity string `json:"p2p_connectivity"`
	Firewall        string `json:"firewall"`
	UPnP            string `json:"upnp"`
	WebUI           string `json:"webui"`
	RPC             string `json:"rpc"`
	SetupOrigin     string `json:"setup_origin"`
}

type setupPreflightCheck struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"` // ok | warn | err
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Fix      string   `json:"fix,omitempty"`
	Commands []string `json:"commands,omitempty"`
}

type setupPreflightResponse struct {
	OK          bool                  `json:"ok"`
	LikelyCGNAT bool                  `json:"likely_cgnat"`
	Checks      []setupPreflightCheck `json:"checks"`
}

func registerSetupPreflight(mux *http.ServeMux) {
	mux.HandleFunc("/api/setup/preflight", handleSetupPreflight)
}

func handleSetupPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req setupPreflightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp := buildSetupPreflight(r.Context(), req)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func buildSetupPreflight(ctx context.Context, req setupPreflightRequest) setupPreflightResponse {
	var checks []setupPreflightCheck
	p2pPort := p2pPortForSetupNetwork(req.Network)
	inbound := p2pInboundForConnectivity(req.P2PConnectivity)
	fwMode := netfw.ParseMode(req.Firewall)

	checks = append(checks, portBindCheck("webui", req.WebUI, "Web dashboard", req.SetupOrigin))
	if strings.TrimSpace(req.RPC) != "" {
		checks = append(checks, portBindCheck("rpc", req.RPC, "JSON-RPC", ""))
	}
	checks = append(checks, p2pPortConflictCheck(p2pPort, req.WebUI))

	if fwMode != netfw.ModeNever {
		checks = append(checks, firewallPreflightCheck(p2pPort, inbound, fwMode))
	} else {
		checks = append(checks, setupPreflightCheck{
			ID:      "firewall",
			Status:  "warn",
			Title:   "OS firewall",
			Message: "Firewall rules are disabled in config (never). Peers may be blocked by Windows/macOS/Linux unless you manage rules yourself.",
			Fix:     "Set firewall to auto or always in the next step, or add manual allow rules for DogeGo and P2P port " + strconv.Itoa(p2pPort) + ".",
		})
	}

	checks = append(checks, cgnatPreflightCheck(p2pPort, req.P2PConnectivity, req.UPnP, inbound))
	likely := cgnat.Likely(ctx, req.P2PConnectivity, req.UPnP, p2pPort)
	checks = append(checks, dgrWizardPreflightCheck(req.P2PConnectivity, likely))

	ok := true
	for _, c := range checks {
		if c.Status == "err" {
			ok = false
			break
		}
	}
	return setupPreflightResponse{OK: ok, LikelyCGNAT: likely, Checks: checks}
}

func p2pPortForSetupNetwork(network string) int {
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		return 22556
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return 22556
	}
	return p.Port
}

func p2pInboundForConnectivity(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "cgnat":
		return false
	case "classic":
		return true
	default:
		return true
	}
}

func splitHostPort(addr string, defaultPort int) (host string, port int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1", defaultPort
	}
	if strings.HasPrefix(addr, ":") {
		if p, err := strconv.Atoi(strings.TrimPrefix(addr, ":")); err == nil && p > 0 {
			return "", p
		}
		return "", defaultPort
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		if p, e := strconv.Atoi(addr); e == nil && p > 0 {
			return "127.0.0.1", p
		}
		return "127.0.0.1", defaultPort
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 {
		p = defaultPort
	}
	return host, p
}

func portBindCheck(id, addr, label, setupOrigin string) setupPreflightCheck {
	host, port := splitHostPort(addr, 0)
	if port <= 0 {
		return setupPreflightCheck{
			ID: id, Status: "warn", Title: label,
			Message: "No listen port configured.",
		}
	}
	if portMatchesSetupOrigin(port, setupOrigin) {
		return setupPreflightCheck{
			ID: id, Status: "ok", Title: label,
			Message: "This setup wizard is already open on port " + strconv.Itoa(port) + " - the dashboard will use the same port when the node starts.",
		}
	}
	if dogegoDashboardReachable(host, port) {
		return setupPreflightCheck{
			ID: id, Status: "ok", Title: label,
			Message: "DogeGo dashboard is already running on port " + strconv.Itoa(port) + ".",
		}
	}
	ln, err := net.Listen("tcp", bindProbeAddr(host, port))
	if err != nil {
		msg := err.Error()
		fix := "Choose a different port or stop the program already using " + strconv.Itoa(port) + "."
		status := "warn"
		if strings.Contains(strings.ToLower(msg), "permission") {
			status = "err"
			fix = "Binding to ports below 1024 may need administrator rights. Use a port ≥ 1024 (e.g. 2013 for web UI)."
		}
		return setupPreflightCheck{
			ID: id, Status: status, Title: label + " port " + strconv.Itoa(port),
			Message: label + " cannot bind to " + net.JoinHostPort(host, strconv.Itoa(port)) + ": " + msg,
			Fix:     fix,
		}
	}
	_ = ln.Close()
	return setupPreflightCheck{
		ID: id, Status: "ok", Title: label,
		Message: "Port " + strconv.Itoa(port) + " is free on this machine.",
	}
}

func p2pPortConflictCheck(p2pPort int, webUI string) setupPreflightCheck {
	host, webPort := splitHostPort(webUI, 2013)
	if dogegoDashboardReachable(host, webPort) {
		return setupPreflightCheck{
			ID: "p2p_port", Status: "ok",
			Title:   "P2P port " + strconv.Itoa(p2pPort),
			Message: "DogeGo is already running - P2P port " + strconv.Itoa(p2pPort) + " is in use by this node.",
		}
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(p2pPort))
	if err != nil {
		return setupPreflightCheck{
			ID: "p2p_port", Status: "err",
			Title:   "P2P port " + strconv.Itoa(p2pPort),
			Message: "Port " + strconv.Itoa(p2pPort) + " is already in use. Dogecoin Core or another node may be running.",
			Fix:     "Stop Dogecoin Core (or any other node on port " + strconv.Itoa(p2pPort) + ") before starting DogeGo. Two nodes cannot share the same P2P port.",
		}
	}
	_ = ln.Close()
	return setupPreflightCheck{
		ID: "p2p_port", Status: "ok",
		Title:   "P2P port " + strconv.Itoa(p2pPort),
		Message: "P2P port is free - DogeGo can listen when configured for inbound peers.",
	}
}

func firewallPreflightCheck(p2pPort int, inbound bool, mode netfw.Mode) setupPreflightCheck {
	exe, _ := os.Executable()
	cfg := netfw.DefaultConfig(p2pPort, inbound, true, mode)
	if exe != "" {
		cfg.ExePath = exe
	}
	if netfw.Present(cfg) {
		return setupPreflightCheck{
			ID: "firewall", Status: "ok", Title: "OS firewall",
			Message: "Firewall rules for DogeGo P2P are already present on this system.",
		}
	}
	cmds := netfw.ManualCommands(cfg)
	c := setupPreflightCheck{
		ID: "firewall", Status: "warn", Title: "OS firewall",
		Message: "DogeGo will try to add firewall rules on start. If you dismiss the Administrator prompt (or run as a service), peers may fail with “connection aborted by the software on your host machine”.",
		Fix:     "Allow the UAC/admin prompt on first start, or run these commands in an elevated shell:",
		Commands: cmds,
	}
	if mode == netfw.ModeAlways {
		c.Message = "Firewall mode is always - rules must exist before P2P works reliably."
	}
	return c
}

func cgnatPreflightCheck(p2pPort int, p2pMode, upnp string, inbound bool) setupPreflightCheck {
	mode := strings.ToLower(strings.TrimSpace(p2pMode))
	switch mode {
	case "cgnat":
		return setupPreflightCheck{
			ID: "cgnat", Status: "ok", Title: "CGNAT / Starlink mode",
			Message: "Outbound-only P2P - no inbound port forward required. DogeGo uses multiple outbound peers and block-assist during sync.",
			Fix:     "Keep UPnP disabled unless you later switch to classic/both with a public IP.",
		}
	case "classic", "both":
		if inbound {
			up := strings.ToLower(strings.TrimSpace(upnp))
			if up == "disable" || up == "disabled" || up == "0" || up == "false" {
				return setupPreflightCheck{
					ID: "cgnat", Status: "warn", Title: "Inbound P2P without UPnP",
					Message: "You chose inbound listening but UPnP is off. Home routers need a manual port-forward to TCP " + strconv.Itoa(p2pPort) + " unless you are on a public IP.",
					Fix:     "Forward the P2P port on your router, enable UPnP auto, or switch to cgnat mode if you are behind carrier-grade NAT (Starlink, mobile hotspot).",
				}
			}
			return setupPreflightCheck{
				ID: "cgnat", Status: "ok", Title: "Inbound + UPnP",
				Message: "DogeGo will try UPnP/NAT-PMP when listening. If mapping fails (CGNAT), sync still works via outbound peers.",
				Fix:     "If peers stay at zero after several minutes, switch P2P connectivity to cgnat in Settings → P2P.",
			}
		}
	}
	return setupPreflightCheck{
		ID: "cgnat", Status: "ok", Title: "Network mode",
		Message: "P2P connectivity looks appropriate for automatic peer discovery.",
	}
}

func dgrWizardPreflightCheck(p2pMode string, likelyCGNAT bool) setupPreflightCheck {
	mode := strings.ToLower(strings.TrimSpace(p2pMode))
	if mode != "both" && mode != "classic" && mode != "cgnat" {
		return setupPreflightCheck{
			ID: "dgr", Status: "ok", Title: "DogeGo relay (CGNAT)",
			Message: "DGR auto-config applies when P2P mode is both, classic, or cgnat.",
		}
	}
	if mode == "cgnat" {
		likelyCGNAT = true
	}
	if likelyCGNAT {
		return setupPreflightCheck{
			ID: "dgr", Status: "ok", Title: "DogeGo relay (auto)",
			Message: "CGNAT likely - wizard will enable outbound QUIC relay and skip public inbound relay (rdogego).",
			Fix:     "After connect, copy server_cert_sha256 from GET /api/dgr on your relay into relay_tls_pins (Settings → P2P) for pinned TLS trust.",
		}
	}
	return setupPreflightCheck{
		ID: "dgr", Status: "ok", Title: "DogeGo relay (auto)",
		Message: "Public IP likely - wizard will enable inbound rdogego relay (NODE_DOGEGO_RELAY_CGNAT) on UDP 24433.",
		Fix:     "Forward UDP 24433 on your router if you want other CGNAT nodes to reach this relay.",
	}
}

func bindProbeAddr(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" {
		return ":" + strconv.Itoa(port)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func portMatchesSetupOrigin(port int, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	h := u.Hostname()
	p := u.Port()
	if p == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			p = "443"
		default:
			p = "80"
		}
	}
	op, err := strconv.Atoi(p)
	if err != nil {
		return false
	}
	return op == port && (h == "127.0.0.1" || h == "localhost" || h == "::1" || h == "")
}

func dogegoDashboardReachable(host string, port int) bool {
	if port <= 0 {
		return false
	}
	hosts := probeHosts(host)
	seen := make(map[string]struct{}, len(hosts))
	client := &http.Client{Timeout: 2 * time.Second}
	for _, h := range hosts {
		if h == "" {
			continue
		}
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
			"http://"+net.JoinHostPort(h, strconv.Itoa(port))+"/api/capabilities", nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		ok := func() bool {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return false
			}
			var m map[string]any
			if json.NewDecoder(resp.Body).Decode(&m) != nil {
				return false
			}
			_, has := m["rpc_methods"]
			return has
		}()
		if ok {
			return true
		}
	}
	return false
}

func probeHosts(host string) []string {
	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0":
		return []string{"127.0.0.1", "localhost"}
	case "127.0.0.1", "localhost", "::1":
		return []string{host}
	default:
		return []string{host, "127.0.0.1"}
	}
}
