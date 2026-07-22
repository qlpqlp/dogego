// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"dogego/secp256k1"

	"dogego/chain"
)

func formatBIP32Path(path []uint32) string {
	parts := []string{"m"}
	for _, p := range path {
		if p >= hardenedFlag {
			parts = append(parts, strconv.FormatUint(uint64(p-hardenedFlag), 10)+"'")
		} else {
			parts = append(parts, strconv.FormatUint(uint64(p), 10))
		}
	}
	return strings.Join(parts, "/")
}

// AddressHDPath returns Core-shaped hdkeypath and ischange for a known HD address.
func (w *Disk) AddressHDPath(addr string) (hdpath string, ischange bool, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() || w.priv == nil {
		return "", false, false
	}
	addr = strings.TrimSpace(addr)
	for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
		d, err := w.deriveReceive(i)
		if err != nil || d.Addr != addr {
			continue
		}
		return formatBIP32Path(bip44ReceivePath(0, i)), false, true
	}
	for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
		d, err := w.deriveChange(i)
		if err != nil || d.Addr != addr {
			continue
		}
		return formatBIP32Path(bip44ChangePath(0, i)), true, true
	}
	if w.hdNodeTipEnabled {
		d, err := w.deriveNodeTip(0)
		if err == nil && d.Addr == addr {
			return formatBIP32Path(bip44NodeTipPath(0, 0)), false, true
		}
	}
	return "", false, false
}

// IsNodeTipAddress reports whether addr is the dedicated node-tip HD key.
func (w *Disk) IsNodeTipAddress(addr string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() || !w.hdNodeTipEnabled || w.priv == nil {
		return false
	}
	addr = strings.TrimSpace(addr)
	d, err := w.deriveNodeTip(0)
	return err == nil && d.Addr == addr
}

// PeekReceiveAddress returns the next reserved keypool receive address without consuming it (Core getaccountaddress).
func (w *Disk) PeekReceiveAddress() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return w.addr
	}
	if w.priv == nil {
		return w.addr
	}
	if len(w.hdKeypool) > 0 {
		if d, err := w.deriveReceive(w.hdKeypool[0]); err == nil {
			return d.Addr
		}
	}
	return w.addr
}

// ConsumeReceiveKeypoolAddress removes addr from the unused receive keypool when present
// (Core-style: a payment to a reserved keypool address marks that key used).
func (w *Disk) ConsumeReceiveKeypoolAddress(addr string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	changed, err := w.consumeReceiveKeypoolAddressLocked(addr)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return w.saveLocked()
}

func (w *Disk) consumeKeypoolFromScannedLocked(rows []ScannedTx) error {
	for _, r := range rows {
		if r.Category != "receive" || strings.TrimSpace(r.Address) == "" {
			continue
		}
		if _, err := w.consumeReceiveKeypoolAddressLocked(r.Address); err != nil {
			return err
		}
	}
	return nil
}

func (w *Disk) consumeReceiveKeypoolAddressLocked(addr string) (bool, error) {
	addr = strings.TrimSpace(addr)
	if !w.hdEnabled() || addr == "" {
		return false, nil
	}
	idx, ok := w.receiveIndexForAddressLocked(addr)
	if !ok {
		return false, nil
	}
	found := -1
	for i, k := range w.hdKeypool {
		if k == idx {
			found = i
			break
		}
	}
	if found < 0 {
		return false, nil
	}
	w.hdKeypool = append(w.hdKeypool[:found], w.hdKeypool[found+1:]...)
	if err := w.ensureKeypoolLocked(); err != nil {
		return false, err
	}
	return true, nil
}

// DefaultAddress returns the primary P2PKH address (BIP44 index 0 when HD is enabled).
func (w *Disk) DefaultAddress() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.addr
}

// NewReceiveAddress returns a fresh BIP44 receive address (Core getnewaddress).
func (w *Disk) NewReceiveAddress() (string, error) {
	if err := w.requireUnlocked(); err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return w.addr, nil
	}
	if len(w.hdKeypool) == 0 {
		if err := w.topUpKeypoolLocked(defaultKeypoolSize); err != nil {
			return "", err
		}
		if err := w.saveLocked(); err != nil {
			return "", err
		}
	}
	idx := w.hdKeypool[0]
	w.hdKeypool = w.hdKeypool[1:]
	d, err := w.deriveReceive(idx)
	if err != nil {
		return "", err
	}
	if err := w.ensureKeypoolLocked(); err != nil {
		return "", err
	}
	return d.Addr, w.saveLocked()
}

