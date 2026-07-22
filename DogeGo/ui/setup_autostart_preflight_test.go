package ui

import "testing"

func TestBuildSetupAutostartPreflightDisabled(t *testing.T) {
	r := buildSetupAutostartPreflight("disable")
	if !r.OK || len(r.Checks) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestBuildSetupAutostartPreflightLogin(t *testing.T) {
	r := buildSetupAutostartPreflight("login")
	if !r.OK || len(r.Checks) < 2 {
		t.Fatalf("%+v", r)
	}
}
