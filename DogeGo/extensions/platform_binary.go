// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// materializePlatformBinary copies the host-specific binary from entry.binaries into entry.binary.
func materializePlatformBinary(extDir string, man Manifest) error {
	if man.Entry.Type != EntrySubprocess {
		return nil
	}
	binName := strings.TrimSpace(man.Entry.Binary)
	if binName == "" {
		return nil
	}
	removeForeignSubprocessBinaries(extDir, binName)
	if subprocessBinaryExists(extDir, binName) {
		return nil
	}
	if len(man.Entry.Binaries) == 0 {
		return nil
	}
	_, rel, err := SelectPlatformBinaryPath(man.Entry.Binaries)
	if err != nil {
		return fmt.Errorf("extension %q: %w", man.ID, err)
	}
	src, err := safeExtPath(extDir, rel)
	if err != nil {
		return err
	}
	if st, err := os.Stat(src); err != nil || st.IsDir() {
		// Soft-miss: allow source build when the universal zip omitted this platform.
		return nil
	}
	if !hostNativeExecutable(src) {
		return nil
	}
	dst := filepath.Join(extDir, binName)
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(dst), ".exe") {
		dst += ".exe"
	}
	if err := copyFile(src, dst, 0o755); err != nil {
		return fmt.Errorf("extension %q: copy platform binary: %w", man.ID, err)
	}
	return nil
}

func safeExtPath(extDir, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, `\`)
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	absDir, err := filepath.Abs(extDir)
	if err != nil {
		return "", err
	}
	target := filepath.Join(absDir, clean)
	if !strings.HasPrefix(target, absDir+string(os.PathSeparator)) && target != absDir {
		return "", fmt.Errorf("path escape %q", rel)
	}
	return target, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
