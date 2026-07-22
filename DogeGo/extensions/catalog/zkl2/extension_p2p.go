// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/json"
	"fmt"
	"strings"

	"dogego/extensions"
)

// ProtocolID implements extensions.P2PProtocol.
func (e *Extension) ProtocolID() string { return ProtocolID }

// P2PCommands implements extensions.P2PProtocol.
func (e *Extension) P2PCommands() []string {
	return []string{CmdZKInv, CmdGetZKProof, CmdZKProof, CmdGetZKHeaders, CmdZKHeaders, CmdGetZKBlockProofs}
}

// HandleP2P serves zkproof-v1 overlay messages.
func (e *Extension) HandleP2P(cmd string, payload []byte, peer string, send func(string, []byte) error) error {
	switch cmd {
	case CmdZKInv:
		hashes, err := DecodeZKInv(payload)
		if err != nil {
			return err
		}
		e.mu.Lock()
		host := e.host
		e.mu.Unlock()
		e.requestMissingFromPeer(peer, hashes, send)
		if host != nil {
			e.relayZKInv(host, hashes, peer)
		}
		return nil
	case CmdGetZKHeaders:
		start, count, err := DecodeGetZKHeaders(payload)
		if err != nil || send == nil {
			return err
		}
		e.mu.Lock()
		st := e.store
		e.mu.Unlock()
		if st == nil {
			return nil
		}
		lim := int(count)
		if lim <= 0 {
			lim = 256
		}
		heights, counts, err := st.ProofHeightSummary(lim)
		if err != nil {
			return err
		}
		if start > 0 {
			var fh []int64
			var fc []uint32
			for i, h := range heights {
				if h >= start {
					fh = append(fh, h)
					fc = append(fc, counts[i])
				}
			}
			heights, counts = fh, fc
		}
		raw, err := EncodeZKHeaders(heights, counts)
		if err != nil {
			return err
		}
		return send(CmdZKHeaders, raw)
	case CmdZKHeaders:
		heights, counts, err := DecodeZKHeaders(payload)
		if err != nil || send == nil {
			return err
		}
		e.mu.Lock()
		st := e.store
		host := e.host
		e.mu.Unlock()
		if st == nil || host == nil {
			return nil
		}
		for i, h := range heights {
			local, _ := st.ListProofHashesAtHeight(h, 10000)
			if uint32(len(local)) >= counts[i] {
				continue
			}
			bh, err := host.BlockHashAtHeight(h)
			if err != nil {
				continue
			}
			req, err := EncodeGetZKBlockProofs(bh)
			if err != nil {
				continue
			}
			_ = send(CmdGetZKBlockProofs, req)
		}
		return nil
	case CmdGetZKBlockProofs:
		bh, err := DecodeGetZKBlockProofs(payload)
		if err != nil || send == nil {
			return err
		}
		e.mu.Lock()
		st := e.store
		e.mu.Unlock()
		if st == nil {
			return nil
		}
		proofs, err := st.ListProofsByBlock(bh, 10000)
		if err != nil || len(proofs) == 0 {
			return nil
		}
		raw, err := EncodeZKProof(proofs)
		if err != nil {
			return err
		}
		return send(CmdZKProof, raw)
	case CmdGetZKProof:
		hashes, err := DecodeGetZKProof(payload)
		if err != nil || send == nil {
			return err
		}
		var proofs []Proof
		e.mu.Lock()
		st := e.store
		e.mu.Unlock()
		if st == nil {
			return nil
		}
		for _, h := range hashes {
			p, ok, err := st.GetProof(h)
			if err != nil || !ok {
				continue
			}
			proofs = append(proofs, p)
		}
		if len(proofs) == 0 {
			return nil
		}
		raw, err := EncodeZKProof(proofs)
		if err != nil {
			return err
		}
		return send(CmdZKProof, raw)
	case CmdZKProof:
		proofs, err := DecodeZKProof(payload)
		if err != nil {
			return err
		}
		e.mu.Lock()
		st := e.store
		host := e.host
		e.mu.Unlock()
		if st == nil || host == nil {
			return nil
		}
		for _, p := range proofs {
			if err := e.acceptProof(host, st, p); err != nil {
				host.Log(fmt.Sprintf("reject proof from %s: %v", peer, err))
				continue
			}
			e.announceProof(host, p.ProofHash, peer)
		}
		return nil
	default:
		return nil
	}
}

