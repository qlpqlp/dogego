// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import (
	"regexp"
	"strings"
)

var certutilSHA1Line = regexp.MustCompile(`(?i)Cert Hash\(sha1\):\s*([0-9a-f ]+)`)

// sha1FingerprintsForSubject parses certutil -store output and returns SHA-1
// fingerprints (lowercase hex, no spaces) for certificates whose Subject contains cn.
func sha1FingerprintsForSubject(storeOutput, cn string) []string {
	cn = strings.TrimSpace(cn)
	if cn == "" {
		return nil
	}
	var out []string
	blocks := strings.Split(storeOutput, "====")
	for _, block := range blocks {
		if !strings.Contains(block, cn) {
			continue
		}
		if !strings.Contains(strings.ToLower(block), "subject:") {
			continue
		}
		m := certutilSHA1Line.FindStringSubmatch(block)
		if len(m) < 2 {
			continue
		}
		fp := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(m[1]), " ", ""))
		if fp != "" {
			out = append(out, fp)
		}
	}
	return out
}

func storeTrustsFingerprint(storeOutput, cn, wantSHA1 string) bool {
	wantSHA1 = strings.ToLower(strings.TrimSpace(wantSHA1))
	if wantSHA1 == "" {
		return false
	}
	for _, fp := range sha1FingerprintsForSubject(storeOutput, cn) {
		if fp == wantSHA1 {
			return true
		}
	}
	return false
}
