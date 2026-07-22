// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
)

// BIP44 coin type for Dogecoin (SLIP-44).
const bip44CoinType uint32 = 3

const (
	hardenedFlag            = 0x80000000
	bip44Purpose            = 44
	defaultKeypoolSize      = 100
	keypoolRefillThreshold  = defaultKeypoolSize / 2 // Core TopUpKeyPool when pool falls below half target
	maxHDExternalIndex      = 1_000_000
)

// bip44ReceivePath is m/44'/3'/account'/0/index.
func bip44ReceivePath(account, index uint32) []uint32 {
	return []uint32{
		hardenedFlag + bip44Purpose,
		hardenedFlag + bip44CoinType,
		hardenedFlag + account,
		0,
		index,
	}
}

// bip44ChangePath is m/44'/3'/account'/1/index (internal / change).
func bip44ChangePath(account, index uint32) []uint32 {
	return []uint32{
		hardenedFlag + bip44Purpose,
		hardenedFlag + bip44CoinType,
		hardenedFlag + account,
		1,
		index,
	}
}

// bip44NodeTipPath is m/44'/3'/account'/2/index (dedicated public node-tip key; not Receive/Change).
func bip44NodeTipPath(account, index uint32) []uint32 {
	return []uint32{
		hardenedFlag + bip44Purpose,
		hardenedFlag + bip44CoinType,
		hardenedFlag + account,
		2,
		index,
	}
}

type hdDerived struct {
	Index  uint32
	Addr   string
	Script []byte
	Priv   *secp256k1.PrivateKey
}

func deriveHDAt(seed []byte, addrVer byte, path []uint32) (hdDerived, error) {
	if len(seed) < 16 || len(seed) > 64 {
		return hdDerived{}, fmt.Errorf("hd seed length %d out of range", len(seed))
	}
	ek, err := derivePath(seed, path)
	if err != nil {
		return hdDerived{}, err
	}
	addr := p2pkh(addrVer, ek.key.PubKey())
	script := p2pkhScriptFromHash160(addrVer, addr)
	return hdDerived{Addr: addr, Script: script, Priv: ek.key}, nil
}

func p2pkhScriptFromHash160(addrVer byte, addr string) []byte {
	_, h160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		return nil
	}
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	return append(pk, 0x88, 0xac)
}

func p2pkhScriptFromPubkey(addrVer byte, pub *secp256k1.PublicKey) []byte {
	comp := pub.SerializeCompressed()
	h := sha256.Sum256(comp)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	h160 := r.Sum(nil)
	pk := append([]byte{0x76, 0xa9, 0x14}, h160...)
	return append(pk, 0x88, 0xac)
}

func newHDSeed() ([]byte, error) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	return seed[:], nil
}

func (w *Disk) hdEnabled() bool {
	return len(w.hdSeed) >= 16
}

func (w *Disk) deriveReceive(index uint32) (hdDerived, error) {
	d, err := deriveHDAt(w.hdSeed, w.addrVer, bip44ReceivePath(0, index))
	if err != nil {
		return d, err
	}
	d.Index = index
	return d, nil
}

func (w *Disk) deriveChange(index uint32) (hdDerived, error) {
	d, err := deriveHDAt(w.hdSeed, w.addrVer, bip44ChangePath(0, index))
	if err != nil {
		return d, err
	}
	d.Index = index
	return d, nil
}

func (w *Disk) deriveNodeTip(index uint32) (hdDerived, error) {
	d, err := deriveHDAt(w.hdSeed, w.addrVer, bip44NodeTipPath(0, index))
	if err != nil {
		return d, err
	}
	d.Index = index
	return d, nil
}

// hdMaxReceiveIndexLocked is the highest allocated BIP44 external index (issued or keypool).
func (w *Disk) hdMaxReceiveIndexLocked() uint32 {
	var max uint32
	if w.hdExternalNext > 1 {
		max = w.hdExternalNext - 1
	}
	for _, idx := range w.hdKeypool {
		if idx > max {
			max = idx
		}
	}
	return max
}

