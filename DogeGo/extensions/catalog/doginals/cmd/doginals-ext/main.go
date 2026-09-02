// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

// doginals-ext is the DogeGo doginals/DRC-20 L2 subprocess extension.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"dogego/extensions/catalog/doginals"
)

const maxLineBytes = 4 << 20

type rpcReq struct {
	ID       uint64          `json:"id"`
	HostCall uint64          `json:"host_call"`
	HostResp uint64          `json:"host_resp"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params"`
}

type rpcResp struct {
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
	P2PSend []p2pSend       `json:"p2p_send,omitempty"`
}

type p2pSend struct {
	Cmd        string `json:"cmd"`
	PayloadHex string `json:"payload_hex"`
	Peer       string `json:"peer,omitempty"`
	ProtocolID string `json:"protocol_id,omitempty"`
}

var (
	extMu    sync.Mutex
	ext      *doginals.Extension
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
		if len(req.Params) > 0 {
			var m map[string]interface{}
			if json.Unmarshal(req.Params, &m) == nil {
				if d, ok := m["data_dir"].(string); ok && d != "" {
					dataDir = d
				}
			} else {
				var arr []json.RawMessage
				if json.Unmarshal(req.Params, &arr) == nil && len(arr) > 0 {
					_ = json.Unmarshal(arr[0], &m)
					if d, ok := m["data_dir"].(string); ok && d != "" {
						dataDir = d
					}
				}
			}
		}
		host = newRemoteHost(dataDir)
		iface, err := doginals.NewExtension(doginals.DefaultManifest())
		if err != nil {
			writeErr(req.ID, err.Error())
			return
		}
		e := iface.(*doginals.Extension)
		if err := e.OnEnable(context.Background(), host); err != nil {
			writeErr(req.ID, err.Error())
			return
		}
		extMu.Lock()
		ext = e
		extMu.Unlock()
		writeOK(req.ID, map[string]string{"status": "ready"})
	case "dogego_on_disable":
		extMu.Lock()
		e := ext
		ext = nil
		extMu.Unlock()
		if e != nil {
			_ = e.OnDisable()
		}
		writeOK(req.ID, map[string]string{"status": "bye"})
	case "dogego_p2p_meta":
		writeOK(req.ID, map[string]interface{}{
			"protocol_id": doginals.ProtocolID,
			"commands":    doginals.P2PCommands(),
		})
	case "dogego_p2p":
		handleP2P(req)
	case "dogego_block_connected":
		handleBlock(req)
	case "dogego_block_disconnected":
		handleBlockDisconnect(req)
	case "dogego_peer_connected":
		handlePeer(req)
	default:
		handleExtRPC(req)
	}
}

func withExt(req rpcReq, fn func(*doginals.Extension) (interface{}, []p2pSend, error)) {
	extMu.Lock()
	e := ext
	extMu.Unlock()
	if e == nil {
		writeErr(req.ID, "extension not enabled")
		return
	}
	out, sends, err := fn(e)
	if err != nil {
		writeErr(req.ID, err.Error())
		return
	}
	writeResult(req.ID, out, sends)
}

func handleP2P(req rpcReq) {
	var params []interface{}
	_ = json.Unmarshal(req.Params, &params)
	if len(params) < 2 {
		writeErr(req.ID, "dogego_p2p: want [cmd, payload_hex]")
		return
	}
	cmd, _ := params[0].(string)
	payloadHex, _ := params[1].(string)
	peer := ""
	if len(params) > 2 {
		peer, _ = params[2].(string)
	}
	payload, err := hex.DecodeString(strings.TrimSpace(payloadHex))
	if err != nil {
		writeErr(req.ID, err.Error())
		return
	}
	withExt(req, func(e *doginals.Extension) (interface{}, []p2pSend, error) {
		var outbound []p2pSend
		send := func(c string, p []byte) error {
			outbound = append(outbound, p2pSend{Cmd: c, PayloadHex: hex.EncodeToString(p), ProtocolID: doginals.ProtocolID})
			return nil
		}
		if err := e.HandleP2P(cmd, payload, peer, send); err != nil {
			return nil, nil, err
		}
		return nil, outbound, nil
	})
}

func handleBlock(req rpcReq) {
	var params []interface{}
	_ = json.Unmarshal(req.Params, &params)
	height := int64(0)
	if len(params) > 0 {
		if v, ok := params[0].(float64); ok {
			height = int64(v)
		}
	}
	withExt(req, func(e *doginals.Extension) (interface{}, []p2pSend, error) {
		return nil, nil, e.OnBlockConnected(height, host)
	})
}

func handleBlockDisconnect(req rpcReq) {
	var params []interface{}
	_ = json.Unmarshal(req.Params, &params)
	height := int64(0)
	if len(params) > 0 {
		if v, ok := params[0].(float64); ok {
			height = int64(v)
		}
	}
	withExt(req, func(e *doginals.Extension) (interface{}, []p2pSend, error) {
		return nil, nil, e.OnBlockDisconnected(height, host)
	})
}

