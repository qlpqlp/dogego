// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
)

// testBatchView builds a batchView with journal tip0 and in-batch headers at heights tip0+1 .. tip0+len(times).
func testBatchView(tip0 int64, times, bits []uint32) *batchView {
	return &batchView{tip0: tip0, times: times, bits: bits}
}

func TestStrictMinDifficultyRejectsConsecutivePowLimit(t *testing.T) {
	const stubTime = uint32(1_000_000)
	const prevHeight = int64(200_000)
	dc := DogeConsensus{
		PowTargetSpacing:                      60,
		PowAllowMinDifficultyBlocks:           true,
		PowAllowDigishieldMinDifficultyBlocks: true,
		EnforceStrictMinDifficulty:            true,
	}
	times := make([]uint32, 12)
	bits := make([]uint32, 12)
	for i := range times {
		times[i] = stubTime
		bits[i] = pow.DogePowLimitCompact()
	}
	v := testBatchView(prevHeight-11, times, bits)
	ok, err := allowDigishieldMinDifficultyForBlock(v, prevHeight, stubTime+601, dc)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected reject when parent is already at pow limit")
	}
}

func TestStrictMinDifficultyAcceptsAfterGap(t *testing.T) {
	const stubTime = uint32(1_000_000)
	const prevHeight = int64(200_000)
	dc := DogeConsensus{
		PowTargetSpacing:                      60,
		PowAllowMinDifficultyBlocks:           true,
		PowAllowDigishieldMinDifficultyBlocks: true,
		EnforceStrictMinDifficulty:            true,
	}
	times := make([]uint32, 12)
	bits := make([]uint32, 12)
	for i := range times {
		times[i] = stubTime
		bits[i] = 0x1c05a3f4
	}
	v := testBatchView(prevHeight-11, times, bits)
	ok, err := allowDigishieldMinDifficultyForBlock(v, prevHeight, stubTime+601, dc)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected allow when parent is not at pow limit and times satisfy strict rules")
	}
}

func TestStrictMinDifficultyRejectsAtMTPThreshold(t *testing.T) {
	const stubTime = uint32(1_000_000)
	const prevHeight = int64(200_000)
	dc := DogeConsensus{
		PowTargetSpacing:                      60,
		PowAllowMinDifficultyBlocks:           true,
		PowAllowDigishieldMinDifficultyBlocks: true,
		EnforceStrictMinDifficulty:            true,
	}
	times := make([]uint32, 12)
	bits := make([]uint32, 12)
	for i := range times {
		times[i] = stubTime
		bits[i] = 0x1c05a3f4
	}
	v := testBatchView(prevHeight-11, times, bits)
	mtp, err := medianTimePast(v, prevHeight)
	if err != nil {
		t.Fatal(err)
	}
	cand := uint32(mtp + dc.PowTargetSpacing*10)
	ok, err := allowDigishieldMinDifficultyForBlock(v, prevHeight, cand, dc)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected reject when candidate time is not strictly past MTP+10*spacing")
	}
}

func TestLegacyMinDifficultyWhenStrictOff(t *testing.T) {
	const stubTime = uint32(1_000_000)
	const prevHeight = int64(200_000)
	dc := DogeConsensus{
		PowTargetSpacing:                      60,
		PowAllowMinDifficultyBlocks:           true,
		PowAllowDigishieldMinDifficultyBlocks: true,
		EnforceStrictMinDifficulty:            false,
	}
	times := make([]uint32, 12)
	bits := make([]uint32, 12)
	for i := range times {
		times[i] = stubTime
		bits[i] = pow.DogePowLimitCompact()
	}
	v := testBatchView(prevHeight-11, times, bits)
	ok, err := allowDigishieldMinDifficultyForBlock(v, prevHeight, stubTime+121, dc)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("legacy path should allow min-difficulty after 2*spacing even when parent is at pow limit")
	}
}

