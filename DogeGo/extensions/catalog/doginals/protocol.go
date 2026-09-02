// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

// Package doginals is an experimental DogeGo L2 for Doginals / DRC-20 / NFT-style assets.
// It indexes L1 OP_RETURN / data-carrier inscriptions (observe-only) and stores off-L1
// metadata/media that syncs among DogeGo peers. It does not change Dogecoin consensus.
package doginals

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	ExtensionID = "dogego.doginals"
	ProtocolID  = "doginals-v1"
	CmdDInv     = "dinv"
	CmdGetAsset = "getdasset"
	CmdAsset    = "dasset"
)

// Inscription is one L1-observed data carrier / DRC-20 / doginal-like event.
type Inscription struct {
	ID           string `json:"id"` // height:txid:vout or txidi{vin}@height
	Height       int64  `json:"height"`
	TxID         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	Vin          uint32 `json:"vin,omitempty"`
	Kind         string `json:"kind"` // drc20 | doginal | data | ordinal
	Tick         string `json:"tick,omitempty"`
	Op           string `json:"op,omitempty"` // deploy|mint|transfer
	Address      string `json:"address,omitempty"`   // sender / owner when known
	Recipient    string `json:"recipient,omitempty"` // transfer destination when known
	Amount       string `json:"amount,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	TextPreview  string `json:"text_preview,omitempty"`
	PayloadHex   string `json:"payload_hex,omitempty"`
	Source       string `json:"source,omitempty"` // opreturn | envelope
	Outpoint     string `json:"outpoint,omitempty"` // created transferable outpoint when applicable
	RecordedUnix int64  `json:"recorded_unix"`
}

// Asset is an off-L1 L2 record (NFT / token metadata / image pointer).
type Asset struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"` // nft | token | image | collection
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
	URI             string `json:"uri,omitempty"`
	ContentB64      string `json:"content_b64,omitempty"`
	L1InscriptionID string `json:"l1_inscription_id,omitempty"`
	CreatedUnix     int64  `json:"created_unix"`
	UpdatedUnix     int64  `json:"updated_unix"`
	CreatorNote     string `json:"creator_note,omitempty"`
}

// AssetIDFromContent hashes kind+name+uri+content for a stable id when none provided.
func AssetIDFromContent(a Asset) string {
	h := sha256.Sum256([]byte(strings.ToLower(a.Kind) + "|" + a.Name + "|" + a.URI + "|" + a.ContentB64 + "|" + a.L1InscriptionID))
	return hex.EncodeToString(h[:16])
}

// NormalizeAsset fills defaults and id.
func NormalizeAsset(a Asset) (Asset, error) {
	a.Kind = strings.ToLower(strings.TrimSpace(a.Kind))
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return a, fmt.Errorf("asset name required")
	}
	switch a.Kind {
	case "nft", "token", "image", "collection":
	case "":
		a.Kind = "nft"
	default:
		return a, fmt.Errorf("asset kind must be nft|token|image|collection")
	}
	now := time.Now().Unix()
	if a.CreatedUnix == 0 {
		a.CreatedUnix = now
	}
	a.UpdatedUnix = now
	if strings.TrimSpace(a.ID) == "" {
		a.ID = AssetIDFromContent(a)
	}
	a.ID = strings.ToLower(strings.TrimSpace(a.ID))
	return a, nil
}

// DRC20Payload is the common JSON shape used on Dogecoin data carriers.
type DRC20Payload struct {
	P    string `json:"p"`
	Op   string `json:"op"`
	Tick string `json:"tick"`
	Amt  string `json:"amt"`
	Max  string `json:"max,omitempty"`
	Lim  string `json:"lim,omitempty"`
}

// ParseDRC20JSON tries to decode a DRC-20 JSON object from bytes.
func ParseDRC20JSON(b []byte) (DRC20Payload, bool) {
	var p DRC20Payload
	s := strings.TrimSpace(string(b))
	if s == "" || s[0] != '{' {
		return p, false
	}
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return p, false
	}
	if !strings.EqualFold(p.P, "drc-20") && !strings.EqualFold(p.P, "drc20") {
		return p, false
	}
	p.P = "drc-20"
	p.Op = strings.ToLower(strings.TrimSpace(p.Op))
	p.Tick = strings.ToUpper(strings.TrimSpace(p.Tick))
	return p, p.Tick != ""
}
