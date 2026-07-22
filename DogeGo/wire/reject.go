// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Reject wire layout (BIP61 / Core net_processing): var-string message (≤12),
// uint8 code, var-string reason (≤111), optional 32-byte hash if message is "block" or "tx".
const maxRejectMsgLen = 12
const maxRejectReasonLen = 111

// Reject is a decoded P2P "reject" payload.
type Reject struct {
	Message string
	Code    byte
	Reason  string
	// HashLE is set when Message is "block" or "tx" and a 32-byte hash follows (wire LE order).
	HashLE *[32]byte
}

func (r Reject) String() string {
	s := fmt.Sprintf("%s code=0x%02x %q", r.Message, r.Code, r.Reason)
	if r.HashLE != nil {
		s += fmt.Sprintf(" hash=%x", r.HashLE[:])
	}
	return s
}

func readRejectString(r *bytes.Reader, maxLen uint64) (string, error) {
	n, err := ReadCompactSize(r)
	if err != nil {
		return "", err
	}
	if n > maxLen {
		return "", fmt.Errorf("reject string length %d > max %d", n, maxLen)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// DecodeRejectPayload parses a P2P "reject" message body (Core order: message, code, reason[, hash]).
func DecodeRejectPayload(payload []byte) (Reject, error) {
	var out Reject
	r := bytes.NewReader(payload)
	msg, err := readRejectString(r, maxRejectMsgLen)
	if err != nil {
		return out, fmt.Errorf("reject message field: %w", err)
	}
	out.Message = strings.TrimRight(msg, "\x00")
	var code [1]byte
	if _, err := io.ReadFull(r, code[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return out, fmt.Errorf("reject: missing code byte")
		}
		return out, err
	}
	out.Code = code[0]
	reason, err := readRejectString(r, maxRejectReasonLen)
	if err != nil {
		return out, fmt.Errorf("reject reason field: %w", err)
	}
	out.Reason = reason

	rest := r.Len()
	m := out.Message
	if m == "block" || m == "tx" {
		if rest == 0 {
			return out, nil
		}
		if rest < 32 {
			return out, fmt.Errorf("reject: want 32-byte hash for %s, have %d trailing bytes", m, rest)
		}
		if rest > 32 {
			return out, fmt.Errorf("reject: trailing junk after hash (%d bytes)", rest-32)
		}
		var h [32]byte
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return out, err
		}
		out.HashLE = &h
		return out, nil
	}
	if rest != 0 {
		return out, fmt.Errorf("reject: %d unexpected trailing bytes for message %q", rest, m)
	}
	return out, nil
}

// BIP61 reject codes (Dogecoin Core net_processing.h).
const (
	RejectMalformed        = 0x01
	RejectInvalid          = 0x10
	RejectObsolete         = 0x11
	RejectDuplicate        = 0x12
	RejectNonstandard      = 0x40
	RejectInsufficientFee  = 0x42
	RejectCheckpoint       = 0x43
)

func writeRejectString(w *bytes.Buffer, s string, maxLen uint64) error {
	if uint64(len(s)) > maxLen {
		return fmt.Errorf("reject string too long")
	}
	if err := WriteCompactSize(w, uint64(len(s))); err != nil {
		return err
	}
	_, err := w.WriteString(s)
	return err
}

// EncodeReject builds a P2P "reject" message payload.
func EncodeReject(message string, code byte, reason string, hashLE *[32]byte) ([]byte, error) {
	if message != "block" && message != "tx" && message != "version" && message != "headers" {
		// allow common Core message types only
	}
	if hashLE != nil && message != "block" && message != "tx" {
		return nil, fmt.Errorf("reject hash only valid for block/tx")
	}
	var b bytes.Buffer
	if err := writeRejectString(&b, message, maxRejectMsgLen); err != nil {
		return nil, err
	}
	if err := b.WriteByte(code); err != nil {
		return nil, err
	}
	if err := writeRejectString(&b, reason, maxRejectReasonLen); err != nil {
		return nil, err
	}
	if hashLE != nil {
		if _, err := b.Write(hashLE[:]); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
