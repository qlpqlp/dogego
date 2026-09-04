// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

// dogeos-ext connects DogeGo to the DogeOS EVM application layer (Chikyū testnet today).
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

	"dogego/extensions/catalog/dogeos"
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
	svc      *dogeos.Service
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
		svc = dogeos.NewService(dataDir, host)
		svc.Start()
		writeOK(req.ID, map[string]string{"status": "ready"})
	case "dogego_on_disable":
		if svc != nil {
			svc.Stop()
			svc = nil
		}
		writeOK(req.ID, map[string]string{"status": "bye"})
	case "info", "ui_status":
		needSvc(req, func() {
			writeOK(req.ID, svc.Snapshot())
		})
	case "probe":
		needSvc(req, func() {
			p, err := svc.ProbeNow(context.Background())
			out := map[string]interface{}{"probe": p}
			if err != nil {
				out["error"] = err.Error()
			}
			writeOK(req.ID, out)
		})
	case "helpers":
		needSvc(req, func() {
			n, rpc, _ := svc.EffectiveNetwork()
			writeOK(req.ID, dogeos.Helpers(n, rpc))
		})
	case "networks":
		needSvc(req, func() {
			writeOK(req.ID, dogeos.BuiltInNetworks())
		})
	case "metrics":
		needSvc(req, func() {
			writeOK(req.ID, svc.Snapshot()["metrics"])
		})
	case "getbalance":
		needSvc(req, func() {
			addr := strField(paramObject(req.Params), "address")
			c, n, err := svc.Client()
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			wei, doge, err := c.GetBalance(context.Background(), addr)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, map[string]interface{}{
				"address": addr, "wei": wei, "doge": doge,
				"explorer": dogeos.ExplorerAddressURL(n, addr),
			})
		})
	case "getcode":
		needSvc(req, func() {
			addr := strField(paramObject(req.Params), "address")
			c, n, err := svc.Client()
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			code, isContract, err := c.GetCode(context.Background(), addr)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, map[string]interface{}{
				"address": addr, "is_contract": isContract,
				"code_len": len(strings.TrimPrefix(strings.TrimPrefix(code, "0x"), "0X")) / 2,
				"explorer": dogeos.ExplorerAddressURL(n, addr),
			})
		})
	case "getreceipt":
		needSvc(req, func() {
			hash := strField(paramObject(req.Params), "tx_hash", "hash", "txid")
			c, n, err := svc.Client()
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			rcpt, err := c.GetTransactionReceipt(context.Background(), hash)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, map[string]interface{}{
				"receipt": rcpt, "explorer": dogeos.ExplorerTxURL(n, hash),
			})
		})
	case "getblock":
		needSvc(req, func() {
			num := strField(paramObject(req.Params), "number", "block")
			if num == "" {
				num = "latest"
			}
			c, _, err := svc.Client()
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			block, err := c.GetBlockByNumber(context.Background(), num, false)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, block)
		})
	case "rpccall":
		needSvc(req, func() {
			p := paramObject(req.Params)
			method := strField(p, "method")
			if method == "" {
				writeErr(req.ID, "method required")
				return
			}
			var params []interface{}
			if raw, ok := p["params_json"].(string); ok && strings.TrimSpace(raw) != "" {
				if err := json.Unmarshal([]byte(raw), &params); err != nil {
					writeErr(req.ID, "params_json must be a JSON array")
					return
				}
			} else if arr, ok := p["params"].([]interface{}); ok {
				params = arr
			}
			c, _, err := svc.Client()
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			raw, lat, err := c.Call(context.Background(), method, params)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			var decoded interface{}
			_ = json.Unmarshal(raw, &decoded)
			writeOK(req.ID, map[string]interface{}{
				"method": method, "result": decoded, "latency_ms": lat.Milliseconds(),
			})
		})
	case "getconfig":
		needSvc(req, func() {
			writeOK(req.ID, svc.Config())
		})
	case "setconfig":
		needSvc(req, func() {
			cfg, err := svc.SetConfig(paramObject(req.Params))
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, cfg)
		})
	case "httphandle":
		needSvc(req, func() {
			var params []json.RawMessage
			if len(req.Params) > 0 {
				if err := json.Unmarshal(req.Params, &params); err != nil {
					params = []json.RawMessage{req.Params}
				}
			}
			out, err := svc.HandleHTTP(params)
			if err != nil {
				writeErr(req.ID, err.Error())
				return
			}
			writeOK(req.ID, out)
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
