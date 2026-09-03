// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"dogego/chain"
	"dogego/secp256k1/ecdsa"
	"dogego/wire"

	"golang.org/x/crypto/ripemd160"
)

const (
	L2ProtocolName = "doginals-l2"
	L2ProtocolV    = 1
	// MaxL2MintBodyBytes caps inline media for L2 mints (images/files).
	MaxL2MintBodyBytes = 4 << 20
)

// L2MintRecord is a wallet-signed off-L1 mint / inscription (not Dogecoin consensus).
// Peers verify Signature with Dogecoin signmessage rules before accepting.
type L2MintRecord struct {
	ID           string `json:"id"`
	P            string `json:"p"`  // doginals-l2
	V            int    `json:"v"`  // 1
	Op           string `json:"op"` // mint|deploy|transfer|inscribe
	Kind         string `json:"kind"` // token|image|file|nft
	Tick         string `json:"tick,omitempty"`
	Amt          string `json:"amt,omitempty"`
	Max          string `json:"max,omitempty"`
	Lim          string `json:"lim,omitempty"`
	Address      string `json:"address"` // signer (P2PKH)
	To           string `json:"to,omitempty"`
	Name         string `json:"name,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	ContentHash  string `json:"content_hash,omitempty"` // sha256 hex of body
	ContentB64   string `json:"content_b64,omitempty"`  // optional inline (omitted on wire if large)
	URI          string `json:"uri,omitempty"`
	Nonce        string `json:"nonce"`
	CreatedUnix  int64  `json:"created_unix"`
	Network      string `json:"network,omitempty"`
	Signature    string `json:"signature,omitempty"`
	MediaKind    string `json:"media_kind,omitempty"`
	Size         int    `json:"size,omitempty"`
	HasContent   bool   `json:"has_content,omitempty"`
	RecordedUnix int64  `json:"recorded_unix,omitempty"`
}

// CanonicalSignMessage returns the stable JSON string the wallet must sign (no signature field).
func (r L2MintRecord) CanonicalSignMessage() (string, error) {
	cp := r
	cp.Signature = ""
	cp.RecordedUnix = 0
	// Keep content_b64 out of the signed payload when content_hash is set (sign the hash).
	if cp.ContentHash != "" {
		cp.ContentB64 = ""
	}
	b, err := json.Marshal(cp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ContentSHA256Hex returns hex sha256 of body bytes.
func ContentSHA256Hex(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func newNonce() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func mintRecordID(r L2MintRecord) string {
	msg, _ := r.CanonicalSignMessage()
	h := sha256.Sum256([]byte(msg + "|" + r.Signature))
	return hex.EncodeToString(h[:16])
}

// PrepareL2Mint builds an unsigned L2 mint record from a request map.
func PrepareL2Mint(raw map[string]interface{}, network string) (L2MintRecord, []byte, error) {
	var z L2MintRecord
	if raw == nil {
		return z, nil, fmt.Errorf("json body required")
	}
	addr := strings.TrimSpace(fmt.Sprint(raw["address"]))
	if addr == "" {
		addr = strings.TrimSpace(fmt.Sprint(raw["to"]))
	}
	if addr == "" {
		return z, nil, fmt.Errorf("address required (P2PKH signer)")
	}
	op := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw["op"])))
	if op == "" {
		op = "mint"
	}
	kind := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw["kind"])))
	tick := strings.ToUpper(strings.TrimSpace(fmt.Sprint(raw["tick"])))
	amt := strings.TrimSpace(fmt.Sprint(raw["amount"]))
	if amt == "" {
		amt = strings.TrimSpace(fmt.Sprint(raw["amt"]))
	}
	name := strings.TrimSpace(fmt.Sprint(raw["name"]))
	uri := strings.TrimSpace(fmt.Sprint(raw["uri"]))
	ct := strings.TrimSpace(fmt.Sprint(raw["content_type"]))
	b64 := strings.TrimSpace(fmt.Sprint(raw["content_b64"]))
	if b64 == "" {
		b64 = strings.TrimSpace(fmt.Sprint(raw["file_b64"]))
	}

	var body []byte
	if b64 != "" {
		rawBody, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			// try raw std without padding issues
			rawBody, err = base64.RawStdEncoding.DecodeString(b64)
			if err != nil {
				return z, nil, fmt.Errorf("content_b64: %w", err)
			}
		}
		if len(rawBody) > MaxL2MintBodyBytes {
			return z, nil, fmt.Errorf("content exceeds %d bytes", MaxL2MintBodyBytes)
		}
		body = rawBody
	}

	switch kind {
	case "token", "image", "file", "nft":
	case "":
		if len(body) > 0 {
			mk := ClassifyMediaKind(ct, body, false)
			switch mk {
			case "image":
				kind = "image"
			case "token":
				kind = "token"
			case "text", "json":
				kind = "file"
			default:
				kind = "file"
			}
		} else if tick != "" {
			kind = "token"
		} else {
			kind = "nft"
		}
	default:
		return z, nil, fmt.Errorf("kind must be token|image|file|nft")
	}

	switch op {
	case "mint", "deploy", "transfer", "inscribe":
	default:
		return z, nil, fmt.Errorf("op must be mint|deploy|transfer|inscribe")
	}

	if kind == "token" && tick == "" {
		return z, nil, fmt.Errorf("tick required for token mints")
	}
	if (kind == "image" || kind == "file") && len(body) == 0 && uri == "" {
		return z, nil, fmt.Errorf("content_b64 or uri required for image/file mint")
	}
	if name == "" {
		switch kind {
		case "token":
			name = tick + " L2"
		case "image":
			name = "Doginal image"
		case "file":
			name = "Doginal file"
		default:
			name = "Doginal L2"
		}
	}
	if ct == "" && len(body) > 0 {
		ct = sniffContentType(body)
	}
	to := strings.TrimSpace(fmt.Sprint(raw["to"]))
	if to == "" {
		to = addr
	}

	r := L2MintRecord{
		P:           L2ProtocolName,
		V:           L2ProtocolV,
		Op:          op,
		Kind:        kind,
		Tick:        tick,
		Amt:         amt,
		Max:         strings.TrimSpace(fmt.Sprint(raw["max"])),
		Lim:         strings.TrimSpace(fmt.Sprint(raw["lim"])),
		Address:     addr,
		To:          to,
		Name:        name,
		ContentType: ct,
		URI:         uri,
		Nonce:       newNonce(),
		CreatedUnix: time.Now().Unix(),
		Network:     network,
		Size:        len(body),
		HasContent:  len(body) > 0,
	}
	if len(body) > 0 {
		r.ContentHash = ContentSHA256Hex(body)
		r.MediaKind = ClassifyMediaKind(ct, body, kind == "token")
		// Keep small inline preview in prepare response only; commit stores body separately.
		if len(body) <= 64*1024 {
			r.ContentB64 = base64.StdEncoding.EncodeToString(body)
		}
	} else if kind == "token" {
		r.MediaKind = "token"
	} else {
		r.MediaKind = kind
	}
	return r, body, nil
}

// VerifyDogecoinMessage checks Core/Dogecoin signmessage compact signature for a P2PKH address.
func VerifyDogecoinMessage(network, address, sigB64, message string) (bool, error) {
	address = strings.TrimSpace(address)
	sigB64 = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, sigB64)
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		net = chain.MainnetDogecoin
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return false, err
	}
	v, wantH160, err := chain.Base58CheckDecode(address)
	if err != nil {
		return false, fmt.Errorf("invalid address")
	}
	if v != p.PubkeyHashAddrID {
		return false, fmt.Errorf("signer must be P2PKH address")
	}
	rawSig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(rawSig) != 65 {
		return false, fmt.Errorf("malformed signature")
	}
	h := dogecoinMessageHash(message)
	pub, _, err := ecdsa.RecoverCompact(rawSig, h[:])
	if err != nil {
		return false, nil
	}
	if hash160(pub.SerializeCompressed()) == wantH160 || hash160(pub.SerializeUncompressed()) == wantH160 {
		return true, nil
	}
	return false, nil
}

func dogecoinMessageHash(msg string) [32]byte {
	var buf bytes.Buffer
	magic := []byte("Dogecoin Signed Message:\n")
	_ = wire.WriteCompactSize(&buf, uint64(len(magic)))
	buf.Write(magic)
	mb := []byte(msg)
	_ = wire.WriteCompactSize(&buf, uint64(len(mb)))
	buf.Write(mb)
	s := sha256.Sum256(buf.Bytes())
	return sha256.Sum256(s[:])
}

func hash160(pub []byte) [20]byte {
	sh := sha256.Sum256(pub)
	r := ripemd160.New()
	_, _ = r.Write(sh[:])
	var out [20]byte
	copy(out[:], r.Sum(nil))
	return out
}

// AcceptL2Mint verifies signature + content hash and returns the normalized record.
func AcceptL2Mint(r L2MintRecord, body []byte, network string) (L2MintRecord, []byte, error) {
	if r.P != L2ProtocolName || r.V != L2ProtocolV {
		return r, nil, fmt.Errorf("unsupported L2 mint protocol")
	}
	if strings.TrimSpace(r.Signature) == "" {
		return r, nil, fmt.Errorf("signature required")
	}
	if r.Address == "" {
		return r, nil, fmt.Errorf("address required")
	}
	if network == "" {
		network = r.Network
	}
	if len(body) == 0 && r.ContentB64 != "" {
		b, err := base64.StdEncoding.DecodeString(r.ContentB64)
		if err == nil {
			body = b
		}
	}
	if len(body) > MaxL2MintBodyBytes {
		return r, nil, fmt.Errorf("content too large")
	}
	if len(body) > 0 {
		sum := ContentSHA256Hex(body)
		if r.ContentHash != "" && !strings.EqualFold(r.ContentHash, sum) {
			return r, nil, fmt.Errorf("content_hash mismatch")
		}
		r.ContentHash = sum
		r.Size = len(body)
		r.HasContent = true
		if r.MediaKind == "" {
			r.MediaKind = ClassifyMediaKind(r.ContentType, body, r.Kind == "token")
		}
	}
	// Strip body from signed message path.
	unsigned := r
	unsigned.Signature = ""
	unsigned.ContentB64 = ""
	msg, err := unsigned.CanonicalSignMessage()
	if err != nil {
		return r, nil, err
	}
	// Also accept signing the prepare payload that included small content_b64.
	ok, err := VerifyDogecoinMessage(network, r.Address, r.Signature, msg)
	if err != nil {
		return r, nil, err
	}
	if !ok && r.ContentB64 != "" {
		// Retry with content_b64 present (prepare signed the small inline form).
		withB64 := unsigned
		withB64.ContentB64 = r.ContentB64
		msg2, err2 := withB64.CanonicalSignMessage()
		if err2 == nil {
			ok, err = VerifyDogecoinMessage(network, r.Address, r.Signature, msg2)
			if err != nil {
				return r, nil, err
			}
			if ok {
				msg = msg2
			}
		}
	}
	if !ok {
		return r, nil, fmt.Errorf("invalid signature")
	}
	r.ContentB64 = "" // store body separately
	if r.ID == "" {
		r.ID = mintRecordID(r)
	}
	r.RecordedUnix = time.Now().Unix()
	_ = msg
	return r, body, nil
}
