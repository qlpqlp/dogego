// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"strings"

	"dogego/chain"
)

// Known pre-fix bad-cb-amount at mainnet height 2 (Boost uniform_int used % instead of /).
const (
	legacySubsidyBugOutKoinu     = int64(729752) * KoinuPerCoin
	legacySubsidyBugWrongSubsidy = int64(553518) * KoinuPerCoin
)

// LegacySubsidyBugHint returns an operator message when err matches the old subsidy RNG bug.
func LegacySubsidyBugHint(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if !strings.Contains(s, "bad-cb-amount") {
		return ""
	}
	if strings.Contains(s, "55351800000000") && strings.Contains(s, "72975200000000") {
		return " - rebuild dogego from current source (legacy coinbase subsidy now matches Core GetDogecoinBlockSubsidy)"
	}
	return ""
}

// IsLegacySubsidyBug reports the known wrong subsidy cap from builds before the uniform_int fix.
func IsLegacySubsidyBug(err error) bool {
	return LegacySubsidyBugHint(err) != ""
}

// VerifyLegacySubsidyRNG returns nil when height-2 mainnet subsidy matches Core (sanity at startup).
func VerifyLegacySubsidyRNG() error {
	block1, err := chain.Hash256FromDisplayHex("82bc68038f6034c0596b6e313729793a887fded6e92a31fbdf70863f89d9bea2")
	if err != nil {
		return err
	}
	got := BlockSubsidy(2, block1, chain.MainnetDogecoin)
	if got == legacySubsidyBugWrongSubsidy {
		return errors.New("legacy subsidy RNG still uses pre-Core formula (rebuild dogego)")
	}
	if got != legacySubsidyBugOutKoinu {
		return errors.New("legacy subsidy RNG self-check failed")
	}
	return nil
}
