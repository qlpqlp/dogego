#!/usr/bin/env node
"use strict";

/**
 * Sync DogeGo operator markdown into docs/md/ for the public guide at /guide/.
 * Also writes docs/guide/manifest.json (hub sections + file list).
 */
const fs = require("fs");
const path = require("path");

const siteRoot = path.join(__dirname, "..");
const repoRoot = path.join(siteRoot, "..");
const outMd = path.join(siteRoot, "md");
const manifestPath = path.join(siteRoot, "guide", "manifest.json");

function rmrf(dir) {
  if (!fs.existsSync(dir)) return;
  fs.rmSync(dir, { recursive: true, force: true });
}

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function copyFile(src, dest) {
  ensureDir(path.dirname(dest));
  fs.copyFileSync(src, dest);
}

function walkMd(dir, relBase, out) {
  if (!fs.existsSync(dir)) return;
  for (const name of fs.readdirSync(dir)) {
    const full = path.join(dir, name);
    const st = fs.statSync(full);
    const rel = path.join(relBase, name).split(path.sep).join("/");
    if (st.isDirectory()) {
      walkMd(full, path.join(relBase, name), out);
      continue;
    }
    if (!name.toLowerCase().endsWith(".md")) continue;
    out.push({ src: full, rel });
  }
}

rmrf(outMd);
ensureDir(outMd);
ensureDir(path.dirname(manifestPath));

const files = [];
walkMd(path.join(repoRoot, "DogeGo", "docs"), "docs", files);
walkMd(path.join(repoRoot, "DogeGo", "DIPs"), "DIPs", files);

// Selected extension docs (user-facing guides, not every catalog file)
const extDocs = [
  "extensions/catalog/README.md",
  "extensions/catalog/AUTHORING.md",
  "extensions/catalog/BUILDING.md",
  "extensions/catalog/HELLO_WORLD.md",
  "extensions/catalog/doginals/docs/README.md",
  "extensions/catalog/doginals/docs/USER_GUIDE.md",
  "extensions/catalog/doginals/docs/PROTOCOL.md",
  "extensions/catalog/zkl2/docs/README.md",
  "extensions/catalog/zkl2/docs/USER_GUIDE.md",
  "extensions/catalog/zkl2/docs/PROTOCOL.md",
  "extensions/catalog/radiodoge/docs/USER_GUIDE.md",
  "extensions/catalog/dogeos/docs/USER_GUIDE.md",
  "extensions/catalog/dogeos/docs/EXAMPLES.md",
  "extensions/catalog/bbpow/docs/README.md",
  "extensions/catalog/bbpow/docs/USER_GUIDE.md",
  "extensions/catalog/bbpow/docs/PROTOCOL.md",
  "extensions/catalog/bbpow/docs/PLAIN_ENGLISH.md",
  "extensions/catalog/bbpow/docs/HARD_FORK.md",
];
for (const rel of extDocs) {
  const src = path.join(repoRoot, "DogeGo", ...rel.split("/"));
  if (fs.existsSync(src)) files.push({ src, rel });
}

const roadmap = path.join(repoRoot, "DogeGo", "ROADMAP.md");
if (fs.existsSync(roadmap)) files.push({ src: roadmap, rel: "ROADMAP.md" });

const written = [];
for (const f of files) {
  const dest = path.join(outMd, f.rel);
  copyFile(f.src, dest);
  written.push(f.rel);
}
written.sort();

