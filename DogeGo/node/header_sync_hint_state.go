// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"strings"
	"sync/atomic"
)

var (
	headerSyncRecoveryHint atomic.Value // string
	headerSyncLastErr      atomic.Value // string
)

func setHeaderSyncRecoveryHint(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		headerSyncRecoveryHint.Store("")
		return
	}
	headerSyncRecoveryHint.Store(msg)
}

func noteHeaderSyncFailure(err error) {
	if err == nil {
		return
	}
	headerSyncLastErr.Store(err.Error())
	setHeaderSyncRecoveryHint(headerSyncRecoveryHintForErr(err))
}

func clearHeaderSyncRecoveryHint() {
	headerSyncRecoveryHint.Store("")
	headerSyncLastErr.Store("")
}

func headerSyncRecoveryHintStr() string {
	v := headerSyncRecoveryHint.Load()
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func headerSyncLastFailure() error {
	v := headerSyncLastErr.Load()
	if v == nil {
		return nil
	}
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return errors.New(s)
}

func headerSyncRecoveryHintForErr(err error) string {
	if err == nil {
		return "Header sync is recovering in the background; block download continues. Use dogego_recoverheaders if this persists."
	}
	s := err.Error()
	if strings.Contains(s, "bad nBits") {
		return "Header sync is recovering from damaged headers (bad nBits); automatic rewind and retry are in progress. Use dogego_recoverheaders or Overview → Recover header journal if this repeats."
	}
	if strings.Contains(s, "legacy scrypt header after auxpow") || strings.Contains(s, "auxpow header before activation") {
		return "Header sync retrying peers at the merge-mining boundary (height 371337 on mainnet); block download continues. Use dogego_recoverheaders if this repeats."
	}
	if strings.Contains(s, "checkpoint hash mismatch") {
		return "Header sync rewinding past a Core checkpoint height (wrong hash in headers.bin); block download continues. Use dogego_recoverheaders if this repeats."
	}
	if strings.Contains(s, "chain id must be zero (litecoin merge-mining parent)") {
		return "Header sync is stuck on an outdated auxpow check (valid peers rejected). Rebuild and restart with the current dogego.exe - headers should advance past ~500k. Block download continues via block-assist."
	}
	if strings.Contains(s, "header sync incomplete") {
		return "Header sync paused while block bodies catch up; headers will resume when the contiguous chain is closer to the peer tip."
	}
	if strings.Contains(s, "background catch-up required") || strings.Contains(s, "header sync stall") {
		return "Header sync is catching up automatically on a background peer while this connection downloads blocks; no manual recovery needed."
	}
	if recoverableHeaderPeerErr(err) {
		return "Header sync retrying after a peer disconnect; block download continues via block-assist workers."
	}
	return "Header sync is recovering in the background (" + s + "); block download continues."
}