func (e *Extension) acceptProof(host extensions.Host, st *Store, p Proof) error {
	p, err := NormalizeProof(p)
	if err != nil {
		return err
	}
	if err := e.validateProofChain(host, p); err != nil {
		return err
	}
	if err := VerifyProof(p); err != nil {
		return err
	}
	if _, ok, _ := st.GetProof(p.ProofHash); ok {
		return fmt.Errorf("duplicate proof")
	}
	if err := st.PutProof(p); err != nil {
		return err
	}
	proofs, _ := st.ListProofsByBlock(p.BlockHash, 10000)
	root, err := ComputeProofRoot(proofs)
	if err != nil {
		return err
	}
	return st.PutProofRoot(p.BlockHash, root, len(proofs))
}

func (e *Extension) validateProofChain(host extensions.Host, p Proof) error {
	bh, err := host.BlockHashAtHeight(p.BlockHeight)
	if err != nil {
		return fmt.Errorf("block height: %w", err)
	}
	if !strings.EqualFold(bh, p.BlockHash) {
		return fmt.Errorf("block_hash mismatch at height %d", p.BlockHeight)
	}
	idx, ok := host.ConfirmedTxInBlock(p.BlockHash, p.TransactionID)
	if !ok {
		return fmt.Errorf("transaction not confirmed in block")
	}
	if p.TransactionIndex != 0 && p.TransactionIndex != idx {
		return fmt.Errorf("transaction_index mismatch")
	}
	p.TransactionIndex = idx
	_, err = ProofCommitment(p.BlockHash, p.TransactionID, p.ProofHash)
	return err
}

func (e *Extension) rpcSubmitProof(host extensions.Host, p Proof) (map[string]interface{}, error) {
	e.mu.Lock()
	st := e.store
	h := host
	if h == nil {
		h = e.host
	}
	e.mu.Unlock()
	if st == nil {
		return nil, fmt.Errorf("zkl2 not enabled")
	}
	if h == nil {
		return nil, fmt.Errorf("chain unwired")
	}
	if err := e.acceptProof(h, st, p); err != nil {
		return nil, err
	}
	e.announceProof(h, p.ProofHash, "")
	commit, _ := ProofCommitment(p.BlockHash, p.TransactionID, p.ProofHash)
	root, _, _, _ := st.GetProofRoot(p.BlockHash)
	return map[string]interface{}{
		"accepted":    true,
		"proof_hash":  p.ProofHash,
		"commitment":  commit,
		"proof_root":  root,
		"checkzkp":    "verified off L1 (extension OP_CHECKZKP analogue)",
	}, nil
}

func (e *Extension) rpcGetProof(hash string) (Proof, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return Proof{}, fmt.Errorf("zkl2 not enabled")
	}
	p, ok, err := st.GetProof(hash)
	if err != nil {
		return Proof{}, err
	}
	if !ok {
		return Proof{}, fmt.Errorf("proof not found")
	}
	return p, nil
}

func (e *Extension) rpcListProofs(blockHash string, limit int) ([]Proof, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return nil, fmt.Errorf("zkl2 not enabled")
	}
	if strings.TrimSpace(blockHash) == "" {
		return st.ListRecentProofs(limit)
	}
	return st.ListProofsByBlock(blockHash, limit)
}

func (e *Extension) rpcProofRoot(blockHash string) (map[string]interface{}, error) {
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return nil, fmt.Errorf("zkl2 not enabled")
	}
	root, count, ok, err := st.GetProofRoot(blockHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		proofs, _ := st.ListProofsByBlock(blockHash, 10000)
		root, err = ComputeProofRoot(proofs)
		if err != nil {
			return nil, err
		}
		count = len(proofs)
	}
	return map[string]interface{}{
		"block_hash": blockHash,
		"proof_root": root,
		"count":      count,
		"note":       "overlay root; not written to Dogecoin block header",
	}, nil
}

func (e *Extension) rpcVerifyProof(p Proof) (map[string]interface{}, error) {
	if err := ValidateProofPayload(p); err != nil {
		return map[string]interface{}{"valid": false, "error": err.Error()}, nil
	}
	if proofHasChainAnchor(p) {
		if err := ValidateProofStructure(p); err != nil {
			return map[string]interface{}{"valid": false, "error": err.Error()}, nil
		}
	}
	if err := VerifyProof(p); err != nil {
		return map[string]interface{}{"valid": false, "error": err.Error(), "checkzkp": false}, nil
	}
	commit, _ := ProofCommitment(p.BlockHash, p.TransactionID, p.ProofHash)
	return map[string]interface{}{
		"valid":      true,
		"commitment": commit,
		"checkzkp":   true,
		"mode":       p.ProofType,
		"note":       "OP_CHECKZKP-equivalent verify runs in extension only (not L1)",
	}, nil
}

func decodeProofParam(params []json.RawMessage) (Proof, error) {
	if len(params) < 1 {
		return Proof{}, fmt.Errorf("want proof object")
	}
	var p Proof
	if err := json.Unmarshal(params[0], &p); err != nil {
		return Proof{}, err
	}
	return p, nil
}
