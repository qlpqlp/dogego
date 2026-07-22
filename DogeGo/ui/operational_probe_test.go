package ui

import (
	"testing"

	"dogego/config"
)

func TestProbeOperationalMainnet(t *testing.T) {
	out := ProbeOperational("mainnet", "", config.File{Network: "mainnet", NodeMode: "full", WebUI: "127.0.0.1:2013"}, "")
	if !out.OK {
		t.Fatalf("%+v", out)
	}
	if out.Verify.Role != "mainnet_full_node" {
		t.Fatalf("role %q", out.Verify.Role)
	}
}

func TestProbeOperationalTestnet(t *testing.T) {
	out := ProbeOperational("testnet", "", config.File{Network: "testnet", NodeMode: "full", Mine: true, P2PConnectivity: "both"}, "")
	if !out.OK {
		t.Fatalf("%+v", out)
	}
}
