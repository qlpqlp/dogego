// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/json"
	"strings"
)

// HandleP2P processes doginals-v1 overlay commands.
func (e *Extension) HandleP2P(cmd string, payload []byte, _ string, send func(cmd string, payload []byte) error) error {
	st, err := e.storeOrErr()
	if err != nil {
		return err
	}
	switch cmd {
	case CmdDInv:
		for _, id := range DecodeInv(payload) {
			id = strings.ToLower(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			if _, ok, _ := st.GetAsset(id); ok {
				continue
			}
			if send != nil {
				_ = send(CmdGetAsset, []byte(id))
			}
		}
	case CmdGetAsset:
		id := strings.ToLower(strings.TrimSpace(string(payload)))
		a, ok, err := st.GetAsset(id)
		if err != nil || !ok {
			return nil
		}
		if send != nil {
			_ = send(CmdAsset, EncodeAssetWire(a))
		}
	case CmdAsset:
		a, err := DecodeAssetWire(payload)
		if err != nil {
			return nil
		}
		a, err = NormalizeAsset(a)
		if err != nil {
			return nil
		}
		_ = st.PutAsset(a)
	case CmdDMintInv:
		for _, id := range DecodeInv(payload) {
			id = strings.ToLower(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			if _, ok, _ := st.GetL2Mint(id); ok {
				continue
			}
			if send != nil {
				_ = send(CmdGetMint, []byte(id))
			}
		}
	case CmdGetMint:
		id := strings.ToLower(strings.TrimSpace(string(payload)))
		r, ok, err := st.GetL2Mint(id)
		if err != nil || !ok {
			return nil
		}
		body, _, _ := st.GetL2MintBody(id)
		if send != nil {
			_ = send(CmdMint, EncodeMintWire(r, body))
		}
	case CmdMint:
		r, body, err := DecodeMintWire(payload)
		if err != nil {
			return nil
		}
		net := r.Network
		e.mu.Lock()
		host := e.host
		e.mu.Unlock()
		if host != nil && net == "" {
			net = host.Network()
		}
		accepted, body, err := AcceptL2Mint(r, body, net)
		if err != nil {
			return nil
		}
		if err := st.PutL2Mint(accepted, body); err != nil {
			return nil
		}
		if accepted.Kind == "token" && accepted.Tick != "" && accepted.Amt != "" &&
			(accepted.Op == "mint" || accepted.Op == "deploy") {
			to := accepted.To
			if to == "" {
				to = accepted.Address
			}
			_ = st.CreditL2Balance(to, accepted.Tick, accepted.Amt)
		}
	}
	return nil
}

// OnPeerConnected announces local L2 inventory (assets + signed mints).
func (e *Extension) OnPeerConnected(_ string, _ []string, send func(cmd string, payload []byte) error) {
	st, err := e.storeOrErr()
	if err != nil || send == nil {
		return
	}
	if ids, err := st.ListAssetIDs(200); err == nil && len(ids) > 0 {
		_ = send(CmdDInv, EncodeInv(ids))
	}
	if ids, err := st.ListL2MintIDs(200); err == nil && len(ids) > 0 {
		_ = send(CmdDMintInv, EncodeInv(ids))
	}
}

// P2PCommands lists overlay commands for dogego_p2p_meta.
func P2PCommands() []string {
	return []string{CmdDInv, CmdGetAsset, CmdAsset, CmdDMintInv, CmdGetMint, CmdMint}
}

type mintWire struct {
	Record L2MintRecord `json:"record"`
	BodyB64 string      `json:"content_b64,omitempty"`
}

func EncodeMintWire(r L2MintRecord, body []byte) []byte {
	w := mintWire{Record: r}
	if len(body) > 0 {
		w.BodyB64 = encodeB64(body)
	}
	b, _ := json.Marshal(w)
	return b
}

func DecodeMintWire(b []byte) (L2MintRecord, []byte, error) {
	var w mintWire
	if err := json.Unmarshal(b, &w); err != nil {
		return L2MintRecord{}, nil, err
	}
	var body []byte
	if w.BodyB64 != "" {
		body, _ = decodeB64(w.BodyB64)
	}
	return w.Record, body, nil
}
