// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

// UpdateSource is a GitHub repository checked for DogeGo releases.
// Disable sources by setting Enabled false when consolidating to one canonical repo.
type UpdateSource struct {
	Owner   string
	Repo    string
	Enabled bool
}

// DefaultUpdateSources lists GitHub repos checked for DogeGo releases.
// github.com/qlpqlp/dogego is the canonical home (website + app + releases).
var DefaultUpdateSources = []UpdateSource{
	{Owner: "qlpqlp", Repo: "dogego", Enabled: true},
	{Owner: "dogeorg", Repo: "dogego", Enabled: false},
	{Owner: "dogecoinfoundation", Repo: "dogego", Enabled: false},
}

func enabledUpdateSources() []UpdateSource {
	out := make([]UpdateSource, 0, len(DefaultUpdateSources))
	for _, s := range DefaultUpdateSources {
		if s.Enabled && s.Owner != "" && s.Repo != "" {
			out = append(out, s)
		}
	}
	return out
}
