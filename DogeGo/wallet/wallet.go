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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
)

// DefaultPayTxFeeDOGE is the wallet send fee rate (settxfee, DOGE/kB) for newly created wallets.
const DefaultPayTxFeeDOGE = 0.01

// Disk is the built-in wallet (wallet.json): legacy single-key or BIP44 HD (m/44'/3'/0'/…).
type Disk struct {
	mu           sync.Mutex
	path         string
	addrVer      byte
	priv         *secp256k1.PrivateKey
	addr         string
	hdSeed       []byte
	hdExternalNext   uint32
	hdChangeNext     uint32
	hdKeypool        []uint32
	hdKeypoolCoreIdx map[uint32]int64 // BIP44 receive index -> Core BDB pool index
	hdChangeKeypool  []uint32
	hdNodeTipEnabled bool
	watchScripts [][]byte
	watchRedeems map[string]string // scriptPubKey hex -> redeemScript hex
	payTxFee     float64 // DOGE per kB; 0 = use node relay/min fee
	locked       []LockedOutpoint
	labels       map[string]string // address -> label (setlabel)
	replacements map[string]string // old display txid -> new (bumpfee)
	abandoned    []AbandonedTx
	prunedImports  []PrunedImport // importprunedfunds credits
	extraPrivHex   []string       // additional privkey_hex for multisig cosigners
	encrypted      bool
	encSalt        []byte
	encNonce       []byte
	encCipher      []byte
	sessionKey     []byte // scrypt key while unlocked (encrypted wallet)
	unlockUntil    int64  // Unix seconds; 0 = no timeout
	scannedTx       []ScannedTx
	avoidReuse              bool
	pqCommitmentsEnabled    bool
	pqCarrierEnabled        bool
	pqCommitSeed            []byte
	pqTag                   string
	pqSendCounter           uint64
	pqKeys                  map[string]pqKeyPair
	pqDiskMigration         *pqDiskMigration
	pkhVer          byte
	shVer           byte
	usedRecvScripts map[string]struct{} // hex scriptPubKey
	importedDesc    []ImportedDescriptor
	rotatePending  *rotationPending
}

