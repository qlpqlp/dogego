(function () {
  "use strict";

  var reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /* --- Dashboard tab demo --- */
  var tabs = ["overview", "blockstep", "mempool"];
  var tabIndex = 0;
  var navItems = document.querySelectorAll(".demo-nav-item[data-demo-tab]");
  var panels = document.querySelectorAll(".demo-panel");
  var syncFill = document.getElementById("demo-sync-fill");
  var syncPct = document.getElementById("demo-sync-pct");
  var syncDockPct = document.getElementById("demo-sync-dock-pct");
  var tipEl = document.getElementById("demo-tip");
  var mempoolEl = document.getElementById("demo-mempool");
  var peersOut = document.getElementById("demo-peers-out");
  var peersOutKpi = document.getElementById("demo-peers-out-kpi");
  var peersIn = document.getElementById("demo-peers-in");
  var consoleEl = document.getElementById("demo-console-log");
  var bsThumb = document.getElementById("demo-bs-thumb");
  var bsSynced = document.getElementById("demo-bs-synced");
  var bsTime = document.getElementById("demo-bs-time");
  var bsHeight = document.getElementById("demo-bs-height");

  var syncVal = 42.3;
  var tipVal = 4829102;
  var mempoolVal = 87;
  var peerOut = 4;
  var peerIn = 0;
  var bsPos = 78;
  var bsBlock = 1024;

  function switchTab() {
    if (reduced) return;
    tabIndex = (tabIndex + 1) % tabs.length;
    var tab = tabs[tabIndex];
    navItems.forEach(function (item) {
      item.classList.toggle("demo-nav-active", item.getAttribute("data-demo-tab") === tab);
    });
    panels.forEach(function (panel) {
      panel.classList.toggle("demo-panel-active", panel.getAttribute("data-demo-panel") === tab);
    });
    if (explainerIndex !== tab) {
      explainerIndex = tab;
      showExplainer(explainers[tab]);
    }
  }

  function tickMetrics() {
    if (syncVal < 99.2) {
      syncVal += 0.15 + Math.random() * 0.35;
      if (syncVal > 99.2) syncVal = 99.2;
    }
    tipVal += Math.floor(Math.random() * 3);
    if (Math.random() > 0.6) mempoolVal += Math.floor(Math.random() * 5) - 2;
    if (mempoolVal < 40) mempoolVal = 40;
    if (mempoolVal > 220) mempoolVal = 220;
    if (peerOut < 11 && Math.random() > 0.7) peerOut += 1;
    if (peerIn < 3 && syncVal > 70 && Math.random() > 0.85) peerIn += 1;

    if (syncFill) syncFill.style.width = syncVal.toFixed(1) + "%";
    var pctStr = syncVal.toFixed(1);
    if (syncPct) syncPct.textContent = pctStr;
    if (syncDockPct) syncDockPct.textContent = pctStr;
    if (tipEl) tipEl.textContent = tipVal.toLocaleString("en-US");
    if (mempoolEl) mempoolEl.textContent = mempoolVal + " tx";
    if (peersOut) peersOut.textContent = String(peerOut);
    if (peersOutKpi) peersOutKpi.textContent = String(peerOut);
    if (peersIn) peersIn.textContent = String(peerIn);

    if (tabs[tabIndex] === "blockstep" && !reduced) {
      bsPos += Math.random() > 0.45 ? 2.5 : -2.5;
      if (bsPos < 52) bsPos = 52;
      if (bsPos > 90) bsPos = 90;
      bsBlock += Math.floor(Math.random() * 800);
      if (bsThumb) bsThumb.style.left = bsPos.toFixed(1) + "%";
      if (bsSynced) bsSynced.style.width = bsPos.toFixed(1) + "%";
      if (bsHeight) bsHeight.textContent = "Block #" + bsBlock.toLocaleString("en-US");
      if (bsTime) {
        var year = 2013 + Math.floor((bsPos / 100) * 12);
        bsTime.textContent = "Dec " + (6 + Math.floor(bsPos / 15)) + ", " + year;
      }
    }
  }

  /* --- Feature explainer (separate block below dashboard) --- */
  var explainers = {
    overview: {
      icon: "insights",
      title: "Live operator metrics",
      body: "Header tip, sync curve, mempool size, peer counts, and disk breakdown update in real time. Chart.js sparklines and an analytics SQLite sidecar track sync progress, block sizes, and miner distribution. Detail Core's Qt UI never surfaces in one place.",
      tags: ["KPI dashboard", "Analytics DB", "Sync dock"]
    },
    blockstep: {
      icon: "pets",
      title: "BlockStep: search the public ledger",
      body: "Dogecoin is a public ledger. DogeGo indexes the full chain so you can search height, hash, txid, or address from the dashboard. BlockStep turns history into an interactive timeline from December 2013 to tip. Drill into any block or transaction in a fun, visual way.",
      tags: ["Tx index", "Explorer", "BlockStep timeline"]
    },
    mempool: {
      icon: "pending_actions",
      title: "Mempool policy you can see",
      body: "Pause and resume admission, inspect feefilter and package limits, and compare against Core's policy corpus. Every transaction waiting for a block is visible with standardness and RBF status. Operator control without dropping to the CLI.",
      tags: ["BIP125 RBF", "Fee estimate", "Mempool pause"]
    }
  };

  var explainerEl = document.getElementById("demo-explainer");
  var explainerIndex = 0;

  function showExplainer(data) {
    if (!explainerEl || !data) return;
    explainerEl.innerHTML =
      '<div class="explainer-icon"><span class="material-icons-round">' + data.icon + '</span></div>' +
      '<div class="explainer-body">' +
      '<h3>' + data.title + '</h3>' +
      '<p>' + data.body + '</p>' +
      '<div class="explainer-tags">' +
      data.tags.map(function (t) { return '<span>' + t + '</span>'; }).join("") +
      '</div></div>';
    explainerEl.classList.remove("explainer-fade");
    void explainerEl.offsetWidth;
    explainerEl.classList.add("explainer-fade");
  }

  if (explainerEl) showExplainer(explainers.overview);

  /* --- Console log (terminal block, separate from explainer) --- */
  var consoleLines = [
    { t: "info", m: "DogeGo 0.1.0-beta · user-agent /DogeGo:0.1.0-beta/" },
    { t: "dim", m: "$ ./dogego node -network mainnet" },
    { t: "ok", m: "Web UI  http://127.0.0.1:2013/" },
    { t: "ok", m: "RPC     http://127.0.0.1:22557/" },
    { t: "info", m: "P2P mode: both · up to 12 outbound, 16 inbound peers" },
    { t: "ok", m: "CGNAT relay ready (outbound multi-peer, no port forward)" },
    { t: "info", m: "Block storage: per-file layout + optional zstd compression" },
    { t: "ok", m: "PQ commitments enabled (FLC1 tag on wallet sends)" },
    { t: "info", m: "Tx index on · search height, hash, txid, address" },
    { t: "warn", m: "IBD 42.3% · headers-first sync, bodies downloading …" },
    { t: "dim", m: "Reboot testnet: addresses start with T · RelaxedPoW for solo mining" }
  ];

  var consoleIdx = 0;
  var charIdx = 0;
  var currentLine = null;

  function appendConsoleChar() {
    if (!consoleEl || consoleIdx >= consoleLines.length) return;

    var entry = consoleLines[consoleIdx];
    if (!currentLine) {
      currentLine = document.createElement("div");
      currentLine.className = "demo-log-line demo-log-" + entry.t;
      consoleEl.appendChild(currentLine);
      charIdx = 0;
    }

    if (charIdx < entry.m.length) {
      currentLine.textContent += entry.m.charAt(charIdx);
      charIdx += 1;
      consoleEl.scrollTop = consoleEl.scrollHeight;
    } else {
      consoleIdx += 1;
      currentLine = null;
      if (consoleIdx >= consoleLines.length && !reduced) {
        window.setTimeout(function () {
          consoleEl.innerHTML = "";
          consoleIdx = 0;
          charIdx = 0;
          currentLine = null;
        }, 4000);
      }
    }
  }

  /* --- Setup wizard animation --- */
  var wizardSteps = document.querySelectorAll(".wiz-setup-steps li");
  var wizardPanels = document.querySelectorAll(".wiz-demo-panel");
  var wizKicker = document.getElementById("wiz-demo-kicker");
  var wizIndex = 0;

  function advanceWizard() {
    if (reduced || !wizardSteps.length) return;
    if (wizKicker) wizKicker.textContent = "Step " + (wizIndex + 1) + " of 5";
    wizardSteps.forEach(function (s, i) {
      s.classList.toggle("active", i === wizIndex);
      s.classList.toggle("done", i < wizIndex);
    });
    wizardPanels.forEach(function (p, i) {
      p.classList.toggle("wiz-panel-active", i === wizIndex);
    });
    wizIndex = (wizIndex + 1) % wizardSteps.length;
  }

  if (!reduced) {
    window.setInterval(switchTab, 5000);
    window.setInterval(tickMetrics, 900);
    window.setInterval(appendConsoleChar, 22);
    window.setInterval(advanceWizard, 3200);
  }

  tickMetrics();
  advanceWizard();
})();
