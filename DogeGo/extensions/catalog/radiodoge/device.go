// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package radiodoge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Device is an HTTP client for RadioDoge SoftAP firmware V3
// (https://github.com/dogecoinfoundation/radiodoge Heltec V3 SoftAP at 192.168.4.1).
type Device struct {
	BaseURL string
	Client  *http.Client
}

func NewDevice(baseURL string) *Device {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Device{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (d *Device) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, err
}

func (d *Device) postForm(ctx context.Context, path string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return body, resp.StatusCode, err
}

// Reachable returns true if the SoftAP API answers (any 2xx-4xx).
func (d *Device) Reachable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, d.BaseURL+"/api/broadcast", nil)
	if err != nil {
		return false
	}
	c := *d.Client
	c.Timeout = 2 * time.Second
	resp, err := c.Do(req)
	if err != nil {
		// Some firmware rejects HEAD; try status GET.
		_, code, err2 := d.getWithTimeout(ctx, "/api/status", 2*time.Second)
		return err2 == nil && code >= 200 && code < 500
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func (d *Device) getWithTimeout(ctx context.Context, path string, timeout time.Duration) ([]byte, int, error) {
	c := *d.Client
	c.Timeout = timeout
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return body, resp.StatusCode, err
}

// StatusJSON returns raw /api/status body and whether it looks Ready.
func (d *Device) StatusJSON(ctx context.Context) (map[string]interface{}, string, bool, error) {
	body, code, err := d.get(ctx, "/api/status")
	if err != nil {
		return nil, "", false, err
	}
	if code < 200 || code >= 300 {
		return nil, string(body), false, fmt.Errorf("status HTTP %d", code)
	}
	ready := looksReady(string(body))
	var m map[string]interface{}
	_ = json.Unmarshal(body, &m)
	return m, string(body), ready, nil
}

func looksReady(statusJSON string) bool {
	hasSuccess := strings.Contains(statusJSON, `"success":true`) || strings.Contains(statusJSON, `"success": true`)
	hasDevice := strings.Contains(statusJSON, `"device"`)
	hasName := strings.Contains(statusJSON, `"name":"RadioDoge"`) || strings.Contains(statusJSON, `"wifi":"RadioDoge"`) ||
		strings.Contains(statusJSON, `"name": "RadioDoge"`) || strings.Contains(statusJSON, `"wifi": "RadioDoge"`)
	hasReady := strings.Contains(statusJSON, `"status":"Ready"`) || strings.Contains(statusJSON, `"status": "Ready"`) ||
		strings.Contains(strings.ToLower(statusJSON), `"status":"ready"`)
	if hasSuccess && hasDevice && (hasName || hasReady) {
		return true
	}
	// Firmware variants may omit name; accept success + Ready.
	return hasSuccess && hasReady
}

// BroadcastTransaction posts signed tx hex to the mesh (wallet-compatible path).
func (d *Device) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	txHex = strings.TrimSpace(txHex)
	if txHex == "" {
		return "", fmt.Errorf("empty transaction hex")
	}
	form := url.Values{}
	form.Set("type", "transaction")
	form.Set("priority", "normal")
	form.Set("message", txHex)
	body, code, err := d.postForm(ctx, "/api/broadcast", form)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return string(body), fmt.Errorf("broadcast HTTP %d: %s", code, truncate(string(body), 200))
	}
	return string(body), nil
}

// SendDirectTransaction sends a signed tx to one LoRa address.
func (d *Device) SendDirectTransaction(ctx context.Context, address, txHex string) (string, error) {
	address = strings.TrimSpace(address)
	txHex = strings.TrimSpace(txHex)
	if address == "" || txHex == "" {
		return "", fmt.Errorf("address and transaction hex required")
	}
	form := url.Values{}
	form.Set("address", address)
	form.Set("type", "signed")
	form.Set("data", txHex)
	body, code, err := d.postForm(ctx, "/api/transaction", form)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return string(body), fmt.Errorf("transaction HTTP %d: %s", code, truncate(string(body), 200))
	}
	return string(body), nil
}

// Logs returns /api/logs JSON body.
func (d *Device) Logs(ctx context.Context) (string, error) {
	body, code, err := d.get(ctx, "/api/logs")
	if err != nil {
		return "", err
	}
	if code < 200 || code >= 300 {
		return string(body), fmt.Errorf("logs HTTP %d", code)
	}
	return string(body), nil
}

// QueueStatus returns /api/queue/status body.
func (d *Device) QueueStatus(ctx context.Context) (string, error) {
	body, code, err := d.get(ctx, "/api/queue/status")
	if err != nil {
		return "", err
	}
	if code < 200 || code >= 300 {
		return string(body), fmt.Errorf("queue HTTP %d", code)
	}
	return string(body), nil
}

// SaveGateway configures the device internet gateway (Core / custom / dogebox).
func (d *Device) SaveGateway(ctx context.Context, typ, ip, port, user, password, endpoint string) (string, error) {
	form := url.Values{}
	form.Set("type", typ)
	if ip != "" {
		form.Set("ip", ip)
	}
	if port != "" {
		form.Set("port", port)
	}
	if user != "" {
		form.Set("username", user)
	}
	if password != "" {
		form.Set("password", password)
	}
	if endpoint != "" {
		form.Set("endpoint", endpoint)
	}
	body, code, err := d.postForm(ctx, "/api/gateway/save", form)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return string(body), fmt.Errorf("gateway/save HTTP %d: %s", code, truncate(string(body), 200))
	}
	return string(body), nil
}

// HasInternet probes a public URL (not the SoftAP). RadioDoge SoftAP itself is offline-only.
func HasInternet(ctx context.Context, probeURL string) bool {
	probeURL = strings.TrimSpace(probeURL)
	if probeURL == "" {
		probeURL = "http://1.1.1.1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, probeURL, nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
