// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// WeeklyVar is one GitHub Actions repository variable to set.
type WeeklyVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Note  string `json:"note,omitempty"`
}

// EnableWeeklyOptions configures EnableScheduledLive.
type EnableWeeklyOptions struct {
	WeeklyOnly       bool
	RequireWalletDat bool
	DryRun           bool
	Repo             string
	GitRoot          string
}

// EnableWeeklyResult reports gh variable set outcome.
type EnableWeeklyResult struct {
	OK       bool        `json:"ok"`
	Repo     string      `json:"repo,omitempty"`
	Vars     []WeeklyVar `json:"vars,omitempty"`
	Issues   []string    `json:"issues,omitempty"`
	Notes    []string    `json:"notes,omitempty"`
	Commands []string    `json:"commands,omitempty"`
	DryRun   bool        `json:"dry_run,omitempty"`
	Doc      string      `json:"doc,omitempty"`
}

var ghRepoRE = regexp.MustCompile(`github\.com[:/](.+?)(?:\.git)?$`)

// EnableScheduledLive sets DogeGo scheduled live CI repo variables via gh CLI.
func EnableScheduledLive(opts EnableWeeklyOptions) EnableWeeklyResult {
	r := EnableWeeklyResult{
		DryRun: opts.DryRun,
		Doc:    DogegoLiveWorkflow10Doc,
	}
	if _, err := exec.LookPath("gh"); err != nil {
		r.Issues = append(r.Issues, "gh_cli_missing")
		return r
	}

	repo := strings.TrimSpace(opts.Repo)
	if repo == "" {
		repo = strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))
	}
	if repo == "" {
		if detected, err := detectGitHubRepo(opts.GitRoot); err == nil {
			repo = detected
		} else {
			r.Issues = append(r.Issues, "repo_not_detected")
			r.Notes = append(r.Notes, err.Error())
			return r
		}
	}
	r.Repo = repo

	vars := []WeeklyVar{
		{Name: "DOGEGO_SCHEDULED_WEEKLY_LIVE", Value: "1", Note: "weekly: Core 24/24 + corruption mini"},
	}
	if !opts.WeeklyOnly {
		vars = append(vars,
			WeeklyVar{Name: "DOGEGO_SCHEDULED_CORE_GATE", Value: "1", Note: "weekly: Core-aligned gate only"},
			WeeklyVar{Name: "DOGEGO_SCHEDULED_LIVE_SOAK", Value: "1", Note: "weekly: corruption long soak"},
		)
	}
	if opts.RequireWalletDat {
		vars = append(vars, WeeklyVar{
			Name:  "DOGEGO_WALLET_DAT_REQUIRED",
			Value: "1",
			Note:  "weekly: require live Core wallet.dat probe",
		})
	}
	r.Vars = vars

	for _, v := range vars {
		cmd := exec.Command("gh", "variable", "set", v.Name, "--body", v.Value, "--repo", repo)
		r.Commands = append(r.Commands, fmt.Sprintf("gh variable set %s --body %q --repo %s", v.Name, v.Value, repo))
		if opts.DryRun {
			continue
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			r.Issues = append(r.Issues, "gh_set_failed:"+v.Name)
			r.Notes = append(r.Notes, strings.TrimSpace(string(out)))
			return r
		}
	}
	r.OK = true
	dispatch := "gh workflow run dogego.yml --repo " + repo + " -f live_weekly=true"
	if opts.RequireWalletDat {
		dispatch += " -f require_wallet_dat=true"
	}
	r.Notes = append(r.Notes, "dispatch_test="+dispatch)
	r.Notes = append(r.Notes, "next= dogego cert weekly-live -mine-bootstrap -require-wallet-dat")
	r.Notes = append(r.Notes, "next= dogego cert live-soak (Milestone B full; DOGEGO_SCHEDULED_LIVE_SOAK=1)")
	return r
}

func detectGitHubRepo(gitRoot string) (string, error) {
	root := strings.TrimSpace(gitRoot)
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	cmd := exec.Command("git", "-C", root, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote origin: %w", err)
	}
	remote := strings.TrimSpace(string(out))
	if m := ghRepoRE.FindStringSubmatch(remote); len(m) == 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("could not parse GitHub repo from %q", remote)
}