const sections = [
  {
    id: "start",
    title: "Start here",
    icon: "rocket_launch",
    body: "Pick a path: first run, operator runbook, JSON-RPC, or foundations.",
    links: [
      { label: "Documentation index", path: "docs/DOCUMENTATION.md" },
      { label: "Standalone node quickstart", path: "docs/STANDALONE_NODE_QUICKSTART.md" },
      { label: "Web UI", path: "docs/WEB_UI.md" },
      { label: "Operator config", path: "docs/OPERATOR.md" },
    ],
  },
  {
    id: "foundations",
    title: "Bitcoin white paper",
    icon: "history_edu",
    body: "Satoshi Nakamoto (2008). The peer-to-peer cash design behind Bitcoin and Dogecoin.",
    links: [
      { label: "Bitcoin white paper (full text)", path: "docs/BITCOIN_WHITEPAPER.md" },
      { label: "Chain parameters", path: "docs/CHAIN_PARAMETERS.md" },
      { label: "Architecture", path: "docs/ARCHITECTURE.md" },
    ],
  },
  {
    id: "operate",
    title: "Run a node",
    icon: "dns",
    body: "Mainnet and reboot testnet: IBD, recovery, mining, indexes, dual-run.",
    links: [
      { label: "Core operator runbook", path: "docs/CORE_OPERATOR_RUNBOOK.md" },
      { label: "Mainnet + testnet operational", path: "docs/MAINNET_TESTNET_OPERATIONAL.md" },
      { label: "Security", path: "docs/SECURITY.md" },
      { label: "Beta release notes", path: "docs/BETA_RELEASE.md" },
    ],
  },
  {
    id: "integrate",
    title: "Integrate & RPC",
    icon: "hub",
    body: "JSON-RPC, Console tutorial, wallet, and OpenRPC-style workflows.",
    links: [
      { label: "Integration", path: "docs/INTEGRATION.md" },
      { label: "RPC reference", path: "docs/RPC.md" },
      { label: "RPC console tutorial", path: "docs/RPC_CONSOLE_TUTORIAL.md" },
      { label: "Wallet", path: "docs/WALLET.md" },
    ],
  },
  {
    id: "extensions",
    title: "Extensions",
    icon: "extension",
    body: "Official catalog packages: ZK L2, Doginals, RadioDoge, DogeOS, BBPoW, plus authoring guides.",
    links: [
      { label: "Extensions overview", path: "docs/EXTENSIONS.md" },
      { label: "Catalog README", path: "extensions/catalog/README.md" },
      { label: "ZK L2 user guide", path: "extensions/catalog/zkl2/docs/USER_GUIDE.md" },
      { label: "ZK L2 protocol", path: "extensions/catalog/zkl2/docs/PROTOCOL.md" },
      { label: "Doginals user guide", path: "extensions/catalog/doginals/docs/USER_GUIDE.md" },
      { label: "Doginals protocol", path: "extensions/catalog/doginals/docs/PROTOCOL.md" },
      { label: "RadioDoge user guide", path: "extensions/catalog/radiodoge/docs/USER_GUIDE.md" },
      { label: "DogeOS user guide", path: "extensions/catalog/dogeos/docs/USER_GUIDE.md" },
      { label: "DogeOS examples", path: "extensions/catalog/dogeos/docs/EXAMPLES.md" },
      { label: "BBPoW user guide", path: "extensions/catalog/bbpow/docs/USER_GUIDE.md" },
      { label: "BBPoW protocol", path: "extensions/catalog/bbpow/docs/PROTOCOL.md" },
      { label: "BBPoW plain English", path: "extensions/catalog/bbpow/docs/PLAIN_ENGLISH.md" },
      { label: "Authoring", path: "extensions/catalog/AUTHORING.md" },
      { label: "Building packages", path: "extensions/catalog/BUILDING.md" },
      { label: "Hello world", path: "extensions/catalog/HELLO_WORLD.md" },
    ],
  },
  {
    id: "dips",
    title: "DIPs",
    icon: "account_tree",
    body: "Dogecoin Improvement Proposals tracked in DogeGo.",
    links: [
      { label: "DIPs index", path: "DIPs/README.md" },
      { label: "Intentional differences", path: "docs/INTENTIONAL_DIFFERENCES.md" },
      { label: "Core parity gaps", path: "docs/CORE_PARITY_GAPS.md" },
    ],
  },
  {
    id: "develop",
    title: "Develop DogeGo",
    icon: "code",
    body: "Repo map, protocol lock, contributing, and roadmap.",
    links: [
      { label: "Developer guide", path: "docs/DEVELOPER_GUIDE.md" },
      { label: "Roadmap", path: "ROADMAP.md" },
      { label: "Contributing", path: "docs/CONTRIBUTING.md" },
      { label: "Overview", path: "docs/OVERVIEW.md" },
    ],
  },
];

const manifest = {
  title: "DogeGo documentation",
  subtitle: "Read how DogeGo works without installing. Same guides as the in-app Docs tab, plus Satoshi's Bitcoin white paper.",
  defaultPath: "docs/DOCUMENTATION.md",
  sections,
  files: written,
  generatedAt: new Date().toISOString(),
};

fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");

const manifestJs =
  "(function(g){g.DogeGoGuideManifest=" +
  JSON.stringify(manifest) +
  ";})(window);\n";
fs.writeFileSync(path.join(siteRoot, "guide", "manifest.js"), manifestJs);

const contentMap = {};
for (const rel of written) {
  const full = path.join(outMd, ...rel.split("/"));
  contentMap[rel] = fs.readFileSync(full, "utf8");
}
const contentJs =
  "(function(g){g.DogeGoGuideMarkdown=" +
  JSON.stringify(contentMap) +
  ";})(window);\n";
fs.writeFileSync(path.join(siteRoot, "guide", "content-bundle.js"), contentJs);

console.log("synced " + written.length + " markdown files to docs/md/");
console.log("wrote guide/manifest.json + manifest.js + content-bundle.js (file:// fallback)");