// TestGetNextWorkMainnet30480 ports Core pow_tests.cpp get_next_work (Bitcoin-era retarget before height 145k).
func TestGetNextWorkMainnet30480(t *testing.T) {
	const (
		lastRetargetTime = uint32(1388149872) // block 30240
		prevHeight       = int64(30479)
		prevTime         = uint32(1388163922)
		prevBits         = uint32(0x1c00974f)
		wantBits         = uint32(0x1c0093a1)
	)
	times := make([]uint32, 241)
	bits := make([]uint32, 241)
	times[0] = lastRetargetTime
	times[240] = prevTime
	bits[240] = prevBits
	v := testBatchView(30238, times, bits)
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := getNextWorkRequired(v, prevHeight, prevTime+60, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("next nBits got 0x%x want 0x%x", got, wantBits)
	}
}

// TestCalculateDogecoinNextWorkMainnet240 ports Core dogecoin_tests.cpp get_next_work_difficulty_limit
// (CalculateDogecoinNextWorkRequired with firstBlockTime at block 1, not the GetNextWorkRequired ancestor walk).
func TestCalculateDogecoinNextWorkMainnet240(t *testing.T) {
	const (
		firstBlockTime = int64(1386474927) // block 1
		prevHeight     = int64(239)
		prevTime       = uint32(1386475638)
		prevBits       = uint32(0x1e0ffff0)
		wantBits       = uint32(0x1e00ffff)
	)
	v := testBatchView(238, []uint32{prevTime}, []uint32{prevBits})
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight+1)
	got, err := calculateDogecoinNextWorkRequired(v, prevHeight, firstBlockTime, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("next nBits got 0x%x want 0x%x", got, wantBits)
	}
}

// TestGetNextWorkPreDigishield9599 ports Core dogecoin_tests.cpp get_next_work_pre_digishield (testnet params = early mainnet rules).
func TestGetNextWorkPreDigishield9599(t *testing.T) {
	const (
		lastRetargetTime = uint32(1386942008) // block 9359
		prevHeight       = int64(9599)
		prevTime         = uint32(1386954113)
		prevBits         = uint32(0x1c1a1206)
		wantBits         = uint32(0x1c15ea59)
	)
	times := make([]uint32, 241)
	bits := make([]uint32, 241)
	times[0] = lastRetargetTime
	times[240] = prevTime
	bits[240] = prevBits
	v := testBatchView(9358, times, bits)
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := getNextWorkRequired(v, prevHeight, prevTime+60, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("next nBits at height %d got 0x%x want 0x%x", prevHeight+1, got, wantBits)
	}
}

// TestCalculateDigishield145000 ports Core dogecoin_tests.cpp get_next_work_digishield
// (post-145000 retarget interval is 1 block - use CalculateDogecoinNextWorkRequired directly).
func TestCalculateDigishield145000(t *testing.T) {
	const (
		lastRetargetTime = int64(1395094427)
		prevHeight       = int64(145000)
		prevTime         = uint32(1395094679)
		prevBits         = uint32(0x1b499dfd)
		wantBits         = uint32(0x1b671062)
	)
	v := testBatchView(prevHeight-1, []uint32{prevTime}, []uint32{prevBits})
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := calculateDogecoinNextWorkRequired(v, prevHeight, lastRetargetTime, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("height %d nBits got 0x%x want 0x%x", prevHeight+1, got, wantBits)
	}
}

// TestCalculateDigishieldModulatedUpper ports get_next_work_digishield_modulated_upper (mainnet #145107).
func TestCalculateDigishieldModulatedUpper(t *testing.T) {
	const (
		lastRetargetTime = int64(1395100835)
		prevHeight       = int64(145107)
		prevTime         = uint32(1395101360)
		prevBits         = uint32(0x1b3439cd)
		wantBits         = uint32(0x1b4e56b3)
	)
	v := testBatchView(145106, []uint32{prevTime}, []uint32{prevBits})
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := calculateDogecoinNextWorkRequired(v, prevHeight, lastRetargetTime, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("nBits got 0x%x want 0x%x", got, wantBits)
	}
}

