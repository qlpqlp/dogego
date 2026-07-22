// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

//go:generate go run github.com/akavel/rsrc@v0.10.2 -ico ../../assets/dogecoin.ico -o rsrc_windows_amd64.syso -arch amd64

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"dogego/chain"
	"dogego/config"
	"dogego/consensus"
	"dogego/desktop"
	"dogego/launch"
	"dogego/httptls"
	"dogego/indexer"
	"dogego/node"
	"dogego/store"
	"dogego/pow"
	"dogego/ui"
	"dogego/version"
	"dogego/wire"
)

func main() {
	if len(os.Args) < 2 {
		// Convenience default: "dogego" behaves like "dogego node".
		runNode(nil)
		return
	}
	switch os.Args[1] {
	case "genesis":
		runGenesis()
	case "version":
		runVersion(os.Args[2:])
	case "ping":
		runPing(os.Args[2:])
	case "node":
		runNode(os.Args[2:])
	case "spvnode":
		runSPVNode(os.Args[2:])
	case "address":
		runAddress(os.Args[2:])
	case "indexer", "dogeindexer":
		os.Exit(indexer.Run(os.Args[2:], os.Args[0]))
	case "cert":
		runCert(os.Args[2:])
	case "tls":
		runTLS(os.Args[2:])
	case "open":
		runOpen(os.Args[2:])
	case "register-url-protocol":
		runRegisterURLProtocol(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	const text = "usage:\n" +
		"  %s version   (build info; -check queries GitHub; -json for scripts)\n" +
		"  %s genesis\n" +
		"  %s address [-network testnet|mainnet] [-n N]  (random sample P2PKH strings; not real wallets)\n" +
		"  %s ping [-network testnet|mainnet] [-host H] [-port P] [-uacomment TEXT] (port 0 = chain default)\n" +
		"  %s node [-datadir DIR] [-peer HOST:PORT] [-network testnet|mainnet] [-mode full|spv] [-rpc ADDR] [-webui ADDR] [-nowebui] [-nobrowser] [-tray]\n" +
		"        [-nowallet] [-mine] [-uacomment TEXT] [-rawblock_backfill N] [-no_tx_index] [-allowunverifiedmempool] [-mempoolfullrbf]\n" +
		"        [-p2p classic|cgnat|both] [-maxoutbound N] [-maxinbound N]  (CGNAT/Starlink: use cgnat or both for multi-peer relay without inbound)\n" +
		"        [-firewall auto|always|never]  (OS firewall rules for P2P; default auto)\n" +
		"        [-upnp auto|enable|disable]  (router port mapping when listening; default auto)\n" +
		"        (default full node: downloads headers + raw block payloads; use -mode spv or `spvnode` for headers-only)\n" +
		"  %s spvnode  (same flags as node; default -mode spv / headers-only, no raw block store)\n" +
		"        (-peer optional: Core-style DNS seeds plus fixed seeds from chainparamsseeds.h; without -datadir loads dogecoinconf.json or setup page)\n" +
		"        (-nowallet disables built-in wallet; reboot testnet auto-mines to the wallet unless -mine=false)\n" +
		"  %s indexer|dogeindexer  (no args → help + status; or: reindex-tx | verify-bodies | …)\n" +
		"  %s cert offline   (cross-platform offline certification via go test; no scripts required)\n" +
		"  %s cert autostart (verify OS login autostart vs dogecoinconf.json; -conf PATH)\n" +
		"  %s cert founder  (reboot testnet founder preflight; -conf PATH -datadir DIR)\n" +
		"  %s cert provision (dogego-live runner checklist; -json -offline -preflight -run-setup -mine-bootstrap)\n" +
		"  %s cert preflight (dogego-live runner RPC preflight; -json -offline -require-core -require-wallet-dat)\n" +
		"  %s cert weekly    (dogego-live weekly live readiness; -json -skip-preflight -require-wallet-dat -mine-bootstrap)\n" +
		"  %s cert weekly-live (dogego-live scheduled weekly bundle; -mine-bootstrap -require-wallet-dat -include-long-soak -skip-scripts -json)\n" +
		"  %s cert live-soak   (Milestone B multi-hour corruption soak; -duration-min -require-soak-env -skip-scripts -json)\n" +
		"  %s cert bip152-soak (BIP152 AuxPoW/cmpct offline edges; -skip-live=false for live PS1 soak; -json)\n" +
		"  %s cert enable-weekly (set DOGEGO_SCHEDULED_WEEKLY_LIVE via gh; -weekly-only -require-wallet-dat -dry-run -repo)\n" +
		"  %s cert workflow10    (dogego-live workflow 10: optional -enable-github → provision → weekly-live → optional -include-live-soak)\n" +
		"  %s cert setup-parity (reboottestnet Core parity setup; -mine-bootstrap -mine-blocks -json)\n" +
		"  %s cert wallet-migration (Core wallet.dat migration cert; -wallet-dat PATH -passphrase -live-probe -live-import -require-wallet-dat -json)\n" +
		"  %s cert wallet-import   (BIP39/BIP38 + signer + wallet.dat import cert; mirrors wallet_import_cert.ps1; -json)\n" +
		"  %s cert operator        (Milestone E standalone operator cert; -skip-field-evidence -skip-wallet-import -json)\n" +
		"  %s cert pq              (PQ format/carrier cert; no production PQ safety claim; -json)\n" +
		"  %s cert field-evidence (Milestone A mainnet field evidence offline gates)\n" +
		"  %s tls trust-ca [-datadir DIR]  (install local HTTPS CA for browser-trusted loopback TLS)\n" +
		"  %s tls status [-datadir DIR]    (show local TLS paths and trust state)\n" +
		"  %s open [--url URL]  open dashboard (dogecoin://node or http://localhost:2013/)\n" +
		"  %s register-url-protocol [-unregister]  register dogecoin:// handler for this user\n"
	base := os.Args[0]
	fmt.Fprintf(os.Stderr, text, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base)
	fmt.Fprintf(os.Stdout, text, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base, base)
	fmt.Fprintf(os.Stdout, "\nQuick start:\n  %s node     full node (+ browser setup when -datadir is unset)\n  %s indexer   chain folder status (or: %s indexer status)\n", base, base, base)
	showUsageDialog()
	os.Exit(2)
}

func runNode(args []string) { runNodeMode(args, "full") }

func runSPVNode(args []string) { runNodeMode(args, "spv") }

func runNodeMode(args []string, defaultNodeMode string) {
	cmdName := "node"
	if defaultNodeMode == "spv" {
		cmdName = "spvnode"
	}
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	netName := fs.String("network", "testnet", "testnet|mainnet")
	dataDir := fs.String("datadir", "", "data directory (optional if dogecoinconf.json or setup wizard)")
	peer := fs.String("peer", "", "peer host:port (optional: Core-style DNS seeds then fixed seeds from chainparamsseeds.h)")
	mode := fs.String("mode", "", "full|spv (default: full with `node`, spv with `spvnode`; dogecoinconf.json node_mode overrides when flag omitted)")
	rpcAddr := fs.String("rpc", "", "optional JSON-RPC listen address, e.g. 127.0.0.1:18556")
	webui := fs.String("webui", config.DefaultWebUIListen, "local web dashboard listen address (default localhost:2013 for WebAuthn; 127.0.0.1:2013 also works)")
	nowebui := fs.Bool("nowebui", false, "disable the local web dashboard")
	nobrowser := fs.Bool("nobrowser", false, "do not open the dashboard in a browser automatically")
	tray := fs.Bool("tray", false, "show system tray icon (Open Dashboard / Shutdown Node)")
	nowallet := fs.Bool("nowallet", false, "disable built-in wallet (wallet.json under datadir/<network>/; Receive tab)")
	mine := fs.Bool("mine", false, "reboot testnet background mining (default on; use -mine=false to disable)")
	uaComment := fs.String("uacomment", "", "optional P2P user-agent comment (full agent is /DogeGo:"+version.Display()+"(...)/)")
	rawBF := fs.Int("rawblock_backfill", 0, "full blocks at header tip after genesis (0=tip batch off when set on CLI; omit flag for dogecoinconf.json; default large batch with tx index, small with -no_tx_index; ignored in SPV)")
	noTxIdx := fs.Bool("no_tx_index", false, "disable indexes/tx and use a smaller default tip raw-block batch (full node only)")
	allowUV := fs.Bool("allowunverifiedmempool", false, "accept mempool/P2P txs without script verification (NOT production; breaks full-node guarantees)")
	mempoolFullRBF := fs.Bool("mempoolfullrbf", false, "accept replacements of non-BIP125-signaling mempool txs (Core -mempoolfullrbf)")
	maxOrphanTx := fs.Int("maxorphantx", 0, "max orphan transactions in memory (0 = default 100; dogecoinconf.json maxorphantx)")
	p2pConn := fs.String("p2p", "", "P2P connectivity: classic (inbound listen + outbound), cgnat (outbound-only multi-peer), both (default when omitted; dogecoinconf.json p2p_connectivity)")
	maxOutbound := fs.Int("maxoutbound", 0, "max P2P sessions including primary sync peer (0 = default 12)")
	maxInbound := fs.Int("maxinbound", 0, "max inbound P2P connections when listening (0 = default 16)")
	alertNotify := fs.String("alertnotify", "", "shell command when chain warnings change; %s = message (Core -alertnotify)")
	assumeValid := fs.String("assumevalid", "", "Core -assumevalid block hash (empty = mainnet default; 0 = verify all scripts)")
	dnsSeedLookup := fs.Bool("dnsseed", true, "query peer addresses via DNS seeds (Core -dnsseed; false = fixed seeds only)")
	maxTipAge := fs.Int("maxtipage", 0, "max tip age in seconds for initial block download (0 = default 86400; Core -maxtipage)")
	firewall := fs.String("firewall", "", "auto|always|never - add OS firewall rules for P2P (default auto; dogecoinconf.json firewall)")
	upnp := fs.String("upnp", "", "auto|enable|disable - UPnP/NAT-PMP port mapping when listening (default auto; dogecoinconf.json upnp)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })

	if visited["mode"] {
		m := strings.ToLower(strings.TrimSpace(*mode))
		if m != "full" && m != "spv" {
			fmt.Fprintln(os.Stderr, "-mode must be full or spv")
			os.Exit(2)
		}
	}
	if visited["p2p"] {
		p := strings.ToLower(strings.TrimSpace(*p2pConn))
		if p != "classic" && p != "cgnat" && p != "both" {
			fmt.Fprintln(os.Stderr, "-p2p must be classic, cgnat, or both")
			os.Exit(2)
		}
	}
	confFile, confPath := config.LoadFirst()
	merged := config.MergeNode(visited, confFile, *dataDir, *peer, *netName, *rpcAddr, *webui, *nowebui, *nobrowser, *mine, *nowallet, *uaComment, *rawBF, *allowUV, *mempoolFullRBF, *noTxIdx, *mode, visited["mode"], defaultNodeMode, *alertNotify)
	if !visited["tray"] && confFile.Tray == nil && desktop.InteractiveSession() && desktop.TraySupported() {
		merged.Tray = true
	}
	if visited["tray"] {
		merged.Tray = *tray
	}
	if visited["p2p"] {
		merged.P2PConnectivity = strings.ToLower(strings.TrimSpace(*p2pConn))
	}
	if visited["maxoutbound"] {
		merged.MaxOutbound = *maxOutbound
	}
	if visited["maxinbound"] {
		merged.MaxInbound = *maxInbound
	}
	if visited["assumevalid"] {
		merged.AssumeValid = strings.TrimSpace(*assumeValid)
	}
	if visited["dnsseed"] {
		merged.DNSSeedLookup = *dnsSeedLookup
	}
	if visited["maxtipage"] {
		merged.MaxTipAge = *maxTipAge
	}
	if visited["maxorphantx"] {
		merged.MaxOrphanTx = *maxOrphanTx
	}
	if visited["firewall"] {
		merged.Firewall = strings.ToLower(strings.TrimSpace(*firewall))
	}
	if visited["firewall"] {
		switch merged.Firewall {
		case "auto", "always", "never", "":
		default:
			fmt.Fprintln(os.Stderr, "-firewall must be auto, always, or never")
			os.Exit(2)
		}
	}
	if visited["upnp"] {
		merged.Upnp = strings.ToLower(strings.TrimSpace(*upnp))
		switch merged.Upnp {
		case "auto", "enable", "disable", "":
		default:
			fmt.Fprintln(os.Stderr, "-upnp must be auto, enable, or disable")
			os.Exit(2)
		}
	}
	if merged.DataDir != "" {
		abs, err := config.ResolveDataDir(merged.DataDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		merged.DataDir = abs
	}

	setupListen := merged.WebUI
	if setupListen == "" {
		setupListen = config.DefaultWebUIListen
	}

	skipBrowserThisStart := false

	ctx, stopSignal := signal.NotifyContext(context.Background(), platformShutdownSignals()...)
	stop := node.WrapStopWithForceExit(stopSignal)
	defer stop()
	installConsoleGracefulShutdown(stop)

	if merged.DataDir == "" {
		savePath, err := config.ResolveSavePath(confPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		seed := config.SetupWizardSeed(config.File{
			DataDir:         merged.DataDir,
			Peer:            merged.Peer,
			Network:         merged.Network,
			NodeMode:        merged.NodeMode,
			P2PConnectivity: merged.P2PConnectivity,
			MaxOutbound:     merged.MaxOutbound,
			MaxInbound:      merged.MaxInbound,
			RPCAddr:         merged.RPCAddr,
			WebUI:           merged.WebUI,
			NoWebUI:         merged.NoWebUI,
			NoBrowser:       merged.NoBrowser,
			NoWallet:        merged.NoWallet,
			Mine:            merged.Mine,
			UAComment:       merged.UAComment,
			NoTxIndex:       merged.NoTxIndex,
		})
		desktop.ApplyWizardDefaults(&seed)
		openWizard := desktop.OpenWizardInBrowser(merged.NoBrowser)
		fmt.Fprintf(os.Stderr, "DogeGo: need data directory - starting setup web UI (save to %s)\n", savePath)
		saved, err := ui.RunSetupWizard(ctx, setupListen, seed, savePath, openWizard)
		if err != nil {
			if err != context.Canceled {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(1)
		}
		merged = config.FromFile(saved)
		skipBrowserThisStart = true
	}

	confSavePath, err := config.ResolveSavePath(confPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	effectiveFile := config.FileFromMerged(merged)

	waitParentPID := parseWaitPIDArg(args)
	if waitParentPID > 0 {
		fmt.Fprintf(os.Stderr, "DogeGo: waiting for previous process (pid %d) to exit…\n", waitParentPID)
		if !store.WaitForProcessExit(waitParentPID, 90*time.Second) {
			fmt.Fprintf(os.Stderr, "DogeGo: previous process (pid %d) still running; starting anyway\n", waitParentPID)
		}
	}
	maybeReplaceInstallBinary(args)

	restartFn := func() error {
		parentPID := os.Getpid()
		if err := spawnReplacementNode(parentPID); err != nil {
			return err
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			stop()
		}()
		return nil
	}
	applyUpdateFn := func(newExePath string) error {
		installPath, _ := os.Executable()
		parentPID := os.Getpid()
		if err := spawnReplacementFrom(newExePath, parentPID, installPath); err != nil {
			return err
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			stop()
		}()
		return nil
	}

	nid, err := chain.ParseNetwork(merged.Network)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	consensus.ApplyNodeRelayFees(
		merged.IncrementalRelayFeeKoinuPerKB,
		merged.MinRelayTxFeeKoinuPerKB,
		merged.MinRelayTxFeeKoinuPerKB > 0,
	)
	webAddr := merged.WebUI
	if merged.NoWebUI {
		webAddr = ""
	}
	walletEn := !merged.NoWallet
	nodeMode := strings.ToLower(strings.TrimSpace(merged.NodeMode))
	if nodeMode == "" {
		nodeMode = "full"
	}
	fullNode := nodeMode != "spv"
	if desktop.InteractiveSession() && launch.ShouldRegisterURLScheme(merged.DataDir, merged.Network) {
		if err := desktop.RegisterURLScheme(desktop.DefaultURLScheme); err != nil {
			fmt.Fprintf(os.Stderr, "DogeGo: dogecoin:// protocol: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "DogeGo: registered dogecoin:// handler for this user (node dashboard + payment links)")
		}
	}
	var updateChecker *version.UpdateChecker
	if merged.DataDir != "" && !version.UpdateCheckDisabled() {
		updateChecker = version.NewUpdateChecker(merged.DataDir)
		updateChecker.SetOnAvailable(func(st version.UpdateStatus) {
			if desktop.InteractiveSession() {
				desktop.NotifyUpdateAvailable(st.LatestVersion, st.ReleaseURL)
			}
		})
		updateChecker.Start(ctx)
	}
	if !merged.NoWebUI && desktop.InteractiveSession() {
		startConsoleHideRetry()
	}
	if merged.Tray && desktop.TraySupported() {
		setTrayMinimizeOnClose(true)
		startTrayMinimizeWatcher()
		trayConf := effectiveFile
		trayStop := stop
		dataDir := merged.DataDir
		dashURL := desktop.DashboardURL(trayConf)
		trayVer := version.Display()
		peerLinks := desktop.PeerTrayLinks(dataDir, merged.Network)
		quitLabel := "Shutdown Node"
		if len(peerLinks) > 0 {
			quitLabel = "Shutdown all nodes"
		}
		go func() {
			netLabel := desktop.TrayNetworkLabel(merged.Network)
			_ = desktop.StartTray(desktop.TrayConfig{
				Title:        "DogeGo " + netLabel,
				Tooltip:      desktop.TrayBaseTooltip(netLabel),
				Version:      trayVer,
				Network:      merged.Network,
				DashboardURL: dashURL,
				PeerLinks:    peerLinks,
				QuitLabel:    quitLabel,
				OnOpen: func() {
					if dashURL != "" {
						desktop.OpenURLForce(dashURL)
						return
					}
					_ = desktop.OpenDashboard(trayConf)
				},
				OnOpenConsole: func() {
					desktop.OpenDashboardTab(dashURL, "console")
				},
				OnOpenLogs: func() {
					desktop.OpenDashboardTab(dashURL, "console")
				},
				OnShutdown: func() {
					fmt.Fprintln(os.Stderr, "DogeGo: tray shutdown")
					// Cancel local node immediately so quit is responsive; peer shutdown runs in parallel.
					if trayStop != nil {
						trayStop()
					}
					if dataDir != "" {
						go launch.ShutdownDualPeers(dataDir, merged.Network)
					}
				},
				OnCheckUpdates: func() {
					if updateChecker != nil {
						updateChecker.RefreshNow(context.Background())
					}
				},
				OnDownloadUpdate: func() (string, error) {
					if updateChecker == nil {
						return "", fmt.Errorf("update checker unavailable")
					}
					res, err := updateChecker.DownloadReleaseAssetVerified(dataDir)
					if err != nil {
						return "", err
					}
					return res.Path, nil
				},
				OnApplyUpdate: func() error {
					if updateChecker == nil {
						return fmt.Errorf("update checker unavailable")
					}
					st := updateChecker.Status()
					path, err := version.LatestDownloadedAsset(dataDir, st.LatestVersion)
					if err != nil {
						res, dlErr := updateChecker.DownloadReleaseAssetVerified(dataDir)
						if dlErr != nil {
							return dlErr
						}
						path = res.Path
					} else if st.ChecksumURL != "" || st.ChecksumSHA256 != "" {
						if _, err := updateChecker.VerifyDownloadedAsset(path); err != nil {
							return err
						}
					}
					return applyUpdateFn(path)
				},
				OnOpenRelease: func() {
					if updateChecker == nil {
						return
					}
					if u := updateChecker.Status().ReleaseURL; u != "" {
						desktop.OpenURLLog(u)
					}
				},
				OnDismissUpdate: func() error {
					if updateChecker == nil {
						return fmt.Errorf("update checker unavailable")
					}
					return updateChecker.Dismiss()
				},
				UpdateStatus: func() desktop.TrayUpdateInfo {
					if updateChecker == nil {
						return desktop.TrayUpdateInfo{Current: trayVer}
					}
					st := updateChecker.Status()
					return desktop.TrayUpdateInfo{
						Available:      st.Available,
						Dismissed:      st.Dismissed,
						Current:        st.CurrentVersion,
						Latest:         st.LatestVersion,
						ReleaseURL:     st.ReleaseURL,
						DownloadURL:    st.DownloadURL,
						DirectDownload: st.DirectUpdate,
						CheckError:     st.CheckError,
					}
				},
			})
		}()
		fmt.Fprintln(os.Stderr, "DogeGo: system tray enabled (dashboard, console, logs, updates)")
	} else if merged.Tray {
		fmt.Fprintln(os.Stderr, "DogeGo: system tray is not supported on this platform")
	}
	trayOn := merged.Tray
	err = node.Run(ctx, node.Config{
		Network:                nid,
		DataDir:                merged.DataDir,
		Peer:                   merged.Peer,
		RPCAddr:                merged.RPCAddr,
		UAComment:              merged.UAComment,
		WebUIAddr:              webAddr,
		WebUINoBrowser:         merged.NoBrowser || skipBrowserThisStart,
		EnableWallet:           walletEn,
		Mine:                   config.EffectiveMine(merged, visited["mine"], *mine),
		MiningAddress:          merged.MiningAddress,
		FullNode:               fullNode,
		NodeMode:               nodeMode,
		ConfSavePath:           confSavePath,
		EffectiveFile:          effectiveFile,
		Stop:                   stop,
		Restart:                restartFn,
		ApplyUpdate:            applyUpdateFn,
		WaitParentPID:          waitParentPID,
		UpdateChecker:          updateChecker,
		RawBlockBackfill:       merged.EffectiveRawBlockBackfillCount(),
		AllowUnverifiedMempool: merged.AllowUnverifiedMempool,
		FullRBF:                merged.MempoolFullRBF,
		Standard: consensus.StandardPolicyFromNodeConfig(
			merged.HardDustLimitKoinu, merged.AcceptDataCarrier, merged.PermitBareMultisig, merged.DatacarrierSize),
		MempoolLimits: consensus.MempoolRelayLimits{
			MaxTxFeeKoinu:         merged.MaxTxFeeKoinu,
			LimitAncestorCount:    merged.LimitAncestorCount,
			LimitDescendantCount:  merged.LimitDescendantCount,
			LimitAncestorSizeKB:   merged.LimitAncestorSizeKB,
			LimitDescendantSizeKB: merged.LimitDescendantSizeKB,
		},
		BlockMaxWeight:     merged.BlockMaxWeight,
		NoTxIndex:          merged.NoTxIndex,
		BlockStorageOpts:   merged.EffectiveBlockStorageOpts(),
		TxIndexEmbedTx:     merged.EffectiveTxIndexEmbedTx(),
		P2PConnectivity:    merged.P2PConnectivity,
		Firewall:           merged.Firewall,
		Upnp:               merged.Upnp,
		ZmqPubHashBlock:    merged.ZmqPubHashBlock,
		ZmqPubHashTx:       merged.ZmqPubHashTx,
		ZmqPubRawBlock:     merged.ZmqPubRawBlock,
		ZmqPubRawTx:        merged.ZmqPubRawTx,
		MaxOutbound:        merged.MaxOutbound,
		MaxInbound:         merged.MaxInbound,
		BlockSyncWorkers:   merged.BlockSyncWorkers,
		MaxOrphanTx:        merged.EffectiveMaxOrphanTx(),
		MaxMempoolMB:       merged.MaxMempoolMB,
		MempoolExpiryHours: merged.MempoolExpiryHours,
		PersistMempool:     merged.PersistMempool,
		AlertNotify:        merged.AlertNotify,
		AssumeValid:        merged.AssumeValid,
		CheckpointsEnabled: merged.CheckpointsEnabled(),
		MaxTipAge:          merged.MaxTipAge,
		RpcTLS:             httptls.Pair{CertFile: merged.RpcTLSCert, KeyFile: merged.RpcTLSKey},
		WebUITLS:           httptls.Pair{CertFile: merged.WebUITLSCert, KeyFile: merged.WebUITLSKey},
		RpcUser:            merged.RpcUser,
		RpcPassword:        merged.RpcPassword,
		RpcCookie:          merged.RpcCookie,
		RpcAllowIP:         merged.RpcAllowIP,
		RpcWhitelist:       merged.RpcWhitelist,
		RpclimitPerMin:     merged.RpclimitPerMin,
		RpcAuthMaxFail:     merged.RpcAuthMaxFail,
		DogeGoRelayCGNAT:   merged.DogeGoRelayCGNAT,
		CoreRPCAddr:        merged.CoreRPCAddr,
		CoreRPCUser:        merged.CoreRPCUser,
		CoreRPCPassword:    merged.CoreRPCPassword,
		SignerCmd:          merged.SignerCmd,
		OnWebUIReady: func() {
			if merged.DataDir != "" {
				launch.StartDualPeersOnce(merged.DataDir, merged.Network, trayOn)
			}
		},
	})
	if err != nil && err != context.Canceled {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAddress(args []string) {
	fs := flag.NewFlagSet("address", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	netName := fs.String("network", "testnet", "testnet|mainnet")
	count := fs.Int("n", 1, "number of random sample addresses to print (max 50)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	n := *count
	if n < 1 {
		n = 1
	}
	if n > 50 {
		n = 50
	}
	nid, err := chain.ParseNetwork(*netName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	p, err := chain.ParamsFor(nid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	label := "testnet (reboot testnet, P2PKH version 0x41)"
	if nid == chain.MainnetDogecoin {
		label = "mainnet (P2PKH version 30)"
	}
	fmt.Fprintf(os.Stderr, "# Dogecoin %s - random hash160, not a real wallet\n", label)
	for i := 0; i < n; i++ {
		a, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(a)
	}
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "query GitHub Releases now and exit non-zero when an update is available")
	jsonOut := fs.Bool("json", false, "print version and optional update check as JSON")
	_ = fs.Parse(args)

	bi, ok := debug.ReadBuildInfo()
	cur := version.Display()
	ua := version.BuildSubVersion("")

	var updateSt version.UpdateStatus
	if *checkOnly || *jsonOut || !version.UpdateCheckDisabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		if *checkOnly {
			updateSt = version.CheckUpdateOnce(ctx)
		} else if !version.UpdateCheckDisabled() {
			updateSt = version.CheckUpdateOnce(ctx)
		}
		cancel()
	}

	if *jsonOut {
		out := map[string]any{
			"version":         cur,
			"user_agent":      ua,
			"beta":            version.Beta,
			"update_disabled": version.UpdateCheckDisabled(),
		}
		if bi, ok := debug.ReadBuildInfo(); ok {
			out["module"] = bi.Main.Path
			out["go"] = bi.GoVersion
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				out["main_version"] = bi.Main.Version
			}
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					out["vcs_revision"] = s.Value
				case "vcs.modified":
					out["vcs_modified"] = s.Value
				case "vcs.time":
					out["vcs_time"] = s.Value
				}
			}
		}
		if !version.UpdateCheckDisabled() && (updateSt.LatestVersion != "" || updateSt.CheckError != "") {
			out["update"] = map[string]any{
				"available":       updateSt.Available,
				"current":           updateSt.CurrentVersion,
				"latest":            updateSt.LatestVersion,
				"latest_tag":        updateSt.LatestTag,
				"release_url":       updateSt.ReleaseURL,
				"download_url":      updateSt.DownloadURL,
				"checksum_url":      updateSt.ChecksumURL,
				"direct_download":   updateSt.DirectUpdate,
				"build_command":     updateSt.BuildCommand,
				"check_error":       updateSt.CheckError,
				"sources_checked":   updateSt.SourcesChecked,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		if *checkOnly && updateSt.Available {
			os.Exit(2)
		}
		return
	}

	if !ok {
		fmt.Println("dogego (no module build info)")
	} else {
		fmt.Printf("%s\n", version.Banner())
		fmt.Printf("module: %s\n", bi.Main.Path)
		fmt.Printf("version: %s\n", cur)
		fmt.Printf("user-agent: %s\n", ua)
		if version.Beta {
			fmt.Println("release: beta")
		}
		fmt.Printf("go: %s\n", bi.GoVersion)
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			fmt.Printf("main.version: %s\n", bi.Main.Version)
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				fmt.Printf("vcs.revision: %s\n", s.Value)
			case "vcs.modified":
				fmt.Printf("vcs.modified: %s\n", s.Value)
			case "vcs.time":
				fmt.Printf("vcs.time: %s\n", s.Value)
			}
		}
	}

	if version.UpdateCheckDisabled() {
		if *checkOnly {
			fmt.Fprintln(os.Stderr, "update check disabled (DOGEGO_NO_UPDATE_CHECK)")
			os.Exit(1)
		}
		return
	}

	if *checkOnly {
		if updateSt.CheckError != "" {
			fmt.Fprintf(os.Stderr, "update check failed: %s\n", updateSt.CheckError)
			os.Exit(1)
		}
		if updateSt.Available {
			fmt.Printf("update available: %s (running %s)\n", updateSt.LatestVersion, updateSt.CurrentVersion)
			if updateSt.ReleaseURL != "" {
				fmt.Printf("release: %s\n", updateSt.ReleaseURL)
			}
			if updateSt.DownloadURL != "" {
				fmt.Printf("download: %s\n", updateSt.DownloadURL)
			}
			if updateSt.ChecksumURL != "" {
				fmt.Printf("checksum: %s\n", updateSt.ChecksumURL)
			}
			os.Exit(2)
		}
		fmt.Printf("up to date (%s)\n", updateSt.CurrentVersion)
		return
	}

	if updateSt.Available {
		fmt.Printf("\nupdate available: %s (running %s)\n", updateSt.LatestVersion, updateSt.CurrentVersion)
		if updateSt.ReleaseURL != "" {
			fmt.Printf("release: %s\n", updateSt.ReleaseURL)
		}
		if updateSt.DownloadURL != "" {
			fmt.Printf("download: %s\n", updateSt.DownloadURL)
		}
		fmt.Printf("build: %s\n", updateSt.BuildCommand)
		fmt.Println("tip: dogego version -check for exit code 2 when an update exists")
	} else if updateSt.CheckError != "" {
		fmt.Printf("update check: %s\n", updateSt.CheckError)
	}
}

func runGenesis() {
	h, err := pow.Header80()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	got := pow.BlockHashHex(h[:])
	fmt.Println("genesis block hash (SHA256d header):", got)
	if got != chain.GenesisBlockHashHex {
		fmt.Fprintf(os.Stderr, "hash mismatch: want %s\n", chain.GenesisBlockHashHex)
		os.Exit(1)
	}
	if err := pow.CheckScryptPoW(h[:], chain.GenesisBits); err != nil {
		fmt.Fprintln(os.Stderr, "warning: strict scrypt PoW check:", err)
		fmt.Fprintln(os.Stderr, "  (parent chainparams asserts hashGenesisBlock = GetHash() only; remine nonce if you need valid scrypt PoW at height 0.)")
		return
	}
	fmt.Println("scrypt PoW OK for nBits", fmt.Sprintf("0x%x", chain.GenesisBits))
}

func runPing(args []string) {
	fs := flag.NewFlagSet("ping", flag.ExitOnError)
	netName := fs.String("network", "testnet", "testnet|mainnet")
	host := fs.String("host", "127.0.0.1", "peer hostname or IP")
	port := fs.Int("port", 0, "P2P port (0 = default for -network)")
	uaComment := fs.String("uacomment", "", "optional P2P user-agent comment (full agent is /DogeGo:"+version.Display()+"(...)/)")
	_ = fs.Parse(args)

	nid, err := chain.ParseNetwork(*netName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	p, err := chain.ParamsFor(nid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	portUse := *port
	if portUse == 0 {
		portUse = p.Port
	}
	addr := net.JoinHostPort(*host, strconv.Itoa(portUse))
	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()

	tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		fmt.Fprintln(os.Stderr, "expected TCP remote address")
		os.Exit(1)
	}
	var nonceBuf [8]byte
	if _, err := rand.Read(nonceBuf[:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	nonce := binary.LittleEndian.Uint64(nonceBuf[:])
	sub := chain.BuildSubVersion(*uaComment)
	payload := wire.BuildVersionPayload(p.ProtocolVersion, p.NodeNetwork, tcpAddr.IP, uint16(tcpAddr.Port), nonce, sub, 0, true)
	if err := wire.WriteMessage(conn, p.Magic, "version", payload); err != nil {
		fmt.Fprintln(os.Stderr, "write version:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	gotVer := false
	for {
		if err := ctx.Err(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, _, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		switch cmd {
		case "version":
			if !gotVer {
				gotVer = true
				if err := wire.WriteMessage(conn, p.Magic, "verack", nil); err != nil {
					fmt.Fprintln(os.Stderr, "write verack:", err)
					os.Exit(1)
				}
			}
		case "verack":
			fmt.Println("handshake OK (received verack)")
			return
		case "reject":
			fmt.Fprintln(os.Stderr, "peer sent reject")
			os.Exit(1)
		default:
		}
	}
}