// DeriveReceiveMaterial returns the compressed pubkey and private key for BIP44 receive index.
// Used by wallet.dat pool-replay tests and migration tooling.
func (w *Disk) DeriveReceiveMaterial(index uint32) (pubCompressed []byte, priv *secp256k1.PrivateKey, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() || w.priv == nil {
		return nil, nil, fmt.Errorf("hd wallet required")
	}
	d, err := w.deriveReceive(index)
	if err != nil {
		return nil, nil, err
	}
	return d.Priv.PubKey().SerializeCompressed(), d.Priv, nil
}

// PeekChangeAddress returns the next reserved change keypool address without consuming it.
func (w *Disk) PeekChangeAddress() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return w.addr
	}
	if w.priv == nil {
		return w.addr
	}
	if len(w.hdChangeKeypool) == 0 {
		_ = w.topUpChangeKeypoolLocked(defaultKeypoolSize)
	}
	if len(w.hdChangeKeypool) > 0 {
		if d, err := w.deriveChange(w.hdChangeKeypool[0]); err == nil {
			return d.Addr
		}
	}
	return w.addr
}

// CommitChangeAddress consumes the keypool slot when addr is the peeked change address (after fund adds change).
func (w *Disk) CommitChangeAddress(addr string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() || w.priv == nil {
		return nil
	}
	addr = strings.TrimSpace(addr)
	if len(w.hdChangeKeypool) == 0 {
		return nil
	}
	d, err := w.deriveChange(w.hdChangeKeypool[0])
	if err != nil || d.Addr != addr {
		return nil
	}
	w.hdChangeKeypool = w.hdChangeKeypool[1:]
	if err := w.ensureChangeKeypoolLocked(); err != nil {
		return err
	}
	return w.saveLocked()
}

// NewChangeAddress returns a fresh BIP44 change address (internal chain).
func (w *Disk) NewChangeAddress() (string, error) {
	if err := w.requireUnlocked(); err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return w.addr, nil
	}
	if len(w.hdChangeKeypool) == 0 {
		if err := w.topUpChangeKeypoolLocked(defaultKeypoolSize); err != nil {
			return "", err
		}
	}
	idx := w.hdChangeKeypool[0]
	w.hdChangeKeypool = w.hdChangeKeypool[1:]
	d, err := w.deriveChange(idx)
	if err != nil {
		return "", err
	}
	if err := w.ensureChangeKeypoolLocked(); err != nil {
		return "", err
	}
	return d.Addr, w.saveLocked()
}

// KeypoolRefill fills receive and change keypools up to newSize (Core TopUpKeyPool /
// keypoolrefill). When the pool is already at or above the target, it is a no-op.
func (w *Disk) KeypoolRefill(newSize int) error {
	if err := w.requireUnlocked(); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return nil
	}
	if newSize <= 0 {
		newSize = defaultKeypoolSize
	}
	w.syncReceiveNextFromPoolLocked()
	w.syncChangeNextFromPoolLocked()
	if need := newSize - len(w.hdKeypool); need > 0 {
		if err := w.topUpKeypoolLocked(need); err != nil {
			return err
		}
	}
	if need := newSize - len(w.hdChangeKeypool); need > 0 {
		if err := w.topUpChangeKeypoolLocked(need); err != nil {
			return err
		}
	}
	return w.saveLocked()
}

func (w *Disk) syncReceiveNextFromPoolLocked() {
	if w.hdExternalNext == 0 {
		w.hdExternalNext = 1
	}
	for _, idx := range w.hdKeypool {
		if idx >= w.hdExternalNext {
			w.hdExternalNext = idx + 1
		}
	}
}

func (w *Disk) syncChangeNextFromPoolLocked() {
	for _, idx := range w.hdChangeKeypool {
		if idx >= w.hdChangeNext {
			w.hdChangeNext = idx + 1
		}
	}
}

func (w *Disk) topUpKeypoolLocked(count int) error {
	w.syncReceiveNextFromPoolLocked()
	if w.hdExternalNext+uint32(count) > maxHDExternalIndex {
		return fmt.Errorf("hd external index cap exceeded")
	}
	for i := 0; i < count; i++ {
		idx := w.hdExternalNext
		w.hdExternalNext++
		w.hdKeypool = append(w.hdKeypool, idx)
	}
	return nil
}

