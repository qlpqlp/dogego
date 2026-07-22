// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

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

	"dogego/extensions"
	"dogego/extensions/catalog/zkl2"
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
	extMu   sync.Mutex
	ext     *zkl2.Extension
	host    *remoteHost
	rpcCh   = make(chan rpcReq, 8)
	hostMu  sync.Mutex
	hostWait sync.Map
)

func main() {
	go readStdinLoop()
	for req := range rpcCh {
		handleRPC(req)
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

func handleRPC(req rpcReq) {
	switch req.Method {
	case "dogego_on_enable":
		var params []interface{}
		_ = json.Unmarshal(req.Params, &params)
		dataDir := ""
		if len(params) > 0 {
			if m, ok := params[0].(map[string]interface{}); ok {
				dataDir, _ = m["data_dir"].(string)
			}
		}
		host = newRemoteHost(dataDir)
		man := zkl2.DefaultManifest()
		iface, err := zkl2.NewExtension(man)
		if err != nil {
			writeErr(req.ID, err.Error())
			return
		}
		e, ok := iface.(*zkl2.Extension)
		if !ok {
			writeErr(req.ID, "zkl2: bad extension type")
			return
		}
		extMu.Lock()
		ext = e
		extMu.Unlock()
		if err := e.OnEnable(context.Background(), host); err != nil {
			writeErr(req.ID, err.Error())
			return
		}
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
			"protocol_id": zkl2.ProtocolID,
			"commands":    p2pCommands(),
		})
	case "dogego_p2p":
		handleP2P(req)
	case "dogego_block_connected":
		handleBlockConnected(req)
	case "dogego_peer_connected":
		handlePeerConnected(req)
	default:
		handleExtensionRPC(req)
	}
}

func p2pCommands() []string {
	return []string{
		zkl2.CmdZKInv, zkl2.CmdGetZKProof, zkl2.CmdZKProof,
		zkl2.CmdGetZKHeaders, zkl2.CmdZKHeaders, zkl2.CmdGetZKBlockProofs,
	}
}

func withExt(req rpcReq, fn func(*zkl2.Extension) (interface{}, []p2pSend, error)) {
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
		writeErr(req.ID, "dogego_p2p: want [cmd, payload_hex, peer]")
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
	withExt(req, func(e *zkl2.Extension) (interface{}, []p2pSend, error) {
		var outbound []p2pSend
		send := func(c string, p []byte) error {
			outbound = append(outbound, p2pSend{Cmd: c, PayloadHex: hex.EncodeToString(p)})
			return nil
		}
		if err := e.HandleP2P(cmd, payload, peer, send); err != nil {
			return nil, nil, err
		}
		return nil, outbound, nil
	})
}

func handleBlockConnected(req rpcReq) {
	var params []interface{}
	_ = json.Unmarshal(req.Params, &params)
	height := int64(0)
	if len(params) > 0 {
		switch v := params[0].(type) {
		case float64:
			height = int64(v)
		case int64:
			height = v
		}
	}
	withExt(req, func(e *zkl2.Extension) (interface{}, []p2pSend, error) {
		if host == nil {
			return nil, nil, fmt.Errorf("host unavailable")
		}
		return nil, nil, e.OnBlockConnected(height, host)
	})
}

func handlePeerConnected(req rpcReq) {
	var params []interface{}
	_ = json.Unmarshal(req.Params, &params)
	if len(params) < 2 {
		writeErr(req.ID, "dogego_peer_connected: want [peer, protocols]")
		return
	}
	peer, _ := params[0].(string)
	var protocols []string
	if arr, ok := params[1].([]interface{}); ok {
		for _, x := range arr {
			if s, ok := x.(string); ok {
				protocols = append(protocols, s)
			}
		}
	}
	withExt(req, func(e *zkl2.Extension) (interface{}, []p2pSend, error) {
		var outbound []p2pSend
		send := func(c string, p []byte) error {
			outbound = append(outbound, p2pSend{Cmd: c, PayloadHex: hex.EncodeToString(p)})
			return nil
		}
		e.OnPeerConnected(peer, protocols, send)
		return nil, outbound, nil
	})
}

func handleExtensionRPC(req rpcReq) {
	withExt(req, func(e *zkl2.Extension) (interface{}, []p2pSend, error) {
		if host == nil {
			return nil, nil, fmt.Errorf("host unavailable")
		}
		var rawParams []json.RawMessage
		if len(req.Params) > 0 && string(req.Params) != "null" {
			var arr []json.RawMessage
			if json.Unmarshal(req.Params, &arr) == nil {
				rawParams = arr
			}
		}
		out, err := e.HandleRPC(req.Method, rawParams, host)
		return out, nil, err
	})
}

func writeOK(id uint64, result interface{}) {
	writeResult(id, result, nil)
}

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
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(raw))

	line, ok := <-ch
	if !ok {
		return nil, fmt.Errorf("host call cancelled")
	}
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
	case int64:
		return v, nil
	default:
		return 0, fmt.Errorf("tip_height: bad type")
	}
}

func (h *remoteHost) GetRawBlockByHeight(height int64) ([]byte, error) {
	out, err := h.hostCall("get_raw_block_by_height", height)
	if err != nil {
		return nil, err
	}
	s, ok := out.(string)
	if !ok {
		return nil, fmt.Errorf("get_raw_block: bad response")
	}
	return hex.DecodeString(strings.TrimSpace(s))
}

func (h *remoteHost) LookupTxHex(txid string) (string, int64, bool) {
	out, err := h.hostCall("lookup_tx_hex", txid)
	if err != nil {
		return "", 0, false
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		return "", 0, false
	}
	hexStr, _ := m["hex"].(string)
	okVal, _ := m["ok"].(bool)
	var height int64
	switch v := m["height"].(type) {
	case float64:
		height = int64(v)
	case int64:
		height = v
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
	m, ok := out.(map[string]interface{})
	if !ok {
		return 0, false
	}
	okVal, _ := m["ok"].(bool)
	var idx uint32
	switch v := m["tx_index"].(type) {
	case float64:
		idx = uint32(v)
	case int64:
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

func (h *remoteHost) BroadcastOverlay(protocolID, cmd string, payload []byte, excludePeer string) error {
	_, err := h.hostCall("overlay_broadcast", protocolID, cmd, hex.EncodeToString(payload), excludePeer)
	return err
}

func (h *remoteHost) EachOverlayPeer(protocolID string, fn func(peer string, send func(string, []byte) error)) {
	out, err := h.hostCall("overlay_peers", protocolID)
	if err != nil || fn == nil {
		return
	}
	arr, ok := out.([]interface{})
	if !ok {
		return
	}
	for _, x := range arr {
		peer, ok := x.(string)
		if !ok {
			continue
		}
		p := peer
		send := func(cmd string, payload []byte) error {
			_, err := h.hostCall("overlay_send", p, cmd, hex.EncodeToString(payload))
			return err
		}
		fn(p, send)
	}
}

func (h *remoteHost) OverlayPeerCount(protocolID string) int {
	out, err := h.hostCall("overlay_peer_count", protocolID)
	if err != nil {
		return 0
	}
	switch v := out.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
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

var (
	_ extensions.Host          = (*remoteHost)(nil)
	_ extensions.OverlayHost   = (*remoteHost)(nil)
	_ extensions.WalletRPCHost = (*remoteHost)(nil)
)
