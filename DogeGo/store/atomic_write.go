// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"errors"
	"os"
	"time"
)

// atomicWriteFile writes path atomically (tmp + rename), falling back to in-place
// overwrite when rename fails (Windows: manifest.json open for read by dashboard).
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return atomicWriteFileStall(path, data, perm, 0)
}

// atomicWriteFileStall is atomicWriteFile with an optional sleep after the .tmp write (crash tests).
func atomicWriteFileStall(path string, data []byte, perm os.FileMode, stallAfterTmp time.Duration) error {
	tmp := path + ".tmp"
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*25) * time.Millisecond)
		}
		if err := os.WriteFile(tmp, data, perm); err != nil {
			lastErr = err
			if !isReplaceExistingErr(err) {
				return err
			}
			continue
		}
		if stallAfterTmp > 0 {
			time.Sleep(stallAfterTmp)
		}
		if err := os.Rename(tmp, path); err == nil {
			return nil
		} else if !isReplaceExistingErr(err) {
			_ = os.Remove(tmp)
			return err
		} else {
			lastErr = err
		}
		// Windows: destination exists or is briefly open (dashboard Lookup / antivirus).
		// Prefer remove+rename; fall back to in-place overwrite.
		_ = os.Remove(path)
		if err := os.Rename(tmp, path); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if err := os.WriteFile(path, data, perm); err == nil {
			_ = os.Remove(tmp)
			return nil
		} else {
			lastErr = err
			_ = os.Remove(tmp)
			if !isReplaceExistingErr(err) {
				return err
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("atomic write failed")
	}
	return lastErr
}

func isReplaceExistingErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrExist) {
		return true
	}
	// Windows ERROR_ALREADY_EXISTS / sharing violation on Rename over open file.
	msg := err.Error()
	return containsAny(msg, "Access is denied", "being used by another process", "cannot replace", "already exists")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && len(s) >= len(sub) {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
