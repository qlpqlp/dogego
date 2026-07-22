// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"dogego/signer"
	"dogego/wire"
)

func externalSignerClient(paths *DataPaths) *signer.Client {
	if paths == nil || len(paths.SignerCommand) == 0 {
		return nil
	}
	c, err := signer.New(paths.SignerCommand)
	if err != nil {
		return nil
	}
	return c
}

// execEnumerateSigners lists HWI-compatible external signers (Bitcoin Core enumeratesigners subset).
func execEnumerateSigners(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	c := externalSignerClient(paths)
	if c == nil {
		return []interface{}{}, 0, ""
	}
	devs, err := c.Enumerate()
	if err != nil {
		return nil, -1, "enumeratesigners: " + err.Error()
	}
	out := make([]interface{}, len(devs))
	for i, d := range devs {
		out[i] = d
	}
	return out, 0, ""
}

// execSignerDisplayAddress shows a descriptor address on hardware (signerdisplayaddress).
func execSignerDisplayAddress(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var desc string
	if err := json.Unmarshal(params[0], &desc); err != nil {
		return nil, -8, "signerdisplayaddress: descriptor must be a string"
	}
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return nil, -8, "signerdisplayaddress: descriptor required"
	}
	c := externalSignerClient(paths)
	if c == nil {
		return nil, -1, "signerdisplayaddress: external signer not configured (signer_cmd)"
	}
	addr, err := c.DisplayAddress(desc)
	if err != nil {
		return nil, -1, "signerdisplayaddress: " + err.Error()
	}
	return addr, 0, ""
}

func signPsbtWithExternalSigner(paths *DataPaths, p *wire.Psbt) error {
	c := externalSignerClient(paths)
	if c == nil || p == nil {
		return nil
	}
	_, complete := p.ExtractedTx()
	if complete {
		return nil
	}
	raw, err := p.Serialize()
	if err != nil {
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	signed, err := c.SignPSBT(b64)
	if err != nil {
		return err
	}
	dec, err := base64.StdEncoding.DecodeString(signed)
	if err != nil {
		return err
	}
	np, err := wire.ParsePSBT(dec)
	if err != nil {
		return err
	}
	*p = *np
	return nil
}
