// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
)

// TestOpenHeaderJournalRepairsPartialTail documents crash-safe repair for torn append writes.
func TestOpenHeaderJournalRepairsPartialTail(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range []int{1, 17, 23, 79} {
		extra := extra
		t.Run(fmtPartial(extra), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "headers.bin")
			j, err := OpenHeaderJournal(path, g80[:])
			if err != nil {
				t.Fatal(err)
			}
			prev, _ := j.ReadHeaderAt(0)
			h1 := append([]byte(nil), prev...)
			ph := pow.BlockHashLE(prev)
			copy(h1[4:36], ph[:])
			if err := j.AppendHeaders([][]byte{h1}); err != nil {
				t.Fatal(err)
			}
			wantTip, _ := j.TipHeight()

			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write(make([]byte, extra)); err != nil {
				_ = f.Close()
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			j2, err := OpenHeaderJournal(path, g80[:])
			if err != nil {
				t.Fatal(err)
			}
			gotTip, err := j2.TipHeight()
			if err != nil {
				t.Fatal(err)
			}
			if gotTip != wantTip {
				t.Fatalf("extra=%d tip %d want %d", extra, gotTip, wantTip)
			}
		})
	}
}

func fmtPartial(n int) string {
	return fmt.Sprintf("partial_%d", n)
}
