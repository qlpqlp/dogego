package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCertLoadConfigDatadirOverride(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "dogecoinconf.json")
	if err := os.WriteFile(confPath, []byte(`{"network":"testnet","datadir":"/fromfile","autostart":"login"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f, path, err := certLoadConfig(confPath, "/override")
	if err != nil || path != confPath {
		t.Fatalf("load: path=%q err=%v", path, err)
	}
	if f.DataDir != "/override" {
		t.Fatalf("datadir %q", f.DataDir)
	}
	if !f.AutostartOnLogin() {
		t.Fatal("expected autostart login")
	}
}
