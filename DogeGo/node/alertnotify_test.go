// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeAlertMessage(t *testing.T) {
	got := sanitizeAlertMessage("hello\x00world\nline")
	if got != "helloworld line" {
		t.Fatalf("got %q", got)
	}
}

func TestRunAlertNotifySubstitutesMessage(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "alert.txt")
	cmd := "echo %s > " + out
	if err := RunAlertNotify(cmd, "test warning"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "test warning") {
		t.Fatalf("file %q", b)
	}
}

func TestAlertNotifyStateDedupes(t *testing.T) {
	var st alertNotifyState
	st.maybeNotify("", false, []string{"a"})
	st.maybeNotify("echo ok", true, []string{"a"})
	if st.last != "" {
		t.Fatalf("skip IBD should not set last")
	}
	st.maybeNotify("echo ok", false, []string{"warn"})
	if st.last != "warn" {
		t.Fatalf("last %q", st.last)
	}
	st.maybeNotify("echo ok", false, []string{"warn"})
	// unchanged last - second call is no-op (no panic)
}
