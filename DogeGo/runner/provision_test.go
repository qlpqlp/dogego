package runner

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dogego/wallet/corewallet"
)

func TestVerifyProvisionOfflineMinGo(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true})
	if !r.Checklist[0].Done {
		t.Fatalf("expected go step done: %+v", r)
	}
	if r.Total != 9 {
		t.Fatalf("total %d", r.Total)
	}
	if !strings.Contains(r.Doc, "workflow-10-dogego-live") {
		t.Fatalf("doc %q", r.Doc)
	}
}

func TestVerifyProvisionWalletDatEnv(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	if err := corewallet.WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	t.Setenv("DOGEGO_WALLET_DAT_REQUIRED", "1")
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true})
	found := false
	for _, a := range r.Auto {
		if a == "wallet_dat_fixture_ok" {
			found = true
			break
		}
	}
	if !found || !r.Checklist[8].Done {
		t.Fatalf("auto=%v checklist9=%v notes=%v", r.Auto, r.Checklist[8], r.Notes)
	}
}

func TestVerifyProvisionWalletDatPoolNote(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 9)
	}
	if err := corewallet.WriteTestWalletDatWithPool(path, pub, secret, 2); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true})
	foundPool := false
	for _, n := range r.Notes {
		if strings.Contains(n, "pool=1") {
			foundPool = true
			break
		}
	}
	if !foundPool {
		t.Fatalf("expected pool note in %v", r.Notes)
	}
	foundIdx := false
	for _, n := range r.Notes {
		if strings.Contains(n, "pool_idx=2") {
			foundIdx = true
			break
		}
	}
	if !foundIdx {
		t.Fatalf("expected pool_idx note in %v", r.Notes)
	}
	foundMatched := false
	for _, n := range r.Notes {
		if strings.Contains(n, "pool_keys_matched=1") {
			foundMatched = true
			break
		}
	}
	if !foundMatched {
		t.Fatalf("expected pool_keys_matched note in %v", r.Notes)
	}
	foundKeypool := false
	for _, n := range r.Notes {
		if strings.Contains(n, "keypool_note=keypoolrefill") {
			foundKeypool = true
			break
		}
	}
	if !foundKeypool {
		t.Fatalf("expected keypool_note in %v", r.Notes)
	}
}

func TestVerifyProvisionWalletDatMixedPoolUnmatchedHint(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	path := filepath.Join(t.TempDir(), "wallet.dat")
	spendPub := append([]byte{0x02}, make([]byte, 32)...)
	poolOnlyPub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	if err := corewallet.WriteTestWalletDatWithMixedPool(path, spendPub, secret, poolOnlyPub, 2, 9); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true})
	found := false
	for _, n := range r.Notes {
		if strings.Contains(n, "pool_keys_unmatched=1") && strings.Contains(n, "pool_unmatched_hint=") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pool_unmatched_hint note in %v", r.Notes)
	}
}

func TestVerifyProvisionSetupParityEnv(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	t.Setenv("DOGEGO_CORE_COMPARE", "1")
	t.Setenv("DOGEGO_CORE_COMPARE_REQUIRED", "1")
	t.Setenv("DOGEGO_CORE_COMPARE_MIN", "24")
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true})
	found := false
	for _, a := range r.Auto {
		if a == "setup_parity_env_ok" {
			found = true
			break
		}
	}
	if !found || !r.Checklist[6].Done {
		t.Fatalf("setup parity env not detected: auto=%v step7=%v", r.Auto, r.Checklist[6])
	}
}

func TestVerifyProvisionLiveSoakEnv(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	t.Setenv("DOGEGO_SCHEDULED_WEEKLY_LIVE", "1")
	t.Setenv("DOGEGO_SCHEDULED_LIVE_SOAK", "1")
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true})
	foundSoak := false
	for _, a := range r.Auto {
		if a == "live_soak_env_ok" {
			foundSoak = true
		}
	}
	if !foundSoak || !r.Checklist[5].Done {
		t.Fatalf("live soak env not detected: auto=%v step6=%v", r.Auto, r.Checklist[5])
	}
}

func TestVerifyProvisionPreflightPorts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	r := VerifyProvision(ProvisionOptions{
		Preflight:  true,
		DogeGoPort: port,
		CorePort:   port,
	})
	if !r.Checklist[3].Done || !r.Checklist[4].Done {
		t.Fatalf("ports not marked done: %+v", r.Checklist[3:5])
	}
}
