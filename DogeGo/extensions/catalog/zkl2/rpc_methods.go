// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import "dogego/extensions"

// RPCMethods implements extensions.RPCProvider (extension-owned RPC surface).
func (e *Extension) RPCMethods() []extensions.RPCMethod {
	return []extensions.RPCMethod{
		{Name: "info", Help: "zkproof-v1 extension status (requires dogego.zkl2 enabled)."},
		{Name: "installdefaultvk", Help: "Install bundled demo Groth16 verifying key to data/vk/default.vk (pairing smoke tests)."},
		{Name: "submitproof", Help: "Accept a tx-anchored ZK proof after off-L1 CheckZKP verify."},
		{Name: "generateproof", Help: "Generate a proof from text/file (commitment or Groth16). Optionally submit when anchored to a confirmed tx."},
		{Name: "getproof", Help: "Fetch proof by proof_hash."},
		{Name: "listproofs", Help: "List proofs for a block_hash."},
		{Name: "verifyproof", Help: "Structural + CheckZKP verify without storing. Optional verifying_key or verifying_key_chunks (6×80 B #3869 stack)."},
		{Name: "checkzkp", Help: "Alias for verifyproof (OP_CHECKZKP analogue off L1)."},
		{Name: "proofroot", Help: "Overlay ProofRoot for a Dogecoin block (not in L1 header)."},
		{Name: "listanchors", Help: "List indexed optional ZKDG OP_RETURN anchors."},
		{Name: "verifyanchor", Help: "Verify ZKDG OP_RETURN script format."},
		{Name: "prepareanchor", Help: "Prepare optional ZKDG anchor + signmessage payload."},
		{Name: "signanchor", Help: "Prepare anchor and sign via wallet_rpc (requires walletpassphrase + signer_address in header)."},
		{Name: "submitl2block", Help: "Store an L2 block in local Pebble index."},
		{Name: "getl2block", Help: "Fetch L2 block by height."},
		{Name: "listl2blocks", Help: "List recent L2 blocks."},
		{Name: "verifyl2block", Help: "Structural L2 block verify."},
	}
}