// TestCalculateDigishieldModulatedLower ports get_next_work_digishield_modulated_lower (mainnet #149423).
func TestCalculateDigishieldModulatedLower(t *testing.T) {
	const (
		lastRetargetTime = int64(1395380517)
		prevHeight       = int64(149423)
		prevTime         = uint32(1395380447)
		prevBits         = uint32(0x1b446f21)
		wantBits         = uint32(0x1b335358)
	)
	v := testBatchView(149422, []uint32{prevTime}, []uint32{prevBits})
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := calculateDogecoinNextWorkRequired(v, prevHeight, lastRetargetTime, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("nBits got 0x%x want 0x%x", got, wantBits)
	}
}

// TestCalculateDigishieldRounding ports get_next_work_digishield_rounding (mainnet #145001).
func TestCalculateDigishieldRounding(t *testing.T) {
	const (
		lastRetargetTime = int64(1395094679)
		prevHeight       = int64(145001)
		prevTime         = uint32(1395094727)
		prevBits         = uint32(0x1b671062)
		wantBits         = uint32(0x1b6558a4)
	)
	v := testBatchView(145000, []uint32{prevTime}, []uint32{prevBits})
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := calculateDogecoinNextWorkRequired(v, prevHeight, lastRetargetTime, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantBits {
		t.Fatalf("nBits got 0x%x want 0x%x", got, wantBits)
	}
}

// TestGetNextWorkMainnet1920Retarget checks Dogecoin's 240-block retarget (Core pow_tests #2015 uses Bitcoin's 2016-block window).
func TestGetNextWorkMainnet1920Retarget(t *testing.T) {
	const (
		windowStartTime = uint32(1231006505) // height 1680 (1920-240)
		prevHeight      = int64(1919)
		prevTime        = uint32(1233061996)
		prevBits        = uint32(0x1d00ffff)
	)
	times := make([]uint32, 241)
	bits := make([]uint32, 241)
	times[0] = windowStartTime
	times[240] = prevTime
	bits[240] = prevBits
	v := testBatchView(1678, times, bits)
	dc := LookupConsensus(chain.MainnetDogecoin, prevHeight)
	got, err := getNextWorkRequired(v, prevHeight, prevTime+60, dc)
	if err != nil {
		t.Fatal(err)
	}
	if got == prevBits {
		t.Fatalf("height %d retarget: expected nBits change from 0x%x", prevHeight+1, prevBits)
	}
	if got != 0x1d03fffc {
		t.Fatalf("height 1920 nBits got 0x%x (sanity: Core Bitcoin-era #2015 vector targets 0x1d03fffc with 2016-block interval)", got)
	}
}

func TestRebootTestnetConsensusModernDigishield(t *testing.T) {
	dc := LookupConsensus(chain.RebootTestnet, 100)
	if !dc.Digishield {
		t.Fatal("reboot testnet from block 1 should use Digishield")
	}
	if !dc.EnforceStrictMinDifficulty {
		t.Fatal("reboot testnet should enforce PR #3967 strict min-diff from block 1")
	}
	if !dc.PowAllowDigishieldMinDifficultyBlocks {
		t.Fatal("reboot testnet should allow hardened digishield min-diff from block 1")
	}
	if dc.CoinbaseMaturity != 240 {
		t.Fatalf("maturity %d want 240", dc.CoinbaseMaturity)
	}
	dcAux := LookupConsensus(chain.RebootTestnet, 200_000)
	if dcAux.AllowLegacyBlocks {
		t.Fatal("post-auxpow reboot testnet should require auxpow blocks")
	}
	if !dcAux.EnforceStrictMinDifficulty {
		t.Fatal("post-auxpow reboot testnet should keep strict min-diff")
	}
}
