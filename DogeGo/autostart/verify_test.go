package autostart

import "testing"

func TestVerifyLoginDisabled(t *testing.T) {
	r := VerifyLogin(false)
	if !r.OK || r.WantLogin {
		t.Fatalf("%+v", r)
	}
}

func TestVerifyLoginWant(t *testing.T) {
	r := VerifyLogin(true)
	if r.WantLogin != true {
		t.Fatal("want login")
	}
	if r.Status.Supported && r.Status.Installed {
		if !r.OK {
			t.Fatalf("expected ok when installed: %+v", r)
		}
	} else if r.Status.Supported && !r.Status.Installed {
		if r.OK || len(r.Issues) == 0 {
			t.Fatalf("expected issue when missing: %+v", r)
		}
	}
}
