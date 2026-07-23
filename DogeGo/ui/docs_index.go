// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// DocsLink points to a repo doc or external reference.
type DocsLink struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

// DocsSection is one documentation topic for GET /api/docs (web Docs tab).
type DocsSection struct {
	ID    string      `json:"id"`
	Title string      `json:"title"`
	Body  string      `json:"body"`
	Terms []GuideTerm `json:"terms,omitempty"`
	Links []DocsLink  `json:"links,omitempty"`
}

// DocsManifest is the in-dashboard documentation hub.
type DocsManifest struct {
	Title    string        `json:"title"`
	Subtitle string        `json:"subtitle"`
	Sections []DocsSection `json:"sections"`
}

func guideSectionToDocs(gs GuideSection) DocsSection {
	return DocsSection{
		ID:    gs.ID,
		Title: gs.Title,
		Body:  gs.Body,
		Terms: gs.Terms,
		Links: gs.Links,
	}
}

func appendUniqueGuideTerms(dst []GuideTerm, extra []GuideTerm) []GuideTerm {
	seen := make(map[string]struct{}, len(dst))
	for _, t := range dst {
		seen[t.Term] = struct{}{}
	}
	for _, t := range extra {
		if t.Term == "" {
			continue
		}
		if _, ok := seen[t.Term]; ok {
			continue
		}
		seen[t.Term] = struct{}{}
		dst = append(dst, t)
	}
	return dst
}

