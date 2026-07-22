package runner

import (
	"os/exec"
	"strings"
	"testing"
)

func TestEnableScheduledLiveDryRun(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not in PATH")
	}
	r := EnableScheduledLive(EnableWeeklyOptions{DryRun: true, Repo: "owner/repo"})
	if !r.OK || r.Repo != "owner/repo" || len(r.Vars) != 3 {
		t.Fatalf("%+v", r)
	}
	if r.Doc != DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", r.Doc)
	}
}

func TestEnableScheduledLiveWeeklyOnly(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not in PATH")
	}
	r := EnableScheduledLive(EnableWeeklyOptions{DryRun: true, WeeklyOnly: true, Repo: "owner/repo"})
	if len(r.Vars) != 1 || r.Vars[0].Name != "DOGEGO_SCHEDULED_WEEKLY_LIVE" {
		t.Fatalf("%+v", r)
	}
}

func TestEnableScheduledLiveRequireWalletDat(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not in PATH")
	}
	r := EnableScheduledLive(EnableWeeklyOptions{DryRun: true, WeeklyOnly: true, RequireWalletDat: true, Repo: "owner/repo"})
	if len(r.Vars) != 2 {
		t.Fatalf("%+v", r.Vars)
	}
	if r.Vars[1].Name != "DOGEGO_WALLET_DAT_REQUIRED" {
		t.Fatalf("%+v", r.Vars)
	}
	if !strings.Contains(r.Notes[0], "require_wallet_dat=true") {
		t.Fatalf("notes=%v", r.Notes)
	}
}

func TestDetectGitHubRepoParse(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://github.com/dogecoin/dogecoin.git", "dogecoin/dogecoin"},
		{"git@github.com:qlpqlp/dogego.git", "qlpqlp/dogego"},
	}
	for _, c := range cases {
		if m := ghRepoRE.FindStringSubmatch(c.in); len(m) != 2 || m[1] != c.want {
			t.Fatalf("%q -> %v want %q", c.in, m, c.want)
		}
	}
}
