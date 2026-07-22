// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package indexer implements the DogeGo analytics side-DB CLI (init / status / scan).
// It is invoked as `dogego indexer …` (same binary as the node).
package indexer

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dogego/analytics"
	"dogego/chain"
	"dogego/config"
	"dogego/consensus"
	"dogego/node"
	"dogego/pow"
	"dogego/store"
)

// Usage prints help to w; exe is argv[0] (e.g. dogego.exe) for example lines.
func Usage(w io.Writer, exe string) {
	base := filepath.Base(exe)
	fmt.Fprintf(w, `%[1]s indexer - DogeGo analytics side-DB and native chain folder tools.

Native layout: headers.bin / rawblocks/ / indexes/tx/ under <datadir>/<mainnet|testnet>/.

Commands:
  %[1]s indexer version
  %[1]s indexer init    [-datadir DIR] [-network testnet|mainnet] [-db PATH]
  %[1]s indexer status  [-datadir DIR] [-network testnet|mainnet] [-db PATH]
  %[1]s indexer scan    [-datadir DIR] [-network testnet|mainnet] [-db PATH]
          (counts *.bin under rawblocks/ and writes analytics index_progress row 'rawblocks')
  %[1]s indexer reindex-tx [-datadir DIR] [-network testnet|mainnet] [-clear]
          (rebuild indexes/tx from all rawblocks/*.bin)
  %[1]s indexer verify-bodies [-datadir DIR] [-network testnet|mainnet] [-from H] [-to H]
          (CheckBlock + ConnectBlock on stored raw blocks in height range)

Examples:
  %[2]s indexer version
  %[2]s indexer init -datadir %%APPDATA%%\DogeGo -network testnet
  %[2]s indexer status -datadir . -network testnet
  %[2]s indexer scan -datadir . -network testnet

Live header sync remains in "%[2]s node"; this subcommand prepares the analytics DB and prints counts.
The node also runs an embedded sidecar that keeps the same DB updated while node or spvnode runs (headers tip every ~25s; raw bin count in full-node mode).
`, base, base)
}

func isHelpToken(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "help", "-h", "--help", "?":
		return true
	default:
		return false
	}
}

// defaultConfigArgs returns -datadir / -network flags from dogecoinconf.json or sensible defaults.
func defaultConfigArgs() []string {
	conf, _ := config.LoadFirst()
	dataDir := strings.TrimSpace(conf.DataDir)
	if dataDir == "" {
		dataDir = "."
	}
	network := strings.TrimSpace(conf.Network)
	if network == "" {
		network = "testnet"
	}
	return []string{"-datadir", dataDir, "-network", network}
}

func printWelcome(w io.Writer, exe string) {
	fmt.Fprintf(w, "DogeGo indexer - analytics and native chain maintenance.\n\n")
	Usage(w, exe)
	fmt.Fprintf(w, "\nSubcommands: version | init | status | scan | reindex-tx | verify-bodies\n")
	fmt.Fprintf(w, "Config: -datadir DIR -network testnet|mainnet (or dogecoinconf.json next to the binary)\n")
	fmt.Fprintf(w, "Full node sync: run %q (setup wizard opens in the browser when -datadir is unset).\n\n",
		filepath.Base(exe)+" node")
}

// Run executes the indexer CLI. args should be os.Args[2:] from `dogego indexer …`
// (i.e. first element is the subcommand: version, init, status, scan).
// exe is os.Args[0] for usage text. Returns a process exit code.
func Run(args []string, exe string) int {
	if len(args) < 1 || isHelpToken(args[0]) {
		printWelcome(os.Stdout, exe)
		if len(args) >= 1 && isHelpToken(args[0]) {
			return 0
		}
		fmt.Println("No subcommand given - running status with defaults:")
		fmt.Printf("  %s indexer status %s\n\n", filepath.Base(exe), strings.Join(defaultConfigArgs(), " "))
		if err := statusMain(defaultConfigArgs()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "\nFix -datadir / -network or run: "+filepath.Base(exe)+" node  (setup wizard)")
			return 1
		}
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		printWelcome(os.Stdout, exe)
		return 0
	case "version":
		fmt.Println("DogeGo indexer 0.1.0")
		return 0
	case "init":
		runInit(args[1:])
		return 0
	case "status":
		runStatus(args[1:])
		return 0
	case "scan":
		runScan(args[1:])
		return 0
	case "reindex-tx":
		runReindexTx(args[1:])
		return 0
	case "verify-bodies":
		runVerifyBodies(args[1:])
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown indexer subcommand %q\n\n", args[0])
		printWelcome(os.Stdout, exe)
		return 2
	}
}

func parseCommon(args []string) (dataDir, network, dbPath string) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.StringVar(&dataDir, "datadir", ".", "base data directory (contains mainnet/ or testnet/)")
	fs.StringVar(&network, "network", "testnet", "testnet|mainnet")
	fs.StringVar(&dbPath, "db", "", "analytics Pebble path (default: <chaindir>/dogego_analytics.db)")
	_ = fs.Parse(args)
	return dataDir, network, dbPath
}

func resolveChain(dataDir, network string) (chainRoot string, g80 [80]byte, err error) {
	net, err := chain.ParseNetwork(network)
	if err != nil {
		return "", g80, err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", g80, err
	}
	g80b, err := pow.Header80FromParams(p)
	if err != nil {
		return "", g80, err
	}
	g80 = g80b
	root, _, err := node.PrepareChainDataDir(dataDir, net, g80)
	return root, g80, err
}

