package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dogego/wallet/corewallet"
)

func TestRunPreflightOffline(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	r := RunPreflight(PreflightOptions{OfflineOnly: true})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
	if !r.OfflineOnly {
		t.Fatal("expected offline_only")
	}
	if r.Doc != DogegoLiveWorkflow10Doc {
		t.Fatalf("doc %q", r.Doc)
	}
}

func TestRunPreflightDogeGoRPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"chain": "test", "blocks": 12},
			"error":  nil,
		})
	}))
	defer srv.Close()
	_, portStr, err := netSplitHostPortFromURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	pr := RunPreflight(PreflightOptions{DogeGoPort: port, Host: "127.0.0.1"})
	if pr.DogeGo == nil || pr.DogeGo.Blocks != 12 {
		t.Fatalf("dogego snap %+v issues=%v", pr.DogeGo, pr.Issues)
	}
}

func TestRunPreflightWalletDatRequiredMissing(t *testing.T) {
	pr := RunPreflight(PreflightOptions{OfflineOnly: true, RequireWalletDat: true})
	if pr.OK {
		t.Fatalf("expected required wallet.dat to fail: %+v", pr)
	}
	if len(pr.Issues) != 1 || pr.Issues[0] != "wallet_dat_required_missing" {
		t.Fatalf("issues=%v", pr.Issues)
	}
}

func TestRunPreflightWalletDatFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	if err := corewallet.WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	pr := RunPreflight(PreflightOptions{OfflineOnly: true, RequireWalletDat: true, WalletDatPath: path, WalletDatNetwork: "reboottestnet"})
	if !pr.OK {
		t.Fatalf("preflight failed: %+v", pr)
	}
	if pr.WalletMigration == nil || pr.WalletMigration.Probe == nil || pr.WalletMigration.Probe.KeyCount != 1 {
		t.Fatalf("wallet migration=%+v", pr.WalletMigration)
	}
}

func TestRunPreflightWalletDatFixtureWithPoolNote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 7)
	}
	if err := corewallet.WriteTestWalletDatWithPool(path, pub, secret, 4); err != nil {
		t.Fatal(err)
	}
	pr := RunPreflight(PreflightOptions{OfflineOnly: true, RequireWalletDat: true, WalletDatPath: path, WalletDatNetwork: "reboottestnet"})
	if !pr.OK {
		t.Fatalf("preflight failed: %+v", pr)
	}
	if pr.WalletMigration == nil || pr.WalletMigration.Probe == nil || pr.WalletMigration.Probe.PoolCount != 1 {
		t.Fatalf("wallet migration pool=%+v", pr.WalletMigration)
	}
	foundPool := false
	for _, n := range pr.Notes {
		if strings.Contains(n, "wallet_dat_probe") && strings.Contains(n, "pool=1") {
			foundPool = true
			break
		}
	}
	if !foundPool {
		t.Fatalf("expected wallet_dat_probe pool note, got %v", pr.Notes)
	}
}

func TestRunPreflightWalletDatMixedPoolUnmatchedHint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	spendPub := append([]byte{0x02}, make([]byte, 32)...)
	poolOnlyPub := append([]byte{0x03}, make([]byte, 32)...)
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 3)
	}
	if err := corewallet.WriteTestWalletDatWithMixedPool(path, spendPub, secret, poolOnlyPub, 2, 9); err != nil {
		t.Fatal(err)
	}
	pr := RunPreflight(PreflightOptions{OfflineOnly: true, RequireWalletDat: true, WalletDatPath: path, WalletDatNetwork: "reboottestnet"})
	if !pr.OK {
		t.Fatalf("preflight failed: %+v", pr)
	}
	found := false
	for _, n := range pr.Notes {
		if strings.Contains(n, "pool_keys_unmatched=1") && strings.Contains(n, "pool_unmatched_hint=") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pool_unmatched_hint note, got %v", pr.Notes)
	}
}

func TestRunPreflightWalletDatLiveImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	pub := append([]byte{0x02}, make([]byte, 32)...)
	secret := make([]byte, 32)
	if err := corewallet.WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		switch req.Method {
		case "getblockchaininfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "test", "blocks": 1}})
		case "getwalletinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"walletname": "test"}})
		case "dogego_probewalletdat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"is_bdb": true, "encrypted": false, "key_count": 1,
					"can_import": true, "needs_passphrase": false,
				},
			})
		case "dogego_importwalletdat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{"keys_imported": 1, "via_native_bdb": true, "keypool_refill_size": 50, "pool_indices_replayed": true},
			})
		default:
			t.Fatalf("method %q", req.Method)
		}
	}))
	defer srv.Close()
	_, portStr, err := netSplitHostPortFromURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	pr := RunPreflight(PreflightOptions{
		DogeGoPort:       port,
		Host:             "127.0.0.1",
		RequireWalletDat: true,
		WalletDatPath:    path,
		WalletDatImport:  true,
	})
	if !pr.OK {
		t.Fatalf("preflight failed: %+v", pr)
	}
	if pr.WalletDatImport == nil || pr.WalletDatImport.Status != "passed" {
		t.Fatalf("wallet import=%+v", pr.WalletDatImport)
	}
	foundRefill := false
	for _, n := range pr.Notes {
		if strings.Contains(n, "wallet_dat_keypool_refill_size=50") {
			foundRefill = true
			break
		}
	}
	if !foundRefill {
		t.Fatalf("expected keypool_refill_size note, got %v", pr.Notes)
	}
}

func TestRunPreflightWalletDatLiveProbeOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	pub := append([]byte{0x02}, make([]byte, 32)...)
	secret := make([]byte, 32)
	if err := corewallet.WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	importCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		switch req.Method {
		case "getblockchaininfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"chain": "test", "blocks": 1}})
		case "getwalletinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"walletname": "test"}})
		case "dogego_probewalletdat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"is_bdb": true, "encrypted": false, "key_count": 1,
					"can_import": true, "needs_passphrase": false,
				},
			})
		case "dogego_importwalletdat":
			importCalled = true
			t.Fatal("import should not run in probe-only preflight")
		default:
			t.Fatalf("method %q", req.Method)
		}
	}))
	defer srv.Close()
	_, portStr, err := netSplitHostPortFromURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	pr := RunPreflight(PreflightOptions{
		DogeGoPort:       port,
		Host:             "127.0.0.1",
		RequireWalletDat: true,
		WalletDatPath:    path,
		WalletDatImport:  false,
	})
	if !pr.OK {
		t.Fatalf("preflight failed: %+v", pr)
	}
	if importCalled || pr.WalletDatImport == nil || pr.WalletDatImport.Status != "probe_passed" {
		t.Fatalf("wallet probe=%+v importCalled=%v", pr.WalletDatImport, importCalled)
	}
}

func netSplitHostPortFromURL(raw string) (host, port string, err error) {
	u := raw
	if len(u) > 7 && u[:7] == "http://" {
		u = u[7:]
	}
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] == ':' {
			return u[:i], u[i+1:], nil
		}
	}
	return u, "80", nil
}

func TestRunPreflightWalletDatAutoDiscoverInvalidSkipped(t *testing.T) {
	appData := t.TempDir()
	path := filepath.Join(appData, "Dogecoin", "wallet.dat")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-bdb"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", "")
	t.Setenv("DOGEGO_CORE_DATADIR", "")
	t.Setenv("APPDATA", appData)
	t.Setenv("HOME", t.TempDir())

	pr := RunPreflight(PreflightOptions{OfflineOnly: true})
	if !pr.OK {
		t.Fatalf("expected ok preflight, got issues=%v warnings=%v", pr.Issues, pr.Warnings)
	}
	if pr.WalletMigration != nil {
		t.Fatalf("expected no wallet migration probe, got %+v", pr.WalletMigration)
	}
	for _, w := range pr.Warnings {
		if strings.Contains(w, "wallet_dat_") {
			t.Fatalf("unexpected wallet warning %q", w)
		}
	}
}
