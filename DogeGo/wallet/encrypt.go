// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"dogego/secp256k1"
)

// ErrWalletLocked is returned when spend keys are required but the wallet is encrypted and locked.
var ErrWalletLocked = errors.New("wallet locked")

// IsEncrypted reports whether wallet.json stores keys encrypted at rest.
func (w *Disk) IsEncrypted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encrypted
}

// IsUnlocked reports whether spend keys are loaded (always true when not encrypted).
func (w *Disk) IsUnlocked() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.isUnlockedLocked()
}

func (w *Disk) isUnlockedLocked() bool {
	if !w.encrypted {
		return w.priv != nil
	}
	if w.priv == nil {
		return false
	}
	if w.unlockUntil == 0 {
		return true
	}
	return time.Now().Unix() <= w.unlockUntil
}

// UnlockUntil returns Unix seconds when the wallet auto-locks (0 if not encrypted or no timeout).
func (w *Disk) UnlockUntil() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.encrypted || !w.isUnlockedLocked() {
		return 0
	}
	return w.unlockUntil
}

func (w *Disk) requireUnlocked() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.encrypted && !w.isUnlockedLocked() {
		return ErrWalletLocked
	}
	if w.priv == nil {
		return fmt.Errorf("no key")
	}
	return nil
}

type pqDiskMigration struct {
	commitSeed           []byte
	tag                  string
	sendCounter          uint64
	keys                 map[string]pqKeyPair
	commitmentsEnabled   bool
	carrierEnabled       bool
	commitmentsExplicit  bool
}

func capturePQFromDisk(df diskFile) *pqDiskMigration {
	if df.PqCommitSeedHex == "" && len(df.PqKeys) == 0 && !df.PqCarrierEnabled && df.PqCommitmentsEnabled == nil {
		return nil
	}
	m := &pqDiskMigration{
		tag:            df.PqTag,
		sendCounter:    df.PqSendCounter,
		keys:           loadPQKeys(df.PqKeys),
		carrierEnabled: df.PqCarrierEnabled,
	}
	if df.PqCommitSeedHex != "" {
		if seed, err := hex.DecodeString(df.PqCommitSeedHex); err == nil && len(seed) == 32 {
			m.commitSeed = append([]byte(nil), seed...)
		}
	}
	if df.PqCommitmentsEnabled != nil {
		m.commitmentsEnabled = *df.PqCommitmentsEnabled
		m.commitmentsExplicit = true
	}
	return m
}

func mergePQIntoSecrets(sec *walletSecrets, mig *pqDiskMigration) {
	if sec == nil || mig == nil {
		return
	}
	if sec.PqCommitSeedHex == "" && len(mig.commitSeed) == 32 {
		sec.PqCommitSeedHex = hex.EncodeToString(mig.commitSeed)
	}
	if sec.PqTag == "" && mig.tag != "" {
		sec.PqTag = mig.tag
	}
	if sec.PqSendCounter == 0 && mig.sendCounter > 0 {
		sec.PqSendCounter = mig.sendCounter
	}
	if len(sec.PqKeys) == 0 && len(mig.keys) > 0 {
		sec.PqKeys = savePQKeys(mig.keys)
	}
	if mig.commitmentsExplicit {
		sec.PqCommitmentsEnabled = mig.commitmentsEnabled
	}
	if mig.carrierEnabled {
		sec.PqCarrierEnabled = true
	}
}

func (w *Disk) applyPQSecretsLocked(sec walletSecrets) {
	if sec.PqCommitSeedHex != "" {
		if seed, err := hex.DecodeString(stringsTrimHex(sec.PqCommitSeedHex)); err == nil && len(seed) == 32 {
			w.pqCommitSeed = append(w.pqCommitSeed[:0], seed...)
		}
	}
	if sec.PqTag != "" {
		w.pqTag = sec.PqTag
	}
	w.pqSendCounter = sec.PqSendCounter
	w.pqKeys = loadPQKeys(sec.PqKeys)
	w.pqCommitmentsEnabled = sec.PqCommitmentsEnabled
	w.pqCarrierEnabled = sec.PqCarrierEnabled
	w.pqDiskMigration = nil
}

