// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"dogego/consensus"
	"dogego/extensions"
	"dogego/mempool"
	"dogego/wire"
)

const ExtensionID = "dogego.zkl2"

// Extension implements the optional DogeGo ZK L2 layer (no L1 consensus fork).
type Extension struct {
	manifest extensions.Manifest
	mu       sync.Mutex
	store    *Store
	host     extensions.Host
	syncStop chan struct{}
}

// NewExtension builds the builtin zkl2 extension.
func NewExtension(m extensions.Manifest) (extensions.Extension, error) {
	if m.ID != "" && m.ID != ExtensionID {
		return nil, fmt.Errorf("zkl2: unexpected id %q", m.ID)
	}
	return &Extension{manifest: DefaultManifest()}, nil
}

// DefaultManifest is the dogego.zkl2 catalog entry (subprocess package).
func DefaultManifest() extensions.Manifest {
	return extensions.Manifest{
		ManifestVersion: extensions.ManifestVersion,
		ID:              ExtensionID,
		Name:            "DogeGo ZK Layer 2",
		Version:         "0.1.0",
		Author:          "DogeGo",
		Description:     "Optional zkproof-v1 overlay: tx-anchored ZK proofs with off-L1 OP_CHECKZKP verify. Inspired by Dogecoin #3869; no protocol fork.",
		Homepage:        "https://github.com/qlpqlp/dogego",
		Repository:      "https://github.com/qlpqlp/dogego",
		DogeGoMinVersion: "0.1.0",
		Permissions:     []string{"chain_read", "chain_index", "datadir_write", "rpc_register", "p2p_extension", "ui_panel", "wallet_rpc"},
		Networks:        []string{"mainnet", "testnet"},
		UI: extensions.ManifestUI{
			StatusMethod: "info",
		},
		Entry: extensions.Entry{
			Type:   extensions.EntrySubprocess,
			Module: ExtensionID,
			Binary: "zkl2-ext",
		},
		Capabilities: []string{"rpc", "indexer", "l2_sync", "p2p", "zkproof-v1"},
		DocsPath:     "extensions/catalog/zkl2/docs/USER_GUIDE.md",
		Icon:         "icon.png",
	}
}

func (e *Extension) Manifest() extensions.Manifest {
	if e.manifest.ID == "" {
		e.manifest = DefaultManifest()
	}
	return e.manifest
}

func (e *Extension) OnEnable(ctx context.Context, host extensions.Host) error {
	if host == nil {
		return fmt.Errorf("zkl2: host required")
	}
	dir, err := host.ExtensionDataDir(ExtensionID)
	if err != nil {
		return err
	}
	st, err := OpenStore(dir)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.store = st
	e.host = host
	e.syncStop = make(chan struct{})
	e.mu.Unlock()
	go e.finishEnable(host, dir)
	return nil
}

func (e *Extension) finishEnable(host extensions.Host, dir string) {
	vkDir := filepath.Join(dir, "vk")
	if err := LoadVKDir(vkDir); err != nil {
		host.Log("zkl2 vk load: " + err.Error())
	} else if err := ensureDefaultDemoVK(vkDir); err != nil {
		host.Log("zkl2 demo vk install: " + err.Error())
	}
	host.Log("zkl2 enabled; scanning L1 for ZKDG anchors")
	go e.scanRecentBlocks(host)
	go e.runBackgroundSync()
}

func (e *Extension) OnDisable() error {
	e.mu.Lock()
	stop := e.syncStop
	e.syncStop = nil
	defer e.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if e.store != nil {
		_ = e.store.Close()
		e.store = nil
	}
	e.host = nil
	return nil
}

