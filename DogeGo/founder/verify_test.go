package founder

import (
	"testing"

	"dogego/config"
)

func TestVerifyFounderDefaults(t *testing.T) {
	r := Verify(config.File{Network: "testnet", NodeMode: "full", Mine: true, P2PConnectivity: "both"})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
	if r.P2PPort != 44556 {
		t.Fatalf("port %d", r.P2PPort)
	}
}

func TestVerifyFounderWrongNetwork(t *testing.T) {
	r := Verify(config.File{Network: "mainnet"})
	if r.OK || len(r.Issues) == 0 {
		t.Fatalf("%+v", r)
	}
}

func TestVerifyFounderSPV(t *testing.T) {
	r := Verify(config.File{Network: "testnet", NodeMode: "spv"})
	if r.OK {
		t.Fatal("spv should fail")
	}
}

func TestVerifyFounderCGNATWarn(t *testing.T) {
	r := Verify(config.File{Network: "testnet", NodeMode: "full", P2PConnectivity: "cgnat"})
	if !r.OK {
		t.Fatalf("cgnat is warn only: %+v", r)
	}
	if len(r.Warnings) == 0 {
		t.Fatal("expected inbound warning")
	}
}
