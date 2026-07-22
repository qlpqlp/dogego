// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// readConsensusTestdata prefers on-disk testdata (dev/CI) and falls back to go:embed (shipped binary).
func readConsensusTestdata(name string, embedded []byte) ([]byte, error) {
	if raw, err := readConsensusTestdataFromDisk(name); err == nil && len(raw) > 0 {
		return raw, nil
	}
	if len(embedded) > 0 {
		return embedded, nil
	}
	return nil, fmt.Errorf("consensus testdata %s unavailable", name)
}

func readConsensusTestdataFromDisk(name string) ([]byte, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	return os.ReadFile(path)
}
