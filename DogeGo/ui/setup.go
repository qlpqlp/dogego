// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"dogego/chain"
	"dogego/config"
	"dogego/httptls"
	"dogego/netfw/cgnat"
)

func setupDashboardURL(f config.File) string {
	webui := strings.TrimSpace(f.WebUI)
	if webui == "" {
		webui = config.DefaultWebUIListen
	}
	scheme := "http"
	if f.WebUIHTTPS() {
		scheme = "https"
	}
	if strings.HasPrefix(webui, "http://") || strings.HasPrefix(webui, "https://") {
		if strings.HasSuffix(webui, "/") {
			return webui
		}
		return webui + "/"
	}
	return scheme + "://" + webui + "/"
}

// alignWebUIWithListen keeps a non-loopback wizard listen address in dogecoinconf.json.
// The setup form defaults to localhost:2013; without this, finishing setup on DogeBox
// (or any -webui pup-IP) would bind only loopback and break the reverse proxy.
func alignWebUIWithListen(listenAddr, webui string) string {
	webui = strings.TrimSpace(webui)
	listenAddr = strings.TrimSpace(listenAddr)
	if listenAddr == "" || listenHostIsLoopback(listenAddr) {
		if webui == "" {
			return config.DefaultWebUIListen
		}
		return webui
	}
	if webui == "" || webUIHostIsLoopback(webui) {
		return listenAddr
	}
	return webui
}

func listenHostIsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		host = strings.TrimSpace(addr)
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	case "0.0.0.0", "::", "::0", "*":
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func webUIHostIsLoopback(webui string) bool {
	webui = strings.TrimSpace(webui)
	if strings.HasPrefix(webui, "http://") || strings.HasPrefix(webui, "https://") {
		u := webui
		u = strings.TrimPrefix(u, "https://")
		u = strings.TrimPrefix(u, "http://")
		u = strings.TrimSuffix(u, "/")
		webui = u
	}
	return listenHostIsLoopback(webui)
}