func (e *Extension) HandleRPC(method string, params []json.RawMessage, host extensions.Host) (interface{}, error) {
	switch method {
	case "info":
		return e.rpcInfo()
	case "listanchors":
		limit := 50
		if len(params) > 0 {
			var n float64
			if json.Unmarshal(params[0], &n) == nil && int(n) > 0 {
				limit = int(n)
			}
		}
		return e.rpcListAnchors(limit)
	case "verifyanchor":
		if len(params) < 1 {
			return nil, fmt.Errorf("want script_hex")
		}
		var scriptHex string
		if err := json.Unmarshal(params[0], &scriptHex); err != nil {
			return nil, err
		}
		return consensus.VerifyZKAnchorScriptHex(scriptHex)
	case "prepareanchor":
		if len(params) < 1 {
			return nil, fmt.Errorf("want l2 header object")
		}
		var hdr L2BlockHeader
		if err := json.Unmarshal(params[0], &hdr); err != nil {
			return nil, err
		}
		return e.rpcPrepareAnchor(host, hdr)
	case "signanchor":
		if len(params) < 1 {
			return nil, fmt.Errorf("want l2 header object with signer_address")
		}
		var hdr L2BlockHeader
		if err := json.Unmarshal(params[0], &hdr); err != nil {
			return nil, err
		}
		return e.rpcSignAnchor(host, hdr)
	case "submitl2block":
		if len(params) < 1 {
			return nil, fmt.Errorf("want l2 block object")
		}
		var blk L2Block
		if err := json.Unmarshal(params[0], &blk); err != nil {
			return nil, err
		}
		return e.rpcSubmitL2(blk)
	case "getl2block":
		if len(params) < 1 {
			return nil, fmt.Errorf("want l2 height")
		}
		var h float64
		if err := json.Unmarshal(params[0], &h); err != nil {
			return nil, err
		}
		return e.rpcGetL2(uint64(h))
	case "listl2blocks":
		limit := 20
		if len(params) > 0 {
			var n float64
			if json.Unmarshal(params[0], &n) == nil && int(n) > 0 {
				limit = int(n)
			}
		}
		return e.rpcListL2(limit)
	case "verifyl2block":
		if len(params) < 1 {
			return nil, fmt.Errorf("want l2 block object")
		}
		var blk L2Block
		if err := json.Unmarshal(params[0], &blk); err != nil {
			return nil, err
		}
		return e.rpcVerifyL2(blk)
	case "submitproof":
		p, err := decodeProofParam(params)
		if err != nil {
			return nil, err
		}
		return e.rpcSubmitProof(host, p)
	case "getproof":
		if len(params) < 1 {
			return nil, fmt.Errorf("want proof_hash")
		}
		var h string
		if err := json.Unmarshal(params[0], &h); err != nil {
			return nil, err
		}
		return e.rpcGetProof(h)
	case "listproofs":
		blockHash := ""
		limit := 50
		if len(params) > 0 {
			_ = json.Unmarshal(params[0], &blockHash)
		}
		if len(params) > 1 {
			var n float64
			if json.Unmarshal(params[1], &n) == nil {
				limit = int(n)
			}
		}
		return e.rpcListProofs(blockHash, limit)
	case "verifyproof":
		p, err := decodeProofParam(params)
		if err != nil {
			return nil, err
		}
		return e.rpcVerifyProof(p)
	case "proofroot":
		if len(params) < 1 {
			return nil, fmt.Errorf("want block_hash")
		}
		var bh string
		if err := json.Unmarshal(params[0], &bh); err != nil {
			return nil, err
		}
		return e.rpcProofRoot(bh)
	case "checkzkp":
		p, err := decodeProofParam(params)
		if err != nil {
			return nil, err
		}
		return e.rpcVerifyProof(p)
	case "generateproof":
		req, err := decodeGenerateProofParams(params)
		if err != nil {
			return nil, err
		}
		return e.rpcGenerateProof(host, req)
	case "installdefaultvk":
		return e.rpcInstallDefaultVK(host)
	default:
		return nil, fmt.Errorf("unknown zkl2 rpc %q", method)
	}
}