// hdMaxChangeIndexLocked is the highest allocated BIP44 internal (change) index (issued or keypool).
func (w *Disk) hdMaxChangeIndexLocked() uint32 {
	var max uint32
	if w.hdChangeNext > 0 {
		max = w.hdChangeNext - 1
	}
	for _, idx := range w.hdChangeKeypool {
		if idx > max {
			max = idx
		}
	}
	return max
}

func (w *Disk) initHDLocked() error {
	seed, err := newHDSeed()
	if err != nil {
		return err
	}
	w.hdSeed = seed
	w.hdChangeNext = 0
	d0, err := w.deriveReceive(0)
	if err != nil {
		return err
	}
	w.priv = d0.Priv
	w.addr = d0.Addr
	w.seedInitialReceiveKeypoolLocked()
	return w.topUpChangeKeypoolLocked(defaultKeypoolSize)
}

// seedInitialReceiveKeypoolLocked fills unused receive indices 1..N-1 and sets
// hdExternalNext past the highest allocated index (index 0 is the default spend key).
func (w *Disk) seedInitialReceiveKeypoolLocked() {
	w.hdKeypool = w.hdKeypool[:0]
	for i := uint32(1); i < defaultKeypoolSize; i++ {
		w.hdKeypool = append(w.hdKeypool, i)
	}
	w.hdExternalNext = defaultKeypoolSize
}

func (w *Disk) loadHDKeypoolCoreIndex(df diskFile) {
	w.hdKeypoolCoreIdx = nil
	if len(df.HDKeypoolCoreIdx) == 0 {
		return
	}
	w.hdKeypoolCoreIdx = make(map[uint32]int64, len(df.HDKeypoolCoreIdx))
	for k, v := range df.HDKeypoolCoreIdx {
		var recv uint32
		if _, err := fmt.Sscanf(k, "%d", &recv); err != nil {
			continue
		}
		w.hdKeypoolCoreIdx[recv] = v
	}
}

func (w *Disk) loadHD(df diskFile) error {
	if df.HDSeedHex == "" {
		if df.WalletFormat == "hd" || df.HDExternalNext > 0 || df.HDChangeNext > 0 || len(df.HDKeypool) > 0 || len(df.HDChangeKeypool) > 0 {
			w.hdExternalNext = df.HDExternalNext
			w.hdChangeNext = df.HDChangeNext
			w.hdKeypool = append(w.hdKeypool[:0], df.HDKeypool...)
			w.hdChangeKeypool = append(w.hdChangeKeypool[:0], df.HDChangeKeypool...)
			w.hdNodeTipEnabled = df.HDNodeTipEnabled
			w.loadHDKeypoolCoreIndex(df)
			w.syncReceiveNextFromPoolLocked()
			w.syncChangeNextFromPoolLocked()
		}
		return nil
	}
	seed, err := hex.DecodeString(stringsTrimHex(df.HDSeedHex))
	if err != nil || len(seed) < 16 || len(seed) > 64 {
		return fmt.Errorf("wallet.json: bad hd_seed_hex")
	}
	w.hdSeed = seed
	w.hdExternalNext = df.HDExternalNext
	w.hdChangeNext = df.HDChangeNext
	w.hdKeypool = append(w.hdKeypool[:0], df.HDKeypool...)
	w.hdChangeKeypool = append(w.hdChangeKeypool[:0], df.HDChangeKeypool...)
	w.hdNodeTipEnabled = df.HDNodeTipEnabled
	w.loadHDKeypoolCoreIndex(df)
	w.syncReceiveNextFromPoolLocked()
	w.syncChangeNextFromPoolLocked()
	// Ensure default spend key matches index 0.
	d0, err := w.deriveReceive(0)
	if err != nil {
		return err
	}
	w.priv = d0.Priv
	w.addr = d0.Addr
	return nil
}

func stringsTrimHex(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(s), "0x"))
	return s
}
