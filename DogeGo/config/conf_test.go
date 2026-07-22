// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultRPCListenAddr(t *testing.T) {
	if got := DefaultRPCListenAddr("mainnet"); got != "127.0.0.1:22557" {
		t.Fatalf("mainnet %q", got)
	}
	if got := DefaultRPCListenAddr("testnet"); got != "127.0.0.1:44555" {
		t.Fatalf("testnet %q", got)
	}
}

func TestMergeNodeDefaultsRPCForFullNode(t *testing.T) {
	m := MergeNode(nil, File{}, "", "", "mainnet", "", "", false, false, false, false, "", 0, false, false, false, "", false, "full", "")
	if m.RPCAddr != "127.0.0.1:22557" {
		t.Fatalf("RPCAddr=%q want default full-node listen", m.RPCAddr)
	}
	mSpv := MergeNode(nil, File{}, "", "", "mainnet", "", "", false, false, false, false, "", 0, false, false, false, "", false, "spv", "")
	if mSpv.RPCAddr != "" {
		t.Fatalf("spv RPCAddr=%q want empty", mSpv.RPCAddr)
	}
}
func TestEmbeddedAnalyticsEnabled(t *testing.T) {
	var empty File
	if !empty.EmbeddedAnalyticsEnabled() {
		t.Fatal("default should enable analytics sidecar")
	}
	off := false
	disabled := File{AnalyticsSidecar: &off}
	if disabled.EmbeddedAnalyticsEnabled() {
		t.Fatal("explicit false should disable")
	}
}

func TestIBDOptimizeEnabled(t *testing.T) {
	var empty File
	if !empty.IBDOptimizeEnabled() {
		t.Fatal("default should enable IBD optimize")
	}
	off := false
	disabled := File{IBDOptimize: &off}
	if disabled.IBDOptimizeEnabled() {
		t.Fatal("explicit false should disable")
	}
}

func testUserConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestPreferredSaveDirUsesDogeGo(t *testing.T) {
	dir := testUserConfigHome(t)
	got, err := PreferredSaveDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, AppConfigDirName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if st, err := os.Stat(got); err != nil || !st.IsDir() {
		t.Fatalf("mkdir %q: %v", got, err)
	}
}

func configDirNamesCollide() bool {
	return runtime.GOOS == "windows"
}