// ensureKeypoolLocked tops up the receive keypool when it drops below Core's half-target watermark.
func (w *Disk) ensureKeypoolLocked() error {
	if !w.hdReceiveKeypoolTracked() {
		return nil
	}
	if len(w.hdKeypool) >= keypoolRefillThreshold {
		return nil
	}
	need := defaultKeypoolSize - len(w.hdKeypool)
	if need <= 0 {
		return nil
	}
	return w.topUpKeypoolLocked(need)
}

func (w *Disk) topUpChangeKeypoolLocked(count int) error {
	w.syncChangeNextFromPoolLocked()
	if w.hdChangeNext+uint32(count) > maxHDExternalIndex {
		return fmt.Errorf("hd change index cap exceeded")
	}
	for i := 0; i < count; i++ {
		idx := w.hdChangeNext
		w.hdChangeNext++
		w.hdChangeKeypool = append(w.hdChangeKeypool, idx)
	}
	return nil
}

func (w *Disk) ensureChangeKeypoolLocked() error {
	if !w.hdChangeKeypoolTracked() {
		return nil
	}
	if len(w.hdChangeKeypool) >= keypoolRefillThreshold {
		return nil
	}
	need := defaultKeypoolSize - len(w.hdChangeKeypool)
	if need <= 0 {
		return nil
	}
	return w.topUpChangeKeypoolLocked(need)
}

func (w *Disk) hdReceiveKeypoolTracked() bool {
	return w.hdExternalNext > 0 || len(w.hdKeypool) > 0
}

func (w *Disk) hdChangeKeypoolTracked() bool {
	return w.hdChangeNext > 0 || len(w.hdChangeKeypool) > 0
}

// EnsureKeypoolOnLoad refills depleted HD receive/change keypools after startup (Core keypool top-up).
func (w *Disk) EnsureKeypoolOnLoad() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	beforeRecv := len(w.hdKeypool)
	beforeChg := len(w.hdChangeKeypool)
	if err := w.ensureKeypoolLocked(); err != nil {
		return err
	}
	if err := w.ensureChangeKeypoolLocked(); err != nil {
		return err
	}
	if len(w.hdKeypool) > beforeRecv || len(w.hdChangeKeypool) > beforeChg {
		return w.saveLocked()
	}
	return nil
}

// SpendScripts returns P2PKH scripts for all issued receive + change indices (UTXO scan / fund).
func (w *Disk) SpendScripts() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		if s := w.p2pkhScriptLocked(); len(s) > 0 {
			return [][]byte{append([]byte(nil), s...)}
		}
		return nil
	}
	seen := make(map[string]struct{})
	var out [][]byte
	add := func(script []byte) {
		if len(script) == 0 {
			return
		}
		k := string(script)
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, append([]byte(nil), script...))
	}
	for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
		if d, err := w.deriveReceive(i); err == nil {
			add(d.Script)
		}
	}
	for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
		if d, err := w.deriveChange(i); err == nil {
			add(d.Script)
		}
	}
	if w.hdNodeTipEnabled {
		if d, err := w.deriveNodeTip(0); err == nil {
			add(d.Script)
		}
	}
	return out
}

func (w *Disk) privKeyFromExtraImportsLocked(addr string) *secp256k1.PrivateKey {
	for _, hexKey := range w.extraPrivHex {
		secret, err := hex.DecodeString(strings.TrimSpace(hexKey))
		if err != nil || len(secret) != 32 {
			continue
		}
		priv, _ := secp256k1.PrivKeyFromBytes(secret)
		if p2pkh(w.addrVer, priv.PubKey()) == addr {
			return priv
		}
	}
	return nil
}

// PrivKeyForAddress returns the private key for a known HD or legacy address.
func (w *Disk) PrivKeyForAddress(addr string) (*secp256k1.PrivateKey, error) {
	if err := w.requireUnlocked(); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	addr = strings.TrimSpace(addr)
	if addr == w.addr && w.priv != nil {
		return w.priv, nil
	}
	if !w.hdEnabled() {
		if priv := w.privKeyFromExtraImportsLocked(addr); priv != nil {
			return priv, nil
		}
		return nil, fmt.Errorf("address not in wallet")
	}
	if priv := w.privKeyFromExtraImportsLocked(addr); priv != nil {
		return priv, nil
	}
	for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
		d, err := w.deriveReceive(i)
		if err != nil {
			continue
		}
		if d.Addr == addr {
			return d.Priv, nil
		}
	}
	for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
		d, err := w.deriveChange(i)
		if err != nil {
			continue
		}
		if d.Addr == addr {
			return d.Priv, nil
		}
	}
	if w.hdNodeTipEnabled {
		if d, err := w.deriveNodeTip(0); err == nil && d.Addr == addr {
			return d.Priv, nil
		}
	}
	return nil, fmt.Errorf("address not in wallet")
}