func (w *Disk) secretsLocked() walletSecrets {
	sec := walletSecrets{PrivKeyHex: hex.EncodeToString(w.priv.Serialize())}
	if w.hdEnabled() {
		sec.HDSeedHex = hex.EncodeToString(w.hdSeed)
	}
	if len(w.extraPrivHex) > 0 {
		sec.ExtraPrivkeysHex = append([]string(nil), w.extraPrivHex...)
	}
	return appendPQSecrets(sec, w)
}

func (w *Disk) pqSecretsLocked() walletSecrets {
	sec := walletSecrets{
		PqTag:                w.pqTag,
		PqSendCounter:        w.pqSendCounter,
		PqKeys:               savePQKeys(w.pqKeys),
		PqCommitmentsEnabled: w.pqCommitmentsEnabled,
		PqCarrierEnabled:     w.pqCarrierEnabled,
	}
	if len(w.pqCommitSeed) == 32 {
		sec.PqCommitSeedHex = hex.EncodeToString(w.pqCommitSeed)
	}
	return sec
}

func appendPQSecrets(base walletSecrets, w *Disk) walletSecrets {
	pq := w.pqSecretsLocked()
	base.PqCommitSeedHex = pq.PqCommitSeedHex
	base.PqTag = pq.PqTag
	base.PqSendCounter = pq.PqSendCounter
	base.PqKeys = pq.PqKeys
	base.PqCommitmentsEnabled = pq.PqCommitmentsEnabled
	base.PqCarrierEnabled = pq.PqCarrierEnabled
	return base
}

func clearPQDiskFields(df *diskFile) {
	df.PqCommitSeedHex = ""
	df.PqTag = ""
	df.PqSendCounter = 0
	df.PqKeys = nil
	df.PqCommitmentsEnabled = nil
	df.PqCarrierEnabled = false
}

func (w *Disk) applySecretsLocked(sec walletSecrets) error {
	raw, err := hex.DecodeString(strings.TrimSpace(sec.PrivKeyHex))
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("bad privkey in wallet")
	}
	priv, _ := secp256k1.PrivKeyFromBytes(raw)
	w.priv = priv
	w.extraPrivHex = w.extraPrivHex[:0]
	w.hdSeed = nil
	w.loadExtraPrivkeys(sec.ExtraPrivkeysHex)
	if sec.HDSeedHex != "" {
		seed, err := hex.DecodeString(stringsTrimHex(sec.HDSeedHex))
		if err != nil || len(seed) != 32 {
			return fmt.Errorf("bad hd seed in wallet")
		}
		w.hdSeed = seed
	}
	if w.hdEnabled() {
		d0, err := w.deriveReceive(0)
		if err != nil {
			return err
		}
		w.addr = d0.Addr
	} else {
		w.addr = p2pkh(w.addrVer, priv.PubKey())
	}
	w.applyPQSecretsLocked(sec)
	return nil
}

func (w *Disk) wipeSecretsLocked() {
	w.priv = nil
	w.hdSeed = nil
	w.extraPrivHex = nil
	w.unlockUntil = 0
	w.sessionKey = nil
	w.pqCommitSeed = nil
	w.pqKeys = nil
}