func TestSearchPathsPrefersDogeGoOverLegacy(t *testing.T) {
	if configDirNamesCollide() {
		t.Skip("dogego and DogeGo share one directory on case-insensitive volumes")
	}
	dir := testUserConfigHome(t)
	legacyDir := filepath.Join(dir, legacyAppConfigDirName)
	newDir := filepath.Join(dir, AppConfigDirName)
	for _, d := range []string{legacyDir, newDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	legacyPath := filepath.Join(legacyDir, FileName)
	newPath := filepath.Join(newDir, FileName)
	if err := os.WriteFile(legacyPath, []byte(`{"datadir":"/legacy","network":"testnet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(`{"datadir":"/new","network":"testnet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := SearchPaths()
	var idxNew, idxLegacy int
	foundNew, foundLegacy := false, false
	for i, p := range paths {
		if p == newPath {
			idxNew = i
			foundNew = true
		}
		if p == legacyPath {
			idxLegacy = i
			foundLegacy = true
		}
	}
	if !foundNew || !foundLegacy {
		t.Fatalf("missing paths in %v", paths)
	}
	if idxNew >= idxLegacy {
		t.Fatalf("DogeGo path should precede legacy: new@%d legacy@%d", idxNew, idxLegacy)
	}
	f, gotPath := LoadFirst()
	if gotPath != newPath {
		t.Fatalf("LoadFirst path %q want %q", gotPath, newPath)
	}
	if f.DataDir != "/new" {
		t.Fatalf("datadir %q", f.DataDir)
	}
}

func TestSearchPathsListsDogeGoBeforeLegacy(t *testing.T) {
	paths := SearchPaths()
	cd, err := os.UserConfigDir()
	if err != nil {
		t.Skip(err)
	}
	newPath := filepath.Join(cd, AppConfigDirName, FileName)
	legacyPath := filepath.Join(cd, legacyAppConfigDirName, FileName)
	var idxNew, idxLegacy = -1, -1
	for i, p := range paths {
		if p == newPath {
			idxNew = i
		}
		if p == legacyPath {
			idxLegacy = i
		}
	}
	if idxNew < 0 || idxLegacy < 0 {
		t.Fatalf("missing config search paths: %v", paths)
	}
	if idxNew >= idxLegacy {
		t.Fatalf("DogeGo path should precede legacy: new@%d legacy@%d", idxNew, idxLegacy)
	}
}

func TestSearchPathsFallsBackToLegacyConfig(t *testing.T) {
	if configDirNamesCollide() {
		t.Skip("dogego and DogeGo share one directory on case-insensitive volumes")
	}
	dir := testUserConfigHome(t)
	legacyDir := filepath.Join(dir, legacyAppConfigDirName)
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, FileName)
	if err := os.WriteFile(legacyPath, []byte(`{"datadir":"/legacy-only","network":"testnet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f, gotPath := LoadFirst()
	if gotPath != legacyPath {
		t.Fatalf("LoadFirst path %q want %q", gotPath, legacyPath)
	}
	if f.DataDir != "/legacy-only" {
		t.Fatalf("datadir %q", f.DataDir)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	f := File{
		DataDir:   "/tmp/x",
		Peer:      "127.0.0.1:1",
		Network:   "testnet",
		UAComment: "lab",
	}
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 10 {
		t.Fatal("short file")
	}
}

func TestMergeNode_CLIOverridesFile(t *testing.T) {
	visited := map[string]bool{"datadir": true, "peer": true}
	file := File{DataDir: "/fromfile", Peer: "old:1", Network: "mainnet"}
	m := MergeNode(visited, file, "/cli", "new:2", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "", false, "full", "")
	if m.DataDir != "/cli" || m.Peer != "new:2" {
		t.Fatalf("%+v", m)
	}
	if m.Network != "mainnet" {
		t.Fatalf("network: %q", m.Network)
	}
}

func TestMergeNode_FileFillsMissing(t *testing.T) {
	visited := map[string]bool{}
	file := File{DataDir: "/d", Peer: "p:1", Network: "testnet", WebUI: "127.0.0.1:9999"}
	m := MergeNode(visited, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "", false, "full", "")
	if m.DataDir != "/d" || m.Peer != "p:1" || m.WebUI != "127.0.0.1:9999" {
		t.Fatalf("%+v", m)
	}
}

func TestFromFile(t *testing.T) {
	m := FromFile(File{DataDir: "/a", Peer: "b:1", Network: "", WebUI: ""})
	if m.Network != "testnet" || m.WebUI != "localhost:2013" {
		t.Fatalf("%+v", m)
	}
}

func TestFromFile_preservesTLS(t *testing.T) {
	m := FromFile(File{
		DataDir:         "/d",
		Network:         "testnet",
		WebUI:           "localhost:2013",
		WebUITLSLocal:   true,
		RpcTLSLocal:     true,
		LocalTLSTrustCA: true,
	})
	if !m.WebUITLSLocal || !m.RpcTLSLocal || !m.LocalTLSTrustCA {
		t.Fatalf("TLS flags lost: %+v", m)
	}
}

func TestMergeNode_CLIAllowUnverifiedMempool(t *testing.T) {
	visited := map[string]bool{"allowunverifiedmempool": true}
	file := File{AllowUnverifiedMempool: false}
	m := MergeNode(visited, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, true, false, false, "", false, "full", "")
	if !m.AllowUnverifiedMempool {
		t.Fatal("cli should win")
	}
}

func TestMergeNode_CLIMempoolFullRBF(t *testing.T) {
	visited := map[string]bool{"mempoolfullrbf": true}
	file := File{MempoolFullRBF: false}
	m := MergeNode(visited, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, true, false, "", false, "full", "")
	if !m.MempoolFullRBF {
		t.Fatal("cli should win")
	}
}

func TestMergeNode_SubcommandBeatsFileNodeMode(t *testing.T) {
	visited := map[string]bool{}
	file := File{DataDir: "/d", Network: "testnet", NodeMode: "spv"}
	m := MergeNode(visited, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "", false, "full", "")
	if m.NodeMode != "full" {
		t.Fatalf("dogego node must stay full when JSON says spv; got %q", m.NodeMode)
	}
	m2 := MergeNode(visited, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "", false, "spv", "")
	if m2.NodeMode != "spv" {
		t.Fatalf("dogego spvnode must stay spv when JSON says full is irrelevant; got %q", m2.NodeMode)
	}
	file2 := File{DataDir: "/d", Network: "testnet", NodeMode: "full"}
	m3 := MergeNode(visited, file2, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "", false, "spv", "")
	if m3.NodeMode != "spv" {
		t.Fatalf("spvnode default over file full; got %q", m3.NodeMode)
	}
}

func TestMergeNode_ModeFlagBeatsSubcommand(t *testing.T) {
	visited := map[string]bool{"mode": true}
	file := File{DataDir: "/d", Network: "testnet", NodeMode: "spv"}
	m := MergeNode(visited, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "spv", true, "full", "")
	if m.NodeMode != "spv" {
		t.Fatalf("-mode spv on node want spv got %q", m.NodeMode)
	}
}

func TestEffectiveRawBlockBackfillCount(t *testing.T) {
	if g := (Merged{NodeMode: "full"}).EffectiveRawBlockBackfillCount(); g != MaxRawBlockBackfill {
		t.Fatalf("full node tx index on: unset raw backfill want %d got %d", MaxRawBlockBackfill, g)
	}
	if g := (Merged{NodeMode: "full", NoTxIndex: true}).EffectiveRawBlockBackfillCount(); g != 5 {
		t.Fatalf("full node no_tx_index: unset want 5 got %d", g)
	}
	if g := (Merged{NodeMode: "spv", RawBlockBackfill: 12}).EffectiveRawBlockBackfillCount(); g != 0 {
		t.Fatalf("spv ignores backfill want 0 got %d", g)
	}
	if g := (Merged{NodeMode: "full", RawBlockBackfill: -1}).EffectiveRawBlockBackfillCount(); g != 0 {
		t.Fatalf("file -1 want 0 got %d", g)
	}
	if g := (Merged{NodeMode: "full", RawBlockBackfill: 12}).EffectiveRawBlockBackfillCount(); g != 12 {
		t.Fatalf("file 12 want 12 got %d", g)
	}
	if g := (Merged{NodeMode: "full", RawBlockBackfillFromCLI: true, RawBlockBackfill: 0}).EffectiveRawBlockBackfillCount(); g != 0 {
		t.Fatalf("cli 0 want genesis-only got %d", g)
	}
	if g := (Merged{NodeMode: "full", RawBlockBackfillFromCLI: true, RawBlockBackfill: 3}).EffectiveRawBlockBackfillCount(); g != 3 {
		t.Fatalf("cli 3 want 3 got %d", g)
	}
	if g := (Merged{NodeMode: "full", RawBlockBackfillFromCLI: true, RawBlockBackfill: MaxRawBlockBackfill + 1}).EffectiveRawBlockBackfillCount(); g != MaxRawBlockBackfill {
		t.Fatalf("cli cap want %d got %d", MaxRawBlockBackfill, g)
	}
}

func TestFileFromMerged_RawBlockCLIZero(t *testing.T) {
	m := Merged{RawBlockBackfillFromCLI: true, RawBlockBackfill: 0}
	f := FileFromMerged(m)
	if f.RawBlockBackfill != -1 {
		t.Fatalf("persist cli genesis-only as -1, got %d", f.RawBlockBackfill)
	}
}

func TestLoadFirstLegacyUseragentJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	raw := `{"datadir":"C:\\d","peer":"","network":"testnet","useragent":"from-legacy"}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGECOINCONF", path)
	f, gotPath := LoadFirst()
	if gotPath != path {
		t.Fatalf("path %q", gotPath)
	}
	if f.UAComment != "from-legacy" {
		t.Fatalf("uacomment %q", f.UAComment)
	}
}

func TestMergeNode_CoreRPCAndSignerCmd(t *testing.T) {
	file := File{
		DataDir:         "/d",
		Network:         "testnet",
		CoreRPCAddr:     "127.0.0.1:44555",
		CoreRPCUser:     "coreuser",
		CoreRPCPassword: "corepass",
		SignerCmd:       "python hwi.py --chain dogecoin --stdin",
	}
	m := MergeNode(nil, file, "", "", "testnet", "", "localhost:2013", false, false, false, false, "", 0, false, false, false, "", false, "full", "")
	if m.CoreRPCAddr != file.CoreRPCAddr || m.CoreRPCUser != file.CoreRPCUser ||
		m.CoreRPCPassword != file.CoreRPCPassword || m.SignerCmd != file.SignerCmd {
		t.Fatalf("merged=%+v", m)
	}
	round := FileFromMerged(m)
	if round.CoreRPCAddr != file.CoreRPCAddr || round.SignerCmd != file.SignerCmd {
		t.Fatalf("round=%+v", round)
	}
}
