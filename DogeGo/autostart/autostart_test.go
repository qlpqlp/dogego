package autostart

import (
	"strings"
	"testing"
)

func TestOnLogin(t *testing.T) {
	if !OnLogin("login") || !OnLogin(" LOGIN ") {
		t.Fatal("expected login")
	}
	if OnLogin("disable") || OnLogin("") {
		t.Fatal("expected not login")
	}
}

func TestNormalizeConfig(t *testing.T) {
	v := " LOGIN "
	NormalizeConfig(&v)
	if v != ValueLogin {
		t.Fatalf("got %q", v)
	}
	empty := ""
	NormalizeConfig(&empty)
	if empty != ValueDisable {
		t.Fatalf("got %q", empty)
	}
}

func TestNodeArgv(t *testing.T) {
	args := nodeArgv(Options{Subcommand: "spvnode", DataDir: "/data/doge"})
	if len(args) != 4 || args[0] != "spvnode" || args[2] != "-datadir" || args[3] != "/data/doge" {
		t.Fatalf("argv: %v", args)
	}
}

func TestNormalizeOptsRequiresConf(t *testing.T) {
	err := normalizeOpts(&Options{ExePath: "."})
	if err == nil || !strings.Contains(err.Error(), "config path") {
		t.Fatalf("expected config error, got %v", err)
	}
}
