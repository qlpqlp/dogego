package operational

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/config"
)

func TestVerifyMainnetDefaults(t *testing.T) {
	r := Verify(config.File{Network: "mainnet", NodeMode: "full", WebUI: "127.0.0.1:2013"})
	if !r.OK {
		t.Fatalf("%+v issues=%v", r, r.Issues)
	}
	if r.Role != "mainnet_full_node" {
		t.Fatalf("role %q", r.Role)
	}
}

func TestVerifyMainnetSPVFails(t *testing.T) {
	r := Verify(config.File{Network: "mainnet", NodeMode: "spv"})
	if r.OK {
		t.Fatal("spv should fail mainnet operational")
	}
}

func TestVerifyMainnetCoreLayoutFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "blocks"), 0o700); err != nil {
		t.Fatal(err)
	}
	r := Verify(config.File{Network: "mainnet", NodeMode: "full", DataDir: dir, WebUI: "127.0.0.1:2013"})
	if r.OK {
		t.Fatal("core blocks/ should fail")
	}
}

func TestVerifyRebootTestnetDelegatesFounder(t *testing.T) {
	r := Verify(config.File{Network: "testnet", NodeMode: "full", Mine: true, P2PConnectivity: "both"})
	if !r.OK || r.Founder == nil {
		t.Fatalf("%+v", r)
	}
}