// Encrypt encrypts spend keys at rest and locks the wallet (Core encryptwallet).
func (w *Disk) Encrypt(passphrase string) (string, error) {
	passphrase = strings.TrimSpace(passphrase)
	if passphrase == "" {
		return "", fmt.Errorf("empty passphrase")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.encrypted {
		return "", fmt.Errorf("wallet already encrypted")
	}
	if w.priv == nil {
		return "", fmt.Errorf("no key")
	}
	sec := w.secretsLocked()
	salt, nonce, cipher, err := encryptSecrets(passphrase, sec)
	if err != nil {
		return "", err
	}
	w.encrypted = true
	w.encSalt = salt
	w.encNonce = nonce
	w.encCipher = cipher
	w.wipeSecretsLocked()
	if err := w.saveLocked(); err != nil {
		return "", err
	}
	return "wallet encrypted; restart DogeGo and use walletpassphrase before spending", nil
}

// Unlock decrypts spend keys until timeout seconds elapse (0 = until walletlock).
func (w *Disk) Unlock(passphrase string, timeoutSec int64) error {
	passphrase = strings.TrimSpace(passphrase)
	if passphrase == "" {
		return fmt.Errorf("empty passphrase")
	}
	if timeoutSec < 0 {
		return fmt.Errorf("timeout out of range")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.encrypted {
		return fmt.Errorf("wallet is not encrypted")
	}
	key, err := deriveKey(passphrase, w.encSalt)
	if err != nil {
		return err
	}
	sec, err := openSecrets(key, w.encNonce, w.encCipher)
	if err != nil {
		return err
	}
	mergePQIntoSecrets(&sec, w.pqDiskMigration)
	if err := w.applySecretsLocked(sec); err != nil {
		w.wipeSecretsLocked()
		return err
	}
	w.sessionKey = append([]byte(nil), key...)
	if timeoutSec > 0 {
		w.unlockUntil = time.Now().Unix() + timeoutSec
	} else {
		w.unlockUntil = 0
	}
	if w.pqDiskMigration != nil {
		_ = w.saveLocked()
	}
	return nil
}

// Lock clears spend keys from memory (encrypted wallets only).
func (w *Disk) Lock() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.encrypted {
		return fmt.Errorf("wallet is not encrypted")
	}
	w.wipeSecretsLocked()
	return nil
}

// ChangePassphrase re-encrypts secrets (wallet must be unlocked).
func (w *Disk) ChangePassphrase(oldPass, newPass string) error {
	oldPass = strings.TrimSpace(oldPass)
	newPass = strings.TrimSpace(newPass)
	if oldPass == "" || newPass == "" {
		return fmt.Errorf("empty passphrase")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.encrypted {
		return fmt.Errorf("wallet is not encrypted")
	}
	if !w.isUnlockedLocked() {
		return ErrWalletLocked
	}
	oldKey, err := deriveKey(oldPass, w.encSalt)
	if err != nil {
		return err
	}
	if _, err := openSecrets(oldKey, w.encNonce, w.encCipher); err != nil {
		return fmt.Errorf("passphrase change failed")
	}
	sec := w.secretsLocked()
	salt, nonce, cipher, err := encryptSecrets(newPass, sec)
	if err != nil {
		return err
	}
	w.encSalt = salt
	w.encNonce = nonce
	w.encCipher = cipher
	newKey, err := deriveKey(newPass, salt)
	if err != nil {
		return err
	}
	w.sessionKey = append([]byte(nil), newKey...)
	return w.saveLocked()
}

func (w *Disk) loadEncrypted(df diskFile) error {
	if !df.Encrypted {
		return nil
	}
	salt, err := hex.DecodeString(stringsTrimHex(df.EncryptSaltHex))
	if err != nil || len(salt) == 0 {
		return fmt.Errorf("wallet.json: bad encrypt_salt_hex")
	}
	nonce, err := hex.DecodeString(stringsTrimHex(df.EncryptNonceHex))
	if err != nil || len(nonce) == 0 {
		return fmt.Errorf("wallet.json: bad encrypt_nonce_hex")
	}
	cipher, err := hex.DecodeString(stringsTrimHex(df.SecretsCipherHex))
	if err != nil || len(cipher) == 0 {
		return fmt.Errorf("wallet.json: bad secrets_cipher_hex")
	}
	w.encrypted = true
	w.encSalt = salt
	w.encNonce = nonce
	w.encCipher = cipher
	w.addr = strings.TrimSpace(df.Address)
	if w.addr == "" {
		return fmt.Errorf("wallet.json: missing address for encrypted wallet")
	}
	w.wipeSecretsLocked()
	w.pqDiskMigration = capturePQFromDisk(df)
	return nil
}

func (w *Disk) encryptedFields(df *diskFile) {
	if !w.encrypted {
		return
	}
	df.Encrypted = true
	df.Address = w.addr
	df.PrivKeyHex = ""
	df.HDSeedHex = ""
	df.ExtraPrivkeysHex = nil
	df.EncryptSaltHex = hex.EncodeToString(w.encSalt)
	df.EncryptNonceHex = hex.EncodeToString(w.encNonce)
	df.SecretsCipherHex = hex.EncodeToString(w.encCipher)
	clearPQDiskFields(df)
}