// DefaultDocsManifest returns operator + integrator documentation served by the web UI.
// Dashboard guide topics (formerly the Guide tab) are merged here to avoid duplicate nav.
func DefaultDocsManifest() DocsManifest {
	guide := DefaultGuideManifest()
	guideByID := make(map[string]GuideSection, len(guide.Sections))
	for _, gs := range guide.Sections {
		guideByID[gs.ID] = gs
	}

	wallet := guideSectionToDocs(guideByID["wallet"])
	wallet.Terms = appendUniqueGuideTerms(wallet.Terms, []GuideTerm{
		{Term: "bumpfee / psbtbumpfee", Explain: "BIP125 replace-by-fee; psbtbumpfee returns PSBT without broadcasting."},
		{Term: "simulaterawtransaction", Explain: "Estimated wallet balance change (DOGE) for raw hex txs before broadcast."},
		{Term: "pq_commitments / pq_carrier", Explain: "Optional wallet flags for OP_RETURN PQ commitments and TX_C/TX_R carrier sends. Settings → Wallet toggles; POST /api/wallet/flags; Send Advanced carrier mode (pq_mode: carrier). Verifier-side only."},
		{Term: "GET /api/core-pq-probe", Explain: "Live PQ format/carrier probe (Features tab); offline mirror: dogego cert pq."},
		{Term: "Raccoon-G / raccoon_g", Explain: "In-tree Foundation port by Ed Tubbs (github.com/edtubbs / x.com/EdTubbs); not full libdogecoin. GitHub Releases compile it with CGO on native OS runners (not cross-compile) because GMP/MPFR must match the target. See docs/RACCOON_G_BUILD.md and docs/CREDITS.md."},
		{Term: "-notls / DOGEGO_NO_TLS", Explain: "Plain HTTP: skips local HTTPS, cert generation, and OS CA install. Use on DogeBox or first installs without TLS. Documented in WEB_UI.md and SECURITY.md."},
	})
	wallet.Links = append(wallet.Links, DocsLink{Label: "WALLET.md", Path: "docs/WALLET.md"})
	wallet.Links = append(wallet.Links, DocsLink{Label: "WEB_UI.md (-notls)", Path: "docs/WEB_UI.md"})
	wallet.Links = append(wallet.Links, DocsLink{Label: "RACCOON_G_BUILD.md (CI / why not cross-compile)", Path: "docs/RACCOON_G_BUILD.md"})
	wallet.Links = append(wallet.Links, DocsLink{Label: "CREDITS.md (Ed Tubbs / helpers)", Path: "docs/CREDITS.md"})
	wallet.Links = append(wallet.Links, DocsLink{Label: "SECURITY.md (-notls)", Path: "docs/SECURITY.md"})
	sections := []DocsSection{
		{
			ID:    "start_here",
			Title: "Start here (pick your path)",
			Body:  "DogeGo runs on its own. You never need Dogecoin Core for sync, wallet, or P2P. Core is optional only when you want side-by-side protocol parity checks (Settings → Advanced → core_rpc_addr).\n\nNew to DogeGo? Read STANDALONE_NODE_QUICKSTART.md and WEB_UI.md.\nRunning a node? OPERATOR.md and CORE_OPERATOR_RUNBOOK.md.\nConnecting an app? INTEGRATION.md and RPC.md.\nContributing code? DEVELOPER_GUIDE.md, ARCHITECTURE.md, and ROADMAP.md (protocol lock).\n\nWant the foundations of peer-to-peer cash? Open Satoshi Nakamoto's Bitcoin white paper (BITCOIN_WHITEPAPER.md).",
			Links: []DocsLink{
				{Label: "STANDALONE_NODE_QUICKSTART.md (first run)", Path: "docs/STANDALONE_NODE_QUICKSTART.md"},
				{Label: "WEB_UI.md (dashboard tabs)", Path: "docs/WEB_UI.md"},
				{Label: "OPERATOR.md (config)", Path: "docs/OPERATOR.md"},
				{Label: "INTEGRATION.md (JSON-RPC)", Path: "docs/INTEGRATION.md"},
				{Label: "DEVELOPER_GUIDE.md (hack on DogeGo)", Path: "docs/DEVELOPER_GUIDE.md"},
				{Label: "BITCOIN_WHITEPAPER.md (Satoshi, 2008)", Path: "docs/BITCOIN_WHITEPAPER.md"},
			},
		},
		{
			ID:    "foundations",
			Title: "Bitcoin white paper (learn the foundations)",
			Body:  "Dogecoin (and DogeGo) inherit Bitcoin's peer-to-peer electronic cash design: transactions, proof-of-work, longest-chain consensus, and incentives. Read Satoshi Nakamoto's original paper here in the Docs viewer, or open the canonical PDF at bitcoin.org.",
			Links: []DocsLink{
				{Label: "BITCOIN_WHITEPAPER.md (full text)", Path: "docs/BITCOIN_WHITEPAPER.md"},
				{Label: "CHAIN_PARAMETERS.md (Dogecoin networks)", Path: "docs/CHAIN_PARAMETERS.md"},
				{Label: "ARCHITECTURE.md (how DogeGo fits)", Path: "docs/ARCHITECTURE.md"},
			},
		},
		{
			ID:    "index",
			Title: "Documentation map",
			Body:  "This Docs tab combines dashboard concepts (sync, P2P, mempool, wallet) with integration and operator runbooks. Use the Features tab for the live RPC method table and optional Core parity backlog (GET /api/capabilities). Click a file link to open it here; links inside each document stay in this viewer. Sources: docs/DOCUMENTATION.md (index), docs/BITCOIN_WHITEPAPER.md, docs/DEVELOPER_GUIDE.md, docs/CHAIN_PARAMETERS.md, docs/CORE_PARITY_GAPS.md, docs/CORE_OPERATOR_RUNBOOK.md, docs/INTEGRATION.md, docs/RPC.md, docs/WALLET.md, docs/WEB_UI.md, docs/OPERATOR.md, docs/ARCHITECTURE.md.",
			Links: []DocsLink{
				{Label: "DOCUMENTATION.md (index)", Path: "docs/DOCUMENTATION.md"},
				{Label: "BITCOIN_WHITEPAPER.md (Satoshi, 2008)", Path: "docs/BITCOIN_WHITEPAPER.md"},
				{Label: "DEVELOPER_GUIDE.md", Path: "docs/DEVELOPER_GUIDE.md"},
				{Label: "CHAIN_PARAMETERS.md", Path: "docs/CHAIN_PARAMETERS.md"},
				{Label: "CORE_PARITY_GAPS.md", Path: "docs/CORE_PARITY_GAPS.md"},
				{Label: "CORE_OPERATOR_RUNBOOK.md", Path: "docs/CORE_OPERATOR_RUNBOOK.md"},
				{Label: "WEB_UI.md", Path: "docs/WEB_UI.md"},
			},
		},
		guideSectionToDocs(guideByID["sync"]),
		guideSectionToDocs(guideByID["p2p"]),
		guideSectionToDocs(guideByID["mempool"]),
		guideSectionToDocs(guideByID["storage"]),
		guideSectionToDocs(guideByID["dashboard"]),
		wallet,
		guideSectionToDocs(guideByID["core_parity"]),
		guideSectionToDocs(guideByID["setup"]),
		guideSectionToDocs(guideByID["security"]),
		{
			ID:    "dips",
			Title: "DIPs (Dogecoin Proposals)",
			Body:  "DIPs catalog Bitcoin Improvement Proposals (BIPs) as implemented or tracked in DogeGo, plus overlay proposals such as DIP-3869 (ZK L2 proofs). Open a card below for the full note, or browse DIPs/README.md.",
			Links: []DocsLink{
				{Label: "DIPs index (README)", Path: "DIPs/README.md"},
				{Label: "CORE_PARITY_GAPS.md", Path: "docs/CORE_PARITY_GAPS.md"},
				{Label: "INTENTIONAL_DIFFERENCES.md", Path: "docs/INTENTIONAL_DIFFERENCES.md"},
			},
		},
		{
			ID:    "operator",
			Title: "Mainnet & testnet operator runbook",
			Body:  "Step-by-step for full-node IBD on mainnet, header journal recovery (bad nBits / ~6720), tx index and block filter rebuilds, mining (generatetoaddress vs auxpow), wallet security, GitHub auto-update (Overview banner, Settings → Interface → Updates, tray), and when to prefer Dogecoin Core instead of DogeGo. Reboot testnet: DNS seed seed.dogego.org first (DogeBox DogeGo full node for quick peer discovery) plus Core fixed seeds - see CHAIN_PARAMETERS.md and OPERATOR.md § Founder checklist; CLI: dogego cert founder. dogego-live CI: dogego cert weekly (readiness), dogego cert weekly-live (scheduled bundle), dogego cert live-soak (Milestone B), dogego cert setup-parity, dogego cert enable-weekly, dogego cert workflow10 - see CORE_SIDE_BY_SIDE_WORKFLOWS.md workflow 10.",
			Links: []DocsLink{
				{Label: "CORE_OPERATOR_RUNBOOK.md", Path: "docs/CORE_OPERATOR_RUNBOOK.md"},
				{Label: "OPERATOR.md (config reference)", Path: "docs/OPERATOR.md"},
				{Label: "CHAIN_PARAMETERS.md (networks & seeds)", Path: "docs/CHAIN_PARAMETERS.md"},
				{Label: "CORE_SIDE_BY_SIDE_WORKFLOWS.md (workflow 10: dogego-live CI)", Path: "docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md"},
			},
		},
		{
			ID:    "integrate",
			Title: "Connect external applications",
			Body:  "DogeGo speaks Core-style JSON-RPC over HTTP POST (default 127.0.0.1:22557). Enable rpc_cookie or rpc_user/rpc_password. Machine-readable catalogs: GET /api/openrpc.json, GET /api/rpc/cookbook, GET /api/rpc/reference.html, GET /api/integration/guides. Step-by-step Console + CLI tutorial: RPC_CONSOLE_TUTORIAL.md.",
			Terms: []GuideTerm{
				{Term: "Console tab", Explain: "POST /api/rpc in-process - no HTTP rpc listen required; see RPC_CONSOLE_TUTORIAL.md"},
				{Term: "curl", Explain: "curl -s --user USER:PASS -H content-type:application/json -d '{\"jsonrpc\":\"1.0\",\"id\":1,\"method\":\"getblockchaininfo\",\"params\":[]}' http://127.0.0.1:22557/"},
				{Term: "dogecoin-cli", Explain: "Point -rpcconnect/-rpcport at DogeGo when RPC auth matches; many commands work, unsupported ones return errors from dispatch."},
				{Term: "Dashboard APIs", Explain: "GET /api/summary, /api/mempool, /api/capabilities - loopback only; not a substitute for JSON-RPC on another host."},
			},
			Links: []DocsLink{
				{Label: "RPC_CONSOLE_TUTORIAL.md (Console + CLI)", Path: "docs/RPC_CONSOLE_TUTORIAL.md"},
				{Label: "INTEGRATION.md", Path: "docs/INTEGRATION.md"},
				{Label: "OPERATOR.md (security)", Path: "docs/OPERATOR.md"},
			},
		},
		{
			ID:    "extensions",
			Title: "Extensions & optional ZK L2",
			Body:  "Optional extensions: catalog install, zip upload, enable/disable from Settings → Extensions or dogego_* manager RPCs. Third-party packages ship dogego.extension.json (wasm or subprocess hosts). Extensions add WebUI only via secure JSON panels (ui_panel / workspace menu; no HTML injection). Optional wallet_rpc uses allowlisted methods after dashboard unlock.\n\nThe dogego.zkl2 subprocess extension adds zkproof-v1 P2P overlay: tx-anchored Groth16 proofs verified off-L1 (no OP_CHECKZKP consensus fork). Pairing verify supports snarkjs compressed proofs (192 B), DIP #3869 affine proofs (384 B), and inline verifying_key / verifying_key_chunks on verifyproof.\n\nManager RPC: dogego_listextensions, dogego_enableextension, dogego_instextension. Extension RPC prefix: dogego_ext_<id>_<method> (e.g. dogego_ext_dogego_zkl2_info). HTTP: GET /api/extensions/catalog, POST /api/extensions/enable, GET /api/extensions/panel?id=<extension-id> (extension-owned ui copy via status RPC).",
			Terms: []GuideTerm{
				{Term: "dogego_enableextension dogego.zkl2", Explain: "Enables built-in ZK L2; negotiates zkproof-v1 after verack on P2P peers."},
				{Term: "verifying_key_chunks", Explain: "Optional six 80-byte hex chunks (#3869 stack VK layout) on verifyproof/checkzkp; not hashed into proof_hash."},
				{Term: "data/vk/default.vk", Explain: "Optional snarkjs verifying key under extensions/dogego.zkl2/data/vk/ for full Groth16 pairing."},
			},
			Links: []DocsLink{
				{Label: "EXTENSIONS.md (platform overview)", Path: "docs/EXTENSIONS.md"},
				{Label: "catalog/HELLO_WORLD.md (RPC + Console + UI demo)", Path: "extensions/catalog/HELLO_WORLD.md"},
				{Label: "catalog/zkl2/docs/USER_GUIDE.md (submit & sync)", Path: "extensions/catalog/zkl2/docs/USER_GUIDE.md"},
				{Label: "catalog/zkl2/docs/PROTOCOL.md (zkproof-v1)", Path: "extensions/catalog/zkl2/docs/PROTOCOL.md"},
				{Label: "catalog/AUTHORING.md (third-party)", Path: "extensions/catalog/AUTHORING.md"},
				{Label: "catalog/BUILDING.md (packaging)", Path: "extensions/catalog/BUILDING.md"},
				{Label: "WEB_UI.md (Settings → Extensions)", Path: "docs/WEB_UI.md"},
			},
		},
		{
			ID:    "implement",
			Title: "Implement or extend DogeGo",
			Body:  "Package layout: cmd/dogego (CLI), node/ (run loop), p2p/, consensus/, store/, mempool/, rpc/, wallet/, ui/. Mainnet consensus is locked to Dogecoin Core (no protocol forks; see ROADMAP.md). Offline certification: dogego cert offline (CI gate), cert_offline_prerequisites bundle, dogego cert wallet-import, dogego cert wallet-migration (-offline-only), dogego cert field-evidence, dogego cert pq (format/carrier), dogego cert operator (Milestone E deep cert). Chain params (mainnet, reboot testnet, DNS/fixed seeds, genesis) are under chain/ - see CHAIN_PARAMETERS.md (not a single file). New RPC: add handler in rpc/, register in dispatch.go SupportedMethods, one-line help in help.go, test in rpc/*_test.go, checkbox in ROADMAP.md, and a row in docs/RPC.md + this Docs manifest. Consensus changes must match Dogecoin Core semantics or document intentional differences.",
			Links: []DocsLink{
				{Label: "ROADMAP.md (protocol lock)", Path: "ROADMAP.md"},
				{Label: "CREDITS.md (acknowledgements)", Path: "docs/CREDITS.md"},
				{Label: "DEVELOPER_GUIDE.md", Path: "docs/DEVELOPER_GUIDE.md"},
				{Label: "CHAIN_PARAMETERS.md", Path: "docs/CHAIN_PARAMETERS.md"},
				{Label: "ARCHITECTURE.md", Path: "docs/ARCHITECTURE.md"},
				{Label: "INTENTIONAL_DIFFERENCES.md", Path: "docs/INTENTIONAL_DIFFERENCES.md"},
			},
		},
		{
			ID:    "rpc",
			Title: "JSON-RPC workflows",
			Body:  "Tutorial: RPC_CONSOLE_TUTORIAL.md (Console tab, curl, PowerShell, LAN addnode). Method list: Features tab (search + live/partial/stub) or GET /api/rpc/cookbook (all methods with curl/CLI). Workflows: chain sync (getblockchaininfo, getblock), mempool (sendrawtransaction, testmempoolaccept, submitpackage), filters (getblockfilter, scanblocks), wallet (sendtoaddress, fundrawtransaction, walletcreatefundedpsbt, PSBT family), mining (getblocktemplate, generatetoaddress), peers (addnode, getpeerinfo). Many responses include dogego_* diagnostic fields - do not treat as Core-stable API.",
			Links: []DocsLink{
				{Label: "RPC_CONSOLE_TUTORIAL.md", Path: "docs/RPC_CONSOLE_TUTORIAL.md"},
				{Label: "RPC.md", Path: "docs/RPC.md"},
			},
		},
	}
	return DocsManifest{
		Title:    "Documentation",
		Subtitle: "How DogeGo works, how to use JSON-RPC, and how to connect external apps. Open any file below; in-document links work in the viewer.",
		Sections: sections,
	}
}
