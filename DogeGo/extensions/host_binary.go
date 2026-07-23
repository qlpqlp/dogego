// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// hostNativeExecutable reports whether path looks like a native binary for this OS.
// Used so a Linux ELF shipped in a catalog zip is not treated as a Windows executable.
func hostNativeExecutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [4]byte
	if _, err := f.Read(hdr[:]); err != nil {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		return hdr[0] == 'M' && hdr[1] == 'Z'
	case "darwin":
		magic := binary.BigEndian.Uint32(hdr[:])
		switch magic {
		case 0xFEEDFACE, 0xFEEDFACF, 0xCEFAEDFE, 0xCFFAEDFE, 0xCAFEBABE, 0xBEBAFECA:
			return true
		}
		return false
	default:
		return hdr[0] == 0x7f && hdr[1] == 'E' && hdr[2] == 'L' && hdr[3] == 'F'
	}
}

// removeForeignSubprocessBinaries deletes entry.binary candidates that are not native
// to this host (e.g. Linux ELF on Windows), so install/enable can rebuild or rematerialize.
func removeForeignSubprocessBinaries(extDir, binName string) {
	binName = strings.TrimSpace(binName)
	if binName == "" {
		return
	}
	candidates := []string{filepath.Join(extDir, binName)}
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(strings.ToLower(binName), ".exe") {
			candidates = append(candidates, filepath.Join(extDir, binName)+".exe")
		}
	}
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if !hostNativeExecutable(p) {
			_ = os.Remove(p)
		}
	}
}