// RunSetupWizard serves a local-only setup page until POST /api/setup succeeds or ctx is cancelled.
// savePath is where dogecoinconf.json will be written on success.
func RunSetupWizard(ctx context.Context, listenAddr string, seed config.File, savePath string, openBrowser bool) (config.File, error) {
	seed = config.SetupWizardSeed(seed)
	seed.WebUI = alignWebUIWithListen(listenAddr, seed.WebUI)
	html, err := fs.ReadFile(static, "static/setup.html")
	if err != nil {
		return config.File{}, err
	}
	tlsPair, err := setupWizardTLS(seed, listenAddr)
	if err != nil {
		return config.File{}, err
	}
	ln, scheme, err := httptls.Listen(listenAddr, tlsPair)
	if err != nil {
		return config.File{}, err
	}
	baseURL := publicDashboardURL(scheme, listenAddr, ln)
	if trustPrivateDashboardClients() {
		fmt.Fprintln(os.Stderr, "DogeGo setup: DOGEGO_TRUST_PRIVATE_CLIENTS  -  private/link-local clients treated as local (DogeBox)")
	}

	okCh := make(chan config.File, 1)

	mux := http.NewServeMux()
	registerSetupPreflight(mux)
	registerSetupFounderPreflight(mux)
	registerSetupAutostartPreflight(mux)
	registerSetupWalletBackup(mux)
	registerSetupUAComment(mux)
	registerUACommentPreview(mux)
	addBrandingRoutes(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
	})
	mux.HandleFunc("/api/setup/defaults", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(seed)
	})
	mux.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			config.File
			DualInstance          bool   `json:"dual_instance"`
			StartNode             *bool  `json:"start_node"`
			WalletBackupConfirmed bool   `json:"wallet_backup_confirmed"`
			WalletEncrypt         bool   `json:"wallet_encrypt"`
			WalletPassphrase      string `json:"wallet_passphrase"`
			DashboardPIN          string `json:"dashboard_pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		f := req.File
		if seed.NoTLS {
			config.DisableLocalTLS(&f)
		}
		if !req.DualInstance {
			f.WebUI = alignWebUIWithListen(listenAddr, f.WebUI)
		}
		startNode := true
		if req.StartNode != nil {
			startNode = *req.StartNode
		}
		if err := validateSetupDashboardPIN(req.DashboardPIN); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var primary config.File
		var autostartWarn string
		if req.DualInstance {
			mainnet, testnet, instances, mainPath, testPath, err := buildDualInstanceConfigs(f, f.DataDir)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if !f.NoWallet {
				if !req.WalletBackupConfirmed {
					http.Error(w, "download and confirm wallet backup before starting", http.StatusBadRequest)
					return
				}
				for _, net := range []string{"mainnet", "testnet"} {
					if _, err := ensureSetupWallet(f.DataDir, net); err != nil {
						http.Error(w, "wallet "+net+": "+err.Error(), http.StatusBadRequest)
						return
					}
					ok, err := setupWalletExists(f.DataDir, net)
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					if !ok {
						http.Error(w, "wallet backup not prepared for "+net+" - download backup on the finish step first", http.StatusBadRequest)
						return
					}
				}
				if err := applySetupUACommentTip(&mainnet); err != nil {
					http.Error(w, "mainnet uacomment: "+err.Error(), http.StatusBadRequest)
					return
				}
				if err := applySetupUACommentTip(&testnet); err != nil {
					http.Error(w, "testnet uacomment: "+err.Error(), http.StatusBadRequest)
					return
				}
				if req.WalletEncrypt {
					if strings.TrimSpace(req.WalletPassphrase) == "" {
						http.Error(w, "wallet passphrase required when encryption is enabled", http.StatusBadRequest)
						return
					}
					for _, net := range []string{"mainnet", "testnet"} {
						if err := encryptSetupWallet(f.DataDir, net, req.WalletPassphrase); err != nil {
							http.Error(w, "wallet encrypt "+net+": "+err.Error(), http.StatusBadRequest)
							return
						}
					}
				}
			}
			for _, pair := range []struct {
				cfg  *config.File
				path string
			}{{&mainnet, mainPath}, {&testnet, testPath}} {
				if !pair.cfg.DogeGoRelayCGNAT.UserConfigured() {
					p2pPort := 22556
					if net, err := chain.ParseNetwork(strings.TrimSpace(pair.cfg.Network)); err == nil {
						if p, err := chain.ParamsFor(net); err == nil {
							p2pPort = p.Port
						}
					}
					likely := cgnat.Likely(r.Context(), pair.cfg.P2PConnectivity, pair.cfg.Upnp, p2pPort)
					config.ApplyWizardDGRDefaults(pair.cfg, likely)
				}
				if err := config.ValidateAndNormalize(pair.cfg); err != nil {
					http.Error(w, pair.cfg.Network+": "+err.Error(), http.StatusBadRequest)
					return
				}
				if err := config.Save(pair.path, *pair.cfg); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if err := config.SaveInstances(f.DataDir, instances); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := config.Save(savePath, mainnet); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			autostartWarn = applyAutostart(mainnet, savePath)
			primary = mainnet
			if strings.TrimSpace(req.DashboardPIN) != "" {
				if err := applySetupDashboardPINNetworks(f.DataDir, []string{"mainnet", "testnet"}, req.DashboardPIN); err != nil {
					http.Error(w, "dashboard PIN: "+err.Error(), http.StatusBadRequest)
					return
				}
			}
		} else {
			// Persist the user's nobrowser preference; the post-wizard node start skips
			// auto-open separately in cmd/dogego (wizard tab already navigates to the dashboard).
			if !f.NoWallet {
				if !req.WalletBackupConfirmed {
					http.Error(w, "download and confirm wallet backup before starting", http.StatusBadRequest)
					return
				}
				ok, err := setupWalletExists(f.DataDir, f.Network)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !ok {
					http.Error(w, "wallet backup not prepared - download backup on the finish step first", http.StatusBadRequest)
					return
				}
				if err := applySetupUACommentTip(&f); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if req.WalletEncrypt {
					if strings.TrimSpace(req.WalletPassphrase) == "" {
						http.Error(w, "wallet passphrase required when encryption is enabled", http.StatusBadRequest)
						return
					}
					if err := encryptSetupWallet(f.DataDir, f.Network, req.WalletPassphrase); err != nil {
						http.Error(w, "wallet encrypt: "+err.Error(), http.StatusBadRequest)
						return
					}
				}
			}
			if !f.DogeGoRelayCGNAT.UserConfigured() {
				p2pPort := 22556
				if net, err := chain.ParseNetwork(strings.TrimSpace(f.Network)); err == nil {
					if p, err := chain.ParamsFor(net); err == nil {
						p2pPort = p.Port
					}
				}
				likely := cgnat.Likely(r.Context(), f.P2PConnectivity, f.Upnp, p2pPort)
				config.ApplyWizardDGRDefaults(&f, likely)
			}
			if err := config.ValidateAndNormalize(&f); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := config.Save(savePath, f); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			autostartWarn = applyAutostart(f, savePath)
			primary = f
			if strings.TrimSpace(req.DashboardPIN) != "" {
				if err := applySetupDashboardPIN(f.DataDir, f.Network, req.DashboardPIN); err != nil {
					http.Error(w, "dashboard PIN: "+err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		resp := map[string]any{"ok": true, "saved_to": savePath, "start_node": startNode, "dual_instance": req.DualInstance}
		if strings.TrimSpace(req.DashboardPIN) != "" {
			resp["dashboard_pin_applied"] = true
		}
		if autostartWarn != "" {
			resp["autostart_warning"] = autostartWarn
		}
		if !primary.NoWebUI && strings.TrimSpace(primary.WebUI) != "" {
			resp["dashboard_url"] = setupDashboardURL(primary)
		}
		if req.DualInstance {
			resp["peer_instances"] = instancesForAPI(primary.DataDir, primary.Network)
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			return
		}
		if f2, ok := w.(http.Flusher); ok {
			f2.Flush()
		}
		if startNode {
			select {
			case okCh <- primary:
			default:
			}
		}
	})

	srv := &http.Server{Handler: mux}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ln)
	}()

	if openBrowser {
		time.AfterFunc(400*time.Millisecond, func() { OpenURLLog(baseURL) })
	} else if scheme == "https" {
		fmt.Fprintf(os.Stderr, "DogeGo setup: open %s in your browser (use https://, not http://)\n", baseURL)
	} else {
		fmt.Fprintf(os.Stderr, "DogeGo setup: open %s in your browser\n", baseURL)
	}

	shutdown := func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		wg.Wait()
	}

	select {
	case <-ctx.Done():
		shutdown()
		return config.File{}, ctx.Err()
	case f := <-okCh:
		shutdown()
		return f, nil
	}
}

func setupWizardTLS(seed config.File, listenAddr string) (httptls.Pair, error) {
	if !seed.WebUITLSLocal {
		return httptls.Pair{}, nil
	}
	baseDir := strings.TrimSpace(seed.DataDir)
	if baseDir == "" {
		return httptls.Pair{}, fmt.Errorf("datadir required for setup HTTPS")
	}
	resolved, err := httptls.ResolveLocalTLS(httptls.LocalTLSOptions{
		BaseDataDir:    baseDir,
		WebUITLSLocal:  true,
		WebUIListen:    listenAddr,
		TrustCAOnStart: seed.LocalTLSTrustCA,
	})
	if err != nil {
		return httptls.Pair{}, err
	}
	if seed.LocalTLSTrustCA && resolved.Local != nil {
		var tr httptls.TrustResult
		if resolved.Local.CAGenerated {
			tr = httptls.TrustLocalCAForce(resolved.Local.CACertPath)
		} else {
			tr = httptls.TrustLocalCA(resolved.Local.CACertPath)
		}
		if tr.Detail != "" {
			fmt.Fprintf(os.Stderr, "DogeGo setup TLS: %s\n", tr.Detail)
		}
	}
	return resolved.WebUI, nil
}
