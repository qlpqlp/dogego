// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strings"
)

// addrmanKey is the Core addrman secret (nKey) used to spread peers across buckets.
type addrmanKey [32]byte

func newAddrmanKey() addrmanKey {
	// Deterministic key for reproducible Core-shaped bucket/slot assignment.
	// (Unit tests and doc-audit invariants depend on stable placement.)
	sum := sha256.Sum256([]byte("dogego-addrman-key-v1"))
	var k addrmanKey
	copy(k[:], sum[:])
	return k
}

func parseAddrmanKeyHex(s string) (addrmanKey, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return addrmanKey{}, false
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return addrmanKey{}, false
	}
	var k addrmanKey
	copy(k[:], b)
	return k, true
}

func (k addrmanKey) hex() string {
	return hex.EncodeToString(k[:])
}

// hash256Concat matches Core CHash256 over concatenated parts (double SHA-256).
func hash256Concat(parts ...[]byte) [32]byte {
	h := sha256.New()
	for _, p := range parts {
		if len(p) > 0 {
			h.Write(p)
		}
	}
	first := h.Sum(nil)
	return sha256.Sum256(first)
}

// addrNetKey returns the 16-byte IPv6-mapped key Core uses for addrman hashing.
func addrNetKey(host string) []byte {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return nil
	}
	// Core uses a special-case for IPv4 in addrman hashing:
	// return nil so callers fall back to hashing the full address string.
	// (This matches how CNetAddr::GetKey behaves for IPv4.)
	if ip4 := ip.To4(); ip4 != nil {
		return nil
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return nil
	}
	return ip16
}

func hostFromAddrPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return host
}

func triedBucketFor(nKey addrmanKey, addr string) int {
	host := hostFromAddrPort(addr)
	key := addrNetKey(host)
	if key == nil {
		return int(addrBucketHash(addr) % addrTriedBucketCount)
	}
	sum := hash256Concat(nKey[:], key)
	return int(binary.LittleEndian.Uint64(sum[:8]) % addrTriedBucketCount)
}

func newBucketFor(nKey addrmanKey, addr, group16, sourceHost string) int {
	host := hostFromAddrPort(addr)
	addrKey := addrNetKey(host)
	if addrKey == nil {
		return int(addrBucketHash(addr) % addrNewBucketCount)
	}
	srcKey := addrNetKey(sourceHost)
	if len(srcKey) == 0 {
		srcKey = []byte{} // Core local / unknown source
	}
	if group16 == "" {
		group16 = "_"
	}
	sum := hash256Concat(nKey[:], []byte(group16), srcKey)
	hash64 := binary.LittleEndian.Uint64(sum[:8])
	return int((hash64 >> 23) & (addrNewBucketCount - 1))
}