func handlePeer(req rpcReq) {
	var params []interface{}
	_ = json.Unmarshal(req.Params, &params)
	peer := ""
	if len(params) > 0 {
		peer, _ = params[0].(string)
	}
	withExt(req, func(e *doginals.Extension) (interface{}, []p2pSend, error) {
		var outbound []p2pSend
		send := func(c string, p []byte) error {
			outbound = append(outbound, p2pSend{Cmd: c, PayloadHex: hex.EncodeToString(p), Peer: peer, ProtocolID: doginals.ProtocolID})
			return nil
		}
		e.OnPeerConnected(peer, nil, send)
		return nil, outbound, nil
	})
}

func handleExtRPC(req rpcReq) {
	withExt(req, func(e *doginals.Extension) (interface{}, []p2pSend, error) {
		var rawParams []json.RawMessage
		if len(req.Params) > 0 && string(req.Params) != "null" {
			_ = json.Unmarshal(req.Params, &rawParams)
		}
		out, err := e.HandleRPC(req.Method, rawParams, host)
		return out, nil, err
	})
}

func writeOK(id uint64, result interface{}) { writeResult(id, result, nil) }

func writeResult(id uint64, result interface{}, p2p []p2pSend) {
	resp := rpcResp{ID: id, P2PSend: p2p}
	if result != nil {
		raw, err := json.Marshal(result)
		if err != nil {
			writeErr(id, err.Error())
			return
		}
		resp.Result = raw
	}
	raw, _ := json.Marshal(resp)
	fmt.Println(string(raw))
}

func writeErr(id uint64, msg string) {
	raw, _ := json.Marshal(rpcResp{ID: id, Error: msg})
	fmt.Println(string(raw))
}

type remoteHost struct {
	dataDir  string
	nextHost atomic.Uint64
}

func newRemoteHost(dataDir string) *remoteHost {
	return &remoteHost{dataDir: strings.TrimSpace(dataDir)}
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

func (h *remoteHost) Network() string {
	out, _ := h.hostCall("network")
	s, _ := out.(string)
	return s
}

func (h *remoteHost) TipHeight() (int64, error) {
	out, err := h.hostCall("tip_height")
	if err != nil {
		return 0, err
	}
	switch v := out.(type) {
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("tip_height type")
	}
}

func (h *remoteHost) GetRawBlockByHeight(height int64) ([]byte, error) {
	out, err := h.hostCall("get_raw_block_by_height", height)
	if err != nil {
		return nil, err
	}
	s, ok := out.(string)
	if !ok {
		return nil, fmt.Errorf("bad block response")
	}
	return hex.DecodeString(strings.TrimSpace(s))
}

func (h *remoteHost) LookupTxHex(txid string) (string, int64, bool) {
	out, err := h.hostCall("lookup_tx_hex", txid)
	if err != nil {
		return "", 0, false
	}
	m, _ := out.(map[string]interface{})
	hexStr, _ := m["hex"].(string)
	okVal, _ := m["ok"].(bool)
	var height int64
	if v, ok := m["height"].(float64); ok {
		height = int64(v)
	}
	return hexStr, height, okVal
}

func (h *remoteHost) BlockHashAtHeight(height int64) (string, error) {
	out, err := h.hostCall("block_hash_at_height", height)
	if err != nil {
		return "", err
	}
	s, _ := out.(string)
	return s, nil
}

func (h *remoteHost) ConfirmedTxInBlock(blockHash, txid string) (uint32, bool) {
	out, err := h.hostCall("confirmed_tx_in_block", blockHash, txid)
	if err != nil {
		return 0, false
	}
	m, _ := out.(map[string]interface{})
	okVal, _ := m["ok"].(bool)
	var idx uint32
	if v, ok := m["tx_index"].(float64); ok {
		idx = uint32(v)
	}
	return idx, okVal
}

func (h *remoteHost) DataDir() string {
	out, _ := h.hostCall("data_dir")
	s, _ := out.(string)
	return s
}

func (h *remoteHost) ExtensionDataDir(id string) (string, error) {
	if h.dataDir != "" {
		return h.dataDir, nil
	}
	out, err := h.hostCall("extension_data_dir", id)
	if err != nil {
		return "", err
	}
	s, _ := out.(string)
	return s, nil
}

func (h *remoteHost) Log(line string) {
	_, _ = h.hostCall("log", line)
}

func (h *remoteHost) CallWalletRPC(method string, params []json.RawMessage) (interface{}, error) {
	args := []interface{}{method}
	for _, p := range params {
		var v interface{}
		if json.Unmarshal(p, &v) == nil {
			args = append(args, v)
		}
	}
	return h.hostCall("wallet_call", args...)
}