func (e *Extension) rpcInfo() (map[string]interface{}, error) {
	e.mu.Lock()
	st := e.store
	host := e.host
	e.mu.Unlock()
	out := map[string]interface{}{
		"id":                 ExtensionID,
		"p2p_protocol":       ProtocolID,
		"protocol_version":   ProtocolVersion,
		"anchor_tag":         consensus.ZKAnchorTag,
		"proof_mode_groth16": ProofModeGroth16,
		"proof_mode_commitment": ProofModeCommitment,
		"checkzkp":           "OP_CHECKZKP-equivalent runs in extension only (L1 unchanged)",
		"note":               "zkproof-v1 overlay; Dogecoin L1 never verifies ZK proofs.",
	}
	if host != nil {
		out["network"] = host.Network()
		if tip, err := host.TipHeight(); err == nil {
			out["doge_tip_height"] = tip
		}
	}
	if st != nil {
		if tip, err := st.TipL2Height(); err == nil {
			out["l2_tip_height"] = tip
		}
		if n, err := st.ProofCount(); err == nil {
			out["proof_total"] = n
		}
		if proofs, err := st.ListRecentProofs(12); err == nil && len(proofs) > 0 {
			slim := make([]map[string]interface{}, 0, len(proofs))
			for _, p := range proofs {
				slim = append(slim, map[string]interface{}{
					"proof_hash":        p.ProofHash,
					"transaction_id":    p.TransactionID,
					"block_hash":        p.BlockHash,
					"block_height":      p.BlockHeight,
					"created_timestamp": p.CreatedTimestamp,
					"proof_type":        p.ProofType,
				})
			}
			out["recent_proofs"] = slim
		}
		heights, counts, _ := st.ProofHeightSummary(5)
		if len(heights) > 0 {
			out["proof_heights_sample"] = heights
			out["proof_counts_sample"] = counts
		}
	}
	if oh, ok := host.(extensions.OverlayHost); ok {
		out["zkproof_v1_peers"] = oh.OverlayPeerCount(ProtocolID)
	}
	out["groth16"] = LoadedVKSummary()
	out["ui"] = buildUIPanel(out)
	return out, nil
}

func (e *Extension) rpcInstallDefaultVK(host extensions.Host) (map[string]interface{}, error) {
	if host == nil {
		return nil, fmt.Errorf("zkl2: host required")
	}
	dir, err := host.ExtensionDataDir(ExtensionID)
	if err != nil {
		return nil, err
	}
	vkDir := filepath.Join(dir, "vk")
	n, err := InstallDefaultDemoVK(vkDir)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"installed":        true,
		"path":             filepath.Join(vkDir, defaultVKFile),
		"bytes":            n,
		"pairing_enabled":  true,
		"note":             "Demo Groth16 verifying key (esuwu/groth16-verifier-bls12381, 1 public input). Replace with your circuit VK for production.",
		"groth16":          LoadedVKSummary(),
	}, nil
}

func (e *Extension) rpcListAnchors(limit int) ([]AnchorRecord, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return nil, fmt.Errorf("zkl2 not enabled")
	}
	return st.ListAnchors(limit)
}

func (e *Extension) rpcPrepareAnchor(host extensions.Host, hdr L2BlockHeader) (map[string]interface{}, error) {
	if hdr.ProofMode == 0 {
		hdr.ProofMode = ProofModeGroth16
	}
	if err := ValidateL2Header(hdr); err != nil {
		return nil, err
	}
	anchorHash, err := AnchorHashFromHeader(hdr)
	if err != nil {
		return nil, err
	}
	script, err := consensus.BuildZKAnchorScript(mustDecode32(anchorHash))
	if err != nil {
		return nil, err
	}
	msg, err := PrepareAnchorMessageJSON(host.Network(), hdr)
	if err != nil {
		return nil, err
	}
	signVia := "Use wallet signmessage RPC with signer_address; extensions cannot access private keys."
	if _, ok := host.(extensions.WalletRPCHost); ok {
		signVia = "Unlock wallet with walletpassphrase, then call signanchor (wallet_rpc) or sign sign_message manually via signmessage RPC."
	}
	return map[string]interface{}{
		"anchor_hash":       anchorHash,
		"anchor_script_hex": hex.EncodeToString(script),
		"sign_message":      msg,
		"sign_via":          signVia,
		"discussion_ref":    "https://github.com/dogecoin/dogecoin/discussions/3869",
	}, nil
}