func (w *Disk) hdOwnsScriptLocked(script []byte) bool {
	for _, s := range w.spendScriptsLocked() {
		if bytes.Equal(script, s) {
			return true
		}
	}
	return false
}

func (w *Disk) spendScriptsLocked() [][]byte {
	if !w.hdEnabled() {
		if s := w.p2pkhScriptLocked(); len(s) > 0 {
			return [][]byte{s}
		}
		return nil
	}
	var out [][]byte
	for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
		if d, err := w.deriveReceive(i); err == nil {
			out = append(out, d.Script)
		}
	}
	for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
		if d, err := w.deriveChange(i); err == nil {
			out = append(out, d.Script)
		}
	}
	if w.hdNodeTipEnabled {
		if d, err := w.deriveNodeTip(0); err == nil {
			out = append(out, d.Script)
		}
	}
	return out
}

// ContainsAddress reports whether addr is a known spend (HD receive/change or default).
func (w *Disk) ContainsAddress(addr string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	if addr == w.addr {
		return true
	}
	if _, ok := w.labels[addr]; ok {
		return true
	}
	if !w.hdEnabled() {
		return false
	}
	for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
		if d, err := w.deriveReceive(i); err == nil && d.Addr == addr {
			return true
		}
	}
	for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
		if d, err := w.deriveChange(i); err == nil && d.Addr == addr {
			return true
		}
	}
	if w.hdNodeTipEnabled {
		if d, err := w.deriveNodeTip(0); err == nil && d.Addr == addr {
			return true
		}
	}
	return false
}

// KeypoolSize returns unused BIP44 receive indices reserved for getnewaddress.
func (w *Disk) KeypoolSize() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.hdKeypool)
}

// ChangeKeypoolSize returns unused BIP44 change indices reserved for getrawchangeaddress.
func (w *Disk) ChangeKeypoolSize() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.hdChangeKeypool)
}

// KnownAddresses returns all tracked addresses (HD issued, watch-only, labeled).
func (w *Disk) KnownAddresses(pkhVer, shVer byte) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	seen := make(map[string]struct{})
	var out []string
	add := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	add(w.addr)
	if w.hdEnabled() && w.priv != nil {
		for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
			if d, err := w.deriveReceive(i); err == nil {
				add(d.Addr)
			}
		}
		for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
			if d, err := w.deriveChange(i); err == nil {
				add(d.Addr)
			}
		}
		if w.hdNodeTipEnabled {
			if d, err := w.deriveNodeTip(0); err == nil {
				add(d.Addr)
			}
		}
	}
	for addr := range w.labels {
		add(addr)
	}
	for _, pk := range w.watchScripts {
		add(chain.ScriptPubKeyAddress(pk, pkhVer, shVer))
	}
	return out
}

// HDEnabled reports BIP44 HD derivation (wallet.json hd_seed_hex).
func (w *Disk) HDEnabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hdEnabled()
}

// HDSeedIDHex returns Core-shaped hdseedid (SHA256 of the BIP32 seed) when keys are loaded.
func (w *Disk) HDSeedIDHex() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.hdSeed) < 16 {
		return ""
	}
	h := sha256.Sum256(w.hdSeed)
	return hex.EncodeToString(h[:])
}

// MasterKeyFingerprint returns the BIP32 master key fingerprint when HD is enabled.
func (w *Disk) MasterKeyFingerprint() (uint32, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return 0, false
	}
	fp, err := MasterKeyFingerprint(w.hdSeed)
	if err != nil {
		return 0, false
	}
	return fp, true
}

// CompressedPubKeyForAddress returns the compressed secp256k1 pubkey for a known wallet address.
func (w *Disk) CompressedPubKeyForAddress(addr string) ([]byte, bool) {
	priv, err := w.PrivKeyForAddress(addr)
	if err != nil || priv == nil {
		return nil, false
	}
	return priv.PubKey().SerializeCompressed(), true
}
