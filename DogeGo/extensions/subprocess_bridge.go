// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SubprocessCallTimeout bounds how long the host waits for one subprocess RPC round-trip.
const SubprocessCallTimeout = 2 * time.Minute

type p2pOutboundMsg struct {
	Cmd         string `json:"cmd"`
	PayloadHex  string `json:"payload_hex"`
	Peer        string `json:"peer,omitempty"`
	ProtocolID  string `json:"protocol_id,omitempty"`
}

type subprocessLine struct {
	ID       uint64              `json:"id"`
	HostCall uint64              `json:"host_call"`
	HostResp uint64              `json:"host_resp"`
	Method   string              `json:"method"`
	Params   json.RawMessage     `json:"params"`
	Result   json.RawMessage     `json:"result"`
	Error    string              `json:"error"`
	P2PSend  []p2pOutboundMsg    `json:"p2p_send"`
}

type subprocessBridge struct {
	mu          sync.Mutex
	stdin       io.WriteCloser
	reader      *bufio.Reader
	host        Host
	manifest    Manifest
	nextID      atomic.Uint64
	nextHost    atomic.Uint64
	pending     map[uint64]chan subprocessLine
	hostPending map[uint64]chan subprocessLine
	done        chan struct{}
	wg          sync.WaitGroup
}

func newSubprocessBridge(stdin io.WriteCloser, reader *bufio.Reader, host Host, man Manifest) *subprocessBridge {
	b := &subprocessBridge{
		stdin:       stdin,
		reader:      reader,
		host:        host,
		manifest:    man,
		pending:     make(map[uint64]chan subprocessLine),
		hostPending: make(map[uint64]chan subprocessLine),
		done:        make(chan struct{}),
	}
	b.wg.Add(1)
	go b.readLoop()
	return b
}

func (b *subprocessBridge) Close() {
	select {
	case <-b.done:
	default:
		close(b.done)
	}
	b.wg.Wait()
}

func (b *subprocessBridge) Call(method string, params []interface{}) (interface{}, []p2pOutboundMsg, error) {
	if b == nil {
		return nil, nil, fmt.Errorf("subprocess bridge closed")
	}
	id := b.nextID.Add(1)
	ch := make(chan subprocessLine, 1)
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}()

	req := map[string]interface{}{"id": id, "method": method, "params": params}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	if _, err := b.stdin.Write(append(raw, '\n')); err != nil {
		return nil, nil, fmt.Errorf("subprocess write: %w", err)
	}
	timer := time.NewTimer(SubprocessCallTimeout)
	defer timer.Stop()
	select {
	case <-b.done:
		return nil, nil, fmt.Errorf("subprocess bridge closed")
	case line := <-ch:
		if line.Error != "" {
			return nil, line.P2PSend, fmt.Errorf("%s", line.Error)
		}
		if len(line.Result) == 0 {
			return nil, line.P2PSend, nil
		}
		var out interface{}
		if err := json.Unmarshal(line.Result, &out); err != nil {
			return string(line.Result), line.P2PSend, nil
		}
		return out, line.P2PSend, nil
	case <-timer.C:
		return nil, nil, fmt.Errorf("subprocess rpc timed out after %s", SubprocessCallTimeout)
	}
}

func (b *subprocessBridge) hostCall(method string, params []interface{}) (interface{}, error) {
	if b == nil {
		return nil, fmt.Errorf("subprocess bridge closed")
	}
	id := b.nextHost.Add(1)
	ch := make(chan subprocessLine, 1)
	b.mu.Lock()
	b.hostPending[id] = ch
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.hostPending, id)
		b.mu.Unlock()
	}()

	req := map[string]interface{}{"host_call": id, "method": method, "params": params}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := b.stdin.Write(append(raw, '\n')); err != nil {
		return nil, fmt.Errorf("subprocess host write: %w", err)
	}
	timer := time.NewTimer(SubprocessCallTimeout)
	defer timer.Stop()
	select {
	case <-b.done:
		return nil, fmt.Errorf("subprocess bridge closed")
	case line := <-ch:
		if line.Error != "" {
			return nil, fmt.Errorf("%s", line.Error)
		}
		if len(line.Result) == 0 {
			return nil, nil
		}
		var out interface{}
		if err := json.Unmarshal(line.Result, &out); err != nil {
			return string(line.Result), nil
		}
		return out, nil
	case <-timer.C:
		return nil, fmt.Errorf("subprocess host call timed out after %s", SubprocessCallTimeout)
	}
}