func (e *Extension) rpcSignAnchor(host extensions.Host, hdr L2BlockHeader) (map[string]interface{}, error) {
	addr := strings.TrimSpace(hdr.SignerAddress)
	if addr == "" {
		return nil, fmt.Errorf("signer_address required in l2 header")
	}
	wh, ok := host.(extensions.WalletRPCHost)
	if !ok {
		return nil, fmt.Errorf("wallet_rpc not available: enable wallet_rpc permission and unlock wallet via walletpassphrase RPC")
	}
	prep, err := e.rpcPrepareAnchor(host, hdr)
	if err != nil {
		return nil, err
	}
	msg, _ := prep["sign_message"].(string)
	if msg == "" {
		return nil, fmt.Errorf("prepare anchor sign_message missing")
	}
	addrRaw, err := json.Marshal(addr)
	if err != nil {
		return nil, err
	}
	msgRaw, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	sig, err := wh.CallWalletRPC("signmessage", []json.RawMessage{addrRaw, msgRaw})
	if err != nil {
		return nil, err
	}
	prep["signature"] = sig
	prep["signer_address"] = addr
	prep["signed"] = true
	prep["sign_via"] = "extension wallet_rpc (signmessage)"
	return prep, nil
}

func (e *Extension) rpcSubmitL2(blk L2Block) (map[string]interface{}, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return nil, fmt.Errorf("zkl2 not enabled")
	}
	if err := ValidateL2Block(blk); err != nil {
		return nil, err
	}
	if err := st.PutL2Block(blk); err != nil {
		return nil, err
	}
	return map[string]interface{}{"accepted": true, "l2_height": blk.Header.L2Height}, nil
}

func (e *Extension) rpcGetL2(height uint64) (L2Block, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return L2Block{}, fmt.Errorf("zkl2 not enabled")
	}
	b, ok, err := st.GetL2Block(height)
	if err != nil {
		return L2Block{}, err
	}
	if !ok {
		return L2Block{}, fmt.Errorf("l2 block not found")
	}
	return b, nil
}

func (e *Extension) rpcListL2(limit int) ([]L2Block, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return nil, fmt.Errorf("zkl2 not enabled")
	}
	return st.ListL2Blocks(limit)
}

func (e *Extension) rpcVerifyL2(blk L2Block) (map[string]interface{}, error) {
	if err := ValidateL2Block(blk); err != nil {
		return map[string]interface{}{"valid": false, "error": err.Error()}, nil
	}
	anchor, _ := AnchorHashFromHeader(blk.Header)
	return map[string]interface{}{
		"valid":       true,
		"anchor_hash": anchor,
		"note":        "Groth16 pairing runs when data/vk/default.vk is installed (192 B compressed or 384 B DIP affine proofs)",
	}, nil
}

func (e *Extension) scanRecentBlocks(host extensions.Host) {
	if host == nil {
		return
	}
	tip, err := host.TipHeight()
	if err != nil || tip < 0 {
		return
	}
	start := tip - 512
	if start < 0 {
		start = 0
	}
	for h := start; h <= tip; h++ {
		_ = e.indexBlockHeight(host, h)
	}
}

func (e *Extension) indexBlockHeight(host extensions.Host, height int64) error {
	raw, err := host.GetRawBlockByHeight(height)
	if err != nil || len(raw) < 80 {
		return err
	}
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return fmt.Errorf("zkl2 store closed")
	}
	return wire.ForEachBlockTx(raw, func(i uint32, tx *wire.Tx) error {
		for vout, o := range tx.Vout {
			a, ok := consensus.DetectZKAnchor(o.PkScript)
			if !ok {
				continue
			}
			txid := mempool.TxIDDisplayHex(tx.TxHash())
			rec := AnchorRecord{
				AnchorHash: strings.ToLower(a.AnchorHash),
				TxID:       txid,
				Height:     height,
				Vout:       uint32(vout),
			}
			_ = st.PutAnchor(rec)
		}
		return nil
	})
}

func mustDecode32(h string) []byte {
	b, _ := hex.DecodeString(h)
	return b
}

// Store returns the active Pebble store (tests).
func (e *Extension) Store() *Store {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.store
}
