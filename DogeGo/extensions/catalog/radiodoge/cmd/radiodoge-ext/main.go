// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

// radiodoge-ext connects DogeGo to RadioDoge Heltec V3 SoftAP (HTTP API).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"dogego/extensions/catalog/radiodoge"
)

const maxLineBytes = 4 << 20

type rpcReq struct {
	ID       uint64          `json:"id"`
	HostCall uint64          `json:"host_call"`
	HostResp uint64          `json:"host_resp"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params"`
}

var (
	svc      *radiodoge.Service
	host     *remoteHost
	rpcCh    = make(chan rpcReq, 8)
	hostWait sync.Map
)

func main() {
	go readStdinLoop()
	for req := range rpcCh {
		handle(req)
	}
}

func readStdinLoop() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	for sc.Scan() {
		line := []byte(strings.TrimSpace(sc.Text()))
		if len(line) == 0 {
			continue
		}
		var req rpcReq
		if err := json.Unmarshal(line, &req); err != nil {
			writeErr(0, "bad json")
			continue
		}
		if req.HostResp != 0 {
			if ch, ok := hostWait.Load(req.HostResp); ok {
				if c, ok := ch.(chan []byte); ok {
					c <- append([]byte(nil), line...)
				}
			}
			continue
		}
		if req.HostCall != 0 {
			continue
		}
		rpcCh <- req
	}
	close(rpcCh)
}

func handle(req rpcReq) {
	switch req.Method {
	case "dogego_on_enable":
		dataDir := strings.TrimSpace(os.Getenv("DOGEGO_DATA_DIR"))
		if meta := firstObject(req.Params); meta != nil {
			if d, ok := meta["data_dir"].(string); ok && d != "" {
				dataDir = d
			}
		}
		host = newRemoteHost(dataDir)
		svc = radiodoge.NewService(dataDir, host)
		svc.Start()
		writeOK(req.ID, map[string]string{"status": "ready"})
	case "dogego_on_disable":
		if svc != nil {
			svc.Stop()
			svc = nil
		}
		writeOK(req.ID, map[string]string{"status": "bye"})
	case "info", "ui_status":
		if svc == nil {
			writeErr(req.ID, "not enabled")
			return
		}
		snap := svc.Snapshot()
		snap["ui"] = radiodoge.BuildUI(snap)
		writeOK(req.ID, snap)
	case "probe":
		needSvc(req, func() {
			ctx := context.Background()
			d := radiodoge.NewDevice(svc.Config().BaseURL)
			ok := d.Reachable(ctx)
			m, raw, ready, err := d.StatusJSON(ctx)
			out := map[string]interface{}{
				"reachable": ok,
				"ready":     ready,
				"status":    m,
				"raw":       truncate(raw, 800),
			}
			if err != nil {
				out["error"] = err.Error()
			}
			writeOK(req.ID, out)
		})
	case "should_use_radio":
		needSvc(req, func() {
			use, reason := svc.ShouldUseRadio(context.Background())
			writeOK(req.ID, map[string]interface{}{"use": use, "reason": reason})
		})
	case "broadcast":
		needSvc(req, func() {
			p := paramObject(req.Params)
			hexTx := strField(p, "hex", "message", "tx")
			txid := strField(p, "txid")
			if hexTx == "" {
				hexTx = firstString(req.Params)
			}
			out, err := svc.BroadcastHex(context.Background(), hexTx, txid)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, out)
		})
	case "broadcast_smart":
		needSvc(req, func() {
			p := paramObject(req.Params)
			hexTx := strField(p, "hex", "message", "tx")
			txid := strField(p, "txid")
			if hexTx == "" {
				hexTx = firstString(req.Params)
			}
			out, err := svc.BroadcastSmart(context.Background(), hexTx, txid)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, out)
		})
	case "send_direct":
		needSvc(req, func() {
			p := paramObject(req.Params)
			addr := strField(p, "address")
			hexTx := strField(p, "hex", "data", "tx")
			out, err := svc.SendDirect(context.Background(), addr, hexTx)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, out)
		})
	case "configure_gateway":
		needSvc(req, func() {
			out, err := svc.ConfigureGateway(context.Background())
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, out)
		})
	case "logs":
		needSvc(req, func() {
			body, err := radiodoge.NewDevice(svc.Config().BaseURL).Logs(context.Background())
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, map[string]interface{}{
				"logs":          truncate(body, 8000),
				"confirmations": radiodoge.ParseConfirmations(body),
				"tx_candidates": len(radiodoge.ExtractTxHexCandidates(body)),
			})
		})
	case "getconfig":
		needSvc(req, func() {
			writeOK(req.ID, svc.Config())
		})
	case "setconfig":
		needSvc(req, func() {
			p := paramObject(req.Params)
			cfg, err := svc.SetConfig(p)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, cfg)
		})
	default:
		writeErr(req.ID, "unknown method "+req.Method)
	}
}

func needSvc(req rpcReq, fn func()) {
	if svc == nil {
		writeErr(req.ID, "extension not enabled")
		return
	}
	fn()
}

func writeOK(id uint64, result interface{}) {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "result": result})
	fmt.Println(string(raw))
}

func writeErr(id uint64, msg string) {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "error": msg})
	fmt.Println(string(raw))
}

type remoteHost struct {
	dataDir  string
	nextHost atomic.Uint64
}

func newRemoteHost(dataDir string) *remoteHost {
	return &remoteHost{dataDir: strings.TrimSpace(dataDir)}
}

func (h *remoteHost) Log(line string) {
	_, _ = h.hostCall("log", line)
}

func (h *remoteHost) CallWalletRPC(method string, args ...interface{}) (interface{}, error) {
	params := make([]interface{}, 0, 1+len(args))
	params = append(params, method)
	params = append(params, args...)
	return h.hostCall("wallet_call", params...)
}

func (h *remoteHost) hostCall(method string, params ...interface{}) (interface{}, error) {
	id := h.nextHost.Add(1)
	ch := make(chan []byte, 1)
	hostWait.Store(id, ch)
	defer hostWait.Delete(id)
	req := map[string]interface{}{"host_call": id, "method": method, "params": params}
	raw, _ := json.Marshal(req)
	fmt.Println(string(raw))
	line := <-ch
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	if len(resp.Result) == 0 {
		return nil, nil
	}
	var out interface{}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return string(resp.Result), nil
	}
	return out, nil
}

func firstObject(params json.RawMessage) map[string]interface{} {
	if len(params) == 0 {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(params, &m) == nil {
		return m
	}
	var arr []json.RawMessage
	if json.Unmarshal(params, &arr) == nil && len(arr) > 0 {
		_ = json.Unmarshal(arr[0], &m)
		return m
	}
	return nil
}

func paramObject(params json.RawMessage) map[string]interface{} {
	if m := firstObject(params); m != nil {
		return m
	}
	return map[string]interface{}{}
}

func firstString(params json.RawMessage) string {
	var arr []json.RawMessage
	if json.Unmarshal(params, &arr) == nil && len(arr) > 0 {
		var s string
		if json.Unmarshal(arr[0], &s) == nil {
			return strings.TrimSpace(s)
		}
	}
	var s string
	if json.Unmarshal(params, &s) == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func strField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
