// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	groth16DIPVKLen           = 480 // OP_CHECKZKP mode-0: snarkjs compressed VK for 2 public inputs
	groth16DIPVKChunkCount    = 6   // #3869 stack pushes verifier data 0..5
	groth16DIPVKChunkLen      = 80  // bytes per stack chunk
	groth16CompressedProofLen = 192 // compressed G1+G2+G1 (48+96+48)
	defaultVKFile             = "default.vk"
)

var (
	vkMu      sync.RWMutex
	vkByName  = map[string][]byte{}
	defaultVK []byte
)

// LoadVKDir loads *.vk verifying keys from <datadir>/vk/ (480-byte compressed snarkjs layout).
func LoadVKDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	next := make(map[string][]byte)
	var def []byte
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".vk") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			return err
		}
		if len(raw) == 0 {
			continue
		}
		name := strings.TrimSuffix(ent.Name(), filepath.Ext(ent.Name()))
		next[name] = append([]byte(nil), raw...)
		if ent.Name() == defaultVKFile || name == "default" {
			def = next[name]
		}
	}
	vkMu.Lock()
	vkByName = next
	if def != nil {
		defaultVK = def
	} else if v, ok := next["default"]; ok {
		defaultVK = v
	}
	vkMu.Unlock()
	return nil
}

func vkBytes(name string) []byte {
	vkMu.RLock()
	defer vkMu.RUnlock()
	if name == "" {
		return append([]byte(nil), defaultVK...)
	}
	if v, ok := vkByName[name]; ok {
		return append([]byte(nil), v...)
	}
	return nil
}

// InstallDefaultDemoVK writes the bundled Groth16 demo verifying key to vk/default.vk.
func InstallDefaultDemoVK(vkDir string) (int, error) {
	vk, _, _ := groth16DemoVector()
	if len(vk) == 0 {
		return 0, fmt.Errorf("groth16: empty demo vk")
	}
	if err := os.MkdirAll(vkDir, 0o755); err != nil {
		return 0, err
	}
	path := filepath.Join(vkDir, defaultVKFile)
	if err := os.WriteFile(path, vk, 0o644); err != nil {
		return 0, err
	}
	if err := LoadVKDir(vkDir); err != nil {
		return 0, err
	}
	return len(vk), nil
}

func ensureDefaultDemoVK(vkDir string) error {
	if len(vkBytes("")) > 0 {
		return nil
	}
	_, err := InstallDefaultDemoVK(vkDir)
	return err
}

// LoadedVKSummary reports installed verifying keys for status RPC/UI.
func LoadedVKSummary() map[string]interface{} {
	vkMu.RLock()
	defer vkMu.RUnlock()
	names := make([]string, 0, len(vkByName))
	for n := range vkByName {
		names = append(names, n)
	}
	out := map[string]interface{}{
		"loaded":           len(vkByName) > 0,
		"count":            len(vkByName),
		"names":            names,
		"pairing_enabled":  len(defaultVK) > 0,
		"default_vk_bytes": len(defaultVK),
		"compressed_proof_bytes": groth16CompressedProofLen,
		"dip_proof_bytes":        groth16DIPProofLen,
		"dip_vk_bytes":           groth16DIPVKLen,
		"dip_vk_chunk_count":     groth16DIPVKChunkCount,
		"dip_vk_chunk_bytes":     groth16DIPVKChunkLen,
	}
	return out
}

// JoinDIPVKChunks concatenates #3869 OP_CHECKZKP mode-0 verifier stack chunks (6 × 80 bytes).
func JoinDIPVKChunks(chunks [][]byte) ([]byte, error) {
	if len(chunks) != groth16DIPVKChunkCount {
		return nil, fmt.Errorf("groth16: dip vk want %d chunks got %d", groth16DIPVKChunkCount, len(chunks))
	}
	out := make([]byte, 0, groth16DIPVKLen)
	for i, ch := range chunks {
		if len(ch) != groth16DIPVKChunkLen {
			return nil, fmt.Errorf("groth16: dip vk chunk %d want %d bytes got %d", i, groth16DIPVKChunkLen, len(ch))
		}
		out = append(out, ch...)
	}
	if len(out) != groth16DIPVKLen {
		return nil, fmt.Errorf("groth16: dip vk want %d bytes got %d", groth16DIPVKLen, len(out))
	}
	return out, nil
}

// SplitDIPVKChunks splits a flat 480-byte VK into #3869 stack chunks (tests / tooling).
func SplitDIPVKChunks(vk []byte) ([][]byte, error) {
	if len(vk) != groth16DIPVKLen {
		return nil, fmt.Errorf("groth16: dip vk want %d bytes got %d", groth16DIPVKLen, len(vk))
	}
	out := make([][]byte, groth16DIPVKChunkCount)
	for i := 0; i < groth16DIPVKChunkCount; i++ {
		off := i * groth16DIPVKChunkLen
		out[i] = append([]byte(nil), vk[off:off+groth16DIPVKChunkLen]...)
	}
	return out, nil
}

func resolveVerifyingKey(flatHex string, chunkHex []string) ([]byte, error) {
	if len(chunkHex) > 0 {
		chunks := make([][]byte, 0, len(chunkHex))
		for i, s := range chunkHex {
			b, err := hex.DecodeString(strings.TrimSpace(s))
			if err != nil {
				return nil, fmt.Errorf("verifying_key_chunks[%d]: %w", i, err)
			}
			chunks = append(chunks, b)
		}
		return JoinDIPVKChunks(chunks)
	}
	if strings.TrimSpace(flatHex) != "" {
		b, err := hex.DecodeString(strings.TrimSpace(flatHex))
		if err != nil {
			return nil, fmt.Errorf("verifying_key: %w", err)
		}
		return b, nil
	}
	return vkBytes(""), nil
}

func expectedVKLen(publicInputCount int) int {
	if publicInputCount < 0 {
		publicInputCount = 0
	}
	return (publicInputCount + 8) * 48
}

func validateVKSize(vk []byte, publicInputCount int) error {
	want := expectedVKLen(publicInputCount)
	if len(vk) != want && len(vk) != groth16DIPVKLen && publicInputCount == 2 {
		// DIP mode-0 fixed size for exactly 2 public inputs.
		if len(vk) != groth16DIPVKLen {
			return fmt.Errorf("groth16: vk want %d or %d bytes got %d", want, groth16DIPVKLen, len(vk))
		}
		return nil
	}
	if len(vk) != want {
		return fmt.Errorf("groth16: vk want %d bytes got %d", want, len(vk))
	}
	return nil
}