func (b *subprocessBridge) readLoop() {
	defer b.wg.Done()
	for {
		select {
		case <-b.done:
			return
		default:
		}
		line, err := readSubprocessLine(b.reader)
		if err != nil {
			return
		}
		var msg subprocessLine
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if msg.HostCall != 0 {
			b.handleHostCall(msg.HostCall, msg.Method, msg.Params)
			continue
		}
		if msg.HostResp != 0 {
			b.mu.Lock()
			ch := b.hostPending[msg.HostResp]
			b.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
			continue
		}
		if msg.ID != 0 {
			b.mu.Lock()
			ch := b.pending[msg.ID]
			b.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
		}
	}
}

func (b *subprocessBridge) handleHostCall(callID uint64, method string, paramsRaw json.RawMessage) {
	var params []interface{}
	if len(paramsRaw) > 0 && string(paramsRaw) != "null" {
		_ = json.Unmarshal(paramsRaw, &params)
	}
	result, err := b.execHostCall(method, params)
	resp := subprocessLine{HostResp: callID}
	if err != nil {
		resp.Error = err.Error()
	} else if result != nil {
		raw, encErr := json.Marshal(result)
		if encErr != nil {
			resp.Error = encErr.Error()
		} else {
			resp.Result = raw
		}
	}
	raw, _ := json.Marshal(resp)
	_, _ = b.stdin.Write(append(raw, '\n'))
}

