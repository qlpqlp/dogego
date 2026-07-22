// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// statefulprobe emits wallet-anchored stateful mempool probe JSON for reboottestnet live scripts.
//
//	go run ./cmd/statefulprobe -template p2sh_multisig -wif <wif> -txid <id> -vout 0 -amount 50000000
//	go run ./cmd/statefulprobe -template pq_commitment_op_return -wif <wif> -txid <id> -vout 0 -amount 50000000
//	go run ./cmd/statefulprobe -template p2pk_non_standard_input ... -submitblock -prevheader <80-byte-hex> -mineheight 101
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/secp256k1"
)

func main() {
	template := flag.String("template", "", "stateful template name")
	wifStr := flag.String("wif", "", "wallet WIF (reboottestnet)")
	txid := flag.String("txid", "", "funding txid (display endian)")
	vout := flag.Int("vout", 0, "funding vout")
	amount := flag.Int64("amount", 0, "funding amount in koinu")
	height := flag.Int64("height", 0, "chain height for CLTV")
	submitBlock := flag.Bool("submitblock", false, "attach prep_submit_block_hex (p2pk_non_standard_input)")
	prevHeader := flag.String("prevheader", "", "80-byte previous block header hex")
	mineHeight := flag.Int64("mineheight", 0, "block height for submitblock prep")
	flag.Parse()
	if *template == "" || *wifStr == "" || *txid == "" || *amount <= 0 {
		fmt.Fprintln(os.Stderr, "usage: statefulprobe -template T -wif WIF -txid TXID -vout N -amount KOINU [-height H] [-submitblock -prevheader HEX -mineheight N]")
		os.Exit(2)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sec, compressed, err := chain.DecodeWIF(strings.TrimSpace(*wifStr), p.PrivKeyWIFVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wif:", err)
		os.Exit(2)
	}
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	if !compressed {
		fmt.Fprintln(os.Stderr, "wif: want compressed key")
		os.Exit(2)
	}
	prev, err := txidDisplayToLE(strings.TrimSpace(*txid))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	h160 := consensus.PubKeyHash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	fund := consensus.WalletFundingUTXO{
		PrevHash: prev,
		PrevIdx:  uint32(*vout),
		Value:    *amount,
		PkScript: pkScript,
	}
	probe, err := consensus.BuildWalletAnchoredStatefulProbe(*template, priv, pubC, fund, *height)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *submitBlock {
		if *prevHeader == "" || *mineHeight < 1 {
			fmt.Fprintln(os.Stderr, "submitblock requires -prevheader and -mineheight")
			os.Exit(2)
		}
		hdr, err := hex.DecodeString(strings.TrimSpace(*prevHeader))
		if err != nil || len(hdr) != 80 {
			fmt.Fprintln(os.Stderr, "invalid -prevheader (want 80 bytes hex)")
			os.Exit(2)
		}
		if err := consensus.AttachSubmitBlockPrep(&probe, hdr, *mineHeight, chain.RebootTestnet, h160); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(probe); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func txidDisplayToLE(display string) ([32]byte, error) {
	var zero [32]byte
	b, err := hex.DecodeString(display)
	if err != nil || len(b) != 32 {
		return zero, fmt.Errorf("invalid txid hex")
	}
	var out [32]byte
	for i := 0; i < 32; i++ {
		out[i] = b[31-i]
	}
	return out, nil
}