func runInit(args []string) {
	dataDir, network, dbPath := parseCommon(args)
	chainRoot, g80, err := resolveChain(dataDir, network)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dbPath == "" {
		dbPath = filepath.Join(chainRoot, "dogego_analytics.db")
	}
	db, err := analytics.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	_ = analytics.SetMeta(db, "chain_root", chainRoot)
	_ = analytics.SetMeta(db, "network", network)
	_ = analytics.SetMeta(db, "genesis_hash_hex", pGenesisHash(g80))
	_ = analytics.RecordHeadersSynced(db, -1)
	fmt.Println("analytics db:", dbPath)
	fmt.Println("chain folder:", chainRoot)
	fmt.Println("next: run dogego node to download headers/blocks; extend indexer to ingest into addr_touch / graphs.")
}

func pGenesisHash(g80 [80]byte) string {
	return pow.BlockHashHex(g80[:])
}

func statusMain(args []string) error {
	dataDir, network, dbPath := parseCommon(args)
	chainRoot, g80, err := resolveChain(dataDir, network)
	if err != nil {
		return err
	}
	if dbPath == "" {
		dbPath = filepath.Join(chainRoot, "dogego_analytics.db")
	}
	hdrPath := filepath.Join(chainRoot, "headers.bin")
	j, err := store.OpenHeaderJournal(hdrPath, g80[:])
	if err != nil {
		fmt.Println("headers.bin:", err)
	} else {
		n, _ := j.Count()
		tip, _ := j.TipHeight()
		fmt.Printf("headers.bin: count=%d tip_height=%d\n", n, tip)
		if rb, err := store.OpenRawBlockStore(chainRoot); err == nil {
			rawN, _ := rb.Count()
			cont, _ := store.ContiguousRawBodyHeight(j, rb)
			fmt.Printf("rawblocks: files=%d contiguous_body_height=%d\n", rawN, cont)
		}
	}
	if !analytics.StoreExists(dbPath) {
		fmt.Println("analytics db: not found at", dbPath)
	} else {
		size := analytics.StoreSizeBytes(dbPath)
		db, err := analytics.Open(dbPath)
		if err != nil {
			fmt.Printf("analytics db: %s (%d bytes) open error: %v\n", dbPath, size, err)
		} else {
			ver := analytics.SchemaVersion(db)
			_ = db.Close()
			fmt.Printf("analytics db: %s (%d bytes) schema=%d engine=pebble\n", dbPath, size, ver)
		}
	}
	fmt.Println("chain folder:", chainRoot)
	fmt.Printf("storage: native (headers.bin, rawblocks/, indexes/tx under %s)\n", chainRoot)
	return nil
}

func runStatus(args []string) {
	if err := statusMain(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runScan(args []string) {
	dataDir, network, dbPath := parseCommon(args)
	chainRoot, _, err := resolveChain(dataDir, network)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dbPath == "" {
		dbPath = filepath.Join(chainRoot, "dogego_analytics.db")
	}
	rb, err := store.OpenRawBlockStore(chainRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	n, err := rb.Count()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	db, err := analytics.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()
	if err := analytics.RecordRawBlockScan(db, n); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = analytics.SetMeta(db, "rawblocks_bin_count", fmt.Sprint(n))
	fmt.Printf("rawblocks *.bin count=%d → analytics %s\n", n, dbPath)
}

func runReindexTx(args []string) {
	fs := flag.NewFlagSet("reindex-tx", flag.ExitOnError)
	dataDir := fs.String("datadir", ".", "base data directory")
	network := fs.String("network", "testnet", "testnet|mainnet")
	clear := fs.Bool("clear", true, "remove existing indexes/tx before rebuild")
	_ = fs.Parse(args)
	chainRoot, _, err := resolveChain(*dataDir, *network)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rep, err := store.ReindexTxFromRawBlocks(chainRoot, *clear)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("reindex-tx: blocks=%d tx_files=%d skipped=%d\n", rep.BlocksIndexed, rep.TxFiles, rep.Skipped)
}

func runVerifyBodies(args []string) {
	fs := flag.NewFlagSet("verify-bodies", flag.ExitOnError)
	dataDir := fs.String("datadir", ".", "base data directory")
	network := fs.String("network", "testnet", "testnet|mainnet")
	from := fs.Int64("from", 0, "start height (inclusive)")
	to := fs.Int64("to", -1, "end height (inclusive); -1 = tip")
	_ = fs.Parse(args)
	net, err := chain.ParseNetwork(*network)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	chainRoot, g80, err := resolveChain(*dataDir, *network)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(chainRoot, "headers.bin"), g80[:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := store.OpenRawBlockStore(chainRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	txIx, err := store.OpenTxIndex(chainRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tip, err := j.TipHeight()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	end := *to
	if end < 0 {
		end = tip
	}
	if err := consensus.ValidateStoredBlockBodies(j, raw, txIx, nil, net, *from, end); err != nil {
		fmt.Fprintln(os.Stderr, "verify-bodies:", err)
		os.Exit(1)
	}
	fmt.Printf("verify-bodies: heights %d..%d OK (tip %d)\n", *from, end, tip)
}