func (b *subprocessBridge) execHostCall(method string, params []interface{}) (interface{}, error) {
	if b.host == nil {
		return nil, fmt.Errorf("host unavailable")
	}
	method = strings.TrimSpace(method)
	switch method {
	case "network":
		return b.host.Network(), nil
	case "tip_height":
		return b.host.TipHeight()
	case "data_dir":
		return b.host.DataDir(), nil
	case "extension_data_dir":
		if len(params) < 1 {
			return nil, fmt.Errorf("extension_data_dir: want [id]")
		}
		id, _ := params[0].(string)
		return b.host.ExtensionDataDir(id)
	case "log":
		if len(params) >= 1 {
			if s, ok := params[0].(string); ok {
				b.host.Log(s)
			}
		}
		return nil, nil
	case "get_raw_block_by_height":
		if !b.manifest.HasPermission("chain_read") {
			return nil, fmt.Errorf("chain_read required")
		}
		h, err := paramInt64(params, 0)
		if err != nil {
			return nil, err
		}
		raw, err := b.host.GetRawBlockByHeight(h)
		if err != nil {
			return nil, err
		}
		return hex.EncodeToString(raw), nil
	case "block_hash_at_height":
		if !b.manifest.HasPermission("chain_read") {
			return nil, fmt.Errorf("chain_read required")
		}
		h, err := paramInt64(params, 0)
		if err != nil {
			return nil, err
		}
		return b.host.BlockHashAtHeight(h)
	case "lookup_tx_hex":
		if !b.manifest.HasPermission("chain_read") {
			return nil, fmt.Errorf("chain_read required")
		}
		txid, _ := params[0].(string)
		hexStr, height, ok := b.host.LookupTxHex(txid)
		return map[string]interface{}{"hex": hexStr, "height": height, "ok": ok}, nil
	case "confirmed_tx_in_block":
		if !b.manifest.HasPermission("chain_index") {
			return nil, fmt.Errorf("chain_index required")
		}
		blockHash, _ := params[0].(string)
		txid, _ := params[1].(string)
		idx, ok := b.host.ConfirmedTxInBlock(blockHash, txid)
		return map[string]interface{}{"tx_index": idx, "ok": ok}, nil
	case "overlay_broadcast":
		if !b.manifest.HasPermission("p2p_extension") {
			return nil, fmt.Errorf("p2p_extension required")
		}
		oh, ok := b.host.(OverlayHost)
		if !ok {
			return nil, fmt.Errorf("overlay host unavailable")
		}
		proto, _ := params[0].(string)
		cmd, _ := params[1].(string)
		payload, err := paramBytesHex(params, 2)
		if err != nil {
			return nil, err
		}
		exclude := ""
		if len(params) > 3 {
			exclude, _ = params[3].(string)
		}
		return nil, oh.BroadcastOverlay(proto, cmd, payload, exclude)
	case "overlay_peer_count":
		if !b.manifest.HasPermission("p2p_extension") {
			return nil, fmt.Errorf("p2p_extension required")
		}
		oh, ok := b.host.(OverlayHost)
		if !ok {
			return 0, nil
		}
		proto, _ := params[0].(string)
		return oh.OverlayPeerCount(proto), nil
	case "overlay_peers":
		if !b.manifest.HasPermission("p2p_extension") {
			return nil, fmt.Errorf("p2p_extension required")
		}
		oh, ok := b.host.(OverlayHost)
		if !ok {
			return []string{}, nil
		}
		proto, _ := params[0].(string)
		var peers []string
		oh.EachOverlayPeer(proto, func(peer string, _ func(string, []byte) error) {
			peers = append(peers, peer)
		})
		return peers, nil
	case "overlay_send":
		if !b.manifest.HasPermission("p2p_extension") {
			return nil, fmt.Errorf("p2p_extension required")
		}
		mh, ok := b.host.(*managerHost)
		if !ok {
			return nil, fmt.Errorf("overlay send unavailable")
		}
		peer, _ := params[0].(string)
		cmd, _ := params[1].(string)
		payload, err := paramBytesHex(params, 2)
		if err != nil {
			return nil, err
		}
		return nil, mh.sendOverlayPeer(peer, cmd, payload)
	case "wallet_call":
		if !b.manifest.HasPermission("wallet_rpc") {
			return nil, fmt.Errorf("wallet_rpc required")
		}
		wh, ok := b.host.(WalletRPCHost)
		if !ok {
			return nil, fmt.Errorf("wallet host unavailable")
		}
		if len(params) < 1 {
			return nil, fmt.Errorf("wallet_call: want [method, ...args]")
		}
		wmethod, _ := params[0].(string)
		var args []json.RawMessage
		for _, p := range params[1:] {
			raw, err := json.Marshal(p)
			if err != nil {
				return nil, err
			}
			args = append(args, raw)
		}
		return wh.CallWalletRPC(wmethod, args)
	default:
		return nil, fmt.Errorf("unknown host call %q", method)
	}
}

func paramInt64(params []interface{}, i int) (int64, error) {
	if len(params) <= i {
		return 0, fmt.Errorf("missing param %d", i)
	}
	switch v := params[i].(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	default:
		return 0, fmt.Errorf("param %d not numeric", i)
	}
}

func paramBytesHex(params []interface{}, i int) ([]byte, error) {
	if len(params) <= i {
		return nil, fmt.Errorf("missing param %d", i)
	}
	s, ok := params[i].(string)
	if !ok {
		return nil, fmt.Errorf("param %d not string", i)
	}
	return hex.DecodeString(strings.TrimSpace(s))
}

func (h *managerHost) sendOverlayPeer(peerAddr, cmd string, payload []byte) error {
	if h.m == nil || peerAddr == "" {
		return fmt.Errorf("overlay peer unavailable")
	}
	h.m.mu.Lock()
	ent, ok := h.m.peerOverlays[peerAddr]
	h.m.mu.Unlock()
	if !ok || ent.send == nil {
		return fmt.Errorf("peer %q not connected", peerAddr)
	}
	return ent.send(cmd, payload)
}
