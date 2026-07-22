// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dips

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Entry is one Dogecoin Proposal surfaced in the WebUI Docs tab.
type Entry struct {
	Number  int    `json:"number"`
	ID      string `json:"id"` // e.g. DIP-0021
	Title   string `json:"title"`
	Status  string `json:"status"`
	BIP     string `json:"bip,omitempty"`
	Summary string `json:"summary,omitempty"`
	Path    string `json:"path"`
}

var (
	reDIPFile  = regexp.MustCompile(`(?i)^dip-(\d+)\.md$`)
	reTitle    = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reMetaLine = regexp.MustCompile(`(?i)^\*\*(Status|BIP|Summary):\*\*\s*(.+)$`)
)

// List returns all DIP entries parsed from embedded markdown (excluding README).
func List() ([]Entry, error) {
	var out []Entry
	err := fs.WalkDir(Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := pathBase(p)
		if strings.EqualFold(base, "README.md") {
			return nil
		}
		m := reDIPFile.FindStringSubmatch(base)
		if m == nil {
			return nil
		}
		n, _ := strconv.Atoi(m[1])
		raw, err := Files.ReadFile(p)
		if err != nil {
			return err
		}
		e := parseEntry(n, base, raw)
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func pathBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func parseEntry(num int, base string, raw []byte) Entry {
	id := fmt.Sprintf("DIP-%04d", num)
	e := Entry{
		Number: num,
		ID:     id,
		Title:  id,
		Status: "documented",
		Path:   PathPrefix + base,
	}
	if tm := reTitle.FindSubmatch(raw); len(tm) > 1 {
		title := strings.TrimSpace(string(tm[1]))
		title = strings.TrimPrefix(title, id)
		title = strings.TrimSpace(strings.TrimPrefix(title, ":"))
		title = strings.TrimSpace(strings.TrimPrefix(title, "-"))
		if title != "" {
			e.Title = title
		}
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		mm := reMetaLine.FindStringSubmatch(line)
		if mm == nil {
			continue
		}
		key := strings.ToLower(mm[1])
		val := strings.TrimSpace(mm[2])
		switch key {
		case "status":
			e.Status = val
		case "bip":
			e.BIP = val
		case "summary":
			e.Summary = val
		}
	}
	if e.Summary == "" {
		e.Summary = firstParagraph(raw)
	}
	return e
}

func firstParagraph(raw []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(raw))
	var paras []string
	var cur strings.Builder
	flush := func() {
		s := strings.TrimSpace(cur.String())
		cur.Reset()
		if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "**") {
			return
		}
		paras = append(paras, s)
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			flush()
			if len(paras) > 0 {
				break
			}
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(strings.TrimSpace(line))
	}
	flush()
	if len(paras) == 0 {
		return ""
	}
	s := paras[0]
	if len(s) > 220 {
		return s[:217] + "..."
	}
	return s
}