type diskFile struct {
	PrivKeyHex      string            `json:"privkey_hex,omitempty"`
	WatchScripts    []string          `json:"watch_scripts,omitempty"` // hex scriptPubKey (importaddress)
	WatchRedeems    map[string]string `json:"watch_redeems,omitempty"`   // scriptPubKey hex -> redeemScript hex (P2SH multisig)
	PayTxFee        float64           `json:"paytxfee,omitempty"`      // DOGE per kB (settxfee)
	LockedOutpoints []lockedOutJSON   `json:"locked_outpoints,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	TxReplacements  map[string]string `json:"tx_replacements,omitempty"` // old display txid -> new
	AbandonedTxs    []abandonedTxJSON `json:"abandoned_txs,omitempty"`
	PrunedImports    []prunedImportJSON `json:"pruned_imports,omitempty"`
	ExtraPrivkeysHex []string           `json:"extra_privkeys_hex,omitempty"`
	HDSeedHex        string             `json:"hd_seed_hex,omitempty"`
	HDExternalNext   uint32             `json:"hd_external_next,omitempty"`
	HDChangeNext     uint32             `json:"hd_change_next,omitempty"`
	HDKeypool        []uint32           `json:"hd_keypool,omitempty"`
	HDKeypoolCoreIdx map[string]int64   `json:"hd_keypool_core_index,omitempty"` // receive index (decimal string) -> Core pool index
	HDChangeKeypool  []uint32           `json:"hd_change_keypool,omitempty"`
	HDNodeTipEnabled bool               `json:"hd_node_tip_enabled,omitempty"`
	WalletFormat     string             `json:"wallet_format,omitempty"` // "hd" when BIP44 enabled
	Address          string             `json:"address,omitempty"`       // primary address when encrypted
	Encrypted        bool               `json:"encrypted,omitempty"`
	EncryptSaltHex   string             `json:"encrypt_salt_hex,omitempty"`
	EncryptNonceHex  string             `json:"encrypt_nonce_hex,omitempty"`
	SecretsCipherHex string             `json:"secrets_cipher_hex,omitempty"`
	ScannedTxs       []scannedTxJSON    `json:"scanned_txs,omitempty"`
	AvoidReuse              bool               `json:"avoid_reuse,omitempty"`
	PqCommitmentsEnabled    *bool              `json:"pq_commitments_enabled,omitempty"`
	PqCarrierEnabled        bool               `json:"pq_carrier_enabled,omitempty"`
	PqTag                   string             `json:"pq_tag,omitempty"`
	PqCommitSeedHex         string             `json:"pq_commit_seed_hex,omitempty"`
	PqSendCounter           uint64             `json:"pq_send_counter,omitempty"`
	PqKeys                  map[string]pqKeyPairJSON `json:"pq_keys,omitempty"`
	ImportedDescriptors     []importedDescJSON `json:"imported_descriptors,omitempty"`
}

// LoadOrCreate opens path or creates a new random key (P2PKH for addrVer).
func LoadOrCreate(path string, addrVer byte) (*Disk, error) {
	w := &Disk{path: path, addrVer: addrVer}
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := w.initHDLocked(); err != nil {
			return nil, err
		}
		w.payTxFee = DefaultPayTxFeeDOGE
		w.pqCommitmentsEnabled = true
		if err := w.save(); err != nil {
			return nil, err
		}
		return w, w.ensureKeypoolAfterLoad()
	}
	var df diskFile
	if err := json.Unmarshal(b, &df); err != nil {
		return nil, fmt.Errorf("wallet.json: %w", err)
	}
	if err := w.loadWatchScripts(df.WatchScripts); err != nil {
		return nil, err
	}
	w.loadWatchRedeems(df.WatchRedeems)
	w.payTxFee = df.PayTxFee
	w.loadLockedOutpoints(df.LockedOutpoints)
	w.loadLabels(df.Labels)
	w.avoidReuse = df.AvoidReuse
	w.loadImportedDescriptors(df.ImportedDescriptors)
	w.loadReplacements(df.TxReplacements)
	w.loadAbandonedTxs(df.AbandonedTxs)
	w.loadPrunedImports(df.PrunedImports)
	if df.Encrypted {
		if err := w.loadEncrypted(df); err != nil {
			return nil, err
		}
		if err := w.loadHD(df); err != nil {
			return nil, err
		}
		w.loadScannedTx(df.ScannedTxs)
		_ = w.ensureTxDBMigrated()
		return w, w.ensureKeypoolAfterLoad()
	}
	w.loadPQFromDisk(df)
	raw, err := hex.DecodeString(df.PrivKeyHex)
	if err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("wallet.json: bad privkey_hex")
	}
	priv, _ := secp256k1.PrivKeyFromBytes(raw)
	w.priv = priv
	w.addr = p2pkh(addrVer, priv.PubKey())
	w.loadExtraPrivkeys(df.ExtraPrivkeysHex)
	if err := w.loadHD(df); err != nil {
		return nil, err
	}
	w.loadScannedTx(df.ScannedTxs)
	_ = w.EnsurePQReady()
	_ = w.ensureTxDBMigrated()
	return w, w.ensureKeypoolAfterLoad()
}

func (w *Disk) ensureKeypoolAfterLoad() error {
	return w.EnsureKeypoolOnLoad()
}

func (w *Disk) loadPQFromDisk(df diskFile) {
	w.pqCommitmentsEnabled = true
	if df.PqCommitmentsEnabled != nil {
		w.pqCommitmentsEnabled = *df.PqCommitmentsEnabled
	}
	if df.PqCommitSeedHex != "" {
		if seed, err := hex.DecodeString(df.PqCommitSeedHex); err == nil && len(seed) == 32 {
			w.pqCommitSeed = append(w.pqCommitSeed[:0], seed...)
		}
	}
	w.pqTag = df.PqTag
	w.pqSendCounter = df.PqSendCounter
	w.pqCarrierEnabled = df.PqCarrierEnabled
	w.pqKeys = loadPQKeys(df.PqKeys)
}

func (w *Disk) loadExtraPrivkeys(rows []string) {
	w.extraPrivHex = w.extraPrivHex[:0]
	seen := make(map[string]struct{})
	if w.priv != nil {
		seen[hex.EncodeToString(w.priv.Serialize())] = struct{}{}
	}
	for _, h := range rows {
		h = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(h), "0x"))
		if len(h) != 64 {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		if _, err := hex.DecodeString(h); err != nil {
			continue
		}
		seen[h] = struct{}{}
		w.extraPrivHex = append(w.extraPrivHex, h)
	}
}

func (w *Disk) loadReplacements(m map[string]string) {
	w.replacements = nil
	if len(m) == 0 {
		return
	}
	w.replacements = make(map[string]string, len(m))
	for old, new := range m {
		old = normalizeWalletTxID(old)
		new = normalizeWalletTxID(new)
		if len(old) == 64 && len(new) == 64 && old != new {
			w.replacements[old] = new
		}
	}
}

func (w *Disk) loadWatchScripts(hexScripts []string) error {
	w.watchScripts = w.watchScripts[:0]
	for _, h := range hexScripts {
		h = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(h), "0x"))
		if h == "" {
			continue
		}
		b, err := hex.DecodeString(h)
		if err != nil || len(b) == 0 {
			return fmt.Errorf("wallet.json: bad watch_scripts entry")
		}
		w.watchScripts = append(w.watchScripts, b)
	}
	return nil
}

func (w *Disk) save() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.saveLocked()
}

func (w *Disk) saveLocked() error {
	if w.encrypted {
		if w.isUnlockedLocked() && len(w.sessionKey) == scryptKeyLen {
			sec := w.secretsLocked()
			nonce, cipher, err := sealSecrets(w.sessionKey, sec)
			if err != nil {
				return err
			}
			w.encNonce = nonce
			w.encCipher = cipher
		} else if w.priv != nil {
			return fmt.Errorf("encrypted wallet session missing")
		}
	} else if w.priv == nil {
		return fmt.Errorf("no key")
	}
	df := diskFile{PayTxFee: w.payTxFee}
	if !w.encrypted {
		df.PrivKeyHex = hex.EncodeToString(w.priv.Serialize())
	}
	for _, pk := range w.watchScripts {
		df.WatchScripts = append(df.WatchScripts, hex.EncodeToString(pk))
	}
	if len(w.watchRedeems) > 0 {
		df.WatchRedeems = make(map[string]string, len(w.watchRedeems))
		for k, v := range w.watchRedeems {
			df.WatchRedeems[k] = v
		}
	}
	for _, o := range w.locked {
		df.LockedOutpoints = append(df.LockedOutpoints, lockedOutJSON{TxID: o.TxID, Vout: o.Vout})
	}
	if len(w.labels) > 0 {
		df.Labels = make(map[string]string, len(w.labels))
		for addr, lbl := range w.labels {
			if strings.TrimSpace(lbl) != "" {
				df.Labels[addr] = lbl
			}
		}
		if len(df.Labels) == 0 {
			df.Labels = nil
		}
	}
	if len(w.replacements) > 0 {
		df.TxReplacements = make(map[string]string, len(w.replacements))
		for old, new := range w.replacements {
			df.TxReplacements[old] = new
		}
	}
	if len(w.abandoned) > 0 {
		df.AbandonedTxs = make([]abandonedTxJSON, len(w.abandoned))
		for i, a := range w.abandoned {
			df.AbandonedTxs[i] = abandonedTxJSON{
				TxID: a.TxID, Category: a.Category, AmountKoinu: a.AmountKoinu,
				Address: a.Address, Time: a.Time,
			}
		}
	}
	if len(w.prunedImports) > 0 {
		df.PrunedImports = make([]prunedImportJSON, len(w.prunedImports))
		for i, p := range w.prunedImports {
			df.PrunedImports[i] = prunedImportJSON{
				TxID: p.TxID, BlockHeight: p.BlockHeight, BlockHash: p.BlockHash,
				Vout: p.Vout, AmountKoinu: p.AmountKoinu, ScriptHex: hex.EncodeToString(p.Script),
			}
		}
	}
	if len(w.extraPrivHex) > 0 {
		df.ExtraPrivkeysHex = append([]string(nil), w.extraPrivHex...)
	}
	if w.hdEnabled() {
		if !w.encrypted {
			df.HDSeedHex = hex.EncodeToString(w.hdSeed)
		}
		df.HDExternalNext = w.hdExternalNext
		df.HDChangeNext = w.hdChangeNext
		df.HDKeypool = append([]uint32(nil), w.hdKeypool...)
		if len(w.hdKeypoolCoreIdx) > 0 {
			df.HDKeypoolCoreIdx = make(map[string]int64, len(w.hdKeypoolCoreIdx))
			for recv, core := range w.hdKeypoolCoreIdx {
				df.HDKeypoolCoreIdx[fmt.Sprintf("%d", recv)] = core
			}
		}
		df.HDChangeKeypool = append([]uint32(nil), w.hdChangeKeypool...)
		df.HDNodeTipEnabled = w.hdNodeTipEnabled
		df.WalletFormat = "hd"
	}
	w.encryptedFields(&df)
	df.AvoidReuse = w.avoidReuse
	if !w.encrypted {
		pqOn := w.pqCommitmentsEnabled
		df.PqCommitmentsEnabled = &pqOn
		df.PqCarrierEnabled = w.pqCarrierEnabled
		if len(w.pqCommitSeed) == 32 {
			df.PqCommitSeedHex = hex.EncodeToString(w.pqCommitSeed)
		}
		df.PqTag = w.pqTag
		df.PqSendCounter = w.pqSendCounter
		df.PqKeys = savePQKeys(w.pqKeys)
	}
	if rows := w.importedDescToDisk(); len(rows) > 0 {
		df.ImportedDescriptors = rows
	}
	if len(w.scannedTx) > 0 {
		df.ScannedTxs = w.scannedTxToDisk()
	}
	b, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
	tmp := w.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, w.path)
}

func p2pkh(addrVer byte, pub *secp256k1.PublicKey) string {
	comp := pub.SerializeCompressed()
	h := sha256.Sum256(comp)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	h160 := r.Sum(nil)
	return chain.Base58CheckEncode(addrVer, h160)
}

// Address returns the P2PKH Dogecoin address.
func (w *Disk) Address() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.addr
}

// P2PKHScript returns the standard pay-to-pubkey-hash script for this wallet address.
func (w *Disk) P2PKHScript() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, h160, err := chain.Base58CheckDecode(w.addr)
	if err != nil {
		return nil
	}
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	return append(pk, 0x88, 0xac)
}

// WIFExport returns the compressed WIF for this wallet (privKeyWIFVersion from chain.Params).
func (w *Disk) WIFExport(privKeyWIFVersion byte) (string, error) {
	if err := w.requireUnlocked(); err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.priv == nil {
		return "", fmt.Errorf("no key")
	}
	return chain.EncodeWIF(w.priv.Serialize(), privKeyWIFVersion, true)
}

// Path returns the backing file path.
func (w *Disk) Path() string {
	return w.path
}

// PayTxFee returns the wallet fee rate in DOGE per kB (0 = unset).
func (w *Disk) PayTxFee() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.payTxFee
}

// SetPayTxFee stores the wallet fee rate in DOGE per kB (settxfee).
func (w *Disk) SetPayTxFee(feeDOGEPerKB float64) error {
	if feeDOGEPerKB < 0 {
		return fmt.Errorf("negative fee")
	}
	w.mu.Lock()
	w.payTxFee = feeDOGEPerKB
	w.mu.Unlock()
	return w.saveLocked()
}

func (w *Disk) loadWatchRedeems(m map[string]string) {
	w.watchRedeems = nil
	if len(m) == 0 {
		return
	}
	w.watchRedeems = make(map[string]string, len(m))
	for k, v := range m {
		k = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(k), "0x"))
		v = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "0x"))
		if k != "" && v != "" {
			w.watchRedeems[k] = v
		}
	}
}

// SetWatchRedeem associates a P2SH scriptPubKey with its redeem script (multisig / importmulti).
func (w *Disk) SetWatchRedeem(pkScript, redeem []byte) error {
	if len(pkScript) == 0 || len(redeem) == 0 {
		return fmt.Errorf("empty script")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.watchRedeems == nil {
		w.watchRedeems = make(map[string]string)
	}
	w.watchRedeems[hex.EncodeToString(pkScript)] = hex.EncodeToString(redeem)
	return w.saveLocked()
}

// WatchRedeemScript returns the stored redeem script for a watch P2SH scriptPubKey, or nil.
func (w *Disk) WatchRedeemScript(pkScript []byte) []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(pkScript) == 0 || len(w.watchRedeems) == 0 {
		return nil
	}
	h, ok := w.watchRedeems[hex.EncodeToString(pkScript)]
	if !ok {
		return nil
	}
	b, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(h), "0x"))
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

// AddWatchScript records a watch-only scriptPubKey (importaddress).
func (w *Disk) AddWatchScript(pkScript []byte) error {
	if len(pkScript) == 0 {
		return fmt.Errorf("empty script")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, existing := range w.watchScripts {
		if bytes.Equal(existing, pkScript) {
			return nil
		}
	}
	dup := append([]byte(nil), pkScript...)
	w.watchScripts = append(w.watchScripts, dup)
	return w.saveLocked()
}

// WatchScripts returns a copy of imported watch-only scriptPubKeys.
func (w *Disk) WatchScripts() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([][]byte, len(w.watchScripts))
	for i, pk := range w.watchScripts {
		out[i] = append([]byte(nil), pk...)
	}
	return out
}

// IsWatchAddress reports whether addr is an imported watch-only address (not the spend key).
func (w *Disk) IsWatchAddress(addr string, pubVer, scriptVer byte) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	addr = strings.TrimSpace(addr)
	if addr == "" || addr == w.addr {
		return false
	}
	for _, pk := range w.watchScripts {
		if chain.ScriptPubKeyAddress(pk, pubVer, scriptVer) == addr {
			return true
		}
	}
	return false
}

// HasWatchScript reports whether pkScript is tracked watch-only.
func (w *Disk) HasWatchScript(pkScript []byte) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, existing := range w.watchScripts {
		if bytes.Equal(existing, pkScript) {
			return true
		}
	}
	return false
}

// ImportSpendPrivKey replaces the wallet spend key (importwallet / dump round-trip).
func (w *Disk) ImportSpendPrivKey(wif string, wifVer, addrVer byte) error {
	if err := w.requireUnlocked(); err != nil {
		return err
	}
	secret, _, err := chain.DecodeWIF(strings.TrimSpace(wif), wifVer)
	if err != nil {
		return fmt.Errorf("invalid wif: %w", err)
	}
	if len(secret) != 32 {
		return fmt.Errorf("invalid wif length")
	}
	priv, _ := secp256k1.PrivKeyFromBytes(secret)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hdSeed = nil
	w.hdKeypool = nil
	w.hdExternalNext = 0
	w.hdChangeNext = 0
	w.priv = priv
	w.addr = p2pkh(addrVer, priv.PubKey())
	return w.saveLocked()
}

// ImportPrivKey sets the spend key when the WIF matches the current address; otherwise stores a cosigner key
// in extra_privkeys_hex (for P2SH multisig signing via signrawtransaction / sendtoaddress).
func (w *Disk) ImportPrivKey(wif string, wifVer, addrVer byte) error {
	if err := w.requireUnlocked(); err != nil {
		return err
	}
	secret, _, err := chain.DecodeWIF(strings.TrimSpace(wif), wifVer)
	if err != nil {
		return fmt.Errorf("invalid wif: %w", err)
	}
	if len(secret) != 32 {
		return fmt.Errorf("invalid wif length")
	}
	priv, _ := secp256k1.PrivKeyFromBytes(secret)
	newAddr := p2pkh(addrVer, priv.PubKey())
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.priv != nil && newAddr == w.addr {
		w.priv = priv
		return w.saveLocked()
	}
	hexKey := hex.EncodeToString(secret)
	for _, existing := range w.extraPrivHex {
		if existing == hexKey {
			return nil
		}
	}
	if w.priv != nil {
		mainHex := hex.EncodeToString(w.priv.Serialize())
		if mainHex == hexKey {
			return nil
		}
	}
	w.extraPrivHex = append(w.extraPrivHex, hexKey)
	return w.saveLocked()
}

// AllWIFs returns spend WIFs for all known HD / legacy keys plus cosigner imports (deduped).
func (w *Disk) AllWIFs(wifVer byte) ([]string, error) {
	if err := w.requireUnlocked(); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.priv == nil {
		return nil, fmt.Errorf("no key")
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(priv *secp256k1.PrivateKey) error {
		if priv == nil {
			return nil
		}
		wif, err := chain.EncodeWIF(priv.Serialize(), wifVer, true)
		if err != nil {
			return err
		}
		if _, ok := seen[wif]; ok {
			return nil
		}
		seen[wif] = struct{}{}
		out = append(out, wif)
		return nil
	}
	if w.hdEnabled() {
		inPool := make(map[uint32]struct{}, len(w.hdKeypool))
		for _, idx := range w.hdKeypool {
			inPool[idx] = struct{}{}
		}
		for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
			if i > 0 {
				if _, reserved := inPool[i]; reserved {
					continue
				}
			}
			if d, err := w.deriveReceive(i); err == nil {
				_ = add(d.Priv)
			}
		}
		inChgPool := make(map[uint32]struct{}, len(w.hdChangeKeypool))
		for _, idx := range w.hdChangeKeypool {
			inChgPool[idx] = struct{}{}
		}
		for i := uint32(0); i < w.hdChangeNext; i++ {
			if _, reserved := inChgPool[i]; reserved {
				continue
			}
			if d, err := w.deriveChange(i); err == nil {
				_ = add(d.Priv)
			}
		}
	} else {
		if err := add(w.priv); err != nil {
			return nil, err
		}
	}
	for _, h := range w.extraPrivHex {
		raw, err := hex.DecodeString(h)
		if err != nil || len(raw) != 32 {
			continue
		}
		wif, err := chain.EncodeWIF(raw, wifVer, true)
		if err != nil {
			continue
		}
		if _, ok := seen[wif]; ok {
			continue
		}
		seen[wif] = struct{}{}
		out = append(out, wif)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no key")
	}
	return out, nil
}
