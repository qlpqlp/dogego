// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "strings"

// ImportedDescriptor is metadata for a descriptor imported via importdescriptors.
type ImportedDescriptor struct {
	Desc      string
	Timestamp int64
	Internal  bool
	Spendable bool
}

type importedDescJSON struct {
	Desc      string `json:"desc"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Internal  bool   `json:"internal,omitempty"`
	Spendable bool   `json:"spendable,omitempty"`
}

// AddImportedDescriptor records or updates importdescriptors metadata (persisted in wallet.json).
func (w *Disk) AddImportedDescriptor(desc string, timestamp int64, internal, spendable bool) error {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, row := range w.importedDesc {
		if row.Desc == desc {
			w.importedDesc[i].Timestamp = timestamp
			w.importedDesc[i].Internal = internal
			w.importedDesc[i].Spendable = spendable
			return w.saveLocked()
		}
	}
	w.importedDesc = append(w.importedDesc, ImportedDescriptor{
		Desc: desc, Timestamp: timestamp, Internal: internal, Spendable: spendable,
	})
	return w.saveLocked()
}

func (w *Disk) loadImportedDescriptors(rows []importedDescJSON) {
	w.importedDesc = w.importedDesc[:0]
	for _, r := range rows {
		if strings.TrimSpace(r.Desc) == "" {
			continue
		}
		w.importedDesc = append(w.importedDesc, ImportedDescriptor{
			Desc: strings.TrimSpace(r.Desc), Timestamp: r.Timestamp,
			Internal: r.Internal, Spendable: r.Spendable,
		})
	}
}

func (w *Disk) importedDescToDisk() []importedDescJSON {
	if len(w.importedDesc) == 0 {
		return nil
	}
	out := make([]importedDescJSON, len(w.importedDesc))
	for i, r := range w.importedDesc {
		out[i] = importedDescJSON{
			Desc: r.Desc, Timestamp: r.Timestamp, Internal: r.Internal, Spendable: r.Spendable,
		}
	}
	return out
}
