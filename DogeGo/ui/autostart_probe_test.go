package ui

import (
	"testing"

	"dogego/config"
)

func TestProbeAutostartLoginDisabled(t *testing.T) {
	r := ProbeAutostartLogin(config.File{Autostart: "disable"})
	if !r.OK || r.WantLogin {
		t.Fatalf("%+v", r)
	}
}

func TestProbeAutostartLoginWantMissing(t *testing.T) {
	r := ProbeAutostartLogin(config.File{Autostart: "login"})
	if r.WantLogin != true {
		t.Fatal("want login")
	}
	if r.Status.Supported && r.Status.Installed {
		if !r.OK {
			t.Fatalf("expected ok when installed: %+v", r)
		}
	} else if r.Status.Supported && !r.Status.Installed {
		if r.OK || len(r.Issues) == 0 {
			t.Fatalf("expected issue: %+v", r)
		}
	}
}
