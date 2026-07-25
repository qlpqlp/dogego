/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* DogeGo dashboard - modern UI */
(function () {
  const $ = (id) => document.getElementById(id);
  const EMPTY = "...";
  function wait(el, msg, opts) {
    if (!el) return;
    if (window.DogeGoWait) window.DogeGoWait.set(el, msg, opts);
    else el.textContent = msg || "Loading…";
  }
  const LS_SUM = "dogego_show_summary";
  const LS_P2P = "dogego_show_p2p";
  const POLL_MS = 1000;
  const SLOW_POLL_MS = 8000;
  const BRAND_LOGO_MAINNET = "/dogecoin.svg";
  const BRAND_LOGO_TESTNET = "/dogecoin_testnet.svg";
  window.DogeGoLogo = BRAND_LOGO_MAINNET;
  let peerInstancesCache = [];
  let netSwitcherOpen = false;
  const SLOW_POLL_IBD_MS = 12000;
  const BOOT_GRACE_MS = 90000;
  const BOOT_MAX_OVERLAY_MS = 45000;
  const PAGE_LOAD = Date.now();
  const BOOT_STATUS_MESSAGES = [
    "Preparing your dashboard ... the node may still be waking up.",
    "Sniffing the chain… such patience.",
    "Much sync. Very load.",
    "DogeGo is digging for blocks and peers…",
    "One moment. Many treat soon.",
    "Hold the leash ... data inbound.",
    "Warming up RPC and wallet services…",
  ];
  let refreshGen = 0;
  let lastSlowPollAt = 0;
  let apiFailStreak = 0;
  let lastApiSuccessAt = 0;
  let refreshInFlight = false;
  let walletPanelInFlight = false;
  let walletAutoRescanStarted = false;
  let walletHistoryScanDeferred = false;
  let lastRecvMetaText = "";
  let walletAddressBookRows = [];
  let walletAddressBookSig = "";
  let walletAddressBookLoaded = false;
  let walletAddressBookInFlight = false;
  let walletAbFilterTimer = null;
  let walletUnlockCountdownTimer = null;
  const API_FAIL_HARD_THRESHOLD = 15;
  const LOCAL_API_TIMEOUT_MS = 45000;
  const LIVE_API_TIMEOUT_MS = 30000;
  const WALLET_API_TIMEOUT_MS = 45000;
  const WALLET_TX_API_TIMEOUT_MS = 60000;
  const CONNECT_LAG_POLL_DEFER = 32;
  const CONNECT_LAG_HEAVY_DEFER = 64;
  let bootOverlayHidden = false;
  let bootAppReady = false;
  let bootWalletHistoryReady = false;
  let lastAnalyticsLoadAt = 0;
  const ANALYTICS_REFRESH_MS = 15000;
  let bootMsgTimer = null;
  let bootMsgIdx = 0;
  let summaryHydrated = false;
  let analyticsHydrated = false;

  function setUIPending(el, pending) {
    if (!el) return;
    if (pending) {
      el.classList.add("ui-pending");
      el.setAttribute("aria-busy", "true");
      if (!el.querySelector(":scope > .ui-skel-bar") && !el.querySelector(".ui-skel-bar")) {
        const sk = document.createElement("span");
        sk.className = "ui-skel-bar";
        sk.setAttribute("aria-hidden", "true");
        el.insertBefore(sk, el.firstChild);
      }
      return;
    }
    el.classList.remove("ui-pending");
    el.removeAttribute("aria-busy");
    el.querySelectorAll(":scope > .ui-skel-bar").forEach((n) => n.remove());
  }

  function setMetricText(id, text) {
    const el = $(id);
    if (!el) return;
    setUIPending(el, false);
    el.textContent = text;
  }

  function clearOverviewMetricPending() {
    ["tip", "mempool", "ov-tx-processed", "ov-metric-sync-wrap", "ov-peers-wrap"].forEach((id) => {
      const el = $(id);
      if (!el) return;
      setUIPending(el, false);
    });
    const syncPct = $("ov-metric-sync-pct");
    const syncSuf = $("ov-metric-sync-pct-suffix");
    if (syncPct) syncPct.hidden = false;
    if (syncSuf) syncSuf.hidden = false;
    const out = $("ov-conn-out");
    const inn = $("ov-conn-in");
    const slash = $("ov-peers-slash");
    if (out) out.hidden = false;
    if (inn) inn.hidden = false;
    if (slash) slash.hidden = false;
  }

  function setChartPending(wrapId, pending) {
    const wrap = $(wrapId);
    if (!wrap) return;
    wrap.classList.toggle("ui-chart-pending", !!pending);
    if (pending) wrap.setAttribute("aria-busy", "true");
    else wrap.removeAttribute("aria-busy");
  }

  const TABS = ["overview", "send", "receive", "transactions", "blockstep", "explorer", "mempool", "analytics", "features", "docs", "console", "extensions", "settings"];
  const HASH_ALIASES = { debug: "console", lookup: "explorer", mining: "features", help: "docs", guide: "docs", documentation: "docs" };
  const RPC_PRESETS = [
    { label: "getblockchaininfo", method: "getblockchaininfo", params: [] },
    { label: "getnetworkinfo", method: "getnetworkinfo", params: [] },
    { label: "getpeerinfo", method: "getpeerinfo", params: [] },
    { label: "getconnectioncount", method: "getconnectioncount", params: [] },
    { label: "addnode LAN", method: "addnode", params: ["HOST:44556", "add"] },
    { label: "getaddednodeinfo", method: "getaddednodeinfo", params: [true] },
    { label: "help method", method: "help", params: ["getpeerinfo"] },
    { label: "getdifficulty", method: "getdifficulty", params: [] },
    { label: "recover headers", method: "dogego_recoverheaders", params: [] },
    { label: "verifychain", method: "verifychain", params: [3] },
    { label: "getmininginfo", method: "getmininginfo", params: [] },
    { label: "mempool info", method: "getmempoolinfo", params: [] },
    { label: "reindextx", method: "reindextx", params: [false] },
    { label: "reindexblockfilters", method: "reindexblockfilters", params: [] },
    { label: "generatetoaddress", method: "generatetoaddress", params: [1, "WALLET"], wallet: true },
    { label: "createauxblock", method: "createauxblock", params: ["WALLET"], wallet: true },
    { label: "getauxblock", method: "getauxblock", params: [] },
    { label: "getblocktemplate", method: "getblocktemplate", params: [{ rules: ["segwit"] }] },
    { label: "submitauxblock", method: "submitauxblock", params: ["HASH", "AUXPOW_HEX"], wallet: false },
  ];
  let docsCache = null;
  let currentDocPath = "";
  const docsPathHistory = [];

  let chartMempool, chartMempoolPanel, chartMiners, chartSync, chartAnMiners, chartHeaderDt;
  let chartReorgDepth, chartReorgHour, chartReorgMiners;
  if (typeof Chart !== "undefined") {
    Chart.defaults.animation = false;
    Chart.defaults.responsive = true;
    Chart.defaults.resizeDelay = 250;
  }
  let chartDiskSize, chartMempoolSize, chartBlockSize;
  let capabilitiesCache = null;
  let rpcCookbookCache = null;
  let extensionsCatalogCache = [];
  let extNavSubmenuOpen = false;
  let dogegoSavedConfig = {};
  let lastTLSCache = null;
  let settingsRuntime = {};
  let sigMempoolPanel = "";
  let lastSummary = null;
  let lastAnalyticsJson = null;
  let lastAnalyticsPeersCache = null;
  let sigMempool = "";
  let sigMiners = "";
  let sigSync = "";
  let sigAnMiners = "";
  let sigHeaderDt = "";
  let sigReorgDepth = "";
  let sigReorgHour = "";
  let sigReorgMiners = "";
  let sigDisk = "";
  let sigMempoolTimeline = "";
  let sigBlockSize = "";
  let chartOvTip, chartOvSync, chartOvMempool, chartOvPeers;
  let chartOvDashSync, chartOvDashPeers, chartOvDashMempoolTimeline, chartOvDashMempoolDist;
  let sigOvDashSync = "";
  let sigOvDashPeers = "";
  let sigOvDashMempoolTimeline = "";
  const sparkTip = [];
  const sparkSync = [];
  const sparkMempool = [];
  const sparkPeers = [];
  const sparkPeersOut = [];
  const sparkPeersIn = [];

  const CHART_FONT = "'Inter', system-ui, sans-serif";
  const chartColors = {
    accent: "rgba(194, 166, 51, 0.85)",
    accentFill: "rgba(194, 166, 51, 0.15)",
    blue: "rgba(37, 99, 235, 0.8)",
    blueFill: "rgba(37, 99, 235, 0.12)",
    green: "rgba(22, 163, 74, 0.85)",
    greenFill: "rgba(22, 163, 74, 0.12)",
    grid: "rgba(148, 163, 184, 0.25)",
  };

  function pushSpark(arr, v, max) {
    const n = Number(v);
    if (!isFinite(n)) return arr;
    arr.push(n);
    while (arr.length > (max || 24)) arr.shift();
    return arr;
  }

  function rpcOverviewLabel(live) {
    if (!live.rpc_enabled) return "off";
    if (live.rpc_status_display) return live.rpc_status_display;
    if (live.rpc_dispatch_ready) return live.rpc_addr || "ready";
    if (live.rpc_listening) return (live.rpc_addr || "RPC") + " (warming up)";
    return (live.rpc_addr || "RPC") + " (starting)";
  }

  function rpcSummaryLabel(s) {
    if (!s.rpc_enabled && !s.rpc_addr) return { text: "Not enabled", cls: "rpc-off" };
    const text = s.rpc_status_display || s.rpc_addr || "RPC";
    if (s.rpc_dispatch_ready) return { text: text, cls: "" };
    if (s.rpc_listening) return { text: text, cls: "rpc-warmup" };
    return { text: text, cls: "rpc-starting" };
  }

  function modernScales() {
    return {
      x: { display: false, grid: { display: false } },
      y: { display: false, grid: { display: false }, beginAtZero: true },
    };
  }

  function modernChartPlugins() {
    return {
      legend: { display: false },
      tooltip: {
        enabled: true,
        backgroundColor: "#1b1f24",
        titleFont: { family: CHART_FONT },
        bodyFont: { family: CHART_FONT },
        padding: 10,
        cornerRadius: 8,
      },
    };
  }

  function renderSpark(canvas, chartRef, data, color, fill) {
    if (!canvas || typeof Chart === "undefined" || data.length < 2) return destroyChart(chartRef);
    return upsertChart(chartRef, canvas, {
      type: "line",
      data: {
        labels: data.map((_, i) => i),
        datasets: [{
          data,
          borderColor: color,
          backgroundColor: fill,
          fill: true,
          tension: 0.35,
          borderWidth: 2,
          pointRadius: 0,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        plugins: modernChartPlugins(),
        scales: modernScales(),
      },
    });
  }

  function fmtHashrate(hs) {
    if (hs == null || !isFinite(hs)) return "...";
    if (hs >= 1e15) return (hs / 1e15).toFixed(2) + " PH/s";
    if (hs >= 1e12) return (hs / 1e12).toFixed(2) + " TH/s";
    if (hs >= 1e9) return (hs / 1e9).toFixed(2) + " GH/s";
    if (hs >= 1e6) return (hs / 1e6).toFixed(2) + " MH/s";
    return hs.toFixed(0) + " H/s";
  }

  function fmtDoge(koinu) {
    if (koinu == null) return "...";
    return (koinu / 1e8).toLocaleString(undefined, { maximumFractionDigits: 2 }) + " DOGE";
  }

  function fmtBytes(n) {
    if (n == null || !isFinite(n)) return "...";
    let x = Number(n);
    const u = ["B", "KB", "MB", "GB", "TB"];
    let i = 0;
    while (x >= 1024 && i < u.length - 1) {
      x /= 1024;
      i++;
    }
    return x.toFixed(i === 0 ? 0 : 1) + " " + u[i];
  }

  function fmtDate(ts) {
    if (!ts) return "...";
    const d = new Date(Number(ts) * 1000);
    if (Number.isNaN(d.getTime())) return "...";
    return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
  }

  function pickNum() {
    for (let i = 0; i < arguments.length; i++) {
      const n = Number(arguments[i]);
      if (isFinite(n)) return n;
    }
    return null;
  }

  function fitStatEl(el) {
    if (!el) return;
    el.style.removeProperty("font-size");
    const minPx = el.classList.contains("stat-compact") ? 10 : 11;
    const maxPx = el.classList.contains("stat-compact") ? 18 : 22;
    let px = maxPx;
    el.style.fontSize = px + "px";
    while (px > minPx && el.scrollWidth > el.clientWidth + 1) {
      px -= 1;
      el.style.fontSize = px + "px";
    }
  }
  window.DogeGoFitStat = fitStatEl;

  function setCompactStat(el, n, opts) {
    if (window.DogeGoFormat && window.DogeGoFormat.setCompactStat) {
      window.DogeGoFormat.setCompactStat(el, n, opts);
      return;
    }
    if (!el) return;
    setUIPending(el, false);
    const x = Number(n);
    if (!isFinite(x)) {
      el.textContent = (opts && opts.fallback) || "…";
      return;
    }
    el.textContent = x.toLocaleString(undefined, (opts && opts.maximumFractionDigits != null)
      ? { maximumFractionDigits: opts.maximumFractionDigits }
      : undefined) + ((opts && opts.suffix) || "");
    fitStatEl(el);
  }

  function fitAnKpiStats() {
    document.querySelectorAll("#an-kpi .stat-fit").forEach(fitStatEl);
  }

  function filterTimelineSamples(samples, hours) {
    if (!samples || !samples.length) return [];
    const h = Number(hours);
    if (!h || h <= 0) return samples;
    const cutoff = Math.floor(Date.now() / 1000) - h * 3600;
    return samples.filter((s) => Number(s.recorded_unix) >= cutoff);
  }

  function timelineLabels(samples) {
    return samples.map((s) => {
      const d = new Date(s.recorded_unix * 1000);
      return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    });
  }

  function renderRecvQR(address) {
    const wrap = $("recv-qr-wrap");
    const canvas = $("recv-qr");
    if (!wrap || !canvas || !window.QRCode) return;
    const a = String(address || "").trim();
    if (!a || a === EMPTY || a.startsWith("Wallet")) {
      wrap.hidden = true;
      return;
    }
    wrap.hidden = false;
    window.QRCode.toCanvas(canvas, "dogecoin:" + a, {
      width: 200,
      margin: 1,
      color: { dark: "#1a1d21", light: "#ffffff" },
    }).catch(() => {
      wrap.hidden = true;
    });
  }

  let lastWalletSnap = null;
  let lastWalletTxs = [];
  let lastTxTypeFilter = "all";
  const WALLET_TX_PAGE_SIZE = 40;
  const WALLET_TX_CACHE_KEY = "dogego_wallet_tx_cache";
  let walletTxCacheNetwork = "";
  function walletTxCacheStorageKey(network) {
    const net = String(network || walletTxCacheNetwork || "default").trim().toLowerCase() || "default";
    return WALLET_TX_CACHE_KEY + ":" + net;
  }

  const LS_SUMMARY_SNAP = "dogego_ui_summary_snap";
  function summarySnapStorageKey(network) {
    const net = String(network || "").trim().toLowerCase() || "default";
    return LS_SUMMARY_SNAP + ":" + net;
  }
  function persistSummarySnap(s) {
    if (!s || s.tip_height == null) return;
    try {
      const net = String(s.network || s.chain || "default").toLowerCase();
      const slim = {
        tip_height: s.tip_height,
        header_count: s.header_count,
        contiguous_raw_height: s.contiguous_raw_height,
        chain_active_height: s.chain_active_height,
        mempool_txs: s.mempool_txs,
        network: s.network || s.chain,
        chain: s.chain,
        node_mode: s.node_mode,
        dogego_version: s.dogego_version,
        client_version: s.client_version,
        connections_out: s.connections_out,
        connections_in: s.connections_in,
        blocks_behind_headers: s.blocks_behind_headers,
        initialblockdownload: s.initialblockdownload,
        ibd_active: s.ibd_active,
        verification_progress: s.verification_progress,
        dogego_body_verification_progress: s.dogego_body_verification_progress,
        transactions_processed: s.transactions_processed,
        from_disk_snapshot: true,
        summary_stale: true,
        sync_status_line: "Showing last known data · refreshing…",
        saved_at: Date.now()
      };
      localStorage.setItem(summarySnapStorageKey(net), JSON.stringify(slim));
      localStorage.setItem(LS_SUMMARY_SNAP + ":last", net);
    } catch (_) { /* */ }
  }
  function hydrateSummarySnapFromLocalStorage() {
    try {
      let net = localStorage.getItem(LS_SUMMARY_SNAP + ":last") || "";
      let raw = net ? localStorage.getItem(summarySnapStorageKey(net)) : null;
      if (!raw) {
        for (let i = 0; i < localStorage.length; i++) {
          const k = localStorage.key(i);
          if (k && k.indexOf(LS_SUMMARY_SNAP + ":") === 0 && k !== LS_SUMMARY_SNAP + ":last") {
            raw = localStorage.getItem(k);
            break;
          }
        }
      }
      if (!raw) return false;
      const s = JSON.parse(raw);
      if (!s || s.tip_height == null) return false;
      lastSummary = s;
      applySyncProgress(s);
      if (window.DogeGoSyncDock && window.DogeGoSyncDock.update) {
        window.DogeGoSyncDock.update(s);
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  function renderWalletTxHistoryItems(items, total) {
    const el = $("tx-list");
    if (!el || !items.length) return false;
    const pageItems = dedupeWalletTxItems(items);
    walletTxHistory.loaded = pageItems.slice();
    walletTxHistory.offset = pageItems.length;
    walletTxHistory.total = Number(total) || pageItems.length;
    walletTxHistory.hasMore = walletTxHistory.offset < walletTxHistory.total;
    lastWalletTxs = walletTxHistory.loaded.slice();
    let html = "";
    pageItems.forEach((tx, i) => {
      html += walletTxRowHtml(tx, i);
    });
    el.className = "wallet-tx-list wallet-tx-list-scroll";
    el.innerHTML =
      '<div class="wallet-tx-feed" id="wallet-tx-feed">' + html + "</div>" +
      '<div class="wallet-tx-scroll-footer" id="wallet-tx-scroll-footer" hidden>' +
      '<div class="wallet-tx-progress-track"><div class="wallet-tx-progress-fill"></div></div>' +
      '<span class="wallet-tx-progress-count">0 / 0</span>' +
      '<span class="wallet-tx-progress-hint">Scroll for more</span>' +
      "</div>" +
      '<div class="wallet-tx-load-sentinel" id="wallet-tx-load-sentinel" aria-hidden="true">' +
      '<span class="wallet-tx-spinner" aria-hidden="true"></span>' +
      "</div>" +
      '<div class="wallet-tx-load-end" id="wallet-tx-load-end" hidden>End of history</div>';
    bindWalletTxRows($("wallet-tx-feed"), walletTxHistory.loaded, true);
    updateWalletTxHistoryMeta();
    updateWalletTxScrollFooter();
    ensureWalletTxScrollObserver();
    return true;
  }

  function restoreWalletTxHistoryFromCache(network, cached) {
    if (walletTxHistory.loaded.length) return true;
    try {
      const data = cached || JSON.parse(localStorage.getItem(walletTxCacheStorageKey(network)) || "null");
      if (!data || !Array.isArray(data.items) || !data.items.length) return false;
      if (renderWalletTxHistoryItems(data.items, data.total)) {
        bootWalletHistoryReady = true;
        maybeHideBootOverlay(lastSummary, lastWalletSnap);
        return true;
      }
    } catch (_) {}
    return false;
  }

  function restoreWalletTxHistoryFromAnyCache() {
    let best = null;
    let bestNet = "";
    for (const net of ["mainnet", "testnet", "reboottestnet", "default"]) {
      try {
        const raw = localStorage.getItem(walletTxCacheStorageKey(net));
        if (!raw) continue;
        const data = JSON.parse(raw);
        if (!data || !Array.isArray(data.items) || !data.items.length) continue;
        if (!best || Number(data.saved_at || 0) > Number(best.saved_at || 0)) {
          best = data;
          bestNet = net;
        }
      } catch (_) {}
    }
    if (!best) return false;
    walletTxCacheNetwork = bestNet;
    return restoreWalletTxHistoryFromCache(bestNet, best);
  }

  function persistWalletTxHistoryCache(network) {
    if (!walletTxHistory.loaded.length) return;
    try {
      localStorage.setItem(walletTxCacheStorageKey(network), JSON.stringify({
        items: walletTxHistory.loaded.slice(0, WALLET_TX_PAGE_SIZE * 4),
        total: walletTxHistory.total,
        saved_at: Date.now(),
      }));
    } catch (_) {}
  }

  let walletTxHistory = {
    total: 0,
    loaded: [],
    offset: 0,
    loading: false,
    hasMore: false,
    filter: "",
    typeFilter: "all",
    observer: null,
    scrollRoot: null,
    loadGen: 0,
  };
  let txHistoryFilterTimer = null;
  let lastSendUtxos = [];
  let sendUtxosLoadedAt = 0;
  let pendingSendRetry = null;
  const SEND_QUEUE_KEY = "dogego_send_queue";
  let sendQueueBusy = false;

  function loadSendQueue() {
    try {
      const raw = localStorage.getItem(SEND_QUEUE_KEY);
      const arr = raw ? JSON.parse(raw) : [];
      return Array.isArray(arr) ? arr : [];
    } catch (_) {
      return [];
    }
  }

  function saveSendQueue(list) {
    try {
      localStorage.setItem(SEND_QUEUE_KEY, JSON.stringify(list));
    } catch (_) {}
  }

  function walletSendReady() {
    const wal = lastWalletSnap || {};
    const s = lastSummary || {};
    if (wal.enabled === false || s.wallet_enabled === false) return false;
    if (shouldDeferHeavyWalletAPI(s)) return false;
    return !!(wal.send_ready || s.wallet_rpc_ready || wal.address);
  }

  function isRetryableSendError(e, body, httpStatus) {
    if (httpStatus === 503 || httpStatus === 502) return true;
    if (body && body.retryable) return true;
    const msg = ((e && friendlyAPIError(e)) || (body && body.error) || "").toLowerCase();
    return msg.includes("not available") || msg.includes("still starting") ||
      msg.includes("connection failed") || msg.includes("timed out") ||
      msg.includes("warming up") || msg.includes("failed to fetch") ||
      msg.includes("network");
  }

  function enqueueSend(payload) {
    const item = { id: "q-" + Date.now(), payload: payload, created_at: Date.now(), attempts: 0 };
    const q = loadSendQueue();
    q.push(item);
    saveSendQueue(q);
    return item.id;
  }

  async function processSendQueue() {
    if (sendQueueBusy || !walletSendReady()) return;
    let q = loadSendQueue();
    if (!q.length) return;
    sendQueueBusy = true;
    const item = q[0];
    try {
      const r = await fetchAPI("/api/wallet/send", 120000, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(item.payload),
      });
      const body = await r.json().catch(() => ({}));
      if (r.ok && body.txid) {
        q = q.slice(1);
        saveSendQueue(q);
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackSend) {
          window.DogeGoTxFlight.trackSend({
            txid: body.txid,
            hex: body.hex,
            amount: item.payload.amount,
            address: item.payload.address,
            status: body.status || "broadcasting",
            broadcast_error: body.broadcast_error,
          });
        }
        patchWalletTxFromSendResponse(body);
        refresh();
        return;
      }
      if (!isRetryableSendError(null, body, r.status)) {
        q = q.slice(1);
        saveSendQueue(q);
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackFailure) {
          window.DogeGoTxFlight.trackFailure(item.id, {
            amount: item.payload.amount,
            address: item.payload.address,
            error: formatSendError(body, item.payload.amount),
          });
        }
        return;
      }
      item.attempts = (item.attempts || 0) + 1;
      q[0] = item;
      saveSendQueue(q);
      if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackQueued) {
        window.DogeGoTxFlight.trackQueued(item.id, {
          amount: item.payload.amount,
          address: item.payload.address,
        });
      }
    } catch (e) {
      if (!isRetryableSendError(e, null, 0)) {
        q = q.slice(1);
        saveSendQueue(q);
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackFailure) {
          window.DogeGoTxFlight.trackFailure(item.id, {
            amount: item.payload.amount,
            address: item.payload.address,
            error: friendlyAPIError(e),
          });
        }
      } else if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackQueued) {
        window.DogeGoTxFlight.trackQueued(item.id, {
          amount: item.payload.amount,
          address: item.payload.address,
        });
      }
    } finally {
      sendQueueBusy = false;
    }
  }

  setInterval(() => { void processSendQueue(); }, 8000);

  function formatDOGE(n, digits) {
    const x = Number(n);
    if (!isFinite(x)) return "0.00";
    return x.toLocaleString(undefined, { minimumFractionDigits: digits != null ? digits : 4, maximumFractionDigits: 8 });
  }

  function shortTxid(txid) {
    const t = String(txid || "");
    if (t.length <= 16) return t;
    return t.slice(0, 8) + "…" + t.slice(-8);
  }

  function currentSendFeeRate() {
    const el = $("send-fee-rate");
    const custom = el && el.value !== "" ? parseFloat(el.value) : NaN;
    if (isFinite(custom) && custom > 0) return custom;
    if (lastWalletSnap && lastWalletSnap.fee_per_kb > 0) return lastWalletSnap.fee_per_kb;
    return 0.01;
  }

  function estimateSendFeeDOGE(feePerKb) {
    const rate = feePerKb > 0 ? feePerKb : 0.01;
    return Math.max(rate * 0.25, 0.0001);
  }

  function updateSendFeeEstimate() {
    const est = $("send-fee-est");
    if (!est) return;
    const rate = currentSendFeeRate();
    const fee = estimateSendFeeDOGE(rate);
    est.textContent = "~" + formatDOGE(fee, 4) + " DOGE (" + formatDOGE(rate, 6) + "/kB)";
  }

  async function copyClipboard(text, hintEl) {
    if (!text) return false;
    try {
      await navigator.clipboard.writeText(String(text).trim());
      if (hintEl) {
        const prev = hintEl.textContent;
        hintEl.textContent = "Copied!";
        hintEl.classList.add("copied");
        setTimeout(() => {
          hintEl.textContent = prev;
          hintEl.classList.remove("copied");
        }, 1600);
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  function renderSendBalance(wal) {
    const strip = $("send-balance-strip");
    const amtEl = $("send-balance-amt");
    const chips = $("send-balance-chips");
    if (!strip || !amtEl) return;
    const show = !!(wal && wal.enabled);
    strip.hidden = !show;
    if (!show) return;
    if (typeof wal.balance === "number") {
      amtEl.textContent = formatDOGE(wal.balance, 4);
    } else if (wal.connect_lag > 0) {
      amtEl.textContent = "…";
    } else {
      amtEl.textContent = "…";
    }
    if (!chips) return;
    const items = [];
    const pending = Number(wal.unconfirmed_balance) || 0;
    const immature = Number(wal.immature_balance) || 0;
    const utxos = wal.utxo_count;
    if (Math.abs(pending) > 1e-12) {
      items.push({ icon: "hourglass_top", label: "Pending", value: formatDOGE(pending, 4) + " DOGE", tone: "warn" });
    }
    if (immature > 1e-12) {
      items.push({ icon: "construction", label: "Mining (maturing)", value: formatDOGE(immature, 4) + " DOGE", tone: "mining" });
    } else if (wal.connect_lag > 0 && typeof wal.balance !== "number") {
      items.push({ icon: "sync", label: "Sync", value: wal.connect_lag + " block(s) connecting", tone: "muted" });
    }
    if (utxos != null && utxos >= 0) {
      items.push({ icon: "layers", label: "UTXOs", value: String(utxos), tone: "muted" });
    }
    if (!items.length) {
      chips.innerHTML = "";
      chips.hidden = true;
      return;
    }
    chips.hidden = false;
    chips.innerHTML = items.map((c) =>
      '<div class="send-balance-chip send-balance-chip-' + escHtml(c.tone) + '">' +
      '<span class="material-icons-round" aria-hidden="true">' + escHtml(c.icon) + "</span>" +
      "<span><strong>" + escHtml(c.label) + "</strong> " + escHtml(c.value) + "</span></div>"
    ).join("");
  }

  const WALLET_TX_KIND_META = {
    sent: { label: "Sent", icon: "north_east", cls: "out" },
    sent_pq: { label: "Quantum send", icon: "verified_user", cls: "pq" },
    received: { label: "Received", icon: "south_west", cls: "in" },
    received_pq: { label: "Quantum receive", icon: "verified_user", cls: "pq" },
    mining: { label: "Mining reward", icon: "precision_manufacturing", cls: "mining" },
    mining_immature: { label: "Mining (maturing)", icon: "construction", cls: "mining" },
    abandoned: { label: "Abandoned", icon: "block", cls: "abandoned" },
    unknown: { label: "Transaction", icon: "receipt_long", cls: "neutral" },
  };

  function walletTxKindMeta(tx) {
    const kind = String(tx.tx_kind || tx.category || "unknown").toLowerCase();
    if (kind === "send") return WALLET_TX_KIND_META.sent;
    if (kind === "receive") return WALLET_TX_KIND_META.received;
    return WALLET_TX_KIND_META[kind] || WALLET_TX_KIND_META.unknown;
  }

  function walletTxConfBadge(tx) {
    const conf = Number(tx.confirmations) || 0;
    if (tx.abandoned) {
      return '<span class="wallet-tx-conf abandoned" title="Abandoned">-</span>';
    }
    if (conf < 1) {
      return '<span class="wallet-tx-conf pending" title="Pending">·</span>';
    }
    return '<span class="wallet-tx-conf confirmed" title="' + conf + ' confirmations">' + conf + "</span>";
  }

  function walletTxTypeChip(tx) {
    const meta = walletTxKindMeta(tx);
    let label = meta.label;
    if (tx.pq_tag) label += " · " + tx.pq_tag;
    else if (tx.tx_kind === "sent_pq" && lastWalletSnap && lastWalletSnap.pq_commitments_enabled) {
      label = "Quantum send";
    }
    if (tx["bip125-replaceable"] === "yes") {
      label += " · RBF";
    }
    return '<span class="wallet-tx-type wallet-tx-type-' + meta.cls + '">' +
      '<span class="material-icons-round" aria-hidden="true">' + meta.icon + "</span>" +
      escHtml(label) + "</span>";
  }

  let walletTxSheetIndex = -1;
  let walletTxSheetRows = [];

  function closeWalletTxSheet() {
    const sheet = $("wallet-tx-sheet");
    if (sheet) sheet.hidden = true;
    walletTxSheetIndex = -1;
  }

  function openWalletTxSheet(idx, rows) {
    const sheet = $("wallet-tx-sheet");
    const body = $("wallet-tx-sheet-body");
    const title = $("wallet-tx-sheet-title");
    if (!sheet || !body || !rows || idx < 0 || idx >= rows.length) return;
    const tx = rows[idx];
    walletTxSheetIndex = idx;
    walletTxSheetRows = rows;
    const meta = walletTxKindMeta(tx);
    if (title) title.textContent = meta.label;
    const conf = Number(tx.confirmations) || 0;
    const pending = conf < 1 && !tx.abandoned;
    const dt = tx.time ? new Date(Number(tx.time) * 1000) : null;
    const dateStr = dt ? dt.toLocaleString() : "-";
    const amt = Number(tx.amount) || 0;
    const isIn = amt > 0 || ["receive", "mining", "mining_immature", "generate", "immature"].indexOf(String(tx.tx_kind || tx.category)) >= 0;
    const fee = tx.fee != null && Number(tx.fee) > 0 ? formatDOGE(tx.fee, 4) + " DOGE" : "";
    let html = '<div class="wallet-tx-sheet-hero wallet-tx-sheet-hero-' + (isIn ? "in" : "out") + '">';
    html += '<div class="wallet-tx-sheet-amt">' + (isIn ? "+" : "") + escHtml(formatDOGE(Math.abs(amt), 4)) + " <span>DOGE</span></div>";
    html += '<div class="wallet-tx-sheet-chips">' + walletTxTypeChip(tx) + walletTxConfBadge(tx) + "</div>";
    html += "</div>";
    if (tx.address || tx.txid) {
      html += '<div class="wallet-tx-sheet-ids">';
      if (tx.address) {
        html +=
          '<div class="bs-copy-row bs-copy-row-hero wallet-tx-sheet-id-row">' +
          '<div class="bs-copy-row-value"><span class="mono" id="wallet-tx-sheet-addr" title="' + escHtml(tx.address) + '">' + escHtml(tx.address) + "</span></div>" +
          '<button type="button" class="btn btn-ghost btn-sm wallet-tx-copy" data-copy-target="wallet-tx-sheet-addr" aria-label="Copy address"><span class="material-icons-round">content_copy</span> Copy</button>' +
          "</div>";
      }
      if (tx.txid) {
        html +=
          '<div class="bs-copy-row bs-copy-row-hero wallet-tx-sheet-id-row">' +
          '<div class="bs-copy-row-value"><span class="mono bs-txid-full" id="wallet-tx-sheet-txid" title="' + escHtml(tx.txid) + '">' + escHtml(tx.txid) + "</span></div>" +
          '<button type="button" class="btn btn-ghost btn-sm wallet-tx-copy" data-copy-target="wallet-tx-sheet-txid" aria-label="Copy txid"><span class="material-icons-round">content_copy</span> Copy</button>' +
          "</div>";
      }
      html += "</div>";
    }
    html += '<dl class="wallet-tx-sheet-dl">';
    html += "<dt>When</dt><dd>" + escHtml(dateStr) + "</dd>";
    if (fee) html += "<dt>Fee</dt><dd>" + escHtml(fee) + "</dd>";
    if (tx.blockheight != null && tx.blockheight >= 0) {
      html += "<dt>Block</dt><dd>#" + escHtml(tx.blockheight) + "</dd>";
    }
    if (tx.label) html += "<dt>Label</dt><dd>" + escHtml(tx.label) + "</dd>";
    html += "</dl>";
    html += '<div class="wallet-tx-sheet-actions">';
    html += '<button type="button" class="btn btn-ghost btn-sm" id="wallet-tx-sheet-copy-json"><span class="material-icons-round" aria-hidden="true">data_object</span> Copy JSON</button>';
    if (tx.txid) {
      html += '<button type="button" class="btn btn-primary btn-sm" id="wallet-tx-sheet-blockstep"><span class="material-icons-round" aria-hidden="true">travel_explore</span> BlockStep</button>';
    }
    if (tx.blockheight != null && tx.blockheight >= 0) {
      html += '<button type="button" class="btn btn-ghost btn-sm" id="wallet-tx-sheet-block"><span class="material-icons-round" aria-hidden="true">view_module</span> Block #' + escHtml(tx.blockheight) + "</button>";
    }
    if (tx.address) {
      html += '<button type="button" class="btn btn-ghost btn-sm" id="wallet-tx-sheet-addr-bs"><span class="material-icons-round" aria-hidden="true">person_pin</span> Address in BlockStep</button>';
    }
    html += "</div>";
    if (pending) {
      html += '<p class="field-hint wallet-tx-sheet-hint">Still in mempool - confirmations will update after inclusion in a block.</p>';
    }
    body.innerHTML = html;
    body.querySelectorAll(".wallet-tx-copy").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const id = btn.getAttribute("data-copy-target");
        const el = id && $(id);
        if (el) copyClipboard(el.textContent, btn);
      });
    });
    const bsBtn = $("wallet-tx-sheet-blockstep");
    if (bsBtn && tx.txid) bsBtn.addEventListener("click", () => { closeWalletTxSheet(); goBlockStepTx(tx.txid); });
    const blkBtn = $("wallet-tx-sheet-block");
    if (blkBtn && tx.blockheight != null) blkBtn.addEventListener("click", () => { closeWalletTxSheet(); goBlockStepBlock(tx.blockheight); });
    const addrBtn = $("wallet-tx-sheet-addr-bs");
    if (addrBtn && tx.address) addrBtn.addEventListener("click", () => { closeWalletTxSheet(); goBlockStepAddress(tx.address); });
    const jsonBtn = $("wallet-tx-sheet-copy-json");
    if (jsonBtn) jsonBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      copyClipboard(JSON.stringify(tx, null, 2), jsonBtn);
    });
    sheet.hidden = false;
  }

  function bindWalletTxRows(el, rows, onlyNew) {
    if (!el) return;
    const nodes = onlyNew
      ? el.querySelectorAll(".wallet-tx-row:not([data-tx-bound])")
      : el.querySelectorAll(".wallet-tx-row");
    nodes.forEach((row) => {
      row.setAttribute("data-tx-bound", "1");
      const open = () => {
        const idx = parseInt(row.getAttribute("data-tx-idx"), 10);
        if (isFinite(idx)) openWalletTxSheet(idx, rows);
      };
      row.addEventListener("click", open);
      row.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          open();
        }
      });
      row.querySelectorAll(".wallet-tx-row-copy").forEach((btn) => {
        btn.addEventListener("click", (e) => {
          e.stopPropagation();
          const what = btn.getAttribute("data-copy");
          const idx = parseInt(row.getAttribute("data-tx-idx"), 10);
          const tx = rows[idx];
          if (!tx) return;
          if (what === "txid" && tx.txid) copyClipboard(tx.txid, btn);
          else if (what === "address" && tx.address) copyClipboard(tx.address, btn);
        });
      });
    });
  }

  function walletTxMatchesTypeFilter(tx, typeFilter) {
    const f = String(typeFilter || "all").toLowerCase();
    if (f === "all") return true;
    const kind = String(tx.tx_kind || tx.category || "").toLowerCase();
    if (f === "sent") return kind === "sent" || kind === "send" || kind === "sent_pq";
    if (f === "received") return kind === "received" || kind === "receive" || kind === "received_pq";
    if (f === "mining") return kind === "mining" || kind === "mining_immature" || kind === "generate" || kind === "immature";
    if (f === "quantum") return kind === "sent_pq" || kind === "received_pq" || !!tx.pq_tag;
    return true;
  }

  function activeTxTypeFilter() {
    const active = document.querySelector(".wallet-tx-type-filter.active");
    return (active && active.getAttribute("data-tx-type")) || lastTxTypeFilter || "all";
  }

  function filteredWalletTxRows(txs, filter, typeFilter) {
    const q = String(filter || "").trim().toLowerCase();
    const tf = String(typeFilter != null ? typeFilter : lastTxTypeFilter || "all").toLowerCase();
    let rows = Array.isArray(txs) ? txs.slice() : [];
    if (tf !== "all") {
      rows = rows.filter((tx) => walletTxMatchesTypeFilter(tx, tf));
    }
    if (q) {
      rows = rows.filter((tx) => {
        const blob = [tx.txid, tx.address, tx.label, tx.category, tx.tx_kind, tx.pq_tag, tx.amount].join(" ").toLowerCase();
        return blob.indexOf(q) >= 0;
      });
    }
    rows.sort((a, b) => (Number(b.time) || 0) - (Number(a.time) || 0));
    return rows;
  }

  function walletTxCSVCell(v) {
    const s = v == null ? "" : String(v);
    if (/[",\n\r]/.test(s)) return '"' + s.replace(/"/g, '""') + '"';
    return s;
  }

  function walletTxRowsToCSV(rows) {
    const header = [
      "time_unix", "time_iso", "txid", "amount_doge", "fee_doge", "confirmations",
      "category", "tx_kind", "pq_tag", "address", "label", "blockheight", "blockhash",
      "abandoned", "bip125_replaceable", "trusted", "iswatchonly",
    ].join(",");
    const lines = rows.map((tx) => {
      const tUnix = Number(tx.time) || 0;
      const iso = tUnix > 0 ? new Date(tUnix * 1000).toISOString() : "";
      return [
        tUnix, iso, tx.txid, tx.amount, tx.fee != null ? tx.fee : "",
        tx.confirmations != null ? tx.confirmations : "", tx.category, tx.tx_kind, tx.pq_tag,
        tx.address, tx.label, tx.blockheight != null ? tx.blockheight : "", tx.blockhash,
        tx.abandoned ? "true" : "", tx["bip125-replaceable"], tx.trusted ? "true" : "",
        tx.iswatchonly ? "true" : "",
      ].map(walletTxCSVCell).join(",");
    });
    return header + "\n" + lines.join("\n") + (lines.length ? "\n" : "");
  }

  function walletTxRowHtml(tx, idx) {
    const amt = Number(tx.amount) || 0;
    const isIn = amt > 0 || ["receive", "mining", "mining_immature", "received"].indexOf(String(tx.tx_kind || "")) >= 0;
    const dt = tx.time ? new Date(Number(tx.time) * 1000) : null;
    const dateStr = dt ? dt.toLocaleString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }) : "-";
    const fee = tx.fee != null && Number(tx.fee) > 0 ? " · fee " + formatDOGE(tx.fee, 4) : "";
    const amtStr = (isIn ? "+" : "") + formatDOGE(Math.abs(amt), 4) + fee;
    const addr = tx.address ? shortTxid(tx.address) : "";
    const txidLine = tx.txid
      ? '<span class="wallet-tx-row-txid mono" title="' + escHtml(tx.txid) + '">' + escHtml(tx.txid) + "</span>"
      : "";
    return (
      '<div class="wallet-tx-row" role="button" tabindex="0" data-tx-idx="' + idx + '" title="View details">' +
      '<div class="wallet-tx-row-main">' +
      '<div class="wallet-tx-row-top">' + walletTxTypeChip(tx) + walletTxConfBadge(tx) + "</div>" +
      '<div class="wallet-tx-row-amt wallet-tx-amt ' + (isIn ? "in" : "out") + '">' + escHtml(amtStr) + " DOGE</div>" +
      '<div class="wallet-tx-row-sub">' +
      '<span class="wallet-tx-row-date">' + escHtml(dateStr) + "</span>" +
      (addr ? '<span class="wallet-tx-row-addr mono" title="' + escHtml(tx.address) + '">' + escHtml(addr) + "</span>" : "") +
      "</div>" +
      txidLine +
      "</div>" +
      '<div class="wallet-tx-row-side">' +
      (tx.txid ? '<button type="button" class="wallet-tx-row-copy" data-copy="txid" title="Copy txid" aria-label="Copy txid"><span class="material-icons-round">content_copy</span></button>' : "") +
      (tx.address ? '<button type="button" class="wallet-tx-row-copy" data-copy="address" title="Copy address" aria-label="Copy address"><span class="material-icons-round">person</span></button>' : "") +
      '<span class="material-icons-round wallet-tx-row-chevron" aria-hidden="true">chevron_right</span>' +
      "</div></div>"
    );
  }

  function updateWalletTxHistoryMeta() {
    const countEl = $("tx-history-count");
    const countLabel = $("tx-history-count-label");
    const clearBtn = $("tx-history-clear");
    const exportBtn = $("tx-history-export");
    const q = walletTxHistory.filter.trim();
    const tf = walletTxHistory.typeFilter;
    const total = walletTxHistory.total;
    const shown = walletTxHistory.loaded.length;
    if (clearBtn) clearBtn.hidden = !q;
    if (exportBtn) {
      exportBtn.disabled = total === 0;
      exportBtn.title = q || tf !== "all"
        ? "Export filtered wallet history as CSV"
        : "Export wallet history as CSV";
    }
    if (countEl) countEl.textContent = String(total);
    if (countLabel) {
      if (!total) countLabel.textContent = "transactions";
      else if (shown < total) countLabel.textContent = "loaded · " + total + " total";
      else if (q && tf !== "all") countLabel.textContent = "matching filter";
      else if (q) countLabel.textContent = "matching search";
      else if (tf !== "all") countLabel.textContent = "in filter";
      else countLabel.textContent = total === 1 ? "transaction" : "transactions";
    }
    const loadActions = $("wallet-tx-load-actions");
    if (loadActions) loadActions.hidden = true;
    updateWalletTxScrollFooter();
  }

  function updateWalletTxScrollFooter() {
    const footer = $("wallet-tx-scroll-footer");
    const sentinel = $("wallet-tx-load-sentinel");
    const end = $("wallet-tx-load-end");
    const total = walletTxHistory.total;
    const shown = walletTxHistory.loaded.length;
    if (footer) {
      if (!total || shown >= total) {
        footer.hidden = true;
      } else {
        footer.hidden = false;
        const pct = total > 0 ? Math.min(100, Math.round((shown / total) * 100)) : 0;
        const fill = footer.querySelector(".wallet-tx-progress-fill");
        const count = footer.querySelector(".wallet-tx-progress-count");
        if (fill) fill.style.width = pct + "%";
        if (count) count.textContent = shown.toLocaleString() + " / " + total.toLocaleString();
      }
    }
    if (sentinel) {
      const more = walletTxHistory.hasMore && shown < total;
      sentinel.hidden = !more;
      sentinel.classList.toggle("is-loading", !!walletTxHistory.loading);
      sentinel.setAttribute("aria-busy", walletTxHistory.loading ? "true" : "false");
    }
    if (end) end.hidden = !(total > 0 && shown >= total);
  }

  function setWalletTxLoadState(kind) {
    const sentinel = $("wallet-tx-load-sentinel");
    if (sentinel && kind === "loading") {
      sentinel.classList.add("is-loading");
      sentinel.setAttribute("aria-busy", "true");
    }
    updateWalletTxScrollFooter();
  }

  function activePanelId() {
    const p = document.querySelector(".panel.active");
    return p && p.id ? p.id.replace(/^panel-/, "") : "";
  }

  function isPanelActive(name) {
    return activePanelId() === name;
  }

  function summaryConnectLag(s) {
    if (!s) return 0;
    const lag = Number(s.dogego_connect_lag != null ? s.dogego_connect_lag : s.blocks_behind_headers);
    return isFinite(lag) && lag > 0 ? lag : 0;
  }

  function txFlightBusy() {
    return !!(window.DogeGoTxFlight && window.DogeGoTxFlight.hasBusyFlight && window.DogeGoTxFlight.hasBusyFlight());
  }

  function shouldDeferWalletPoll(s) {
    if (!s) return false;
    if (txFlightBusy()) return true;
    return summaryConnectLag(s) > CONNECT_LAG_POLL_DEFER;
  }

  function maybeReloadWalletHistoryAfterScan(wal, s) {
    const deferring = walletHistoryScanBuildingDefer(s, wal);
    if (walletHistoryScanDeferred && !deferring) {
      if (wal && wal.enabled !== false && !shouldDeferHeavyWalletAPI(s, wal)) {
        bootWalletHistoryReady = false;
        if (isPanelActive("transactions") || !walletTxHistory.loaded.length) {
          void loadWalletTxHistoryPage(true);
        }
      }
    }
    walletHistoryScanDeferred = deferring;
  }

  function walletHistoryScanBuildingDefer(s, wal) {
    wal = wal || lastWalletSnap;
    const scanning = !!(wal && wal.scanning) || !!(s && s.scanning);
    if (!scanning) return false;
    const utxoWalk = !!(wal && wal.wallet_listtransactions_utxo_walk) || !!(s && s.wallet_listtransactions_utxo_walk);
    const scanPending = !!(wal && wal.wallet_listtransactions_scan_pending) || !!(s && s.wallet_listtransactions_scan_pending);
    if (!utxoWalk && !scanPending) return false;
    const n = Number(
      wal && wal.utxo_count != null ? wal.utxo_count
        : (wal && wal.spendable_utxo_count != null ? wal.spendable_utxo_count
          : (s && s.wallet_utxo_count != null ? s.wallet_utxo_count : NaN))
    );
    return isFinite(n) && n > 64;
  }

  function walletHistoryDeferMessage(s, wal) {
    wal = wal || lastWalletSnap;
    const serverReason = (wal && wal.wallet_history_defer_reason) || (s && s.wallet_history_defer_reason);
    if (serverReason) {
      return walletHistoryDeferMessageFromReason(serverReason, s, wal);
    }
    if (s && !!s.ibd_active) {
      return typeof i18n === "function" ? i18n("pages.history.pausedIbd") : "Wallet history paused during initial block download.";
    }
    if (s && summaryConnectLag(s) > CONNECT_LAG_HEAVY_DEFER) {
      const lag = summaryConnectLag(s);
      return typeof i18n === "function"
        ? i18n("pages.history.pausedConnectLag", { lag: lag })
        : ("Wallet history paused while the node connects blocks (" + lag + " behind).");
    }
    if (walletHistoryScanBuildingDefer(s, wal)) {
      return typeof i18n === "function"
        ? i18n("pages.history.pausedScanBuilding")
        : "Wallet rescan is building history index - transaction list loads after scan progress or use Settings → Wallet.";
    }
    return "";
  }

  function shouldDeferHeavyWalletAPI(s, wal) {
    return walletHistoryDeferMessage(s, wal) !== "";
  }

  function walletHistoryDeferMessageFromReason(code, s, wal) {
    code = String(code || "");
    if (code === "ibd_active") {
      return typeof i18n === "function" ? i18n("pages.history.pausedIbd") : "Wallet history paused during initial block download.";
    }
    if (code === "connect_lag") {
      s = s || lastSummary;
      const lag = summaryConnectLag(s);
      return typeof i18n === "function"
        ? i18n("pages.history.pausedConnectLag", { lag: lag })
        : ("Wallet history paused while the node connects blocks (" + lag + " behind).");
    }
    if (code === "scan_building") {
      return typeof i18n === "function"
        ? i18n("pages.history.pausedScanBuilding")
        : "Wallet rescan is building history index - transaction list loads after scan progress or use Settings → Wallet.";
    }
    return walletHistoryDeferMessage(s, wal);
  }

  async function onTxSettled(txid) {
    if (!txid) return;
    try {
      const mp = await loadMempoolDetail();
      if (mp) fillMempoolPanel(mp);
    } catch (_) {}
    if (walletTxHistory.loaded.some((t) => t.txid === txid)) {
      await patchWalletTxHistorySoft();
    } else if (isPanelActive("transactions")) {
      void loadWalletTxHistoryPage(true);
    }
    if (window.DogeGoBlockStep && window.DogeGoBlockStep.refreshTx) {
      window.DogeGoBlockStep.refreshTx(txid);
    }
  }

  function patchWalletTxFromSendResponse(body) {
    if (!body || !body.txid) return;
    const tx = walletTxHistory.loaded.find((t) => t.txid === body.txid);
    if (!tx) return;
    if (body.tx_kind) tx.tx_kind = body.tx_kind;
    if (body.pq_tag) tx.pq_tag = body.pq_tag;
    const feed = $("wallet-tx-feed");
    const idx = walletTxHistory.loaded.indexOf(tx);
    if (feed && idx >= 0) {
      const row = feed.querySelector('.wallet-tx-row[data-tx-idx="' + idx + '"]');
      if (row) {
        const top = row.querySelector(".wallet-tx-row-top");
        if (top) top.innerHTML = walletTxTypeChip(tx) + walletTxConfBadge(tx);
      }
    }
  }

  function walletTxRowKey(tx) {
    if (!tx) return "";
    return String(tx.txid || "") + ":" + String(tx.category || "") + ":" + String(tx.vout != null ? tx.vout : "");
  }

  function dedupeWalletTxItems(items) {
    const seen = new Set();
    const out = [];
    (items || []).forEach((tx) => {
      const k = walletTxRowKey(tx);
      if (!k || seen.has(k)) return;
      seen.add(k);
      out.push(tx);
    });
    return out;
  }

  async function patchWalletTxHistorySoft() {
    if (!isPanelActive("transactions")) return;
    if (shouldDeferHeavyWalletAPI(lastSummary)) return;
    const feed = $("wallet-tx-feed");
    if (!feed || walletTxHistory.loading || walletTxHistory.loaded.length === 0) return;
    const totalWanted = walletTxHistory.loaded.length;
    const CHUNK = 200;
    try {
      const byTxid = new Map();
      let total = walletTxHistory.total;
      for (let offset = 0; offset < totalWanted; offset += CHUNK) {
        const limit = Math.min(CHUNK, totalWanted - offset);
        const params = new URLSearchParams({
          limit: String(limit),
          offset: String(offset),
          kind: walletTxHistory.typeFilter || "all",
        });
        if (walletTxHistory.filter.trim()) params.set("q", walletTxHistory.filter.trim());
        const r = await fetchAPI("/api/wallet/txs?" + params.toString(), WALLET_TX_API_TIMEOUT_MS);
        if (!r.ok) return;
        const data = await r.json();
        const items = Array.isArray(data.items) ? data.items : [];
        if (data.total != null) total = Number(data.total);
        items.forEach((tx) => {
          if (tx && tx.txid) byTxid.set(walletTxRowKey(tx), tx);
        });
      }
      let anyChanged = false;
      walletTxHistory.loaded.forEach((tx, i) => {
        const fresh = tx.txid && byTxid.get(walletTxRowKey(tx));
        if (!fresh) return;
        const confOld = Number(tx.confirmations) || 0;
        const confNew = Number(fresh.confirmations) || 0;
        const bhOld = tx.blockheight != null ? Number(tx.blockheight) : null;
        const bhNew = fresh.blockheight != null ? Number(fresh.blockheight) : null;
        const kindOld = String(tx.tx_kind || "");
        const kindNew = String(fresh.tx_kind || "");
        const pqOld = String(tx.pq_tag || "");
        const pqNew = String(fresh.pq_tag || "");
        if (confOld !== confNew || bhOld !== bhNew || !!tx.abandoned !== !!fresh.abandoned || kindOld !== kindNew || pqOld !== pqNew) {
          tx.confirmations = fresh.confirmations;
          tx.blockheight = fresh.blockheight;
          tx.blockhash = fresh.blockhash;
          tx.abandoned = fresh.abandoned;
          tx.time = fresh.time;
          if (fresh.tx_kind) tx.tx_kind = fresh.tx_kind;
          if (fresh.pq_tag) tx.pq_tag = fresh.pq_tag;
          anyChanged = true;
          const row = feed.querySelector('.wallet-tx-row[data-tx-idx="' + i + '"]');
          if (row) {
            const top = row.querySelector(".wallet-tx-row-top");
            if (top) top.innerHTML = walletTxTypeChip(tx) + walletTxConfBadge(tx);
          }
        }
      });
      if (total !== walletTxHistory.total) {
        walletTxHistory.total = total;
        updateWalletTxHistoryMeta();
      }
      if (anyChanged) lastWalletTxs = walletTxHistory.loaded.slice();
    } catch (_) {}
  }

  function ensureWalletTxScrollObserver() {
    const root = $("tx-list");
    const sentinel = $("wallet-tx-load-sentinel");
    if (!root || !sentinel) return;
    if (walletTxHistory.observer) {
      walletTxHistory.observer.disconnect();
      walletTxHistory.observer = null;
    }
    walletTxHistory.scrollRoot = root;
    walletTxHistory.observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.isIntersecting)) return;
        if (walletTxHistory.hasMore && !walletTxHistory.loading) {
          void loadWalletTxHistoryPage(false);
        }
      },
      { root, rootMargin: "160px", threshold: 0.01 }
    );
    walletTxHistory.observer.observe(sentinel);
    if (!root.dataset.txScrollBound) {
      root.dataset.txScrollBound = "1";
      root.addEventListener(
        "scroll",
        () => {
          if (walletTxHistory.hasMore && !walletTxHistory.loading && walletTxSentinelVisible()) {
            void loadWalletTxHistoryPage(false);
          }
        },
        { passive: true }
      );
    }
  }

  function walletTxSentinelVisible() {
    const root = walletTxHistory.scrollRoot || $("tx-list");
    const sentinel = $("wallet-tx-load-sentinel");
    if (!root || !sentinel || sentinel.hidden) return false;
    const rr = root.getBoundingClientRect();
    const sr = sentinel.getBoundingClientRect();
    return sr.top <= rr.bottom + 140;
  }

  async function maybeChainLoadWalletTxHistory() {
    if (!walletTxHistory.hasMore || walletTxHistory.loading) return;
    if (!walletTxSentinelVisible()) return;
    await loadWalletTxHistoryPage(false);
    if (walletTxHistory.hasMore && walletTxSentinelVisible()) {
      await maybeChainLoadWalletTxHistory();
    }
  }

  async function loadAllWalletTxHistory() {
    const guard = 500;
    let n = 0;
    while (walletTxHistory.hasMore && !walletTxHistory.loading && n < guard) {
      await loadWalletTxHistoryPage(false);
      n++;
    }
  }

  async function loadWalletTxHistoryPage(reset, bootPrime) {
    const el = $("tx-list");
    if (!el || walletTxHistory.loading) return;
    const priming = bootPrime || (!bootWalletHistoryReady && lastSummary && lastSummary.wallet_enabled);
    if (!isPanelActive("transactions") && !priming) return;
    if (shouldDeferHeavyWalletAPI(lastSummary)) {
      if (reset) {
        el.className = "wallet-tx-empty";
        const msg = walletHistoryDeferMessage(lastSummary, lastWalletSnap);
        el.textContent = msg || "Wallet history temporarily unavailable.";
      }
      return;
    }
    if (reset) {
      if (walletTxHistory.observer) {
        walletTxHistory.observer.disconnect();
        walletTxHistory.observer = null;
      }
      walletTxHistory.offset = 0;
      walletTxHistory.loaded = [];
      walletTxHistory.hasMore = true;
      walletTxHistory.total = 0;
    } else if (!walletTxHistory.hasMore) {
      return;
    }
    walletTxHistory.loading = true;
    const loadGen = ++walletTxHistory.loadGen;
    setWalletTxLoadState("loading");
    if (reset) {
      el.className = "wallet-tx-list wallet-tx-list-scroll";
      el.innerHTML =
        '<div class="wallet-tx-loading" id="wallet-tx-loading"><span class="wallet-tx-spinner" aria-hidden="true"></span> Loading transactions…</div>' +
        '<div class="wallet-tx-feed" id="wallet-tx-feed"></div>' +
        '<div class="wallet-tx-scroll-footer" id="wallet-tx-scroll-footer" hidden>' +
        '<div class="wallet-tx-progress-track"><div class="wallet-tx-progress-fill"></div></div>' +
        '<span class="wallet-tx-progress-count">0 / 0</span>' +
        '<span class="wallet-tx-progress-hint">Scroll for more</span>' +
        "</div>" +
        '<div class="wallet-tx-load-sentinel" id="wallet-tx-load-sentinel" aria-hidden="true">' +
        '<span class="wallet-tx-spinner" aria-hidden="true"></span>' +
        "</div>" +
        '<div class="wallet-tx-load-end" id="wallet-tx-load-end" hidden>End of history</div>';
      ensureWalletTxScrollObserver();
    }
    const feed = $("wallet-tx-feed");
    const params = new URLSearchParams({
      limit: String(WALLET_TX_PAGE_SIZE),
      offset: String(walletTxHistory.offset),
      kind: walletTxHistory.typeFilter || "all",
    });
    if (walletTxHistory.filter.trim()) params.set("q", walletTxHistory.filter.trim());
    try {
      const r = await fetchAPI("/api/wallet/txs?" + params.toString(), WALLET_TX_API_TIMEOUT_MS);
      if (loadGen !== walletTxHistory.loadGen) return;
      if (!r.ok) {
        if (reset && feed) {
          el.className = "wallet-tx-empty";
          el.textContent = "Could not load wallet history.";
        }
        return;
      }
      const data = await r.json();
      if (data && data.deferred) {
        if (reset) {
          el.className = "wallet-tx-empty";
          el.textContent = walletHistoryDeferMessageFromReason(data.defer_reason, lastSummary, lastWalletSnap)
            || "Wallet history temporarily unavailable.";
        }
        walletHistoryScanDeferred = data.defer_reason === "scan_building";
        return;
      }
      const items = Array.isArray(data) ? data : (Array.isArray(data.items) ? data.items : []);
      const total = Array.isArray(data) ? data.length : (Number(data.total) || items.length);
      walletTxHistory.total = total;
      if (reset && !items.length) {
        bootWalletHistoryReady = true;
        el.className = "wallet-tx-empty";
        el.textContent = walletTxHistory.filter.trim()
          ? "No transactions match your filter."
          : "No wallet transactions yet.";
        updateWalletTxHistoryMeta();
        return;
      }
      if (!feed) return;
      const loadingEl = $("wallet-tx-loading");
      if (loadingEl) loadingEl.remove();
      el.className = "wallet-tx-list wallet-tx-list-scroll";
      const pageItems = dedupeWalletTxItems(items);
      const startIdx = walletTxHistory.loaded.length;
      pageItems.forEach((tx) => walletTxHistory.loaded.push(tx));
      walletTxHistory.offset = walletTxHistory.loaded.length;
      walletTxHistory.hasMore = walletTxHistory.offset < total;
      let html = "";
      pageItems.forEach((tx, i) => {
        html += walletTxRowHtml(tx, startIdx + i);
      });
      feed.insertAdjacentHTML("beforeend", html);
      bindWalletTxRows(feed, walletTxHistory.loaded, true);
      lastWalletTxs = walletTxHistory.loaded.slice();
      updateWalletTxScrollFooter();
      setWalletTxLoadState(walletTxHistory.hasMore ? "" : "end");
      persistWalletTxHistoryCache(walletTxCacheNetwork || (lastSummary && lastSummary.network));
    } catch (_) {
      if (reset) {
        el.className = "wallet-tx-empty";
        el.textContent = "Could not load wallet history.";
      }
    } finally {
      walletTxHistory.loading = false;
      if (reset) bootWalletHistoryReady = true;
      maybeHideBootOverlay(lastSummary, lastWalletSnap);
      updateWalletTxHistoryMeta();
      updateWalletTxScrollFooter();
      ensureWalletTxScrollObserver();
      void maybeChainLoadWalletTxHistory();
    }
  }

  async function refreshWalletTxHistory(reset) {
    const filt = $("tx-history-filter") && $("tx-history-filter").value;
    walletTxHistory.filter = String(filt || "");
    walletTxHistory.typeFilter = activeTxTypeFilter();
    const soft = reset === "soft";
    if (soft && walletTxHistory.loaded.length > WALLET_TX_PAGE_SIZE * 2) {
      await patchWalletTxHistorySoft();
      return;
    }
    await loadWalletTxHistoryPage(!soft);
  }

  function exportWalletTxHistoryCSV() {
    if (shouldDeferHeavyWalletAPI(lastSummary, lastWalletSnap)) return;
    const params = new URLSearchParams({ kind: activeTxTypeFilter() || "all" });
    const filt = $("tx-history-filter") && $("tx-history-filter").value;
    if (filt && filt.trim()) params.set("q", filt.trim());
    fetch("/api/wallet/txs.csv?" + params.toString(), { credentials: "same-origin" })
      .then((r) => {
        if (!r.ok) throw new Error("export failed");
        return r.blob();
      })
      .then((blob) => {
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = "dogego-wallet-history.csv";
        a.click();
        URL.revokeObjectURL(a.href);
      })
      .catch(() => {});
  }

  function rerenderWalletTxHistory() {
    void refreshWalletTxHistory(true);
  }


  function sendFormValid() {
    const dest = $("send-to") && $("send-to").value.trim();
    const amt = parseFloat($("send-amt") && $("send-amt").value);
    return !!(dest && isFinite(amt) && amt > 0);
  }

  function setSendBtnEnabled(on) {
    const btn = $("send-btn");
    if (!btn) return;
    btn.disabled = !on;
  }

  function validateSendForm() {
    const full = ((lastSummary && lastSummary.node_mode) || "full").toLowerCase() !== "spv";
    const wal = lastWalletSnap;
    const ready = !!(wal && wal.enabled && full && wal.send_ready);
    const mainnetBlocked = !!(wal && wal.mainnet_encryption_required);
    setSendBtnEnabled(ready && sendFormValid() && !mainnetBlocked);
    const sendLabel = document.querySelector("#send-btn .send-btn-label");
    const locked = wal && wal.encrypted && wal.unlocked === false;
    if (sendLabel && locked && ready && sendFormValid()) {
      sendLabel.textContent = "Unlock & send";
    } else if (sendLabel) {
      sendLabel.textContent = "Send DOGE";
    }
    updateSendFeeEstimate();
  }

  async function ensureWalletUnlockedForSend() {
    const wal = lastWalletSnap;
    if (!wal || !wal.encrypted || wal.unlocked !== false) return true;
    if (!window.DogeGoWalletPassphrase || !window.DogeGoWalletPassphrase.ensureUnlocked) {
      showSendResult(false, "Wallet is locked. Unlock with dogego-cli walletpassphrase.");
      return false;
    }
    const ok = await window.DogeGoWalletPassphrase.ensureUnlocked({
      wallet: wal,
      message: "Enter your wallet passphrase to sign and send (Core walletpassphrase).",
    });
    if (!ok) return false;
    await refreshWalletPanelAsync(refreshGen);
    validateSendForm();
    return !!(lastWalletSnap && lastWalletSnap.unlocked !== false);
  }

  function showSendResult(ok, text, html) {
    const out = $("send-out");
    if (!out) return;
    out.hidden = false;
    out.className = "wallet-send-result " + (ok ? "ok" : "err");
    if (html) {
      out.innerHTML = html;
    } else {
      out.textContent = text;
    }
  }

  function finishSendUtxoList(list, html) {
    if (!list) return;
    list.removeAttribute("data-doge-wait");
    list.classList.remove("doge-wait-host");
    list.innerHTML = html;
  }

  async function loadSendUtxos(force) {
    const now = Date.now();
    if (!force && sendUtxosLoadedAt && now - sendUtxosLoadedAt < 30000) return;
    if (!force && shouldDeferHeavyWalletAPI(lastSummary) && !isPanelActive("send")) return;
    const list = $("send-utxo-list");
    if (!list || !lastWalletSnap || !lastWalletSnap.enabled) {
      finishSendUtxoList(list, '<p class="field-hint">Enable the wallet to use coin control.</p>');
      return;
    }
    if (lastWalletSnap.encrypted && lastWalletSnap.unlocked === false) {
      finishSendUtxoList(list, '<p class="field-hint">Unlock the wallet to list UTXOs.</p>');
      return;
    }
    try {
      const r = await fetchAPI("/api/wallet/utxos", WALLET_TX_API_TIMEOUT_MS);
      if (!r.ok) {
        finishSendUtxoList(list, '<p class="field-hint err">Could not load UTXOs (HTTP ' + escHtml(String(r.status)) + ").</p>");
        return;
      }
      const utxos = await r.json();
      lastSendUtxos = Array.isArray(utxos) ? utxos : [];
      sendUtxosLoadedAt = Date.now();
      renderSendUtxoList(lastSendUtxos);
    } catch (_) {
      finishSendUtxoList(list, '<p class="field-hint err">Could not load UTXOs.</p>');
    }
  }

  function renderSendUtxoList(utxos) {
    const list = $("send-utxo-list");
    if (!list) return;
    if (!utxos.length) {
      finishSendUtxoList(list, '<p class="field-hint">No UTXOs in wallet yet.</p>');
      return;
    }
    const showMax = 300;
    const slice = utxos.length > showMax ? utxos.slice(0, showMax) : utxos;
    let html = "";
    slice.forEach((u, i) => {
      const spendable = u.spendable !== false;
      const conf = u.confirmations != null ? u.confirmations + " conf" : "";
      html +=
        '<label class="wallet-utxo-row' + (spendable ? "" : " unspendable") + '">' +
        '<input type="checkbox" class="send-utxo-cb" data-txid="' + escHtml(u.txid) + '" data-vout="' + escHtml(u.vout) + '" ' + (spendable ? "" : "disabled") + " />" +
        '<span class="mono">' + escHtml(shortTxid(u.txid)) + ":" + escHtml(u.vout) + "</span>" +
        "<span><strong>" + escHtml(formatDOGE(u.amount, 4)) + "</strong> DOGE · " + escHtml(conf) + "</span>" +
        "</label>";
    });
    if (utxos.length > showMax) {
      html += '<p class="field-hint">Showing first ' + showMax + " of " + utxos.length + " UTXOs. Use fewer inputs or send without coin control for large sets.</p>";
    }
    finishSendUtxoList(list, html);
  }

  function selectedSendUtxos() {
    const out = [];
    document.querySelectorAll(".send-utxo-cb:checked").forEach((cb) => {
      const txid = cb.getAttribute("data-txid");
      const vout = parseInt(cb.getAttribute("data-vout"), 10);
      if (txid && isFinite(vout)) out.push({ txid: txid, vout: vout });
    });
    return out;
  }

  function applySummaryWalletStub(s) {
    if (!s) return;
    const lag = Number(s.dogego_connect_lag ?? s.blocks_behind_headers) || 0;
    const partial = {
      enabled: !!s.wallet_enabled,
      address: s.wallet_address || "",
      send_ready: !!s.wallet_rpc_ready,
      address_ready: !!s.wallet_address_ready,
      network: (s.network || s.chain || "").toLowerCase(),
      connect_lag: lag,
    };
    if (typeof s.wallet_balance === "number") partial.balance = s.wallet_balance;
    if (typeof s.wallet_immature_balance === "number") partial.immature_balance = s.wallet_immature_balance;
    if (s.wallet_utxo_count != null) partial.utxo_count = s.wallet_utxo_count;
    if (s.wallet_index_height != null) partial.wallet_index_height = s.wallet_index_height;
    if (s.chain_active_height != null) partial.chain_active_height = s.chain_active_height;
    if (s.needs_rescan === true) partial.needs_rescan = true;
    if (s.wallet_scan_index_ok === true) partial.wallet_scan_index_ok = true;
    if (s.wallet_history_fast_path === true) partial.wallet_history_fast_path = true;
    if (s.wallet_listtransactions_utxo_walk === true) partial.wallet_listtransactions_utxo_walk = true;
    if (s.wallet_listtransactions_scan_pending === true) partial.wallet_listtransactions_scan_pending = true;
    if (s.scanning === true) partial.scanning = true;
    if (s.wallet_history_deferred === true) partial.wallet_history_deferred = true;
    if (typeof s.wallet_history_defer_reason) partial.wallet_history_defer_reason = s.wallet_history_defer_reason;
    if (typeof s.wallet_encrypted === "boolean") {
      partial.encrypted = s.wallet_encrypted;
      partial.unlocked = !!s.wallet_unlocked;
      partial.private_keys_enabled = s.wallet_private_keys_enabled !== false;
      if (s.wallet_unlocked_until != null) partial.unlocked_until = s.wallet_unlocked_until;
    }
    const merged = Object.assign({}, lastWalletSnap || {}, partial);
    if (partial.enabled && partial.address) {
      if ($("recv-addr")) $("recv-addr").textContent = partial.address;
      renderRecvQR(partial.address);
      updateRecvMeta(merged);
      updateRecvStatus(merged, s);
    }
    updateWalletAbNewButton(s, merged);
    updateKeypoolRefillButton(s, merged);
    updateWalletRescanUI(merged);
    updateSendUI(merged, s);
    syncTopbarLockButton(merged);
    updateWalletEncryptSettingsPanel(merged, false);
    maybeAutoWalletRescan(merged);
    maybeReloadWalletHistoryAfterScan(merged, s);
  }

  function walletAddressRPCReady(s, wal) {
    if (wal && wal.address_ready) return true;
    if (wal && wal.send_ready) return true;
    if (s && s.wallet_address_ready) return true;
    if (s && s.wallet_rpc_ready) return true;
    return false;
  }

  function updateWalletAbNewButton(s, wal) {
    const btn = $("wallet-ab-new");
    if (!btn) return;
    const walletOn = !!(wal && wal.enabled !== false) || !!(s && s.wallet_enabled);
    const ready = walletAddressRPCReady(s, wal);
    btn.disabled = !walletOn || !ready;
    btn.title = walletOn && !ready ? "Wallet RPC is still starting…" : "";
  }

  function updateKeypoolRefillButton(s, wal) {
    const btn = $("wallet-keypool-refill-btn");
    if (!btn) return;
    const walletOn = !!(wal && wal.enabled !== false) || !!(s && s.wallet_enabled);
    const hd = !!(wal && wal.hd_wallet);
    const ready = walletAddressRPCReady(s, wal);
    const show = walletOn && hd && ready;
    btn.hidden = !show;
    btn.disabled = !show;
    const kp = wal && wal.keypool_size != null ? Number(wal.keypool_size) : 0;
    btn.title = kp > 0 && kp < 50 ? "Keypool is low - refill after Core wallet.dat import with pool-only rows" : "";
  }

  async function walletKeypoolRefill(newSize) {
    const body = newSize != null && newSize > 0 ? JSON.stringify({ new_size: newSize }) : "{}";
    const r = await fetch("/api/wallet/keypool-refill", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: body,
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) {
      const msg = (data && data.error && data.error.message) ? data.error.message : ("HTTP " + r.status);
      throw new Error(msg);
    }
    return data;
  }

  function updateRecvMeta(wal) {
    const meta = $("recv-meta");
    if (!meta) return;
    if (!wal || wal.enabled === false) {
      if (lastRecvMetaText !== "") {
        meta.textContent = "";
        lastRecvMetaText = "";
      }
      return;
    }
    const parts = [];
    if (wal.path) parts.push("Wallet: " + wal.path);
    else parts.push("Wallet");
    if (wal.encrypted) parts.push("encrypted");
    if (wal.network) parts.push(wal.network);
    if (wal.keypool_size != null && wal.keypool_size > 0) {
      parts.push(i18n("pages.receive.recvKeypoolSize", { n: wal.keypool_size }));
    }
    if (wal.pool_core_indices_stored > 0) {
      parts.push(i18n("pages.receive.recvCorePoolIndices", { n: wal.pool_core_indices_stored }));
    }
    if (wal.scanning) {
      parts.push(i18n("settings.walletRescanScanning"));
    } else if (wal.needs_rescan && wal.wallet_history_fast_path) {
      parts.push(i18n("settings.walletRescanNeedsFastPath", {
        indexed: wal.wallet_index_height != null ? wal.wallet_index_height : "none",
        tip: wal.chain_active_height != null ? wal.chain_active_height : "?",
      }));
    } else if (wal.needs_rescan) {
      parts.push(i18n("settings.walletRescanNeeds", {
        indexed: wal.wallet_index_height != null ? wal.wallet_index_height : "none",
        tip: wal.chain_active_height != null ? wal.chain_active_height : "?",
      }));
    } else if (wal.wallet_scan_index_ok) {
      parts.push(i18n("settings.walletRescanOk", { tip: wal.chain_active_height != null ? wal.chain_active_height : "?" }));
    } else if (wal.wallet_history_fast_path) {
      parts.push(i18n("settings.walletHistoryFastPath"));
    }
    const text = parts.join(" · ");
    if (text !== lastRecvMetaText) {
      meta.textContent = text;
      lastRecvMetaText = text;
    }
  }

  function updateRecvStatus(wal, s) {
    const status = $("recv-status");
    if (!status) return;
    if (!wal || wal.enabled === false) {
      status.textContent = "";
      return;
    }
    let text = "";
    if (typeof wal.balance === "number") {
      text = formatDOGE(wal.balance, 4) + " DOGE available";
      const imm = Number(wal.immature_balance) || 0;
      if (imm > 0) text += " · " + formatDOGE(imm, 4) + " immature";
    } else {
      const lag = Number(wal.connect_lag != null ? wal.connect_lag : (s && (s.dogego_connect_lag ?? s.blocks_behind_headers))) || 0;
      if (lag > 0) text = "Connecting blocks (" + lag + " behind)";
      else text = "Loading balance…";
    }
    if (status.textContent !== text) status.textContent = text;
  }

  function applyWalletPanelData(wal, s) {
    if (!wal) return;
    if (wal.enabled && wal.address) {
      if ($("recv-addr")) $("recv-addr").textContent = wal.address;
      renderRecvQR(wal.address);
      updateRecvMeta(wal);
      updateRecvStatus(wal, s);
    } else if (wal.enabled === false) {
      if ($("recv-addr")) $("recv-addr").textContent = i18n("pages.receive.walletDisabled");
      renderRecvQR("");
      updateRecvMeta(wal);
      updateRecvStatus(wal, s);
    }
    applyWalletFlagsFromStatus(wal);
    updateWalletSettingsPanel(wal, s || lastSummary);
    updateWalletRescanUI(wal);
    updateWalletAbNewButton(s || lastSummary, wal);
    updateKeypoolRefillButton(s || lastSummary, wal);
    updateSendUI(wal, s || lastSummary);
    updateMainnetEncryptionBanners(wal);
    syncTopbarLockButton(wal);
    updateSecurityChecklist(wal);
    maybeAutoWalletRescan(wal);
    maybeReloadWalletHistoryAfterScan(wal, s || lastSummary);
  }

  function updateWalletRescanUI(wal) {
    const status = $("st-wallet-rescan-status");
    const btn = $("st-wallet-rescan-btn");
    const fullBtn = $("st-wallet-rescan-full-btn");
    if (!status && !btn && !fullBtn) return;
    if (!wal || wal.enabled === false) {
      if (status) status.textContent = "";
      if (btn) btn.disabled = true;
      if (fullBtn) fullBtn.disabled = true;
      return;
    }
    const scanning = !!wal.scanning;
    const tip = wal.chain_active_height != null ? Number(wal.chain_active_height) : -1;
    const indexed = wal.wallet_index_height != null ? Number(wal.wallet_index_height) : -1;
    if (status) {
      if (scanning) {
        status.textContent = i18n("settings.walletRescanScanning");
      } else if (wal.needs_rescan && wal.wallet_history_fast_path && tip >= 0) {
        status.textContent = i18n("settings.walletRescanNeedsFastPath", {
          indexed: indexed >= 0 ? indexed : "none",
          tip: tip,
        });
      } else if (wal.needs_rescan && tip >= 0) {
        status.textContent = i18n("settings.walletRescanNeeds", {
          indexed: indexed >= 0 ? indexed : "none",
          tip: tip,
        });
      } else if (wal.wallet_listtransactions_utxo_walk && (wal.spendable_utxo_count > 64 || wal.utxo_count > 64)) {
        const n = wal.spendable_utxo_count != null ? wal.spendable_utxo_count : wal.utxo_count;
        status.textContent = i18n("settings.walletListtransactionsUtxoWalk", { n: n });
      } else if (tip >= 0 && wal.wallet_scan_index_ok) {
        status.textContent = i18n("settings.walletRescanOk", { tip: tip });
      } else if (wal.wallet_history_fast_path) {
        status.textContent = i18n("settings.walletHistoryFastPath");
      } else if (tip >= 0) {
        status.textContent = i18n("settings.walletRescanOk", { tip: tip });
      } else {
        status.textContent = "";
      }
    }
    const disabled = scanning || !wal.send_ready;
    if (btn) btn.disabled = disabled;
    if (fullBtn) fullBtn.disabled = disabled;
  }

  async function startWalletRescan(full) {
    const msg = $("st-wallet-flags-msg");
    try {
      const r = await fetch("/api/wallet/rescan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify(full ? { full: true } : {}),
      });
      const j = await r.json().catch(() => ({}));
      if (!r.ok) {
        if (msg) msg.textContent = j.error || i18n("settings.walletRescanFailed");
        return;
      }
      if (msg) msg.textContent = i18n("settings.walletRescanStarted");
      refresh();
    } catch (e) {
      if (msg) msg.textContent = String(e.message || e);
    }
  }

  function walletAutoRescanWanted(wal) {
    if (!wal || wal.enabled === false || wal.scanning) return false;
    if (wal.needs_rescan) return true;
    if (wal.wallet_listtransactions_utxo_walk) {
      const n = wal.spendable_utxo_count != null ? Number(wal.spendable_utxo_count) : Number(wal.utxo_count);
      return isFinite(n) && n > 64;
    }
    return false;
  }

  function maybeAutoWalletRescan(wal) {
    if (walletAutoRescanStarted) return;
    if (sessionStorage.getItem("dogego_auto_rescan")) return;
    if (!walletAutoRescanWanted(wal)) return;
    if (shouldDeferHeavyWalletAPI(lastSummary)) return;
    if (wal.encrypted && wal.unlocked === false) return;
    walletAutoRescanStarted = true;
    sessionStorage.setItem("dogego_auto_rescan", "1");
    void startWalletRescan(false);
  }

  async function refreshWalletPanelAsync(gen) {
    if (walletPanelInFlight) return;
    if (shouldDeferWalletPoll(lastSummary)) return;
    walletPanelInFlight = true;
    try {
      const rWal = await fetchAPI("/api/wallet", WALLET_API_TIMEOUT_MS);
      if (gen !== refreshGen) return;
      if (!rWal.ok) {
        if (window.DogeGoSecurity && window.DogeGoSecurity.noteWalletAPIUnauthorized) {
          await window.DogeGoSecurity.noteWalletAPIUnauthorized(rWal);
        }
        return;
      }
      const wal = await rWal.json();
      applyWalletPanelData(wal, lastSummary);
      maybeHideBootOverlay(lastSummary, wal);
      void processSendQueue();
      if (wal && wal.enabled && !bootWalletHistoryReady && !shouldDeferHeavyWalletAPI(lastSummary)) {
        void loadWalletTxHistoryPage(true).then(() => {
          bootWalletHistoryReady = true;
          maybeHideBootOverlay(lastSummary, wal);
        });
      }
      const errEl = $("err");
      if (errEl && errEl.classList.contains("warn")) errEl.classList.remove("show");
    } catch (walletErr) {
      if (gen !== refreshGen) return;
      if (isTransientAPIError(walletErr) && lastSummary) {
        const errEl = $("err");
        if (errEl && !isBootPhase()) {
          const msg = txFlightBusy()
            ? "Send in progress ... wallet balance will refresh when the send finishes."
            : "Wallet balance still loading … dashboard is live. (" + friendlyAPIError(walletErr) + ")";
          errEl.textContent = msg;
          errEl.className = "alert warn show";
        }
      }
    } finally {
      walletPanelInFlight = false;
    }
  }

  function updateWalletLockedOverviewBanner(wal) {
    const box = $("ov-wallet-locked-alert");
    if (!box) return;
    const locked = !!(wal && wal.enabled !== false && wal.encrypted && (wal.unlocked === false || wal.private_keys_enabled === false));
    box.hidden = !locked;
  }

  function updateSendUI(wal, s) {
    if (wal) lastWalletSnap = wal;
    else if (lastWalletSnap) wal = lastWalletSnap;
    const full = ((s && s.node_mode) || (lastSummary && lastSummary.node_mode) || "full").toLowerCase() !== "spv";
    const enabled = !!(wal && wal.enabled && full);
    const sendReady = enabled && !!(wal && wal.send_ready);
    const pqOn = !!(wal && wal.pq_commitments_enabled);
    const carrierOn = !!(wal && wal.pq_carrier_enabled);
    const locked = wal && wal.encrypted && wal.unlocked === false;
    const mainnetBlocked = !!(wal && wal.mainnet_encryption_required);
    const to = $("send-to");
    const amt = $("send-amt");
    const maxBtn = $("send-max-btn");
    const feeRate = $("send-fee-rate");
    const subtractFee = $("send-subtract-fee");
    const pqBanner = $("send-pq-banner");
    const pqTag = $("send-pq-tag");
    const pqCommit = $("send-pq-commit");
    const pqWrap = $("send-pq-wrap");
    const pqHint = $("send-pq-disabled-hint");
    const pqModeRow = $("send-pq-mode-row");
    const pqCarrierHint = $("send-pq-carrier-hint");
    const hint = $("send-status-hint");
    if (pqBanner) pqBanner.hidden = !pqOn;
    if (pqWrap) pqWrap.hidden = !pqOn && !carrierOn;
    if (pqHint) pqHint.hidden = pqOn || carrierOn;
    if (pqModeRow) pqModeRow.hidden = !(pqOn || carrierOn);
    if (pqCarrierHint) {
      const carrierMode = document.querySelector('input[name="send-pq-mode"][value="carrier"]');
      const carrierSelected = carrierMode && carrierMode.checked;
      pqCarrierHint.hidden = !carrierSelected || carrierOn;
    }
    renderSendBalance(wal);
    if (hint) {
      if (!full) {
        hint.hidden = false;
        hint.textContent = "SPV mode - send is not available on this run.";
      } else if (!wal || !wal.enabled) {
        hint.hidden = false;
        hint.textContent = "Wallet is disabled. Enable it in Settings.";
      } else if (mainnetBlocked) {
        hint.hidden = false;
        hint.textContent = typeof i18n === "function" ? i18n("pages.send.mainnetEncryptBody") : "Encrypt wallet on mainnet before sending.";
      } else if (locked) {
        hint.hidden = false;
        hint.textContent = "Wallet file is encrypted. Fill the form and click Unlock & send, or use dogego-cli walletpassphrase.";
      } else if (!sendReady) {
        hint.hidden = false;
        hint.textContent = "Waiting for UTXO cache and RPC wiring… sync must finish before send is enabled.";
      } else if (wal.immature_balance > 0 && (!wal.balance || wal.balance < 1e-8)) {
        hint.hidden = false;
        hint.textContent = "Funds are from recent mining and are not spendable yet (~240 block maturity on testnet). Wait for confirmations or receive confirmed coins.";
      } else {
        hint.hidden = true;
        hint.textContent = "";
      }
    }
    const inputsOn = sendReady && !mainnetBlocked;
    if (to) {
      to.disabled = !inputsOn;
      to.placeholder = inputsOn ? "D… address" : full ? "Wallet unavailable" : "SPV - send unavailable";
    }
    if (amt) amt.disabled = !inputsOn;
    if (maxBtn) maxBtn.disabled = !inputsOn;
    if (feeRate) {
      feeRate.disabled = !inputsOn;
      if (inputsOn && feeRate.value === "" && wal && wal.fee_per_kb > 0) {
        feeRate.value = String(wal.fee_per_kb);
      }
    }
    if (subtractFee) subtractFee.disabled = !inputsOn;
    if (pqTag) pqTag.disabled = !inputsOn;
    if (pqCommit) {
      pqCommit.disabled = !inputsOn;
      pqCommit.placeholder = pqOn ? "Auto-generated on send (optional override)" : "Enable quantum-ready sends in Settings";
    }
    validateSendForm();
    updateWalletLockedOverviewBanner(wal);
  }

  function applyWalletFlagsFromStatus(wal) {
    if ($("st-pq-commitments")) $("st-pq-commitments").checked = !!(wal && wal.pq_commitments_enabled);
    if ($("st-pq-carrier")) $("st-pq-carrier").checked = !!(wal && wal.pq_carrier_enabled);
    if ($("st-avoid-reuse")) $("st-avoid-reuse").checked = !!(wal && wal.avoid_reuse);
  }

  function syncWalletEnabledToggle() {
    const enabled = $("st-wallet-enabled");
    const nowallet = $("st-nowallet");
    if (!enabled || !nowallet) return;
    enabled.checked = !nowallet.checked;
  }

  function formatWalletUnlockUntil(untilSec) {
    const left = Math.max(0, Math.floor(untilSec - Date.now() / 1000));
    if (left <= 0) return "";
    const m = Math.floor(left / 60);
    const s = left % 60;
    if (m >= 60) {
      const h = Math.floor(m / 60);
      const rm = m % 60;
      return h + "h " + rm + "m";
    }
    if (m > 0) return m + "m " + s + "s";
    return s + "s";
  }

  function walletEncryptStatusText(wal) {
    if (!wal) return "";
    if (typeof wal.encrypted !== "boolean") {
      return typeof i18n === "function" ? i18n("settings.walletEncryptLoading") : "Checking wallet encryption…";
    }
    if (!wal.encrypted) {
      return typeof i18n === "function" ? i18n("settings.walletEncryptOff") : "Wallet file is not encrypted.";
    }
    if (wal.unlocked === false || wal.private_keys_enabled === false) {
      return typeof i18n === "function" ? i18n("settings.walletEncryptLocked") : "Spend keys locked.";
    }
    const until = wal.unlocked_until;
    if (until && until > Date.now() / 1000) {
      const timeLeft = formatWalletUnlockUntil(until);
      return typeof i18n === "function"
        ? i18n("settings.walletEncryptUnlocked", { time: timeLeft })
        : ("Spend keys unlocked for " + timeLeft + ".");
    }
    return typeof i18n === "function" ? i18n("settings.walletEncryptUnlockedOpen") : "Spend keys unlocked.";
  }

  function clearWalletUnlockCountdown() {
    if (walletUnlockCountdownTimer) {
      clearInterval(walletUnlockCountdownTimer);
      walletUnlockCountdownTimer = null;
    }
  }

  function updateWalletEncryptSettingsPanel(wal, disableLive) {
    const status = $("st-wallet-encrypt-status");
    const unlockBtn = $("st-wallet-unlock-keys");
    const lockBtn = $("st-wallet-lock-keys");
    const actions = $("st-wallet-encrypt-actions");
    const encForm = $("st-wallet-encrypt-form");
    const passForm = $("st-wallet-passphrase-form");
    clearWalletUnlockCountdown();
    if (!status || !actions) return;
    if (disableLive || !wal || wal.enabled === false) {
      status.textContent = "";
      if (unlockBtn) unlockBtn.hidden = true;
      if (lockBtn) lockBtn.hidden = true;
      if (encForm) encForm.hidden = true;
      if (passForm) passForm.hidden = true;
      return;
    }
    if (typeof wal.encrypted !== "boolean") {
      status.textContent = walletEncryptStatusText(wal);
      if (unlockBtn) unlockBtn.hidden = true;
      if (lockBtn) lockBtn.hidden = true;
      if (encForm) encForm.hidden = true;
      if (passForm) passForm.hidden = true;
      return;
    }
    if (!wal.encrypted) {
      status.textContent = walletEncryptStatusText(wal);
      if (unlockBtn) unlockBtn.hidden = true;
      if (lockBtn) lockBtn.hidden = true;
      if (encForm) encForm.hidden = false;
      if (passForm) passForm.hidden = true;
      return;
    }
    const locked = wal.unlocked === false || wal.private_keys_enabled === false;
    status.textContent = walletEncryptStatusText(wal);
    if (unlockBtn) {
      unlockBtn.hidden = !locked;
      unlockBtn.disabled = false;
    }
    if (lockBtn) {
      lockBtn.hidden = locked;
      lockBtn.disabled = false;
    }
    if (encForm) encForm.hidden = true;
    if (passForm) passForm.hidden = false;
    if (!locked && wal.unlocked_until && wal.unlocked_until > Date.now() / 1000) {
      walletUnlockCountdownTimer = setInterval(() => {
        const w = lastWalletSnap;
        if (!w || !w.encrypted || w.unlocked === false) {
          clearWalletUnlockCountdown();
          return;
        }
        if (!w.unlocked_until || w.unlocked_until <= Date.now() / 1000) {
          clearWalletUnlockCountdown();
          refreshWalletPanelAsync(refreshGen);
          return;
        }
        const el = $("st-wallet-encrypt-status");
        if (el) el.textContent = walletEncryptStatusText(w);
      }, 1000);
    }
  }

  function updateWalletSettingsPanel(wal, s) {
    const hint = $("st-wallet-runtime-hint");
    const configDisabled = $("st-nowallet") && $("st-nowallet").checked;
    if (hint) {
      if (configDisabled) {
        hint.textContent = typeof i18n === "function" ? i18n("settings.walletDisabledConfig") : "Wallet disabled in config (nowallet). Save config and restart.";
      } else if (!wal || wal.enabled === false) {
        hint.textContent = typeof i18n === "function" ? i18n("settings.walletUnavailable") : "Wallet not loaded on this run.";
      } else if (wal.pq_commitments_enabled) {
        const tag = wal.pq_tag || "FLC1";
        hint.textContent = typeof i18n === "function" ? i18n("settings.walletPqOn", { tag: tag }) : ("Post-quantum sends on (" + tag + " OP_RETURN on each send).");
      } else {
        hint.textContent = typeof i18n === "function" ? i18n("settings.walletPqOff") : "Post-quantum sends off. Enable below to attach FLC1 commitments.";
      }
    }
    const disableLive = configDisabled || !wal || wal.enabled === false;
    ["st-pq-commitments", "st-pq-carrier", "st-avoid-reuse", "st-wallet-rescan-btn", "st-wallet-rescan-full-btn"].forEach((id) => {
      const el = $(id);
      if (el) el.disabled = disableLive;
    });
    updateWalletEncryptSettingsPanel(wal, disableLive);
  }

  async function saveWalletFlag(key, value) {
    const msg = $("st-wallet-flags-msg");
    const body = {};
    body[key] = value;
    try {
      const r = await fetch("/api/wallet/flags", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) {
        const j = await r.json().catch(() => ({}));
        if (msg) msg.textContent = j.error || "Flag save failed";
        return false;
      }
      if (msg) msg.textContent = "Wallet flag saved.";
      return true;
    } catch (e) {
      if (msg) msg.textContent = String(e);
      return false;
    }
  }

  function applyNodeMode(s) {
    const mode = (s.node_mode || "full").toLowerCase();
    document.body.classList.toggle("mode-spv", mode === "spv");
    document.body.classList.toggle("mode-full", mode !== "spv");
    const pill = $("mode-pill");
    if (pill) {
      pill.textContent = mode === "spv" ? "spv" : "full";
      pill.className = "badge " + (mode === "spv" ? "mode-spv" : "mode-full");
    }
    let banner = "Full node ... downloading headers and block bodies when syncing.";
    if (mode === "spv") banner = "SPV mode ... headers only; no full block storage on this run.";
    if (s.embedded_analytics_sidecar) banner += " Analytics sidecar active.";
    const recovery = s.dogego_header_sync_recovery;
    const bodyNote = s.dogego_body_sync_note;
    if (s.dogego_genesis_missing === true) {
      const gn = s.dogego_genesis_note || "Downloading genesis block (height 0)…";
      banner = gn + (banner ? " · " + banner : "");
    }
    if (s.dogego_post_aux_era_header_stall === true) {
      const stallMsg = (typeof recovery === "string" && recovery.length > 0)
        ? recovery
        : "Header tip stuck near height 510000 (~8%). Rebuild dogego.exe if logs show aux parent chain-id errors.";
      banner = stallMsg + (banner ? " · " + banner : "");
    } else if (s.dogego_body_ibd_header_paused === true && typeof bodyNote === "string" && bodyNote.length > 0) {
      banner = bodyNote + (banner ? " · " + banner : "");
    } else if (typeof bodyNote === "string" && bodyNote.length > 0) {
      banner = bodyNote + (banner ? " · " + banner : "");
    } else if (typeof recovery === "string" && recovery.length > 0) {
      banner = "Header sync: " + recovery + (banner ? " · " + banner : "");
    }
    const ov = $("ov-banner");
    if (ov) ov.textContent = banner;
    const recoverRow = $("ov-header-recover-row");
    if (recoverRow) {
      recoverRow.hidden = !(
        (typeof recovery === "string" && recovery.length > 0) ||
        s.dogego_post_aux_era_header_stall === true
      );
    }
  }

  function updateReleaseHref(s) {
    if (!s) return "";
    if (s.dogego_update_release_url) return String(s.dogego_update_release_url);
    const tag = s.dogego_update_latest_tag;
    if (tag) return "https://github.com/qlpqlp/dogego/releases/tag/" + encodeURIComponent(String(tag));
    return "";
  }

  function syncDashboardBannerStack() {
    const stack = $("dashboard-banner-stack");
    if (!stack) return;
    let h = 0;
    stack.querySelectorAll(".update-banner").forEach((el) => {
      if (!el.hidden) h += el.offsetHeight;
    });
    document.documentElement.style.setProperty("--banner-stack-h", h > 0 ? h + "px" : "0px");
    document.body.classList.toggle("dashboard-banners-visible", h > 0);
  }

  function scheduleDashboardBannerStackSync() {
    requestAnimationFrame(() => {
      syncDashboardBannerStack();
      requestAnimationFrame(syncDashboardBannerStack);
    });
  }

  function applyUpdateBanner(s) {
    const el = $("update-banner");
    if (!el) {
      applySettingsUpdatePanel(s);
      return;
    }
    const avail = s && s.dogego_update_available === true && s.dogego_update_dismissed !== true;
    el.hidden = !avail;
    if (!avail) {
      applySettingsUpdatePanel(s);
      scheduleDashboardBannerStackSync();
      return;
    }
    const cur = s.dogego_update_current || "";
    const latest = s.dogego_update_latest || "";
    const title = $("update-banner-title");
    if (title) title.textContent = "Update available: DogeGo " + latest;
    const detail = $("update-banner-detail");
    if (detail) {
      let text = "You are running " + cur + ". ";
      if (s.dogego_update_instructions) {
        text += s.dogego_update_instructions;
      } else {
        text += "Download a binary from GitHub Releases or build: " + (s.dogego_update_build_cmd || "go build -o dogego ./cmd/dogego");
      }
      detail.textContent = text;
    }
    const relHref = updateReleaseHref(s);
    const rel = $("update-banner-release");
    if (rel) {
      rel.hidden = !relHref;
      if (relHref) rel.href = relHref;
    }
    const direct = s.dogego_update_direct_available && s.dogego_update_download_url;
    const dl = $("update-banner-download");
    if (dl) dl.hidden = true;
    const apply = $("update-banner-apply");
    if (apply) apply.hidden = !direct;
    const dismiss = $("update-banner-dismiss");
    if (dismiss) dismiss.hidden = !avail;
    applySettingsUpdatePanel(s);
    scheduleDashboardBannerStackSync();
  }

  function applySettingsUpdatePanel(s) {
    const status = $("st-update-status");
    const detail = $("st-update-detail");
    const rel = $("st-update-release");
    const dl = $("st-update-download");
    const applyBtn = $("st-update-apply");
    const dismiss = $("st-update-dismiss");
    if (!status) return;
    const cur = (s && s.dogego_update_current) || (s && s.dogego_version) || "";
    const latest = s && s.dogego_update_latest;
    const avail = s && s.dogego_update_available === true && s.dogego_update_dismissed !== true;
    const direct = s && s.dogego_update_direct_available && s.dogego_update_download_url;
    const relHref = updateReleaseHref(s);
    if (s && s.dogego_update_check_error) {
      status.textContent = "Running " + (cur || "?") + ". Update check error: " + s.dogego_update_check_error;
    } else if (avail && latest) {
      status.textContent = "Running " + cur + ". Update available: " + latest + ".";
    } else {
      status.textContent = "Running " + (cur || "?") + (latest ? " (latest on GitHub: " + latest + ")" : ". Up to date on GitHub.");
    }
    if (detail) {
      let text = "";
      if (s && s.dogego_update_checked_at) text += "Last checked " + s.dogego_update_checked_at + ". ";
      if (avail && s && s.dogego_update_checksum_sha256) text += "Release SHA256 " + s.dogego_update_checksum_sha256 + ". ";
      if (avail && s && s.dogego_update_instructions) text += s.dogego_update_instructions;
      detail.textContent = text.trim();
      detail.hidden = !text.trim();
    }
    if (rel) {
      rel.hidden = !(avail && relHref);
      if (relHref) rel.href = relHref;
    }
    if (dl) dl.hidden = true;
    if (applyBtn) applyBtn.hidden = !(avail && direct);
    if (dismiss) dismiss.hidden = true;
  }

  async function settingsUpdateAction(path, btn) {
    if (btn) btn.disabled = true;
    const detail = $("st-update-detail");
    try {
      const r = await fetch(path, { method: "POST", credentials: "same-origin" });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error || body.message || ("HTTP " + r.status));
      if (path === "/api/update/apply") {
        if (detail) detail.textContent = body.note || "Restarting…";
        return;
      }
      if (detail && body.note) detail.textContent = body.note;
      if (body.path && detail) detail.textContent = "Downloaded to " + body.path + (body.sha256 ? " SHA256 " + body.sha256 : "") + ".";
      await refresh();
    } catch (e) {
      if (detail) detail.textContent = String(e);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  async function recoverHeaderJournal() {
    const btn = $("ov-header-recover-btn");
    const msg = $("ov-header-recover-msg");
    if (btn) btn.disabled = true;
    if (msg) wait(msg, "Recovering headers…", { inline: true });
    try {
      const res = await fetch("/api/chain/recover-headers", { method: "POST" });
      const body = await res.json();
      if (body.error) {
        if (msg) msg.textContent = body.error;
        return;
      }
      const before = Number(body.tip_before);
      const after = Number(body.tip_after);
      if (msg) {
        if (body.rewound) {
          msg.textContent = "Rewound headers " + before.toLocaleString() + " → " + after.toLocaleString() + ". Sync continues.";
        } else if (body.dogego_header_sync_restarted) {
          msg.textContent = body.message || "Header sync restarted; block download continues.";
        } else {
          msg.textContent = body.message || "Done.";
        }
      }
      refresh();
    } catch (e) {
      if (msg) msg.textContent = String(e);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  function chainActiveHeight(s) {
    if (!s) return -1;
    const active = Number(s.chain_active_height);
    if (isFinite(active) && active >= 0) return active;
    return -1;
  }

  function utxoReplayActive(s) {
    return !!(s && s.dogego_utxo_bodies_aligned === false);
  }

  function formatUtxoReplaySummary(s) {
    if (!utxoReplayActive(s)) return "";
    const remain = Number(s.dogego_utxo_body_replay_remaining);
    const cont = Number(s.contiguous_raw_height);
    const target = Number(s.dogego_utxo_replay_target);
    const pct = Number(s.dogego_snapshot_body_replay_pct);
    if (isFinite(remain) && remain > 0) {
      if (isFinite(cont) && isFinite(target) && target >= 0) {
        let txt = cont.toLocaleString() + " / " + target.toLocaleString();
        if (isFinite(pct) && pct > 0) txt += " (" + pct.toFixed(0) + "%)";
        return txt;
      }
      return remain.toLocaleString() + " remaining";
    }
    return "...";
  }

  function applyUtxoReplayUI(s) {
    const txt = formatUtxoReplaySummary(s);
    const active = utxoReplayActive(s);
    const ibdWrap = $("ibd-phase-utxo-replay-wrap");
    if (ibdWrap) ibdWrap.hidden = !active;
    if (active) {
      const el = $("ibd-phase-utxo-replay");
      if (el) el.textContent = txt || "...";
    }
    const dockWrap = $("sync-dock-utxo-replay-wrap");
    if (dockWrap) dockWrap.hidden = !active;
    if (active) {
      const dockEl = $("sync-dock-utxo-replay");
      if (dockEl) dockEl.textContent = txt || "...";
    }
  }

  function formatConnectCatchUpBoost(s) {
    if (!s) return "";
    const passes = Number(s.dogego_connect_catch_up_passes);
    const batch = Number(s.dogego_connect_catch_up_batch);
    const interval = Number(s.dogego_connect_catch_up_interval_ms);
    if (!isFinite(passes) || passes <= 0 || !isFinite(batch) || batch <= 0) return "";
    let t = passes + "×" + batch;
    if (isFinite(interval) && interval > 0) t += " @ " + interval + "ms";
    return t;
  }

  function applyConnectCatchUpBoostUI(s) {
    const boost = formatConnectCatchUpBoost(s);
    const wrap = $("ibd-phase-connect-boost-wrap");
    const el = $("ibd-phase-connect-boost");
    if (wrap) wrap.hidden = !boost;
    if (el) el.textContent = boost || "...";
  }

  function connectLagDominant(s) {
    if (!s || String(s.node_mode || "").toLowerCase() === "spv") return false;
    const lag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
    const dlRate = Number(s.blocks_per_minute);
    const connRate = Number(s.dogego_connect_blocks_per_minute);
    if (!isFinite(lag) || lag < 512) return false;
    if (!isFinite(connRate) || connRate <= 0) return lag >= 1024;
    if (isFinite(dlRate) && dlRate > 0 && connRate > 0) return lag > 256 && connRate < dlRate * 0.5;
    return lag >= 1024;
  }

  function applyIbdPhaseCard(s) {
    const card = $("ibd-phase-card");
    if (!card) return;
    const mode = String(s.node_mode || "full").toLowerCase();
    const ibd = s.initialblockdownload === true || s.ibd_active === true;
    const show = mode !== "spv" && ibd;
    card.hidden = !show;
    if (!show) return;
    const tip = Number(s.tip_height);
    const active = chainActiveHeight(s);
    const stored = Number(s.contiguous_raw_height);
    const lag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
    const dlRate = Number(s.blocks_per_minute);
    const connRate = Number(s.dogego_connect_blocks_per_minute);
    const pill = $("ov-core-parity-pill");
    if (pill) {
      const ok = s.dogego_auxpow_core_parity === true || s.dogego_auxpow_parent_chain_id_core_parity === true;
      pill.hidden = !ok;
      pill.classList.toggle("core-parity-ok", ok);
    }
    const lead = $("ibd-phase-lead");
    if (lead) {
      lead.hidden = false;
      if (utxoReplayActive(s)) {
        lead.textContent = i18n("pages.overview.ibdPhaseUtxoReplay");
      } else if (connectLagDominant(s)) {
        lead.textContent = i18n("pages.overview.ibdPhaseConnect");
      } else if (isFinite(tip) && isFinite(stored) && tip - stored > 1000) {
        lead.textContent = i18n("pages.overview.ibdPhaseDownload", { tip: tip.toLocaleString() });
      } else {
        lead.textContent = i18n("pages.overview.ibdPhaseDefault");
      }
    }
    const set = (id, text) => { const el = $(id); if (el) el.textContent = text; };
    set("ibd-phase-connected", active >= 0 ? active.toLocaleString() : "...");
    set("ibd-phase-stored", stored >= 0 ? stored.toLocaleString() : "...");
    set("ibd-phase-headers", isFinite(tip) ? tip.toLocaleString() : "...");
    set("ibd-phase-download-rate", isFinite(dlRate) && dlRate > 0 ? dlRate.toFixed(1) + " blk/min" : "...");
    set("ibd-phase-connect-rate", isFinite(connRate) && connRate > 0 ? connRate.toFixed(1) + " blk/min" : "...");
    set("ibd-phase-connect-lag", isFinite(lag) && lag > 0 ? lag.toLocaleString() : "0");
    applyConnectCatchUpBoostUI(s);
    const cp = Number(s.dogego_checkpoint_probe);
    set("ibd-phase-checkpoint", isFinite(cp) && cp >= 0 ? cp.toLocaleString() : "...");
    const warnEl = $("ibd-resume-warn");
    if (warnEl) {
      const warns = s.dogego_restart_resume_warnings;
      if (Array.isArray(warns) && warns.length) {
        warnEl.hidden = false;
        warnEl.textContent = i18n("pages.overview.ibdResumeWarn") + ": " + formatResumeWarnings(warns);
      } else {
        warnEl.hidden = true;
        warnEl.textContent = "";
      }
    }
  }

  function applyOverviewCoreCards(s) {
    if (!s) return;
    const ibd = s.initialblockdownload === true || s.ibd_active === true;
    const configured = s.core_rpc_configured === true;
    const compareCard = $("ov-core-compare-card");
    if (compareCard && (ibd || configured || coreCompareCache || s.dogego_deployment_checked === true)) {
      compareCard.hidden = false;
      if (!coreCompareCache) {
        const pill = $("ov-core-compare-pill");
        if (pill) {
          pill.textContent = "...";
          pill.className = "p2p-health-pill starting";
        }
        const addrs = $("ov-core-compare-addrs");
        if (addrs && s.core_rpc_addr) addrs.textContent = "Core " + s.core_rpc_addr;
        const hint = $("ov-core-compare-hint");
        if (hint) {
          hint.textContent = configured
            ? i18n("pages.overview.coreComparePending")
            : i18n("pages.features.coreCompareHint");
        }
      }
    }
    const certCard = $("ov-operator-cert-card");
    if (certCard && (ibd || configured || s.dogego_operator_cert_total != null || s.dogego_mempool_offline_corpus_total != null)) certCard.hidden = false;
  }

  function formatResumeWarnings(warnings) {
    if (!Array.isArray(warnings) || !warnings.length) return "";
    return warnings.map((w) => {
      const key = "syncDock.warn." + w;
      const t = i18n(key);
      return t !== key ? t : String(w).replace(/_/g, " ");
    }).join(", ");
  }

  function applyOverviewResumeCard(s) {
    const card = $("ov-resume-card");
    if (!card) return;
    const ibd = s.initialblockdownload === true || s.ibd_active === true;
    const cp = Number(s.dogego_checkpoint_probe);
    const bodyLag = Number(s.dogego_body_lag_headers);
    const connectLag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
    const pool = Number(s.assist_peer_pool);
    const warns = s.dogego_restart_resume_warnings;
    const hasWarn = Array.isArray(warns) && warns.length > 0;
    const boost = formatConnectCatchUpBoost(s);
    const show = ibd && (isFinite(cp) || hasWarn || (isFinite(bodyLag) && bodyLag > 0) ||
      (isFinite(connectLag) && connectLag > 0) || !!boost);
    card.hidden = !show;
    if (!show) return;
    const set = (id, text) => { const el = $(id); if (el) el.textContent = text; };
    set("ov-resume-checkpoint", isFinite(cp) && cp >= 0 ? cp.toLocaleString() : "...");
    set("ov-resume-assist-pool", isFinite(pool) && pool > 0 ? String(pool) : "0");
    set("ov-resume-body-lag", isFinite(bodyLag) && bodyLag > 0 ? bodyLag.toLocaleString() : "0");
    const connectWrap = $("ov-resume-connect-wrap");
    const boostWrap = $("ov-resume-boost-wrap");
    if (connectWrap) connectWrap.hidden = !(isFinite(connectLag) && connectLag > 0);
    if (boostWrap) boostWrap.hidden = !boost;
    set("ov-resume-connect-lag", isFinite(connectLag) && connectLag > 0 ? connectLag.toLocaleString() : "0");
    set("ov-resume-connect-boost", boost || "...");
    const warnEl = $("ov-resume-warn");
    if (warnEl) {
      const txt = hasWarn ? i18n("pages.overview.ibdResumeWarn") + ": " + formatResumeWarnings(warns) : "";
      warnEl.textContent = txt;
      warnEl.hidden = !txt;
    }
  }

  function syncStripContigLabel(s) {
    const tip = Number(s && s.tip_height);
    const active = chainActiveHeight(s);
    if (active >= 0 && isFinite(tip) && tip >= 0) {
      return active.toLocaleString() + " / " + tip.toLocaleString() + " connected";
    }
    if (isFinite(tip) && tip >= 0) return "0 / " + tip.toLocaleString() + " connected";
    return "...";
  }

  function formatLaneInFlight(lanes) {
    if (!lanes || typeof lanes !== "object") return "";
    const parts = Object.entries(lanes)
      .filter((e) => e[1] > 0)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4)
      .map((e) => e[0] + ": " + e[1] + " in-flight");
    if (!parts.length) return "";
    return " In-flight getdata: " + parts.join("; ") + ".";
  }

  function isHeaderCatchUpPhase(s) {
    if (!s) return false;
    const tip = Number(s.tip_height);
    const peerH = Number(s.peer_start_height);
    const bodyPct = Number(s.dogego_body_verification_progress);
    return s.headers_syncing === true || s.dogego_header_catch_up_pending === true ||
      (isFinite(peerH) && peerH > tip && (!isFinite(bodyPct) || bodyPct < 0.01));
  }

  function capSyncPctByNetworkLag(s, pct) {
    if (!s) return pct;
    const tip = Number(s.tip_height);
    const peerH = Number(s.peer_start_height);
    if (isFinite(tip) && tip >= 0 && isFinite(peerH) && peerH > tip && s.dogego_body_ibd_header_paused !== true) {
      let hdr = s.dogego_header_ibd_progress != null ? Number(s.dogego_header_ibd_progress) : (tip + 1) / (peerH + 1);
      if (!isFinite(hdr)) hdr = 0;
      pct = Math.min(pct, Math.round(Math.min(1, Math.max(0, hdr)) * 100));
    }
    const phase = String(s.sync_phase || "");
    if (phase === "awaiting_genesis_block" || s.dogego_genesis_missing === true) {
      return Math.min(pct, tip > 0 ? 5 : 1);
    }
    if (phase === "forward_block_ibd") {
      const body = s.dogego_body_verification_progress != null ? Number(s.dogego_body_verification_progress) : NaN;
      if (isFinite(body) && body < 0.999) {
        pct = Math.min(pct, Math.round(body * 100));
      }
    }
    const behind = Number(s.blocks_behind_headers);
    if (isFinite(behind) && behind > 0) {
      pct = Math.min(pct, 99);
    }
    if (isFinite(tip) && tip < 1000 && isFinite(peerH) && peerH > tip + 1000) {
      pct = Math.min(pct, Math.max(1, Math.round(((tip + 1) / (peerH + 1)) * 100)));
    }
    return pct;
  }

  function syncProgressPct(s) {
    let pct = Number(s && s.verification_progress);
    const ibd = s && (s.initialblockdownload === true || s.ibd_active === true);
    if (ibd) {
      if (s.dogego_body_ibd_header_paused !== true && s.dogego_header_ibd_progress != null) {
        pct = Math.max(pct, Number(s.dogego_header_ibd_progress));
      }
      const bodyPct = s.dogego_body_verification_progress != null
        ? Number(s.dogego_body_verification_progress) : null;
      if (bodyPct != null && isFinite(bodyPct)) {
        pct = Math.max(pct, bodyPct);
      }
    } else if (s && s.dogego_tx_verification_progress != null) {
      pct = Number(s.dogego_tx_verification_progress);
    }
    if (!isFinite(pct)) pct = 0;
    return capSyncPctByNetworkLag(s, Math.min(100, Math.max(0, Math.round(pct * 100))));
  }

  function updateOverviewHero(s) {
    const mode = ((s && s.node_mode) || "full").toLowerCase();
    const tip = Number(s && s.tip_height);
    let pct = syncProgressPct(s);
    const heroPct = $("sync-pct-hero");
    if (heroPct) heroPct.textContent = String(pct);
    const contigHero = $("ov-sync-contig-hero");
    if (contigHero) contigHero.textContent = syncStripContigLabel(s);
    const tipSpv = $("ov-hero-tip-spv");
    if (tipSpv && isFinite(tip)) tipSpv.textContent = tip.toLocaleString();
    const phase = $("ov-sync-phase");
    const sub = $("ov-sync-sub");
    const strip = $("ov-sync-strip");
    const heroBar = $("ov-sync-bar");
    if (mode === "spv") {
      const spvPhase = $("ov-hero-state-spv");
      if (spvPhase) {
        spvPhase.textContent = isFinite(tip) && tip > 0
          ? "SPV · headers at " + tip.toLocaleString()
          : "SPV · headers only";
      }
      return;
    }
    if (!phase) return;
    if (s && (s.dogego_ui_loading === true || s.warming_up === true) && !s.from_disk_snapshot) {
      const detail = (s.dogego_ui_loading_detail && String(s.dogego_ui_loading_detail)) || "Loading local data…";
      const phaseMap = {
        warming: "Loading local data",
        utxo_cache: "Connecting blocks",
        snapshot_replay: "Connecting stored bodies",
        wallet_scan: "Scanning wallet",
        analytics: "Preparing analytics"
      };
      phase.textContent = (s.dogego_ui_loading_phase && phaseMap[s.dogego_ui_loading_phase]) || "Loading local data";
      if (sub) {
        sub.textContent = detail;
        sub.hidden = false;
      }
      if (strip) strip.classList.toggle("ov-sync-strip--loading", true);
      if (heroBar) {
        heroBar.style.width = "35%";
        heroBar.classList.add("sync-dock-fill--pulse");
      }
      return;
    }
    if (s && (s.from_disk_snapshot === true || s.summary_stale === true)) {
      phase.textContent = "Updating…";
      if (sub) {
        const tipLbl = isFinite(tip) && tip >= 0 ? tip.toLocaleString() + " headers" : "last known tip";
        sub.textContent = "Showing last known data · refreshing (" + tipLbl + ")";
        sub.hidden = false;
      }
      if (strip) strip.classList.toggle("ov-sync-strip--stale", true);
      if (heroBar) heroBar.classList.remove("sync-dock-fill--pulse");
      return;
    }
    if (strip) {
      strip.classList.remove("ov-sync-strip--loading", "ov-sync-strip--stale");
    }
    if (heroBar) heroBar.classList.remove("sync-dock-fill--pulse");
    const behind = Number(s.blocks_behind_headers);
    const bodyPct = s.dogego_body_verification_progress != null
      ? Number(s.dogego_body_verification_progress) : null;
    const bodiesLag = bodyPct != null && isFinite(bodyPct) && bodyPct < 0.999
      && isFinite(tip) && tip > 0;
    const ibd = s.initialblockdownload === true || s.ibd_active === true
      || (isFinite(behind) && behind > 32 && pct < 100)
      || bodiesLag
      || s.dogego_genesis_missing === true;
    let phaseText = "Connecting";
    let subText = "";
    const peerH = Number(s.peer_start_height);
    const headerCatchUp = isHeaderCatchUpPhase(s);
    if (pct >= 100 && !ibd && !bodiesLag) {
      phaseText = "Up to date";
      subText = "";
    } else if (headerCatchUp && isFinite(tip) && isFinite(peerH) && peerH > tip) {
      phaseText = "Downloading headers";
      const hdrPct = s.dogego_header_ibd_progress != null
        ? Math.round(Number(s.dogego_header_ibd_progress) * 100) : pct;
      subText = "Height " + tip.toLocaleString() + " / ~" + peerH.toLocaleString() +
        " (" + hdrPct + "% of network headers)";
      const act = s.dogego_sync_activity;
      if (act && act.headline) {
        phaseText = act.headline.indexOf("header") >= 0 ? act.headline : phaseText;
        if (act.detail) subText = act.detail;
      }
    } else if (s.dogego_header_tip_stale === true && !headerCatchUp) {
      phaseText = "Stale header tip";
      const age = Number(s.dogego_header_tip_age_sec);
      subText = isFinite(age) && age > 0
        ? "Header tip time " + Math.round(age / 3600) + " h behind network time (-maxtipage)"
        : (s.dogego_header_sync_recovery || "");
    } else if (s.dogego_sync_health === "forward_ibd_stalled") {
      phaseText = "Block sync stalled";
      subText = "No recent block progress ... check peers, disk space, and logs (IBD stall recovery runs automatically).";
      const stallPeer = s.dogego_last_block_stall_peer || s.dogego_last_block_download_timeout_peer;
      if (stallPeer) {
        subText += " Last peer disconnect: " + stallPeer + ".";
      }
      if (s.dogego_frontier_stalling_since) {
        const since = new Date(s.dogego_frontier_stalling_since * 1000);
        subText += " Frontier in-flight since " + since.toLocaleTimeString() + ".";
      }
      const laneHint = formatLaneInFlight(s.dogego_lane_in_flight);
      if (laneHint) subText += laneHint;
    } else if (s.dogego_post_aux_era_header_stall === true) {
      phaseText = "Headers near 510k";
      subText = s.dogego_header_sync_recovery ||
        "Tip stuck in the post-aux era band. Rebuild and restart dogego.exe if aux chain-id errors persist; otherwise use Recover header journal.";
    } else if (connectLagDominant(s)) {
      phaseText = "Connecting stored blocks";
      const lag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
      const connRate = Number(s.dogego_connect_blocks_per_minute);
      const stored = Number(s.contiguous_raw_height);
      const active = chainActiveHeight(s);
      subText = "Bodies are on disk ahead of chainActive ... replaying ConnectBlock (Core path).";
      if (isFinite(lag) && lag > 0) subText += " Lag " + lag.toLocaleString() + " blocks.";
      if (isFinite(connRate) && connRate > 0) {
        const etaMin = Math.ceil(lag / connRate);
        subText += " ~" + connRate.toFixed(1) + " connect/min";
        if (isFinite(etaMin) && etaMin > 0 && etaMin < 10080) subText += " (~" + etaMin + " min to catch up).";
      }
      if (isFinite(active) && isFinite(stored)) {
        subText += " Connected " + active.toLocaleString() + " / stored " + stored.toLocaleString() + ".";
      }
    } else if (s.dogego_sync_health === "headers_catching_up") {
      phaseText = "Syncing blocks";
      subText = (s.dogego_header_sync_recovery || s.sync_status_line || "Headers retrying in background; block download continues.");
      if (s.dogego_block_assist_active === true && subText.indexOf("block-assist") < 0) {
        subText += " Block-assist workers active.";
      }
      const actHdr = s.dogego_sync_activity;
      if (actHdr && actHdr.headline) {
        phaseText = actHdr.headline.indexOf("header") >= 0 ? "Catching up headers" : phaseText;
        subText = actHdr.headline + (actHdr.detail ? " · " + actHdr.detail : "");
      }
    } else if (s.dogego_sync_health === "forward_ibd_active" && s.dogego_sync_ok === true) {
      phaseText = "Syncing blocks";
      const lag = Number(s.blocks_behind_headers);
      if (isFinite(lag) && lag > 50000 && isFinite(tip)) {
        subText = "Headers at " + tip.toLocaleString() + " (paused while bodies catch up). " +
          (s.sync_status_line || "");
      } else {
        subText = s.sync_status_line || subText;
      }
      const laneHint = formatLaneInFlight(s.dogego_lane_in_flight);
      if (laneHint) subText += laneHint;
    } else if (ibd) {
      phaseText = "Syncing blocks";
      subText = s.sync_status_line || "";
      const lag = Number(s.blocks_behind_headers);
      if (!subText && isFinite(lag) && lag > 50000 && isFinite(tip)) {
        subText = "Headers at " + tip.toLocaleString() + " (paused while bodies catch up).";
      }
      if (!subText) {
        const parts = [];
        if (isFinite(behind) && behind > 0) parts.push(behind.toLocaleString() + " behind headers");
        const eta = s.sync_eta;
        if (eta) parts.push("~" + eta + " left");
        const rate = Number(s.blocks_per_minute);
        if (isFinite(rate) && rate > 0) parts.push(rate.toFixed(1) + " blk/min");
        subText = parts.join(" · ");
      }
      const act = s.dogego_sync_activity;
      if (act && act.headline && s.dogego_sync_health !== "forward_ibd_stalled") {
        subText = act.headline;
        if (act.detail) subText += " · " + act.detail;
      }
    } else if (s.headers_syncing || (s.sync_phase && s.sync_phase !== "block_chain_connected" && s.sync_phase !== "forward_block_ibd")) {
      phaseText = "Syncing headers";
      subText = isFinite(tip) && tip > 0
        ? "Header tip " + tip.toLocaleString()
        : "Finding peers…";
    } else if (isFinite(tip) && tip > 0) {
      phaseText = "Syncing blocks";
      subText = s.sync_status_line || "";
    } else {
      subText = s.sync_status_line || "Waiting for peers…";
    }
    phase.textContent = phaseText;
    if (sub) {
      sub.textContent = subText;
      sub.hidden = !subText;
    }
    if (strip) {
      strip.classList.toggle("sync-strip-done", pct >= 100 && !ibd);
      strip.classList.toggle("sync-strip-active", ibd || pct < 100);
    }
    const prog = $("ov-sync-progress");
    if (prog) prog.setAttribute("aria-valuenow", String(pct));
    const nums = $("ov-sync-nums");
    if (nums && headerCatchUp && isFinite(tip) && isFinite(peerH) && peerH > tip) {
      const hdrPct = s.dogego_header_ibd_progress != null
        ? Math.round(Number(s.dogego_header_ibd_progress) * 100) : pct;
      nums.innerHTML = '<span id="sync-pct-hero">' + hdrPct + '</span>% headers · ' +
        '<span id="ov-sync-contig-hero">' + tip.toLocaleString() + " / ~" + peerH.toLocaleString() + '</span>';
    }
  }

  function applySyncActivity(s) {
    const act = s.dogego_sync_activity;
    const card = $("sync-activity-card");
    if (!card) return;
    const mode = String(s.node_mode || "").toLowerCase();
    const ibd = s.initialblockdownload === true || s.ibd_active === true;
    const show = mode !== "spv" && act && typeof act === "object" &&
      (ibd || s.headers_syncing === true || act.stalled === true);
    card.hidden = !show;
    if (!show) return;
    const head = $("sync-activity-headline");
    if (head) head.textContent = act.headline || "Running";
    const det = $("sync-activity-detail");
    if (det) {
      const d = act.detail || "";
      det.textContent = d;
      det.hidden = !d;
    }
    const stalledEl = $("sync-activity-stalled");
    if (stalledEl) stalledEl.hidden = act.stalled !== true;
    const ul = $("sync-activity-tasks");
    if (ul) {
      ul.replaceChildren();
      const tasks = Array.isArray(act.tasks) ? act.tasks : [];
      for (const t of tasks) {
        const li = document.createElement("li");
        const name = t.name || "task";
        const state = t.state || "...";
        const detail = t.detail ? " ... " + t.detail : "";
        li.textContent = name + " [" + state + "]" + detail;
        ul.appendChild(li);
      }
    }
  }

  function applyIbdFocusMode(s) {
    const focus = !!(s && s.dogego_ibd_focus);
    document.body.classList.toggle("ibd-focus", focus);
    const ban = $("an-ibd-focus-banner");
    if (ban) ban.hidden = !focus;
  }

  function applySyncProgress(s) {
    applyIbdFocusMode(s);
    const tip = Number(s.tip_height);
    const active = chainActiveHeight(s);
    const cont = Number(s.contiguous_raw_height);
    let pct = syncProgressPct(s);
    updateOverviewHero(s);
    const bar = $("sync-bar");
    const heroBar = $("ov-sync-bar");
    const pctEl = $("sync-pct");
    if (bar) bar.style.width = pct + "%";
    if (heroBar) heroBar.style.width = pct + "%";
    if (pctEl) pctEl.textContent = String(pct);
    const contEl = $("sync-contig");
    if (contEl) {
      if (active >= 0 && isFinite(tip)) contEl.textContent = active + " / " + tip + " connected";
      else contEl.textContent = "...";
    }
    const storedEl = $("sync-stored");
    if (storedEl) {
      if (cont >= 0) storedEl.textContent = String(cont);
      else storedEl.textContent = "...";
    }
    const orphanN = Number(s.orphan_raw_blocks_estimate);
    const orphanWrap = $("sync-orphan-wrap");
    const orphanEl = $("sync-orphan");
    if (orphanWrap && orphanEl) {
      const showOrphan = isFinite(orphanN) && orphanN > 0 && isFinite(cont) && cont >= 0;
      orphanWrap.hidden = !showOrphan;
      if (showOrphan) orphanEl.textContent = String(orphanN);
    }
    const anSync = $("an-sync-pct");
    if (anSync) {
      setUIPending(anSync, false);
      anSync.textContent = pct + "%";
    }
    const ibdLive = $("sync-ibd-live");
    const behind = Number(s.blocks_behind_headers);
    const rate = Number(s.blocks_per_minute);
    const inflight = Number(s.in_flight_batches);
    const workers = Number(s.sync_workers);
    const showIbd = s.ibd_active === true || (isFinite(behind) && behind > 32 && pct < 100);
    if (ibdLive) ibdLive.hidden = !showIbd;
    const behindEl = $("sync-behind");
    if (behindEl) behindEl.textContent = isFinite(behind) && behind > 0 ? behind.toLocaleString() : "0";
    const rateEl = $("sync-rate");
    if (rateEl) {
      if (isFinite(rate) && rate > 0) rateEl.textContent = rate.toFixed(1) + " blk/min";
      else rateEl.textContent = "...";
    }
    const inflightEl = $("sync-inflight");
    if (inflightEl) inflightEl.textContent = isFinite(inflight) ? String(inflight) : "...";
    const workersEl = $("sync-workers");
    if (workersEl) workersEl.textContent = isFinite(workers) && workers > 0 ? String(workers) : "...";
    const pool = Number(s.assist_peer_pool);
    const poolEl = $("sync-assist-pool");
    if (poolEl) poolEl.textContent = isFinite(pool) && pool > 0 ? String(pool) : "...";
    const cpProbe = Number(s.dogego_checkpoint_probe);
    const cpEl = $("sync-checkpoint-probe");
    if (cpEl) cpEl.textContent = isFinite(cpProbe) && cpProbe >= 0 ? cpProbe.toLocaleString() : "...";
    const feed = Number(s.discovery_feed_size);
    const feedEl = $("sync-discovery-feed");
    if (feedEl) feedEl.textContent = isFinite(feed) && feed > 0 ? String(feed) : "...";
    const statusLine = $("sync-status-line");
    if (statusLine) statusLine.textContent = s.sync_status_line || "";
    const etaEl = $("sync-eta");
    if (etaEl) {
      const eta = s.sync_eta;
      etaEl.textContent = eta || (showIbd ? "estimating…" : "...");
    }
    const syncMp = $("sync-mempool");
    if (syncMp) syncMp.textContent = isFinite(Number(s.mempool_txs)) ? String(s.mempool_txs) : "0";
    if (pct >= 100 && !showIbd && Number(s.mempool_txs) > 0) {
      const sub = $("ov-sync-sub");
      if (sub && !s.sync_status_line) {
        sub.textContent = Number(s.mempool_txs).toLocaleString() + " mempool txs";
        sub.hidden = false;
      }
    }
    const connLagEl = $("sync-connect-lag");
    if (connLagEl) {
      const lag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
      connLagEl.textContent = isFinite(lag) && lag > 0 ? lag.toLocaleString() : "0";
    }
    const connRateEl = $("sync-connect-rate");
    if (connRateEl) {
      const cr = Number(s.dogego_connect_blocks_per_minute);
      connRateEl.textContent = isFinite(cr) && cr > 0 ? cr.toFixed(1) + " blk/min" : "...";
    }
    applyIbdPhaseCard(s);
    applyUtxoReplayUI(s);
    applyOverviewResumeCard(s);
    applyOverviewCoreCards(s);
    if (s.dogego_operator_cert_total == null) void loadCoreStatus();
    if (s.dogego_mempool_offline_corpus_total != null || s.dogego_operator_cert_total != null) {
      const certCard = $("ov-operator-cert-card");
      if (certCard) certCard.hidden = false;
    }
    if (s.dogego_operator_cert_total != null) {
      applyOverviewOperatorCert(null);
      applyOverviewMempoolCorpus(s, null);
      if (window.DogeGoSyncDock && window.DogeGoSyncDock.setOperatorCert) {
        window.DogeGoSyncDock.setOperatorCert({
          live_ok: !!s.dogego_operator_cert_live_ok,
          solo_ok: !!s.dogego_operator_cert_solo_ok,
          solo_pass: Number(s.dogego_operator_cert_solo_pass) || 0,
          pass: Number(s.dogego_operator_cert_pass) || 0,
          total: Number(s.dogego_operator_cert_total) || 0,
          cached: !!s.dogego_operator_cert_cached,
          corpus_ok: !!s.dogego_mempool_offline_corpus_ok,
          corpus_passed: Number(s.dogego_mempool_offline_corpus_passed) || 0,
          corpus_total: Number(s.dogego_mempool_offline_corpus_total) || 0
        });
      }
    }
    if (s.dogego_mempool_offline_corpus_total != null) {
      applyOverviewMempoolCorpus(s, null);
    }
    applySyncActivity(s);
    if (window.DogeGoSyncDock) window.DogeGoSyncDock.update(s);
  }

  let overviewLogsVisible = false;

  function setOverviewLogsVisible(on) {
    overviewLogsVisible = !!on;
    const wrap = $("ov-log-wrap");
    const btn = $("ov-log-toggle");
    if (wrap) wrap.hidden = !overviewLogsVisible;
    if (btn) {
      btn.innerHTML = '<span class="material-icons-round" aria-hidden="true">terminal</span> ' +
        (overviewLogsVisible ? "Hide sync logs" : "Show sync logs");
    }
  }

  function renderOnboardingChecklist(s) {
    if (!s) return;
    const net = String(s.network || s.chain || "").toLowerCase();
    const netEl = $("ov-step-network");
    if (netEl) {
      if (net === "mainnet") netEl.textContent = "1. Network: mainnet selected (production chain).";
      else if (net === "testnet") netEl.textContent = "1. Network: testnet selected (safe for first runs).";
      else netEl.textContent = "1. Network: " + (net || "unknown");
    }

    const syncEl = $("ov-step-sync");
    if (syncEl) {
      const pct = syncProgressPct(s);
      if (pct >= 100) syncEl.textContent = "2. Sync progress: complete. Node is ready.";
      else syncEl.textContent = "2. Sync progress: " + pct + "% complete. Leave node running to continue.";
    }

    const ibdStep = $("ov-step-ibd");
    if (ibdStep) {
      const mode = String(s.node_mode || "full").toLowerCase();
      const show = mode !== "spv" && (s.ibd_active || s.initialblockdownload);
      ibdStep.hidden = !show;
      if (show) {
        if (connectLagDominant(s)) {
          const lag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
          ibdStep.textContent = "3. Block sync: connecting stored bodies (" + (isFinite(lag) ? lag.toLocaleString() : "?") + " ahead of chainActive).";
        } else {
          const rate = Number(s.blocks_per_minute);
          ibdStep.textContent = isFinite(rate) && rate > 0
            ? "3. Block sync: downloading bodies (~" + rate.toFixed(1) + " blk/min)."
            : "3. Block sync: downloading block bodies from peers.";
        }
      }
    }

    const out = Number(s.connections_out) || 0;
    const inn = Number(s.connections_in) || 0;
    const total = out + inn;
    const peersEl = $("ov-step-peers");
    if (peersEl) {
      const hint = s.inbound_hint || "";
      if (total > 0) {
        let line = "4. Peer connectivity: connected (" + total + " peers, " + out + " outbound";
        if (inn > 0) line += ", " + inn + " inbound";
        line += ").";
        if (hint && inn === 0) line += " " + hint;
        peersEl.textContent = line;
      } else {
        peersEl.textContent = "4. Peer connectivity: waiting for peers (check firewall and P2P mode).";
      }
    }

    const walletEl = $("ov-step-wallet");
    if (walletEl) {
      const mode = String(s.node_mode || "full").toLowerCase();
      if (s.nowallet === true) walletEl.textContent = "5. Wallet readiness: disabled in config (enable under Settings → Wallet).";
      else if (mode === "spv") walletEl.textContent = "5. Wallet readiness: SPV mode active (send/explorer features are limited).";
      else walletEl.textContent = "5. Wallet readiness: available. Use Receive to get an address.";
    }
  }

  function destroyChart(c) {
    if (c) try { c.destroy(); } catch (_) {}
    return null;
  }

  /** Update Chart.js in place to avoid flicker on poll; recreate only when canvas or type changes. */
  function upsertChart(chartRef, canvas, config) {
    if (!canvas || typeof Chart === "undefined") return destroyChart(chartRef);
    if (!config) return destroyChart(chartRef);
    if (chartRef && chartRef.canvas === canvas) {
      chartRef.data = config.data;
      if (config.options) chartRef.options = config.options;
      if (config.type && chartRef.config.type !== config.type) {
        chartRef.config.type = config.type;
      }
      chartRef.update("none");
      return chartRef;
    }
    chartRef = destroyChart(chartRef);
    return new Chart(canvas, config);
  }

  function setNavOpen(open) {
    const shell = $("app-shell");
    if (shell) shell.classList.toggle("nav-open", open);
    const bd = $("nav-backdrop");
    const btn = $("nav-toggle");
    const bottomMenu = $("bottom-nav-menu");
    if (bd) {
      bd.classList.toggle("show", open);
      bd.hidden = !open;
      bd.setAttribute("aria-hidden", open ? "false" : "true");
    }
    if (btn) btn.setAttribute("aria-expanded", open ? "true" : "false");
    if (bottomMenu) {
      bottomMenu.classList.toggle("active", open);
      bottomMenu.setAttribute("aria-expanded", open ? "true" : "false");
      const icon = bottomMenu.querySelector(".bottom-nav-menu-icon");
      if (icon) icon.textContent = open ? "close" : "menu";
      const label = bottomMenu.querySelector("span:not(.material-icons-round)");
      if (label && window.DogeGoI18n) {
        label.textContent = window.DogeGoI18n.t(open ? "nav.menuClose" : "nav.menu");
      } else if (label) {
        label.textContent = open ? "Close" : "Menu";
      }
      const aria = open
        ? ((window.DogeGoI18n && window.DogeGoI18n.t("nav.menuClose")) || "Close menu")
        : ((window.DogeGoI18n && window.DogeGoI18n.t("nav.menu")) || "Menu");
      bottomMenu.setAttribute("aria-label", aria);
    }
  }

  const LS_SIDEBAR_COLLAPSED = "dogego_sidebar_collapsed";
  function initSidebarCollapse() {
    const shell = $("app-shell");
    if (!shell) return;
    const mobileMq = window.matchMedia("(max-width: 900px)");

    function isMobile() {
      return mobileMq.matches;
    }

    function updateMenuLabel() {
      const btn = $("nav-toggle");
      if (!btn || isMobile()) return;
      const collapsed = shell.classList.contains("sidebar-collapsed");
      const label = collapsed ? "Pin sidebar open" : "Collapse to icons";
      btn.title = label;
      btn.setAttribute("aria-label", label);
    }

    function setDesktopCollapsed(collapsed) {
      shell.classList.toggle("sidebar-collapsed", collapsed);
      document.body.classList.toggle("sidebar-collapsed", collapsed);
      try {
        localStorage.setItem(LS_SIDEBAR_COLLAPSED, collapsed ? "1" : "0");
      } catch (_) { /* */ }
      updateMenuLabel();
      if (isPanelActive("analytics")) {
        requestAnimationFrame(() => {
          setTimeout(() => fitAnKpiStats(), 50);
        });
      }
    }

    function toggleSidebar() {
      if (isMobile()) {
        setNavOpen(!shell.classList.contains("nav-open"));
        return;
      }
      setDesktopCollapsed(!shell.classList.contains("sidebar-collapsed"));
    }

    function applyDesktopPref() {
      if (isMobile()) {
        shell.classList.remove("sidebar-collapsed");
        document.body.classList.remove("sidebar-collapsed");
        return;
      }
      let collapsed = true;
      try {
        if (localStorage.getItem(LS_SIDEBAR_COLLAPSED) === "0") collapsed = false;
      } catch (_) { /* */ }
      shell.classList.toggle("sidebar-collapsed", collapsed);
      document.body.classList.toggle("sidebar-collapsed", collapsed);
      updateMenuLabel();
    }

    applyDesktopPref();
    mobileMq.addEventListener("change", () => {
      setNavOpen(false);
      applyDesktopPref();
    });

    const menuBtn = $("nav-toggle");
    if (menuBtn) {
      menuBtn.addEventListener("click", (e) => {
        e.preventDefault();
        toggleSidebar();
      });
    }
    const bottomMenu = $("bottom-nav-menu");
    if (bottomMenu) {
      bottomMenu.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (!isMobile()) return;
        setNavOpen(!shell.classList.contains("nav-open"));
      });
    }
  }

  function renderMempoolChart(canvas, chartRef, sigKey, sizes) {
    if (!canvas || typeof Chart === "undefined") return chartRef;
    if (sizes.length === 0) {
      if (sigKey === "panel") sigMempoolPanel = "";
      return destroyChart(chartRef);
    }
    const sig = sizes.join(",");
    if (sigKey === "panel" && sig === sigMempoolPanel && chartRef) return chartRef;
    if (sigKey === "panel") sigMempoolPanel = sig;
    return upsertChart(chartRef, canvas, {
      type: "bar",
      data: {
        labels: sizes.map((_, i) => String(i + 1)),
        datasets: [{ label: "vsize (bytes)", data: sizes, backgroundColor: chartColors.accentFill, borderColor: chartColors.accent, borderWidth: 1, borderRadius: 6 }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        plugins: modernChartPlugins(),
        scales: {
          x: { display: false, grid: { color: chartColors.grid } },
          y: { beginAtZero: true, grid: { color: chartColors.grid }, ticks: { font: { family: CHART_FONT, size: 11 } } },
        },
      },
    });
  }

  function formatRelayDOGE(v) {
    if (v == null || v === "") return "...";
    const n = Number(v);
    if (Number.isNaN(n)) return String(v);
    return n.toFixed(8).replace(/\.?0+$/, "") + " DOGE/kB";
  }

  let mpTxCache = [];
  let mpTxFilter = "";

  function renderMempoolTxRows(txs, filter) {
    const q = String(filter || "").trim().toLowerCase();
    const rows = (txs || []).filter((t) => {
      const id = String(t.txid || t.hash || "").toLowerCase();
      return !q || id.includes(q);
    });
    if (!rows.length) {
      return "<tr><td colspan=\"4\" class=\"label\">" + (q ? "No matching transactions" : "Mempool empty") + "</td></tr>";
    }
    return rows.slice(0, 200).map((t, i) => {
      const id = t.txid || t.hash || "...";
      const vs = t.vsize != null ? t.vsize : (t.size != null ? t.size : "...");
      const fee = t.fees && t.fees.base != null ? formatRelayDOGE(t.fees.base) : "...";
      return "<tr class=\"mp-tx-row\" data-txid=\"" + escHtml(id) + "\" title=\"Open in BlockStep\"><td>" + (i + 1) + "</td><td class=\"mono\">" + escHtml(id) + "</td><td>" + vs + "</td><td>" + fee + "</td></tr>";
    }).join("");
  }

  function bindMempoolTxRows() {
    const tbody = $("mp-tx-body");
    if (!tbody) return;
    tbody.querySelectorAll(".mp-tx-row").forEach((row) => {
      row.addEventListener("click", () => {
        const txid = row.getAttribute("data-txid");
        if (txid) goBlockStepTx(txid);
      });
    });
  }

  async function loadMempoolDetail() {
    const r = await fetchAPI("/api/mempool?limit=500", 12000);
    if (!r.ok) return null;
    return r.json();
  }

  function fillMempoolPanel(mp) {
    const line = $("mp-line1");
    const size = mp.size != null ? mp.size : 0;
    const bytes = mp.bytes != null ? mp.bytes : 0;
    const bytesLabel = (bytes / 1024).toFixed(1) + " KB";
    if (line) line.textContent = size + " tx · " + bytesLabel;
    if ($("mp-hero-count")) $("mp-hero-count").textContent = String(size);
    if ($("mp-hero-bytes")) $("mp-hero-bytes").textContent = bytesLabel;
    const strip = $("mp-status-strip");
    const phase = $("mp-status-phase");
    if (strip && phase) {
      const paused = !!mp.paused;
      strip.classList.toggle("mp-paused-state", paused);
      phase.innerHTML =
        '<span class="material-icons-round" aria-hidden="true">' + (paused ? "pause_circle" : "check_circle") + "</span> " +
        (paused ? "Mempool admission paused" : size > 0 ? "Relaying unconfirmed txs" : "Mempool empty on this node");
    }
    if ($("mp-minrelay")) $("mp-minrelay").textContent = formatRelayDOGE(mp.mempoolminfee != null ? mp.mempoolminfee : mp.minrelaytxfee);
    if ($("mp-config-minrelay")) $("mp-config-minrelay").textContent = formatRelayDOGE(mp.minrelaytxfee);
    if ($("mp-incr")) $("mp-incr").textContent = formatRelayDOGE(mp.incrementalrelayfee);
    if ($("mp-paused")) $("mp-paused").textContent = mp.paused ? "paused" : "active";
    const pkg = mp.package_policy;
    const pkgLine = $("mp-package-line");
    if (pkgLine && pkg) {
      pkgLine.textContent =
        "Package: " + pkg.limitancestorcount + " anc / " + pkg.limitdescendantcount + " desc · " +
        pkg.limitancestorsize + "/" + pkg.limitdescendantsize + " kB";
    } else if (pkgLine) pkgLine.textContent = "";
    const std = mp.standard_policy;
    if (pkgLine && std) {
      const extra = " · OP_RETURN " + (std.acceptdatacarrier ? "on" : "off") +
        " · bare multisig " + (std.permitbaremultisig ? "on" : "off");
      pkgLine.textContent = (pkgLine.textContent || "Standardness") + extra;
    }
    const pctLine = $("mp-fee-pct");
    const feeGrid = $("mp-fee-grid");
    const pct = mp.feerate_percentiles;
    if (pct && pct.length === 5) {
      const ids = ["mp-fee-p10", "mp-fee-p25", "mp-fee-p50", "mp-fee-p75", "mp-fee-p90"];
      ids.forEach((id, i) => {
        const el = $(id);
        if (el) el.textContent = formatRelayDOGE(pct[i]);
      });
      if (feeGrid) feeGrid.hidden = false;
      if (pctLine) {
        pctLine.textContent =
          "Feerate spread: " + pct.map((v) => formatRelayDOGE(v)).join(" · ");
      }
    } else {
      if (feeGrid) feeGrid.hidden = true;
      if (pctLine) pctLine.textContent = "";
    }
    const note = $("mp-policy-note");
    if (note) note.textContent = mp.dogego_note || "";
    const anMp = $("an-mempool");
    if (anMp) {
      setCompactStat(anMp, size, { integer: true, suffix: " tx · " + fmtBytes(bytes) });
    }
    if ($("mp-count")) $("mp-count").textContent = String(size);
    if ($("mp-bytes")) $("mp-bytes").textContent = (bytes / 1024).toFixed(1) + " KB";
    const orphans = Number(mp.dogego_orphan_tx_count) || 0;
    if ($("mp-orphan-count")) $("mp-orphan-count").textContent = String(orphans);
    const syncNote = $("mp-sync-note");
    if (syncNote) {
      let note = mp.dogego_sync_note || "";
      if (!note && orphans > 0 && size === 0) {
        note = orphans + " tx waiting for parent blocks (orphan pool - normal during sync)";
      }
      if (!note && size === 0 && !paused && lastSummary && lastSummary.initialblockdownload) {
        note = "This node is still syncing (IBD). Peers rarely relay txs until block download catches up; an empty local mempool is normal.";
      }
      if (!note && size === 0 && !paused) {
        note = "No unconfirmed txs on this node yet. The mempool shows what peers relayed here - not the whole network. Try Console → getrawmempool after sync completes.";
      }
      syncNote.textContent = note;
      syncNote.hidden = !note;
    }
    const txs = mp.transactions || [];
    mpTxCache = txs;
    const tbody = $("mp-tx-body");
    if (tbody) {
      tbody.innerHTML = renderMempoolTxRows(txs, mpTxFilter);
      bindMempoolTxRows();
    }
    const sizes = txs.map((t) => (t.vsize != null ? t.vsize : t.size) || 0).filter((n) => n > 0);
    const canvas = $("chart-mempool");
    const ovDist = $("chart-ov-mempool-dist");
    if (canvas) {
      if (sizes.length === 0) {
        sigMempool = "";
        chartMempool = destroyChart(chartMempool);
      } else {
        const sig = sizes.join(",");
        if (sig !== sigMempool || !chartMempool) {
          sigMempool = sig;
          chartMempool = renderMempoolChart(canvas, chartMempool, "ov", sizes);
        }
      }
    }
    if (ovDist) {
      if (sizes.length === 0) {
        chartOvDashMempoolDist = destroyChart(chartOvDashMempoolDist);
      } else {
        chartOvDashMempoolDist = renderMempoolChart(ovDist, chartOvDashMempoolDist, "ov-dash", sizes);
      }
    }
    const panCanvas = $("chart-mempool-panel");
    const pan = document.getElementById("panel-mempool");
    if (panCanvas && pan && pan.classList.contains("active")) {
      chartMempoolPanel = renderMempoolChart(panCanvas, chartMempoolPanel, "panel", sizes);
    }
  }

  function statusPill(status) {
    const s = (status || "planned").toLowerCase();
    return "<span class=\"status-pill " + s + "\">" + s + "</span>";
  }

  function p2pHealthLabel(h) {
    const m = { ok: "Relay OK", warming: "Warming up", starting: "Connecting", degraded: "Needs peers", single: "Single peer" };
    return m[h] || h || "...";
  }

  function shortenPeerLabel(peer) {
    const p = String(peer || "").trim();
    if (!p) return "...";
    const low = p.toLowerCase();
    if (low.includes("connecting") || low.includes("dialing") || low.includes("handshaking") || low.includes("dns seed") || low.includes("starting")) {
      return "Connecting";
    }
    if (low.startsWith("solo")) return "Solo";
    if (low.includes("header catch-up") || low.includes("block-assist")) return "Syncing";
    if (p.startsWith("(")) return "Connecting";
    return p.split(/\s+/)[0] || p;
  }

  function formatTopbarPeer(s, p2snap) {
    const snap = p2snap || {};
    const primary = String(snap.primary_peer || snap.peer_addr || (s && s.primary_peer) || "").trim();
    if (primary) return primary;
    return shortenPeerLabel((s && s.peer) || "");
  }

  function dgrHealthLabel(h) {
    const m = { ok: "OK", warming: "Warming up", starting: "Starting", degraded: "Needs attention", off: "Off" };
    return m[h] || h || "...";
  }

  function dgrRoleLabel(dgr) {
    if (!dgr || !dgr.enabled) return i18n("dgr.roleOff");
    if (dgr.inbound_relay) return i18n("dgr.roleInbound");
    if (dgr.outbound_relay) return i18n("dgr.roleOutbound");
    if (dgr.using_relay) return i18n("dgr.roleOutbound");
    return i18n("dgr.roleOn");
  }

  function formatDGRUptime(sec) {
    if (sec == null || sec < 0) return "...";
    if (sec < 60) return sec + "s";
    if (sec < 3600) return Math.floor(sec / 60) + "m";
    return Math.floor(sec / 3600) + "h " + Math.floor((sec % 3600) / 60) + "m";
  }

  function formatDGRSha256(fp) {
    const s = String(fp || "").trim();
    if (!s) return "-";
    if (s.length <= 20) return s;
    return s.slice(0, 10) + "…" + s.slice(-10);
  }

  function fillDGRSecurityMetrics(prefix, d) {
    const p = prefix || "ov";
    const setCert = (id, val) => {
      const el = $(id);
      if (!el) return;
      const full = String(val || "").trim();
      el.textContent = formatDGRSha256(full);
      el.title = full || "";
    };
    setCert(p + "-dgr-server-cert", d.server_cert_sha256);
    setCert(p + "-dgr-relay-cert", d.active_relay_cert_sha256);
    const tlsEl = $(p + "-dgr-tls-pin");
    if (tlsEl) {
      const ok = d.tls_pin_ok != null ? d.tls_pin_ok : 0;
      const fail = d.tls_pin_fail != null ? d.tls_pin_fail : 0;
      tlsEl.textContent = ok + " / " + fail;
    }
    const rateEl = $(p + "-dgr-rate-limited");
    if (rateEl) rateEl.textContent = String(d.rate_limited != null ? d.rate_limited : 0);
    if (p === "st") {
      const copyBtn = $("st-dgr-copy-server-cert");
      const useBtn = $("st-dgr-use-server-cert");
      const hasServer = !!String(d.server_cert_sha256 || "").trim();
      if (copyBtn) copyBtn.hidden = !hasServer;
      if (useBtn) useBtn.hidden = !hasServer;
    }
  }

  function fillDGRCard(prefix, dgr, opts) {
    const p = prefix || "ov";
    const d = dgr || {};
    const show = !!(opts && opts.forceShow) || !!d.enabled || !!d.inbound_relay || !!d.outbound_relay || !!d.using_relay || (opts && opts.p2pMode && (opts.p2pMode === "cgnat" || opts.p2pMode === "both" || opts.p2pMode === "classic"));
    const card = $(p === "ov" ? "ov-dgr-card" : "st-dgr-live");
    if (card) card.hidden = !show;
    if (!show) return;
    const health = d.health || (d.starting ? "starting" : d.enabled ? "warming" : "off");
    const healthEl = $(p + "-dgr-health");
    if (healthEl) {
      healthEl.textContent = dgrHealthLabel(health);
      healthEl.className = "p2p-health-pill " + (health === "ok" ? "ok" : health === "degraded" ? "degraded" : health === "off" ? "single" : "warming");
    }
    const roleEl = $(p + "-dgr-role");
    if (roleEl) roleEl.textContent = dgrRoleLabel(d);
    const msgEl = $(p + "-dgr-health-msg");
    if (msgEl) msgEl.textContent = d.health_message || (d.starting ? i18n("dgr.starting") : "");
    if (p === "ov") {
      if ($("ov-dgr-uptime")) $("ov-dgr-uptime").textContent = formatDGRUptime(d.uptime_seconds);
      if ($("ov-dgr-listen-cfg")) $("ov-dgr-listen-cfg").textContent = d.listen || "-";
      if ($("ov-dgr-listen-bound")) $("ov-dgr-listen-bound").textContent = d.listen_bound || (d.listener_ok ? d.listen : "-");
      if ($("ov-dgr-advertise")) $("ov-dgr-advertise").textContent = d.advertise_addr || "-";
      if ($("ov-dgr-port")) $("ov-dgr-port").textContent = d.relay_port != null ? String(d.relay_port) : "24433";
      if ($("ov-dgr-bit")) $("ov-dgr-bit").textContent = d.service_bit_hex || "-";
      if ($("ov-dgr-discovery")) {
        const n = Array.isArray(d.discovery_targets) ? d.discovery_targets.length : 0;
        $("ov-dgr-discovery").textContent = String(n);
      }
      if ($("ov-dgr-dial")) $("ov-dgr-dial").textContent = (d.dial_ok != null ? d.dial_ok : 0) + " / " + (d.dial_fail != null ? d.dial_fail : 0);
      const warn = $("ov-dgr-warn");
      if (warn) {
        const bad = health === "degraded" || (d.outbound_relay && !d.using_relay && (d.dial_attempts || 0) > 2);
        warn.hidden = !bad;
        warn.textContent = bad ? (d.health_message || i18n("dgr.warnNoRelay")) : "";
      }
    }
    if ($("ov-dgr-using") || $(p + "-dgr-using")) {
      const el = $(p + "-dgr-using") || $("ov-dgr-using");
      if (el) el.textContent = d.using_relay ? i18n("common.yes") : i18n("common.no");
    }
    if ($("ov-dgr-active") || $(p + "-dgr-active")) {
      const el = $(p + "-dgr-active") || $("ov-dgr-active");
      if (el) el.textContent = d.active_relay || "-";
    }
    if ($("ov-dgr-clients") || $(p + "-dgr-clients")) {
      const el = $(p + "-dgr-clients") || $("ov-dgr-clients");
      if (el) el.textContent = String(d.registered_clients != null ? d.registered_clients : 0);
    }
    if (p === "st") {
      if ($("st-dgr-listen-bound")) $("st-dgr-listen-bound").textContent = d.listen_bound || "-";
      if ($("st-dgr-advertise")) $("st-dgr-advertise").textContent = d.advertise_addr || "-";
      if ($("st-dgr-discovery")) {
        const n = Array.isArray(d.discovery_targets) ? d.discovery_targets.length : 0;
        $("st-dgr-discovery").textContent = String(n);
      }
    }
    fillDGRSecurityMetrics(p, d);
  }

  let dgrLiveCache = null;
  async function refreshDGRLive() {
    const liveCard = $("st-dgr-live");
    if (!liveCard || liveCard.hidden) return;
    try {
      const r = await fetch("/api/dgr", { cache: "no-store" });
      if (!r.ok) return;
      dgrLiveCache = await r.json();
      fillDGRCard("st", dgrLiveCache, { forceShow: true });
    } catch (_) {}
  }

  let lanPeerHintCache = null;

  function normalizeLanPeerTarget(raw, defaultPort) {
    const s = String(raw || "").trim();
    if (!s) return "";
    if (s.includes(":")) return s;
    const port = defaultPort > 0 ? defaultPort : 44556;
    return s + ":" + port;
  }

  function fillLanPeerCard(hint) {
    const h = hint || {};
    lanPeerHintCache = h;
    const shareEl = $("st-lan-share");
    const noteEl = $("st-lan-peer-note");
    const otherEl = $("st-lan-other");
    const targets = Array.isArray(h.share_targets) ? h.share_targets : [];
    const share = targets[0] || (Array.isArray(h.lan_ipv4) && h.lan_ipv4[0] && h.p2p_port
      ? h.lan_ipv4[0] + ":" + h.p2p_port
      : "");
    if (shareEl) shareEl.textContent = share || "(no LAN IPv4 detected - use ipconfig)";
    if (noteEl && h.note) noteEl.textContent = h.note;
    if (otherEl && !otherEl.value.trim() && h.p2p_port) {
      otherEl.placeholder = "192.168.1.214:" + h.p2p_port;
    }
  }

  async function loadLanPeerHint() {
    const card = $("st-lan-peer-card");
    if (!card) return;
    try {
      const r = await fetch("/api/lan-peer-hint", { cache: "no-store" });
      if (!r.ok) return;
      fillLanPeerCard(await r.json());
    } catch (_) {}
  }

  async function addLanPeerNow() {
    const status = $("st-lan-status");
    const otherEl = $("st-lan-other");
    const raw = otherEl ? otherEl.value : "";
    const port = lanPeerHintCache && lanPeerHintCache.p2p_port ? lanPeerHintCache.p2p_port : 44556;
    const target = normalizeLanPeerTarget(raw, port);
    if (!target) {
      if (status) status.textContent = "Enter the other PC's LAN IP or host:port.";
      return;
    }
    if (status) status.textContent = "Calling addnode…";
    try {
      const r = await fetch("/api/peers", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "add", node: target }),
      });
      const data = await r.json().catch(() => ({}));
      if (!r.ok || data.ok === false) throw new Error(data.error || ("HTTP " + r.status));
      data._loadedAt = Date.now();
      lastAnalyticsPeersCache = data;
      renderAnalyticsPeers(data);
      if (status) status.textContent = "Added " + target + " (saved; dialing now when P2P is multi-peer).";
      refresh();
    } catch (e) {
      if (status) status.textContent = String(e.message || e);
    }
  }

  let dgrUserTouched = false;

  function dgrRoleFromForm() {
    const en = $("st-dgr-enabled") && $("st-dgr-enabled").checked;
    const inbound = $("st-dgr-inbound") && $("st-dgr-inbound").checked;
    const outbound = $("st-dgr-outbound") && $("st-dgr-outbound").checked;
    if (!en) return "off";
    if (inbound && outbound) return "both";
    if (inbound) return "operator";
    if (outbound) return "client";
    return "on";
  }

  function applyDGRRole(role, fromUser) {
    if (fromUser) dgrUserTouched = true;
    const en = $("st-dgr-enabled");
    const inbound = $("st-dgr-inbound");
    const outbound = $("st-dgr-outbound");
    if (role === "off") {
      if (en) en.checked = false;
      if (inbound) inbound.checked = false;
      if (outbound) outbound.checked = false;
    } else if (role === "client") {
      if (en) en.checked = true;
      if (inbound) inbound.checked = false;
      if (outbound) outbound.checked = true;
    } else if (role === "operator") {
      if (en) en.checked = true;
      if (inbound) inbound.checked = true;
      if (outbound) outbound.checked = false;
    }
    syncDGRRoleCards();
    updateDGRFieldsVisibility();
    suggestDGRForP2PMode();
  }

  function syncDGRRoleCards() {
    const role = dgrRoleFromForm();
    let cardRole = role;
    if (role === "on" || role === "both") cardRole = "";
    document.querySelectorAll("[data-dgr-role]").forEach((btn) => {
      const r = btn.getAttribute("data-dgr-role");
      const on = cardRole !== "" && r === cardRole;
      btn.classList.toggle("selected", on);
      btn.setAttribute("aria-checked", on ? "true" : "false");
    });
  }

  function updateDGRFieldsVisibility() {
    const role = dgrRoleFromForm();
    const clientOn = role === "client" || role === "both" || role === "on";
    const opOn = role === "operator" || role === "both";
    document.querySelectorAll(".dgr-client-only").forEach((el) => { el.hidden = !clientOn; });
    document.querySelectorAll(".dgr-operator-only").forEach((el) => { el.hidden = !opOn; });
  }

  function suggestDGRForP2PMode() {
    const hint = $("st-dgr-p2p-hint");
    if (!hint) return;
    if (dgrUserTouched) {
      hint.hidden = true;
      hint.textContent = "";
      return;
    }
    const p2p = $("st-p2p") && $("st-p2p").value;
    const role = dgrRoleFromForm();
    if (p2p === "cgnat" && role === "off") {
      hint.textContent = i18n("settings.dgrSuggestCgnat");
      hint.hidden = false;
      return;
    }
    if ((p2p === "both" || p2p === "classic") && role === "off") {
      hint.textContent = i18n("settings.dgrSuggestBoth");
      hint.hidden = false;
      return;
    }
    hint.hidden = true;
    hint.textContent = "";
  }

  function syncDGRWithP2PMode() {
    suggestDGRForP2PMode();
  }

  function fillP2PCard(s, p2) {
    const snap = p2 && p2.wired !== false ? p2 : {};
    const mode = snap.p2p_connectivity || s.p2p_connectivity || "both";
    const health = snap.health || s.p2p_health || "starting";
    const msg = snap.health_message || s.relay_note || "";
    const elMode = $("ov-p2p-mode");
    if (elMode) elMode.textContent = mode;
    const elHealth = $("ov-p2p-health");
    if (elHealth) {
      elHealth.textContent = p2pHealthLabel(health);
      elHealth.className = "p2p-health-pill " + health;
    }
    const elMsg = $("ov-p2p-health-msg");
    const hint = snap.inbound_hint || s.inbound_hint || "";
    if (elMsg) {
      elMsg.textContent = hint ? (msg ? msg + " " + hint : hint) : msg;
    }
    const total = snap.connections_total != null ? snap.connections_total : (Number(s.connections_out) || 0) + (Number(s.connections_in) || 0);
    if ($("ov-p2p-total")) $("ov-p2p-total").textContent = String(total);
    if ($("ov-p2p-listen")) $("ov-p2p-listen").textContent = snap.listen_enabled === true ? "on" : snap.listen_enabled === false ? "off" : "...";
    const upnpEl = $("ov-p2p-upnp");
    if (upnpEl) {
      if (snap.upnp_mapped) {
        upnpEl.textContent = (snap.upnp_external || "?") + (snap.upnp_method ? " (" + snap.upnp_method + ")" : "");
      } else {
        upnpEl.textContent = "...";
      }
    }
    const primary = formatTopbarPeer(s, snap);
    if ($("ov-p2p-primary")) $("ov-p2p-primary").textContent = primary;
    const addrbookEl = $("ov-p2p-addrbook");
    if (addrbookEl) {
      const triedN = snap.addrbook_tried != null ? snap.addrbook_tried : s.addrbook_tried;
      const newN = snap.addrbook_new != null ? snap.addrbook_new : s.addrbook_new;
      if (triedN != null || newN != null) {
        const tried = triedN != null ? triedN : 0;
        const nw = newN != null ? newN : 0;
        let ab = tried + " tried · " + nw + " new";
        const nKey = snap.addrbook_n_key_set != null ? snap.addrbook_n_key_set : s.addrbook_n_key_set;
        if (nKey) ab += " · nKey";
        const cap = snap.addrbook_bucket_slot_cap != null ? snap.addrbook_bucket_slot_cap
          : (s.addrbook_bucket_slot_cap != null ? s.addrbook_bucket_slot_cap : 64);
        const tbMax = snap.addrbook_tried_bucket_max_fill != null ? snap.addrbook_tried_bucket_max_fill : s.addrbook_tried_bucket_max_fill;
        const nbMax = snap.addrbook_new_bucket_max_fill != null ? snap.addrbook_new_bucket_max_fill : s.addrbook_new_bucket_max_fill;
        if (tbMax != null || nbMax != null) {
          ab += " · max fill " + Math.max(tbMax || 0, nbMax || 0) + "/" + cap;
        }
        addrbookEl.textContent = ab;
      } else {
        addrbookEl.textContent = "...";
      }
    }
    const ua = snap.local_user_agent || s.local_user_agent || "";
    const uaEl = $("ov-p2p-ua");
    if (uaEl) {
      uaEl.textContent = ua || "...";
      uaEl.title = ua || "";
    }
    const pendingEl = $("ov-p2p-ua-pending");
    const configSub = (settingsRuntime && settingsRuntime.p2p_subversion) || "";
    if (pendingEl) {
      if (configSub && ua && configSub !== ua) {
        pendingEl.textContent = "Saved config differs - restart node to apply: " + configSub;
        pendingEl.hidden = false;
      } else {
        pendingEl.hidden = true;
        pendingEl.textContent = "";
      }
    }
    const card = $("ov-p2p-card");
    if (card) card.classList.toggle("p2p-cgnat-active", mode === "cgnat");
    const dgr = snap.dogego_relay_cgnat || {};
    fillDGRCard("ov", dgr, { p2pMode: mode, forceShow: mode === "cgnat" || !!dgr.enabled });
    if (coreCompareCache) fillCoreCompareUI("ov", coreCompareCache);
  }

  function i18n(key, params) {
    return (window.DogeGoI18n && window.DogeGoI18n.t(key, params)) || key;
  }

  function parityStatCard(label, value, pillClass) {
    const pill = pillClass ? "<span class=\"status-pill " + pillClass + "\">" + escapeHtml(String(value)) + "</span>" : "<strong class=\"parity-stat-val\">" + escapeHtml(String(value)) + "</strong>";
    return "<div class=\"parity-stat\"><span class=\"parity-stat-label\">" + escapeHtml(label) + "</span>" + pill + "</div>";
  }

  function renderParitySummary(summary) {
    const el = $("feat-parity-summary");
    if (!el || !summary) return;
    const gatePill = summary.standalone_ready ? "live" : "partial";
    const gateLabel = summary.standalone_ready ? i18n("pages.features.parityStandaloneReady") : i18n("pages.features.parityStandalonePartial");
    el.innerHTML =
      "<h2>" + escapeHtml(i18n("pages.features.parityTitle")) + "</h2>" +
      "<div class=\"parity-summary-grid\">" +
      parityStatCard(i18n("pages.features.parityStandalone"), gateLabel, gatePill) +
      parityStatCard(i18n("pages.features.parityFeaturesLive"), summary.features_live || 0, "live") +
      parityStatCard(i18n("pages.features.parityFeaturesPartial"), summary.features_partial || 0, "partial") +
      parityStatCard(i18n("pages.features.parityGapsOpen"), summary.gaps_open || 0, "planned") +
      parityStatCard(i18n("pages.features.parityGapsPartial"), summary.gaps_partial || 0, "partial") +
      parityStatCard(i18n("pages.features.parityGapsDeclined"), summary.gaps_declined || 0, "na") +
      parityStatCard(i18n("pages.features.parityRoadmap"), (summary.roadmap_done || 0) + " / " + (summary.roadmap_total || 0), "") +
      parityStatCard(i18n("pages.features.parityRpcLive"), summary.rpc_live || 0, "live") +
      parityStatCard(i18n("pages.features.parityRpcPartial"), summary.rpc_partial || 0, "partial") +
      parityStatCard(i18n("pages.features.parityRpcStub"), summary.rpc_stub || 0, "planned") +
      (summary.protocol_lock_checked
        ? parityStatCard(i18n("pages.features.parityProtocolLock"),
          summary.protocol_lock_ok ? i18n("pages.features.parityProtocolLockOk") : i18n("pages.features.parityProtocolLockFail"),
          summary.protocol_lock_ok ? "live" : "degraded")
        : "") +
      (summary.offline_corpus_checked
        ? parityStatCard(i18n("pages.features.parityOfflineCorpus"),
          (summary.offline_corpus_passed || 0) + "/" + (summary.offline_corpus_total || 0),
          summary.offline_corpus_ok ? "live" : "degraded")
        : "") +
      "</div>";
    el.removeAttribute("data-doge-wait");
  }

  function renderCoreGuidance(guidance) {
    const el = $("feat-guidance");
    if (!el || !guidance) return;
    function list(title, items) {
      if (!items || !items.length) return "";
      return "<div class=\"guidance-col\"><h3>" + escapeHtml(title) + "</h3><ul class=\"guidance-list\">" +
        items.map((x) => "<li>" + escapeHtml(x) + "</li>").join("") + "</ul></div>";
    }
    let docs = "";
    if (guidance.doc_links && guidance.doc_links.length) {
      docs = "<div class=\"guidance-docs\"><span class=\"label\">" + escapeHtml(i18n("pages.features.docsLink")) + "</span> " +
        guidance.doc_links.map((l) =>
          "<button type=\"button\" class=\"btn btn-ghost btn-sm docs-open-md\" data-doc-path=\"" + escapeHtml(l.path || "") + "\">" + escapeHtml(l.label || l.path || "") + "</button>"
        ).join(" ") + "</div>";
    }
    el.innerHTML =
      "<h2>" + escapeHtml(i18n("pages.features.guidanceTitle")) + "</h2>" +
      "<div class=\"guidance-grid\">" +
      list(i18n("pages.features.guidanceCore"), guidance.use_core_when) +
      list(i18n("pages.features.guidanceDogeGo"), guidance.use_dogego_when) +
      list(i18n("pages.features.guidanceIntentional"), guidance.intentional_diffs) +
      "</div>" + docs;
    el.hidden = false;
    el.querySelectorAll(".docs-open-md").forEach((btn) => {
      btn.addEventListener("click", () => {
        showTab("docs");
        openEmbeddedDoc(btn.getAttribute("data-doc-path") || "");
      });
    });
  }

  function renderCertification(cert) {
    if (!cert) return;
    const disc = $("feat-cert-disclaimer");
    if (disc) disc.textContent = cert.disclaimer || "";
    const corpus = $("feat-cert-corpus");
    if (corpus && cert.corpus) {
      const c = cert.corpus;
      corpus.innerHTML =
        parityStatCard(i18n("pages.features.certCorpusMempool"), c.mempool_vectors || 0, "") +
        parityStatCard(i18n("pages.features.certCorpusProbe"), c.mempool_parity_probe_rows || 0, "") +
        parityStatCard(i18n("pages.features.certCorpusScript"), c.script_tests_legacy || 0, "") +
        parityStatCard(i18n("pages.features.certCorpusHeaders"), c.header_harness_stored || 0, "") +
        parityStatCard(i18n("pages.features.certCorpusBlocks"), c.block_connect_stored || 0, "");
    }
    const root = $("feat-cert-milestones");
    if (root && cert.milestones) {
      root.innerHTML = cert.milestones.map((m) => {
        const st = m.status === "done" ? "live" : (m.status === "open" ? "planned" : "partial");
        let offline = "";
        if (m.offline_tests && m.offline_tests.length) {
          offline = "<p class=\"core-note\"><strong>" + escapeHtml(i18n("pages.features.certOffline")) + ":</strong> " +
            m.offline_tests.map((t) => "<code class=\"mono\">" + escapeHtml(t) + "</code>").join(", ") + "</p>";
        }
        let scripts = "";
        if (m.scripts && m.scripts.length) {
          scripts = "<p class=\"core-note\"><strong>" + escapeHtml(i18n("pages.features.certScripts")) + ":</strong> " +
            m.scripts.map((s) => "<button type=\"button\" class=\"btn btn-ghost btn-sm docs-open-md\" data-doc-path=\"" + escapeHtml(s.path || "") + "\">" + escapeHtml(s.label || s.path || "") + "</button>").join(" ") + "</p>";
        }
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3><span class=\"mono\">" + escapeHtml(m.phase || "") + "</span> - " + escapeHtml(m.title || "") + "</h3>" +
          "<p>" + escapeHtml(m.summary || "") + "</p>" + offline + scripts + "</div></article>";
      }).join("");
      root.querySelectorAll(".docs-open-md").forEach((btn) => {
        btn.addEventListener("click", () => {
          showTab("docs");
          openEmbeddedDoc(btn.getAttribute("data-doc-path") || "");
        });
      });
    }
  }

  function renderCoreProbeAPIs(apis) {
    const card = $("feat-core-probe-apis");
    const list = $("feat-core-probe-apis-list");
    if (!card || !list) return;
    if (!apis || !apis.length) {
      card.hidden = true;
      return;
    }
    card.hidden = false;
    list.innerHTML = apis.map((a) => {
      const mile = a.milestone ? "<span class=\"status-pill partial mono\">" + escapeHtml(a.milestone) + "</span>" : "";
      const bundled = a.bundled ? " · " + escapeHtml(i18n("pages.features.coreProbeBundled")) : "";
      return "<article class=\"feat-row\"><div>" + mile + "</div><div>" +
        "<h3 class=\"mono\">" + escapeHtml(a.path || "") + "</h3>" +
        "<p class=\"label\">" + escapeHtml(a.label || "") + bundled + "</p></div></article>";
    }).join("");
  }

  function fillCoreWalletUI(data) {
    if (!data) return;
    const pill = $("feat-core-wallet-pill");
    const summary = $("feat-core-wallet-summary");
    const checks = $("feat-core-wallet-checks");
    if (pill) {
      if (data.skipped) {
        pill.textContent = i18n("pages.features.coreWalletSkipped");
        pill.className = "p2p-health-pill starting";
      } else if (data.ok && !(data.warnings || []).length) {
        pill.textContent = i18n("pages.features.coreWalletOk");
        pill.className = "p2p-health-pill ok";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreWalletWarn");
        pill.className = "p2p-health-pill warming";
      } else {
        pill.textContent = i18n("pages.features.coreWalletFail");
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      if (data.skipped) {
        summary.textContent = data.reason || "";
      } else {
        const w = data.wallet && typeof data.wallet === "object" ? data.wallet : {};
        summary.textContent =
          (w.walletname != null ? String(w.walletname) : "wallet") +
          (data.balance != null ? " · balance " + data.balance : "") +
          (data.address ? " · " + data.address : "") +
          (data.address_book_count != null ? " · " + data.address_book_count + " addresses" : "") +
          (data.address_book_keypool_count != null && data.address_book_keypool_count > 0
            ? " · keypool " + data.address_book_keypool_count : "") +
          (data.address_book_core_pool_indices_stored != null && data.address_book_core_pool_indices_stored > 0
            ? " · core pool idx " + data.address_book_core_pool_indices_stored : "") +
          (data.address_book_node_tip_count != null && data.address_book_node_tip_count > 0
            ? " · node tip " + data.address_book_node_tip_count : "") +
          (w.keypoolsize != null && w.keypoolsize > 0 ? " · hd keypool " + w.keypoolsize : "") +
          (data.pool_core_indices_stored != null && data.pool_core_indices_stored > 0
            ? " · pool idx stored " + data.pool_core_indices_stored : "") +
          (data.spendable_utxo_count != null ? " · utxos " + data.spendable_utxo_count : "") +
          (data.wallet_index_height != null ? " · indexed " + data.wallet_index_height : "") +
          (data.needs_rescan ? " · rescan recommended" : "") +
          (data.wallet_scan_index_ok ? " · scan index ok" : "") +
          (data.wallet_history_fast_path ? " · history fast path" : "") +
          (data.wallet_listtransactions_scan_pending ? " · scan building index" : "") +
          (data.wallet_history_defer_reason ? " · history deferred (" + data.wallet_history_defer_reason + ")" : "") +
          (data.wallet_scanning ? " · rescan running" : "") +
          (data.signer_cmd_configured
            ? (data.signer_configured ? " · HWI signer ready" : " · signer_cmd (no device)")
            : "") +
          (data.wallet_dat_path ? " · wallet.dat probe" : "");
      }
    }
    if (checks) {
      const rows = [];
      if (data.wallet) {
        rows.push({ name: "getwalletinfo", status: "ok", value: data.wallet });
        const w = data.wallet;
        if (w && typeof w === "object") {
          if (w.keypoolsize != null && w.keypoolsize > 0) {
            rows.push({ name: "keypoolsize (HD receive)", status: "ok", value: String(w.keypoolsize) });
          }
          if (w.keypoolsize_hd_internal != null && w.keypoolsize_hd_internal > 0) {
            rows.push({ name: "keypoolsize (HD change)", status: "ok", value: String(w.keypoolsize_hd_internal) });
          }
        }
      }
      if (data.balance != null) {
        rows.push({ name: "getbalance", status: "ok", value: data.balance });
      }
      if (data.address) {
        rows.push({ name: "getnewaddress", status: "ok", value: data.address });
      }
      if (data.address_book_count != null) {
        let abVal = String(data.address_book_count);
        if (data.address_book_keypool_count != null) abVal += " keypool=" + data.address_book_keypool_count;
        if (data.address_book_core_pool_indices_stored != null) {
          abVal += " core_pool_idx=" + data.address_book_core_pool_indices_stored;
        }
        if (data.address_book_node_tip_count != null && data.address_book_node_tip_count > 0) {
          abVal += " nodetip=" + data.address_book_node_tip_count;
        }
        rows.push({ name: "dogego_listwalletaddresses", status: "ok", value: abVal });
      }
      if (data.nodetip_validateaddress_ok) {
        rows.push({ name: "validateaddress (node tip)", status: "ok", value: "isnodetip ok" });
      }
      if (data.nodetip_getaddressinfo_ok) {
        rows.push({ name: "getaddressinfo (node tip)", status: "ok", value: "isnodetip ok" });
      }
      if (data.keypool_validateaddress_ok) {
        rows.push({ name: "validateaddress (keypool)", status: "ok", value: "iskeypool ok" });
      }
      if (data.keypool_getaddressinfo_ok) {
        rows.push({ name: "getaddressinfo (keypool)", status: "ok", value: "iskeypool ok" });
      }
      if (data.label_roundtrip_ok) {
        rows.push({ name: "setlabel / getaddressesbylabel", status: "ok", value: "round-trip ok" });
      }
      if (data.label_list_ok) {
        rows.push({ name: "listlabels", status: "ok", value: "round-trip ok" });
      }
      if (data.spendable_utxo_count != null) {
        rows.push({ name: "spendable_utxo_count", status: "ok", value: String(data.spendable_utxo_count) });
      }
      if (data.wallet_scanning) {
        rows.push({ name: "wallet rescan", status: "warning", value: "in progress" });
      } else if (data.needs_rescan) {
        const idx = data.wallet_index_height != null ? String(data.wallet_index_height) : "?";
        const tip = data.chain_active_height != null ? String(data.chain_active_height) : "?";
        rows.push({
          name: "wallet_index_height",
          status: "warning",
          value: idx + " / chain " + tip + " - run rescan to backfill fee/hex",
        });
      } else if (data.wallet_index_height != null && data.chain_active_height != null) {
        rows.push({
          name: "wallet_index_height",
          status: "ok",
          value: String(data.wallet_index_height) + " (chain " + data.chain_active_height + ")",
        });
      }
      if (data.wallet_scan_index_ok === true) {
        rows.push({
          name: "wallet scan index",
          status: "ok",
          value: "listtransactions fast path (wallet.db history)",
        });
      } else if (data.wallet_history_fast_path === true) {
        rows.push({
          name: "wallet history fast path",
          status: "ok",
          value: "listtransactions skips UTXO receive walk (partial index)",
        });
      } else if (data.wallet_listtransactions_utxo_walk === true) {
        rows.push({
          name: "wallet listtransactions UTXO walk",
          status: data.spendable_utxo_count != null && data.spendable_utxo_count > 64 ? "warning" : "ok",
          value: "receive rows from UTXO cache until wallet.db scan index exists",
        });
      } else if (data.wallet_listtransactions_scan_pending === true) {
        rows.push({
          name: "wallet scan building index",
          status: "warning",
          value: "listtransactions deferred until rescan populates wallet.db receive rows",
        });
      } else if (data.wallet_history_defer_reason) {
        rows.push({
          name: "wallet history deferred",
          status: "warning",
          value: data.wallet_history_defer_reason + " (same rules as GET /api/wallet/txs)",
        });
      } else if (data.needs_rescan && data.spendable_utxo_count != null && data.spendable_utxo_count > 64) {
        rows.push({
          name: "wallet scan index",
          status: "warning",
          value: "rescan recommended before large UTXO listtransactions",
        });
      }
      if (data.pq_commitments_ok) {
        rows.push({ name: "pq_commitments", status: "ok", value: "FLC1 OP_RETURN on sends (Settings → Wallet)" });
      } else if (data.pq_commitments_enabled === false) {
        rows.push({ name: "pq_commitments", status: "warning", value: "disabled - enable under Settings > Wallet" });
      }
      if (data.wallet_tx_hex_ok) {
        rows.push({ name: "gettransaction hex", status: "ok", value: "wallet.db / index fast path" });
      }
      if (data.wallet_tx_fee_ok) {
        rows.push({ name: "gettransaction fee", status: "ok", value: "send fee from wallet index" });
      }
      if (data.pool_replay_scan_cap != null && data.pool_replay_scan_cap > 0) {
        rows.push({ name: "pool replay deep scan", status: "ok", value: "up to " + data.pool_replay_scan_cap + " BIP44 indices" });
      }
      if (data.wallet_listtransactions_ms != null) {
        rows.push({
          name: "listtransactions (40 rows)",
          status: data.wallet_listtransactions_ok ? "ok" : "warning",
          value: String(data.wallet_listtransactions_ms) + " ms",
        });
      } else if (data.wallet_history_deferred) {
        rows.push({
          name: "listtransactions (40 rows)",
          status: "warning",
          value: "skipped (wallet_history_deferred)",
        });
      }
      if (data.wallet_pq_send_ok) {
        rows.push({ name: "PQ send history", status: "ok", value: data.wallet_pq_tag || "sent_pq in tx hex" });
      } else if (data.pq_commitments_ok) {
        rows.push({ name: "PQ send history", status: "warning", value: "no PQ-tagged send yet (send with pq_commitments on)" });
      }
      if (data.keypool_topup_ok) {
        rows.push({ name: "keypool top-up", status: "ok", value: "keypoolsize >= 50 after getnewaddress" });
      }
      if (data.psbt_roundtrip_ok) {
        rows.push({ name: "walletcreatefundedpsbt / walletprocesspsbt", status: "ok", value: "PSBT round-trip complete" });
      } else if (data.psbt_create_funded_ok) {
        rows.push({ name: "walletcreatefundedpsbt", status: "ok", value: "funded PSBT ok" });
        if (data.psbt_bip32_deriv_ok) {
          rows.push({ name: "PSBT BIP32 deriv paths", status: "ok", value: "present on inputs/outputs" });
        }
        if (data.psbt_process_complete) {
          rows.push({ name: "walletprocesspsbt", status: "ok", value: "complete" });
        }
      }
      if (data.enumeratesigners_ok) {
        const sc = data.signer_count != null ? String(data.signer_count) + " device(s)" : "ok";
        rows.push({ name: "enumeratesigners", status: data.signer_configured ? "ok" : "warning", value: sc });
      }
      if (data.signer_cmd_configured) {
        rows.push({
          name: "signer_cmd",
          status: data.signer_configured ? "ok" : "warning",
          value: data.signer_configured ? "HWI transport + device" : "configured, no device enumerated",
        });
      }
      if (data.hardware_psbt_hint) {
        rows.push({
          name: i18n("pages.features.coreWalletHardwarePsbt"),
          status: "warning",
          value: String(data.hardware_psbt_hint),
        });
      }
      if (data.wallet_dat_probe && typeof data.wallet_dat_probe === "object") {
        const p = data.wallet_dat_probe;
        rows.push({
          name: "dogego_probewalletdat",
          status: p.is_bdb ? "ok" : "warning",
          value: "keys=" + (p.key_count != null ? p.key_count : "?") + walletDatPoolSuffix(p) +
            (p.can_import != null ? " can_import=" + p.can_import : ""),
        });
        if (p.hint) rows.push({ name: "keypool_hint", status: "warning", value: String(p.hint) });
        if (p.pool_unmatched_hint) {
          rows.push({ name: "pool_unmatched_hint", status: "warning", value: String(p.pool_unmatched_hint) });
        }
      } else if (data.wallet_dat_path) {
        rows.push({ name: "dogego_probewalletdat", status: "warning", value: data.wallet_dat_path });
      }
      if (data.pool_core_indices_stored != null && data.pool_core_indices_stored > 0) {
        rows.push({
          name: "pool_core_indices_stored",
          status: "ok",
          value: String(data.pool_core_indices_stored) + " Core BDB indices in wallet.json",
        });
      }
      if (data.pool_keys_unmatched != null && data.pool_keys_unmatched > 0) {
        rows.push({
          name: "pool_keys_unmatched",
          status: "warning",
          value: String(data.pool_keys_unmatched) + " pool-only (no spend key in wallet.dat)",
        });
      }
      if (data.pool_unmatched_hint) {
        rows.push({ name: "pool_unmatched_hint", status: "warning", value: String(data.pool_unmatched_hint) });
      }
      (data.issues || []).forEach((issue) => rows.push({ name: issue, status: "error" }));
      (data.warnings || []).forEach((w) => rows.push({ name: w, status: "warning" }));
      (data.notes || []).forEach((n) => rows.push({ name: n, status: "ok", value: "informational" }));
      checks.innerHTML = rows.map((c) => {
        const st = c.status === "ok" ? "live" : "partial";
        const val = c.value != null ? (typeof c.value === "object" ? JSON.stringify(c.value) : String(c.value)) : "";
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (val ? "<p class=\"label\"><span class=\"mono\">" + escapeHtml(val) + "</span></p>" : "") +
          "</div></article>";
      }).join("");
    }
  }

  async function loadCoreWalletProbe() {
    const pill = $("feat-core-wallet-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-wallet-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreWalletUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-wallet-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  function fillCoreMaintenanceUI(data) {
    if (!data) return;
    const pill = $("feat-core-maint-pill");
    const summary = $("feat-core-maint-summary");
    const checks = $("feat-core-maint-checks");
    if (pill) {
      if (data.ok) {
        if (data.ibd || (data.headers != null && data.blocks != null && data.blocks < data.headers)) {
          pill.textContent = i18n("pages.features.coreMaintSyncing");
          pill.className = "p2p-health-pill warming";
        } else if ((data.warnings || []).length) {
          pill.textContent = i18n("pages.features.coreMaintWarn");
          pill.className = "p2p-health-pill warming";
        } else {
          pill.textContent = i18n("pages.features.coreMaintOk");
          pill.className = "p2p-health-pill ok";
        }
      } else if ((data.issues || []).length) {
        pill.textContent = i18n("pages.features.coreMaintFail");
        pill.className = "p2p-health-pill degraded";
      } else {
        pill.textContent = i18n("pages.features.coreMaintWarn");
        pill.className = "p2p-health-pill warming";
      }
    }
    if (summary) {
      summary.textContent = "blocks " + (data.blocks != null ? data.blocks : "?") +
        " · headers " + (data.headers != null ? data.headers : "?") +
        (data.ibd ? " · IBD" : "") +
        (data.core_available ? " · Core " + (data.core_rpc_addr || "") : "");
    }
    if (checks && data.checks) {
      checks.innerHTML = data.checks.map((c) => {
        const st = c.status === "ok" ? "live" : (c.status === "warning" ? "partial" : (c.status === "skipped" ? "planned" : "partial"));
        let detail = "";
        if (c.dogego != null && c.core != null) {
          detail = "DogeGo: <strong>" + escapeHtml(String(c.dogego)) + "</strong> · Core: <strong>" + escapeHtml(String(c.core)) + "</strong>";
        } else if (c.dogego != null && typeof c.dogego === "object") {
          detail = "<span class=\"mono\">" + escapeHtml(JSON.stringify(c.dogego)) + "</span>";
        } else if (c.dogego != null) {
          detail = "DogeGo: <strong>" + escapeHtml(String(c.dogego)) + "</strong>";
        }
        if (c.note) detail += (detail ? " · " : "") + escapeHtml(c.note);
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
      if ((data.warnings || []).length) {
        checks.innerHTML += "<p class=\"label\">" + escapeHtml((data.warnings || []).join(" · ")) + "</p>";
      }
    }
  }

  function fillCoreAutostartUI(data) {
    if (!data) return;
    const pill = $("feat-core-autostart-pill");
    const summary = $("feat-core-autostart-summary");
    const checks = $("feat-core-autostart-checks");
    if (pill) {
      if (!data.want_login) {
        pill.textContent = i18n("pages.features.coreAutostartOff");
        pill.className = "p2p-health-pill starting";
      } else if (data.ok && !(data.warnings || []).length) {
        pill.textContent = i18n("pages.features.coreAutostartOk");
        pill.className = "p2p-health-pill ok";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreAutostartWarn");
        pill.className = "p2p-health-pill warming";
      } else {
        pill.textContent = i18n("pages.features.coreAutostartFail");
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      const st = data.status || {};
      summary.textContent = (st.platform || "?") +
        (st.method ? " · " + st.method : "") +
        (data.want_login ? " · login" : " · disabled");
    }
    if (checks) {
      const rows = [];
      if (!data.want_login) {
        rows.push("<p class=\"label\">" + escapeHtml(i18n("pages.features.coreAutostartOffHint")) + "</p>");
      } else {
        if (data.status && data.status.detail) {
          rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div><h3>registration</h3><p class=\"label\">" +
            escapeHtml(data.status.detail) + "</p></div></article>");
        }
        (data.issues || []).forEach((msg) => {
          rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div><h3>issue</h3><p class=\"label\">" +
            escapeHtml(msg) + "</p></div></article>");
        });
        (data.warnings || []).forEach((msg) => {
          rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div><h3>warning</h3><p class=\"label\">" +
            escapeHtml(msg) + "</p></div></article>");
        });
        (data.notes || []).forEach((msg) => {
          rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div><h3>note</h3><p class=\"label\">" +
            escapeHtml(msg) + "</p></div></article>");
        });
      }
      checks.innerHTML = rows.join("");
    }
  }

  function fillCoreSetupParityUI(sp) {
    const checks = $("feat-core-runner-checks");
    if (!checks || !sp || sp.skipped) return;
    const setup = sp.setup || {};
    const rows = [];
    if (setup.dogego_balance != null) {
      rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div>" +
        "<h3 class=\"mono\">setup-parity dogego_balance</h3><p class=\"label\">" + escapeHtml(String(setup.dogego_balance)) + "</p></div></article>");
    }
    if (setup.core_balance != null) {
      rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div>" +
        "<h3 class=\"mono\">setup-parity core_balance</h3><p class=\"label\">" + escapeHtml(String(setup.core_balance)) + "</p></div></article>");
    }
    (setup.issues || []).forEach((msg) => {
      rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div>" +
        "<h3 class=\"mono\">setup-parity issue</h3><p class=\"label\">" + escapeHtml(msg) + "</p></div></article>");
    });
    (setup.warnings || []).forEach((msg) => {
      rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div>" +
        "<h3 class=\"mono\">setup-parity warn</h3><p class=\"label\">" + escapeHtml(msg) + "</p></div></article>");
    });
    (setup.notes || []).forEach((msg) => {
      rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div>" +
        "<h3 class=\"mono\">setup-parity note</h3><p class=\"label\">" + escapeHtml(msg) + "</p></div></article>");
    });
    if (sp.cli) {
      rows.push("<p class=\"core-note mono\">" + escapeHtml(sp.cli) + "</p>");
    }
    if (rows.length) {
      checks.insertAdjacentHTML("beforeend",
        "<article class=\"feat-row\"><div>" + statusPill(sp.ok ? "live" : "partial") + "</div><div>" +
        "<h3 class=\"mono\">Milestone D setup parity</h3><p class=\"label\">" +
        escapeHtml(sp.hint || "dogego cert setup-parity -mine-bootstrap before 24/24 stateful gate") + "</p></div></article>" +
        rows.join(""));
    }
  }

  function fillCoreWorkflow10UI(data) {
    if (!data) return;
    const pill = $("feat-core-workflow10-pill");
    const summary = $("feat-core-workflow10-summary");
    const checks = $("feat-core-workflow10-checks");
    const res = data.result || {};
    if (pill) {
      if (data.skipped) {
        pill.textContent = "Skipped";
        pill.className = "p2p-health-pill warming";
      } else if (data.ok) {
        pill.textContent = "OK";
        pill.className = "p2p-health-pill ok";
      } else {
        pill.textContent = "Issues";
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      if (data.skipped) {
        summary.textContent = data.skip_reason || "";
      } else {
        const stages = res.stages || [];
        const okN = stages.filter((s) => s.ok && !s.skipped).length;
        summary.textContent = "stages " + okN + "/" + stages.length +
          (data.cli ? " · " + data.cli : "");
      }
    }
    if (checks) {
      const rows = [];
      (res.stages || []).forEach((s) => {
        const st = s.skipped ? "partial" : (s.ok ? "live" : "partial");
        rows.push("<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(s.id || "stage") + "</h3>" +
          "<p class=\"label\">" + (s.skipped ? "skipped" : (s.ok ? "ok" : "failed")) + "</p></div></article>");
      });
      (res.issues || []).forEach((msg) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div>" +
          "<h3 class=\"mono\">issue</h3><p class=\"label\">" + escapeHtml(msg) + "</p></div></article>");
      });
      if (data.cli) {
        rows.push("<p class=\"label mono\">full: " + escapeHtml(data.cli) + "</p>");
      }
      if (data.doc) {
        rows.push("<p class=\"label mono\">doc: " + escapeHtml(String(data.doc)) + "</p>");
      }
      checks.innerHTML = rows.join("");
    }
  }

  function fillCoreRunnerUI(data) {
    if (!data) return;
    const pill = $("feat-core-runner-pill");
    const summary = $("feat-core-runner-summary");
    const checks = $("feat-core-runner-checks");
    const prov = data.provision || {};
    const pf = data.preflight || {};
    if (pill) {
      if (data.skipped) {
        pill.textContent = "Skipped";
        pill.className = "p2p-health-pill warming";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreRunnerOk");
        pill.className = "p2p-health-pill ok";
      } else if ((pf.issues || []).length || (prov.issues || []).length) {
        pill.textContent = i18n("pages.features.coreRunnerFail");
        pill.className = "p2p-health-pill degraded";
      } else {
        pill.textContent = i18n("pages.features.coreRunnerWarn");
        pill.className = "p2p-health-pill warming";
      }
    }
    if (summary) {
      const wm = pf.wallet_migration && pf.wallet_migration.probe ? pf.wallet_migration.probe : null;
      const wi = pf.wallet_dat_import || null;
      summary.textContent = "provision " + (prov.done != null ? prov.done : "?") + "/" + (prov.total != null ? prov.total : "?") +
        (pf.dogego && pf.dogego.blocks != null ? " · dogego blocks " + pf.dogego.blocks : "") +
        (pf.core && pf.core.blocks != null ? " · core blocks " + pf.core.blocks : "") +
        (wm ? " · wallet.dat keys " + (wm.key_count || 0) + " encrypted " + (wm.encrypted_keys || 0) + walletDatPoolSuffix(wm) : "") +
        (wi && wi.status ? " · import " + wi.status : "") +
        (wi && wi.keypool_refill_size ? " · keypool_refill_size=" + wi.keypool_refill_size : "");
    }
    if (checks) {
      const rows = [];
      (prov.checklist || []).forEach((c) => {
        const st = c.done ? "live" : "partial";
        rows.push("<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">provision " + escapeHtml(String(c.step || "")) + "</h3>" +
          "<p class=\"label\">" + escapeHtml(c.item || "") + "</p></div></article>");
      });
      (pf.issues || []).forEach((msg) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div>" +
          "<h3 class=\"mono\">preflight issue</h3><p class=\"label\">" + escapeHtml(msg) + "</p></div></article>");
      });
      (pf.warnings || []).forEach((msg) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div>" +
          "<h3 class=\"mono\">preflight warn</h3><p class=\"label\">" + escapeHtml(msg) + "</p></div></article>");
      });
      (pf.notes || []).forEach((msg) => {
        const s = String(msg || "");
        if (!/pool_unmatched_hint=|wallet_dat_pool_unmatched_hint=|wallet_dat_keypool_refill_size=|wallet_dat_keypool_hint=|wallet_dat_pool_indices_replayed=/.test(s)) return;
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div>" +
          "<h3 class=\"mono\">preflight note</h3><p class=\"label\">" + escapeHtml(s) + "</p></div></article>");
      });
      if (pf.wallet_dat_import && pf.wallet_dat_import.status) {
        const wi = pf.wallet_dat_import;
        const st = (wi.status === "passed" || wi.status === "passed_encrypted") ? "live" : "partial";
        rows.push("<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">wallet.dat import</h3><p class=\"label\">status=" + escapeHtml(wi.status) +
          (wi.keys_imported != null ? " keys_imported=" + escapeHtml(String(wi.keys_imported)) : "") +
          (wi.pool_indices_replayed != null ? " pool_indices_replayed=" + escapeHtml(String(wi.pool_indices_replayed)) : "") +
          (wi.keypool_refill_size != null ? " keypool_refill_size=" + escapeHtml(String(wi.keypool_refill_size)) : "") +
          (wi.pool_unmatched_hint ? " · " + escapeHtml(wi.pool_unmatched_hint) : "") +
          (wi.error ? " · " + escapeHtml(wi.error) : "") + "</p></div></article>");
      }
      if (data.cli_provision || data.cli_preflight || data.cli_weekly || data.cli_weekly_live || data.cli_live_soak || data.cli_workflow10) {
        const cliParts = [data.cli_workflow10, data.cli_provision, data.cli_preflight, data.cli_weekly, data.cli_weekly_live, data.cli_live_soak].filter(Boolean);
        rows.push("<p class=\"label mono\">" + escapeHtml(cliParts.join(" · ")) + "</p>");
      }
      if (data.doc) {
        rows.push("<p class=\"label mono\">doc: " + escapeHtml(String(data.doc)) + "</p>");
      } else if (prov.doc) {
        rows.push("<p class=\"label mono\">doc: " + escapeHtml(String(prov.doc)) + "</p>");
      }
      checks.innerHTML = rows.join("");
    }
  }

  function fillCoreFounderUI(data) {
    if (!data) return;
    const pill = $("feat-core-founder-pill");
    const summary = $("feat-core-founder-summary");
    const checks = $("feat-core-founder-checks");
    if (pill) {
      if (data.skipped) {
        pill.textContent = "Skipped";
        pill.className = "p2p-health-pill warming";
      } else if (data.ok) {
        pill.textContent = (data.verify && data.verify.warnings && data.verify.warnings.length) ? "OK (warnings)" : "OK";
        pill.className = "p2p-health-pill ok";
      } else {
        pill.textContent = "Issues";
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      if (data.skipped) {
        summary.textContent = data.skip_reason || "not reboot testnet";
      } else if (data.verify) {
        const v = data.verify;
        summary.textContent = "network " + (v.network || "?") +
          " · p2p " + (v.p2p_port != null ? v.p2p_port : "?") +
          (v.datadir ? " · datadir set" : "");
      } else {
        summary.textContent = data.cli || "dogego cert founder";
      }
    }
    if (checks) {
      const rows = (data.verify && data.verify.checks) || [];
      if (data.skipped) {
        checks.innerHTML = "<p class=\"label\">" + escapeHtml(data.skip_reason || "skipped") + "</p>";
      } else if (rows.length) {
        checks.innerHTML = rows.map((c) => {
          const st = c.status === "ok" ? "live" : (c.status === "warn" ? "partial" : "partial");
          let detail = escapeHtml(c.message || "");
          if (c.fix) detail += (detail ? " · " : "") + escapeHtml(c.fix);
          return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
            "<h3 class=\"mono\">" + escapeHtml(c.id || "") + "</h3>" +
            (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
        }).join("");
      } else {
        checks.innerHTML = "";
      }
    }
  }

  function formatConnectCatchUpBoostFromResume(data) {
    if (!data) return "";
    const passes = Number(data.connect_catch_up_passes);
    const batch = Number(data.connect_catch_up_batch);
    const interval = Number(data.connect_catch_up_interval_ms);
    if (!isFinite(passes) || passes <= 0 || !isFinite(batch) || batch <= 0) return "";
    let t = passes + "×" + batch;
    if (isFinite(interval) && interval > 0) t += " @ " + interval + "ms";
    return t;
  }

  function fillCoreIbdConvergenceUI(data) {
    if (!data) return;
    const pill = $("feat-core-ibd-converge-pill");
    const summary = $("feat-core-ibd-converge-summary");
    const checks = $("feat-core-ibd-converge-checks");
    if (pill) {
      if (data.skipped) {
        pill.textContent = i18n("pages.features.coreIbdConvergeSkipped");
        pill.className = "p2p-health-pill warming";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreIbdConvergeOk");
        pill.className = "p2p-health-pill ok";
      } else if ((data.issues || []).length) {
        pill.textContent = i18n("pages.features.coreIbdConvergeFail");
        pill.className = "p2p-health-pill degraded";
      } else {
        pill.textContent = i18n("pages.features.coreIbdConvergeWarn");
        pill.className = "p2p-health-pill warming";
      }
    }
    if (summary) {
      const snap = data.snapshot || {};
      const headers = snap.headers != null ? snap.headers : "?";
      const blocks = snap.blocks != null ? snap.blocks : "?";
      const contig = snap.contiguous != null ? snap.contiguous : "?";
      let txt = "source " + (snap.source || "?") + " · headers " + headers + " · blocks " + blocks + " · contiguous " + contig;
      if (data.body_coverage_pct > 0) txt += " · coverage " + data.body_coverage_pct.toFixed(1) + "%";
      if (data.connect_boost) txt += " · boost " + data.connect_boost;
      summary.textContent = txt;
    }
    if (checks) {
      const rows = [];
      (data.notes || []).forEach((note) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div><p class=\"label\">" + escapeHtml(note) + "</p></div></article>");
      });
      (data.issues || []).forEach((issue) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div><p class=\"label\">" + escapeHtml(issue) + "</p></div></article>");
      });
      if (data.hint) {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div><p class=\"label\">" + escapeHtml(data.hint) + "</p></div></article>");
      }
      checks.innerHTML = rows.join("");
    }
  }

  function fillCoreAddrmanUI(data) {
    if (!data) return;
    const pill = $("feat-core-addrman-pill");
    const summary = $("feat-core-addrman-summary");
    const checks = $("feat-core-addrman-checks");
    if (pill) {
      if (data.skipped) {
        pill.textContent = i18n("pages.features.coreAddrmanSkipped");
        pill.className = "p2p-health-pill warming";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreAddrmanOk");
        pill.className = "p2p-health-pill ok";
      } else if ((data.issues || []).length) {
        pill.textContent = i18n("pages.features.coreAddrmanFail");
        pill.className = "p2p-health-pill degraded";
      } else {
        pill.textContent = i18n("pages.features.coreAddrmanWarn");
        pill.className = "p2p-health-pill warming";
      }
    }
    if (summary) {
      const tried = data.tried != null ? data.tried : "?";
      const nw = data.new != null ? data.new : "?";
      let txt = "tried " + tried + " · new " + nw;
      if (data.n_key_set != null) txt += " · nKey " + (data.n_key_set ? "yes" : "no");
      if (data.tried_buckets_used != null) txt += " · buckets " + data.tried_buckets_used + "/" + (data.new_buckets_used != null ? data.new_buckets_used : "?");
      summary.textContent = txt;
    }
    if (checks) {
      const rows = [];
      (data.notes || []).forEach((note) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("live") + "</div><div><p class=\"label\">" + escapeHtml(note) + "</p></div></article>");
      });
      (data.issues || []).forEach((issue) => {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div><p class=\"label\">" + escapeHtml(issue) + "</p></div></article>");
      });
      if (data.hint) {
        rows.push("<article class=\"feat-row\"><div>" + statusPill("partial") + "</div><div><p class=\"label\">" + escapeHtml(data.hint) + "</p></div></article>");
      }
      checks.innerHTML = rows.join("");
    }
  }

  function fillCoreRestartResumeUI(data) {
    if (!data) return;
    const pill = $("feat-core-resume-pill");
    const summary = $("feat-core-resume-summary");
    const checks = $("feat-core-resume-checks");
    if (pill) {
      if (data.ok) {
        pill.textContent = i18n("pages.features.coreResumeOk");
        pill.className = "p2p-health-pill ok";
      } else if ((data.issues || []).length) {
        pill.textContent = i18n("pages.features.coreResumeFail");
        pill.className = "p2p-health-pill degraded";
      } else {
        pill.textContent = i18n("pages.features.coreResumeWarn");
        pill.className = "p2p-health-pill warming";
      }
    }
    if (summary) {
      summary.textContent = "headers " + (data.headers != null ? data.headers : "?") +
        " · contiguous " + (data.contiguous_raw != null ? data.contiguous_raw : "?") +
        " · probe " + (data.checkpoint_probe != null ? data.checkpoint_probe : "?") +
        (data.body_lag != null ? " · lag " + data.body_lag : "") +
        (data.connect_lag != null ? " · connect " + data.connect_lag : "") +
        (formatConnectCatchUpBoostFromResume(data) ? " · boost " + formatConnectCatchUpBoostFromResume(data) : "") +
        (data.autostart_want ? " · autostart " + (data.autostart_ok ? "ok" : "missing") : "");
    }
    if (checks && data.checks) {
      checks.innerHTML = data.checks.map((c) => {
        const st = c.status === "ok" ? "live" : (c.status === "warning" ? "partial" : "partial");
        let detail = c.note ? escapeHtml(c.note) : "";
        if (c.value != null) {
          const val = typeof c.value === "object" ? JSON.stringify(c.value) : String(c.value);
          detail = (detail ? detail + " · " : "") + "<span class=\"mono\">" + escapeHtml(val) + "</span>";
        }
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
    }
  }

  let coreCompareCache = null;
  let coreCertCache = null;
  let lastOvCoreProbeAt = 0;
  let lastOvCoreStatusAt = 0;
  const OV_CORE_PROBE_MS = 120000;
  const OV_CORE_STATUS_MS = 60000;
  const CORE_PROBE_PRESETS = [
    { label: "All probes", path: "/api/core-operator-cert?refresh=1" },
    { label: "Probe bundle", path: "/api/core-probes?refresh=1" },
    { label: "Core compare", path: "/api/core-compare" },
    { label: "Mempool parity", path: "/api/mempool/parity-probe" },
    { label: "Mempool stateful", path: "/api/mempool/stateful-status" },
    { label: "Setup parity", path: "/api/core-setup-parity" },
    { label: "Maintenance", path: "/api/core-maintenance" },
    { label: "Restart resume", path: "/api/core-restart-resume" },
    { label: "IBD convergence", path: "/api/core-ibd-convergence-probe" },
    { label: "Addrman probe", path: "/api/core-addrman-probe" },
    { label: "Autostart probe", path: "/api/core-autostart-probe" },
    { label: "Founder probe", path: "/api/core-founder-probe" },
    { label: "Runner readiness", path: "/api/core-runner-probes" },
    { label: "Runner (strict)", path: "/api/core-runner-probes?require_core=1" },
    { label: "Workflow 10 preflight", path: "/api/core-workflow10-probe?skip_provision=1&mine_bootstrap=1" },
    { label: "Wallet probe", path: "/api/core-wallet-probe" },
    { label: "Reindex probe", path: "/api/core-reindex-probe" },
    { label: "BIP152 probe", path: "/api/core-bip152-probe" },
    { label: "Mining probe", path: "/api/core-mining-probe" },
    { label: "PQ probe", path: "/api/core-pq-probe" },
    { label: "Field evidence", path: "/api/core-field-evidence-probe" },
    { label: "End-to-end", path: "/api/core-end-to-end-probe" },
    { label: "Cert matrix", path: "/api/core-operator-cert?matrix=1" },
    { label: "Core status", path: "/api/core-status" }
  ];
  const CERT_PROBE_ANCHORS = {
    core_compare: "feat-core-compare",
    maintenance: "feat-core-maintenance",
    restart_resume: "feat-core-restart-resume",
    cert_autostart: "feat-core-autostart",
    cert_founder: "feat-core-founder",
    runner_readiness: "feat-core-runner",
    cert_workflow10: "feat-core-workflow10",
    restart_connect: "feat-core-restart-resume",
    ibd_converge: "feat-core-ibd-converge",
    addrman: "feat-core-addrman",
    mempool_parity: "feat-core-mempool",
    reindex: "feat-core-reindex",
    bip152_relay: "feat-core-bip152",
    pq_format: "feat-core-pq",
    wallet: "feat-core-wallet",
    mining: "feat-core-mining",
    end_to_end: "feat-core-end-to-end"
  };
  const PROBE_MINI_ANCHORS = {
    "feat-probe-compare-mini": "feat-core-compare",
    "feat-probe-maint-mini": "feat-core-maintenance",
    "feat-probe-resume-mini": "feat-core-restart-resume",
    "feat-probe-ibd-mini": "feat-core-ibd-converge",
    "feat-probe-addrman-mini": "feat-core-addrman",
    "feat-probe-autostart-mini": "feat-core-autostart",
    "feat-probe-founder-mini": "feat-core-founder",
    "feat-probe-runner-mini": "feat-core-runner",
    "feat-probe-workflow10-mini": "feat-core-workflow10",
    "feat-probe-connect-mini": "feat-core-restart-resume",
    "feat-probe-mempool-mini": "feat-core-mempool",
    "feat-probe-wallet-mini": "feat-core-wallet",
    "feat-probe-reindex-mini": "feat-core-reindex",
    "feat-probe-bip152-mini": "feat-core-bip152",
    "feat-probe-mining-mini": "feat-core-mining",
    "feat-probe-pq-mini": "feat-core-pq",
    "feat-probe-e2e-mini": "feat-core-end-to-end"
  };
  function gotoCertProbeAnchor(anchorId) {
    if (!anchorId) return;
    showTab("features", { preserveHash: true });
    location.hash = "features/" + anchorId;
    scrollToFeatAnchor(anchorId);
  }
  function fillCoreCompareUI(prefix, data) {
    if (!data) return;
    coreCompareCache = data;
    const card = $(prefix + "-core-compare-card") || $(prefix + "-core-compare");
    if (card) card.hidden = false;
    const pill = $(prefix + "-core-compare-pill");
    if (pill) {
      const verifyField = (data.fields || []).find((f) => f.name && String(f.name).indexOf("verifychain") === 0);
      const lagField = (data.fields || []).find((f) => f.name === "dogego_connect_lag");
      const lockFail = data.deployment_checked === true && data.protocol_lock_ok === false;
      const lockOk = data.deployment_checked === true && data.protocol_lock_ok === true;
      if (lockFail) {
        pill.textContent = i18n("pages.features.coreCompareProtocolLockFail");
        pill.className = "p2p-health-pill degraded";
      } else if (!data.core_available) {
        if (lockOk) {
          pill.textContent = i18n("pages.features.coreCompareSoloLockOk");
          pill.className = "p2p-health-pill ok";
        } else if (!data.core_configured) {
          pill.textContent = i18n("pages.features.coreCompareOptional");
          pill.className = "p2p-health-pill ok";
        } else {
          pill.textContent = i18n("pages.features.coreCompareUnavailable");
          pill.className = "p2p-health-pill warming";
        }
      } else if (!data.chain_ok) {
        pill.textContent = i18n("pages.features.coreCompareMismatch");
        pill.className = "p2p-health-pill degraded";
      } else if (verifyField && !verifyField.match) {
        pill.textContent = i18n("pages.features.coreCompareVerifyWarn");
        pill.className = "p2p-health-pill warming";
      } else if (data.connect_lag_ok === false || (lagField && !lagField.match)) {
        pill.textContent = "connect lag";
        pill.className = "p2p-health-pill warming";
      } else {
        pill.textContent = i18n("pages.features.coreCompareOk");
        pill.className = "p2p-health-pill ok";
      }
    }
    const addrs = $(prefix + "-core-compare-addrs");
    if (addrs) addrs.textContent = "Core " + (data.core_rpc_addr || "?") + " · DogeGo " + (data.dogego_rpc_addr || "?");
    const hint = $(prefix + "-core-compare-hint");
    if (hint) {
      hint.textContent = (data.errors && data.errors.length) ? data.errors.join(" · ") : (data.hint || "");
    }
    const fields = $(prefix + "-core-compare-fields");
    if (fields && data.fields) {
      const sorted = data.fields.slice().sort((a, b) => {
        const rank = (f) => {
          if (f.name === "deployment.protocol_lock") return 0;
          if ((f.name || "").indexOf("deployment.") === 0) return 1;
          if ((f.name || "").indexOf("softfork.") === 0) return 1;
          if ((f.name || "").indexOf("bip9_softfork.") === 0) return 1;
          return 2;
        };
        const ra = rank(a);
        const rb = rank(b);
        if (ra !== rb) return ra - rb;
        return String(a.name || "").localeCompare(String(b.name || ""));
      });
      fields.innerHTML = sorted.map((f) =>
        "<article class=\"feat-row\"><div>" + statusPill(f.match ? "live" : "partial") + "</div><div>" +
        "<h3 class=\"mono\">" + escapeHtml(f.name || "") + "</h3>" +
        "<p class=\"label\">DogeGo: <strong>" + escapeHtml(String(f.dogego != null ? f.dogego : "-")) + "</strong> · Core: <strong>" + escapeHtml(String(f.core != null ? f.core : "-")) + "</strong>" +
        (f.note ? " · " + escapeHtml(f.note) : "") + "</p></div></article>"
      ).join("");
    }
  }

  function renderCoreCompare(data) {
    fillCoreCompareUI("feat", data);
    fillCoreCompareUI("ov", data);
  }

  async function loadMempoolParityProbe() {
    const pill = $("mp-parity-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/mempool/parity-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillMempoolParityUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const rows = $("mp-parity-rows");
      if (rows) rows.innerHTML = "";
    }
  }

  function fillMempoolStatefulPanel(prefix, live) {
    if (!live) return;
    const pill = $(prefix + "stateful-pill");
    const summary = $(prefix + "stateful-summary");
    const hint = $(prefix + "stateful-hint");
    const cli = $(prefix + "stateful-cli");
    if (hint && live.hint) hint.textContent = live.hint;
    let pillText = i18n("pages.mempool.statefulOfflineOk");
    let pillClass = "p2p-health-pill ok";
    if (!live.offline_ok) {
      pillText = i18n("pages.mempool.statefulOfflineFail");
      pillClass = "p2p-health-pill degraded";
    } else if (!live.reboot_testnet) {
      pillText = i18n("pages.mempool.statefulNotReboot");
      pillClass = "p2p-health-pill warming";
    } else {
      pillText = i18n("pages.mempool.statefulLiveReady");
      pillClass = "p2p-health-pill warming";
    }
    if (pill) {
      pill.textContent = pillText;
      pill.className = pillClass;
    }
    if (summary) {
      let setupNote = "";
      if (live.setup_parity_probe) {
        if (live.setup_parity_ok) {
          setupNote = " · setup-parity ok";
        } else if (!live.setup_parity_skipped) {
          setupNote = " · setup-parity pending";
        }
        if (live.setup_parity_dogego_balance != null) {
          setupNote += " · dogego " + live.setup_parity_dogego_balance + " DOGE";
        }
      }
      summary.textContent =
        (live.offline_corpus_total > 0
          ? "corpus " + (live.offline_corpus_passed || 0) + "/" + live.offline_corpus_total + " · "
          : "") +
        "offline stateful " + (live.offline_passed || 0) + "/" + (live.offline_total || 0) +
        " · live scenarios " + (live.live_scenarios || 0) +
        (live.core_compare_enabled ? " · Core gate configured" : "") +
        setupNote;
    }
    if (cli) {
      const parts = [live.cli_live, live.cli_core_gate].filter(Boolean);
      if (parts.length) {
        cli.textContent = parts.join(" · ");
        cli.hidden = false;
      } else {
        cli.hidden = true;
      }
    }
  }

  function fillMempoolParityPanel(prefix, data) {
    if (!data) return;
    const pill = $(prefix + "parity-pill");
    if (pill) {
      if (data.skipped) {
        pill.textContent = i18n("pages.mempool.paritySkipped");
        pill.className = "p2p-health-pill warming";
      } else if (data.ok && (!data.core_available || data.core_aligned !== false)) {
        pill.textContent = data.core_available ? i18n("pages.mempool.parityOkCore") : i18n("pages.mempool.parityOk");
        pill.className = "p2p-health-pill ok";
      } else if (data.ok && data.core_available && data.core_aligned === false) {
        pill.textContent = i18n("pages.mempool.parityCoreDrift");
        pill.className = "p2p-health-pill warming";
      } else {
        pill.textContent = i18n("pages.mempool.parityFail");
        pill.className = "p2p-health-pill degraded";
      }
    }
    const count = $(prefix + "parity-count");
    if (count) {
      let txt = (data.passed || 0) + " / " + (data.total || 0) + " passed";
      if (data.offline_corpus && data.offline_corpus.total > 0) {
        txt += " · offline corpus " + data.offline_corpus.passed + "/" + data.offline_corpus.total;
      }
      if (data.offline_stateful && data.offline_stateful.total > 0) {
        txt += " · offline stateful " + data.offline_stateful.passed + "/" + data.offline_stateful.total;
      }
      if (data.core_available) txt += " · Core " + (data.core_rpc_addr || "");
      else if (data.core_configured === false) txt += " · Core optional";
      count.textContent = txt;
    }
    const rows = $(prefix + "parity-rows");
    if (rows && data.rows) {
      rows.innerHTML = data.rows.map((row) => {
        let coreTxt = "";
        if (data.core_available) {
          if (row.core_error) {
            coreTxt = " · Core err: " + escapeHtml(row.core_error);
          } else if (row.core_got_accept != null) {
            coreTxt = " · Core " + (row.core_got_accept ? "accept" : "reject") +
              (row.core_got_reject_reason ? " (" + escapeHtml(row.core_got_reject_reason) + ")" : "") +
              (row.core_match === false ? " <strong>mismatch</strong>" : "");
          }
        }
        return "<article class=\"feat-row\"><div>" + statusPill(row.match ? "live" : "partial") + "</div><div>" +
        "<h3 class=\"mono\">" + escapeHtml(row.name || row.template || "") + "</h3>" +
        "<p class=\"label\">want " + (row.want_accept ? "accept" : "reject") +
        (row.want_reject_reason ? " (" + escapeHtml(row.want_reject_reason) + ")" : "") +
        " · DogeGo " + (row.got_accept ? "accept" : "reject") +
        (row.got_reject_reason ? " (" + escapeHtml(row.got_reject_reason) + ")" : "") + coreTxt +
        (row.error ? " · " + escapeHtml(row.error) : "") + "</p></div></article>";
      }).join("");
    }
  }

  function fillMempoolParityUI(data) {
    fillMempoolParityPanel("mp-", data);
    fillMempoolParityPanel("feat-mp-", data);
    if (data && data.stateful_live) {
      fillMempoolStatefulPanel("feat-mp-", data.stateful_live);
      fillMempoolStatefulPanel("mp-", data.stateful_live);
    }
    return data;
  }

  function certRowSoloPass(row) {
    if (row.ok) return true;
    const n = String(row.note || "").toLowerCase();
    return n.indexOf("optional") >= 0 || n.indexOf("skipped") >= 0 ||
      n.indexOf("rpc not ready") >= 0 || n.indexOf("warming up") >= 0 ||
      n.indexOf("hb_not_negotiated") >= 0 || n.indexOf("hb may be deferred") >= 0 ||
      n.indexOf("high connect lag expected") >= 0 || n.indexOf("connect catch-up") >= 0 ||
      n.indexOf("autostart=disable") >= 0 || n.indexOf("not reboot testnet") >= 0;
  }

  function operatorCertPillState(cert, liveRows) {
    const rows = liveRows || (cert.rows || []).filter((r) => r.web_probe);
    const hasResults = rows.some((r) => r.ok != null);
    if (!hasResults) {
      return { text: i18n("pages.features.certLivePending"), cls: "starting" };
    }
    if (cert.live_ok) {
      return { text: i18n("pages.features.certLiveAllOk"), cls: "ok" };
    }
    if (cert.solo_ok) {
      return { text: i18n("pages.features.certLiveSoloOk"), cls: "ok" };
    }
    return { text: i18n("pages.features.certLiveSomeFail"), cls: "warming" };
  }

  function operatorCertRowHTML(row) {
    let st = "planned";
    let statusTxt = i18n("pages.features.certScriptOnly");
    if (row.web_probe && row.ok != null) {
      if (row.ok) {
        st = "live";
        statusTxt = i18n("pages.features.certLivePass");
      } else if (certRowSoloPass(row)) {
        st = "warming";
        statusTxt = i18n("pages.features.certLiveSoloPass");
      } else {
        st = "partial";
        statusTxt = i18n("pages.features.certLiveFail");
      }
    }
    const env = row.env_flag ? "<span class=\"mono label\">" + escapeHtml(row.env_flag) + "</span> · " : "";
    const cmd = row.env_flag
      ? "<p class=\"core-note mono\">$env:" + escapeHtml(row.env_flag) + " = \"1\"; .\\scripts\\core_operator_workflow_cert.ps1</p>"
      : (row.script ? "<p class=\"core-note mono\">.\\" + escapeHtml(row.script) + "</p>" : "");
    const note = row.note ? "<p class=\"label\">" + escapeHtml(row.note) + "</p>" : "";
    const anchor = row.id && CERT_PROBE_ANCHORS[row.id];
    const jump = anchor
      ? "<button type=\"button\" class=\"btn btn-ghost btn-sm cert-probe-jump\" data-cert-anchor=\"" + escapeHtml(anchor) + "\">" +
        escapeHtml(i18n("pages.features.certViewProbe")) + "</button>"
      : "";
    return "<article class=\"feat-row\" data-cert-id=\"" + escapeHtml(row.id || "") + "\"><div>" + statusPill(st) + "</div><div>" +
      "<h3><span class=\"mono\">" + escapeHtml(row.milestone || "") + "</span> - " + escapeHtml(row.label || "") + "</h3>" +
      "<p class=\"label\">" + env + statusTxt + (row.web_probe ? "" : " · " + escapeHtml(i18n("pages.features.certScriptOnly"))) + "</p>" +
      note + cmd + jump + "</div></article>";
  }

  function bindOperatorCertRowClicks(root) {
    if (!root) return;
    root.querySelectorAll(".cert-probe-jump").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        gotoCertProbeAnchor(btn.getAttribute("data-cert-anchor"));
      });
    });
  }

  function publishCoreCertSnapshot(cert) {
    if (!cert) return;
    if (!cert.matrix_only) coreCertCache = cert;
    const rows = (cert.rows || []).filter((r) => r.web_probe);
    const pass = rows.filter((r) => r.ok).length;
    window.DogeGoCoreCert = {
      live_ok: !!cert.live_ok,
      solo_ok: !!cert.solo_ok,
      solo_pass: cert.solo_pass,
      pass: pass,
      total: rows.length,
      cached: !!cert.cached,
      age_sec: cert.cache_age_sec || 0,
      at: cert.checked_at || ""
    };
    if (window.DogeGoSyncDock && window.DogeGoSyncDock.setOperatorCert) {
      window.DogeGoSyncDock.setOperatorCert(window.DogeGoCoreCert);
    }
  }

  function renderOperatorCert(cert) {
    const wrap = $("feat-cert-live-wrap");
    const hint = $("feat-cert-live-hint");
    const list = $("feat-cert-live");
    if (!wrap || !list || !cert) return;
    wrap.hidden = false;
    const liveRows = (cert.rows || []).filter((r) => r.web_probe);
    const scriptRows = (cert.rows || []).filter((r) => !r.web_probe);
    let hintTxt = cert.hint || i18n("pages.features.certLiveHint");
    if (cert.cached && cert.cache_age_sec != null) {
      hintTxt += (hintTxt ? " · " : "") + i18n("pages.features.certCached", { sec: cert.cache_age_sec });
    }
    const soloPass = cert.solo_pass != null ? Number(cert.solo_pass) : null;
    if (soloPass != null && (cert.solo_ok !== cert.live_ok || soloPass !== liveRows.filter((r) => r.ok).length)) {
      hintTxt += (hintTxt ? " · " : "") + i18n("pages.overview.operatorCertSolo") + " " + soloPass + "/" + liveRows.length;
    }
    if (hint) hint.textContent = hintTxt;
    list.innerHTML = liveRows.map((row) => operatorCertRowHTML(row)).join("");
    bindOperatorCertRowClicks(list);
    const scriptWrap = $("feat-cert-script-wrap");
    const scriptList = $("feat-cert-script");
    if (scriptWrap && scriptList) {
      scriptWrap.hidden = scriptRows.length === 0;
      scriptList.innerHTML = scriptRows.map((row) => operatorCertRowHTML(row)).join("");
      bindOperatorCertRowClicks(scriptList);
    }
    const summaryPill = $("feat-cert-live-summary");
    if (summaryPill) {
      const pill = operatorCertPillState(cert, liveRows);
      summaryPill.textContent = pill.text;
      summaryPill.className = "p2p-health-pill " + pill.cls;
    }
    publishCoreCertSnapshot(cert);
    applyOverviewOperatorCert(cert);
  }

  function mempoolProbeMetricsFromSummary(s) {
    if (!s) return null;
    const corpusTotal = Number(s.dogego_mempool_offline_corpus_total);
    if (!corpusTotal) return null;
    return {
      corpus_ok: !!s.dogego_mempool_offline_corpus_ok,
      corpus_passed: Number(s.dogego_mempool_offline_corpus_passed) || 0,
      corpus_total: corpusTotal,
      parity_passed: Number(s.dogego_mempool_parity_passed) || 0,
      parity_total: Number(s.dogego_mempool_parity_total) || 0
    };
  }

  function mempoolProbeMetricsFromCoreStatus(st) {
    if (!st) return null;
    const oc = st.mempool_offline_corpus;
    if (!oc || !oc.total) return null;
    return {
      corpus_ok: !!oc.ok,
      corpus_passed: Number(oc.passed) || 0,
      corpus_total: Number(oc.total) || 0,
      parity_passed: Number(st.mempool_parity_passed) || 0,
      parity_total: Number(st.mempool_parity_total) || 0
    };
  }

  function formatOperatorMempoolCorpusLine(m) {
    if (!m || !m.corpus_total) return "";
    let txt = i18n("pages.overview.operatorCertMempoolCorpus") + " " +
      m.corpus_passed + "/" + m.corpus_total;
    if (m.parity_total) {
      txt += " · " + i18n("pages.overview.operatorCertMempoolParity") + " " +
        m.parity_passed + "/" + m.parity_total;
    }
    return txt;
  }

  function applyOverviewMempoolCorpus(s, st) {
    const el = $("ov-operator-cert-mempool");
    if (!el) return;
    const m = mempoolProbeMetricsFromSummary(s) || mempoolProbeMetricsFromCoreStatus(st);
    if (!m || !m.corpus_total) {
      el.hidden = true;
      el.textContent = "";
      return;
    }
    el.textContent = formatOperatorMempoolCorpusLine(m);
    el.className = "label" + (m.corpus_ok ? "" : " err-inline");
    el.hidden = false;
  }

  function applyOverviewOperatorCert(cert) {
    const card = $("ov-operator-cert-card");
    if (!card) return;
    if (!cert && lastSummary && lastSummary.dogego_operator_cert_total != null) {
      cert = {
        live_ok: lastSummary.dogego_operator_cert_live_ok,
        solo_ok: lastSummary.dogego_operator_cert_solo_ok,
        solo_pass: lastSummary.dogego_operator_cert_solo_pass,
        rows: Array.from({ length: Number(lastSummary.dogego_operator_cert_total) || 0 }, (_, i) => ({
          web_probe: true,
          ok: i < (Number(lastSummary.dogego_operator_cert_pass) || 0)
        }))
      };
    }
    if (!cert) return;
    card.hidden = false;
    const pill = $("ov-operator-cert-pill");
    if (pill) {
      const rows = (cert.rows || []).filter((r) => r.web_probe);
      const pillState = operatorCertPillState(cert, rows);
      pill.textContent = pillState.text;
      pill.className = "p2p-health-pill " + pillState.cls;
    }
    const detail = $("ov-operator-cert-detail");
    if (detail) {
      const rows = (cert.rows || []).filter((r) => r.web_probe);
      const withOk = rows.filter((r) => r.ok != null);
      const pass = withOk.filter((r) => r.ok).length;
      if (!withOk.length) {
        detail.textContent = i18n("pages.features.certLivePending");
      } else {
        let txt = pass + "/" + rows.length + " " + i18n("pages.overview.operatorCertGates");
        if (cert.solo_pass != null && (cert.solo_ok !== cert.live_ok || cert.solo_pass !== pass)) {
          txt += " · " + i18n("pages.overview.operatorCertSolo") + " " + cert.solo_pass + "/" + rows.length;
        }
        if (!cert.live_ok) {
          const fails = rows.filter((r) => r.ok === false).map((r) => r.label || r.id).filter(Boolean);
          if (fails.length) txt += " - " + fails.slice(0, 3).join(", ");
        }
        const e2e = cert.probes && cert.probes.end_to_end;
        if (e2e && !e2e.ok) {
          const step = (e2e.steps || []).find((s) => !s.skipped && !s.ok);
          if (step) txt += " · e2e:" + step.name;
        }
        detail.textContent = txt;
        const firstFail = rows.find((r) => r.ok === false);
        if (firstFail && CERT_PROBE_ANCHORS[firstFail.id]) {
          detail.classList.add("cert-detail-link");
          detail.title = i18n("pages.features.certViewProbe");
          detail.onclick = () => gotoCertProbeAnchor(CERT_PROBE_ANCHORS[firstFail.id]);
        } else {
          detail.classList.remove("cert-detail-link");
          detail.removeAttribute("title");
          detail.onclick = null;
        }
      }
    }
    applyOverviewMempoolCorpus(lastSummary, null);
  }

  function applySettingsCoreStatus(st) {
    const el = $("st-core-cert-status");
    const wrap = el && el.parentElement;
    if (!el || !st) return;
    const oc = st.operator_cert;
    if (!oc || !oc.total) {
      el.hidden = true;
      if (wrap) wrap.hidden = true;
      return;
    }
    let txt = (oc.pass || 0) + "/" + oc.total + " " + i18n("pages.overview.operatorCertGates");
    if (oc.solo_pass != null && (oc.solo_ok !== oc.live_ok || oc.solo_pass !== oc.pass)) {
      txt += " · " + i18n("pages.overview.operatorCertSolo") + " " + oc.solo_pass + "/" + oc.total;
    }
    if (st.probe_cache_fresh && st.probe_cache_age_sec != null) {
      txt += " · " + i18n("pages.overview.coreStatusCached", { sec: st.probe_cache_age_sec });
    }
    const mp = formatOperatorMempoolCorpusLine(mempoolProbeMetricsFromCoreStatus(st));
    if (mp) txt += " · " + mp;
    el.textContent = txt;
    el.hidden = false;
    if (wrap) wrap.hidden = false;
  }

  function maybePollOverviewCoreProbes() {
    const now = Date.now();
    if (now - lastOvCoreProbeAt < OV_CORE_PROBE_MS) return;
    const ov = document.getElementById("panel-overview");
    if (!ov || !ov.classList.contains("active")) return;
    const ibd = lastSummary && (lastSummary.ibd_active || lastSummary.initialblockdownload);
    if (!ibd) return;
    lastOvCoreProbeAt = now;
    fetchCoreOperatorCert({}).then((cert) => {
      if (!cert) return;
      if (!cert.matrix_only) coreCertCache = cert;
      applyOverviewOperatorCert(cert);
      if (cert.probes && cert.probes.compare) renderCoreCompare(cert.probes.compare);
      else if (coreCertCache && coreCertCache.probes && coreCertCache.probes.compare) {
        renderCoreCompare(coreCertCache.probes.compare);
      }
    });
  }

  async function fetchCoreOperatorCert(opts) {
    opts = opts || {};
    const q = [];
    if (opts.matrix) q.push("matrix=1");
    if (opts.refresh) q.push("refresh=1");
    const url = "/api/core-operator-cert" + (q.length ? "?" + q.join("&") : "");
    try {
      const r = await fetch(url, { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      return await r.json();
    } catch (e) {
      return null;
    }
  }

  async function loadCoreOperatorCert(opts) {
    return fetchCoreOperatorCert(opts || {});
  }

  function setProbeMiniPill(id, state, text) {
    const el = $(id);
    if (!el) return;
    el.textContent = text;
    el.className = "p2p-health-pill " +
      (state === "ok" ? "ok" : state === "fail" ? "degraded" : state === "skip" ? "starting" : "warming");
    if (PROBE_MINI_ANCHORS[id]) {
      el.classList.add("probe-mini-jump");
      el.setAttribute("role", "button");
      el.setAttribute("tabindex", "0");
      el.title = i18n("pages.features.coreProbeJumpHint") || "Jump to probe card";
    }
  }

  function bindProbeStripMiniPills() {
    const row = document.querySelector(".feat-probes-pills");
    if (!row || row.dataset.probeJumpBound) return;
    row.dataset.probeJumpBound = "1";
    row.addEventListener("click", (e) => {
      const pill = e.target.closest(".p2p-health-pill[id]");
      if (!pill) return;
      const anchor = PROBE_MINI_ANCHORS[pill.id];
      if (anchor) gotoCertProbeAnchor(anchor);
    });
    row.addEventListener("keydown", (e) => {
      if (e.key !== "Enter" && e.key !== " ") return;
      const pill = e.target.closest(".p2p-health-pill[id]");
      if (!pill || !PROBE_MINI_ANCHORS[pill.id]) return;
      e.preventDefault();
      gotoCertProbeAnchor(PROBE_MINI_ANCHORS[pill.id]);
    });
  }

  function autostartProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "skip", text: "n/a" };
    if (!data.want_login) return { state: "skip", text: "off" };
    if (data.ok && !(data.warnings || []).length) return { state: "ok", text: "ok" };
    if (data.ok) return { state: "warn", text: "warn" };
    return { state: "fail", text: "fail" };
  }

  function founderProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "skip", text: "n/a" };
    if (data.ok && !(data.verify && data.verify.warnings && data.verify.warnings.length)) {
      return { state: "ok", text: "ok" };
    }
    if (data.ok) return { state: "warn", text: "warn" };
    return { state: "fail", text: "fail" };
  }

  function runnerProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "skip", text: "n/a" };
    if (data.ok) return { state: "ok", text: "ok" };
    return { state: "fail", text: "fail" };
  }

  function workflow10ProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "skip", text: "n/a" };
    const stages = (data.result && data.result.stages) || [];
    const okN = stages.filter((s) => s.ok && !s.skipped).length;
    if (data.ok) return { state: "ok", text: okN + "/" + stages.length };
    return { state: "fail", text: okN + "/" + stages.length };
  }

  function mempoolProbeMiniState(data) {
    if (!data || data.skipped) return { state: "skip", text: i18n("pages.mempool.paritySkipped") };
    if (data.ok && !data.core_configured) {
      const corpus = data.offline_corpus;
      const off = data.offline_stateful;
      const live = data.stateful_live;
      const base = (data.passed || 0) + "/" + (data.total || 0);
      if (corpus && corpus.total > 0) {
        let txt = base + " · corpus " + corpus.passed + "/" + corpus.total;
        if (off && off.total > 0) txt += " · stateful " + off.passed + "/" + off.total;
        if (live && live.reboot_testnet && live.offline_ok) {
          txt += " · live " + (live.live_scenarios || 0);
        }
        return { state: "ok", text: txt };
      }
      if (off && off.total > 0) {
        let txt = base + " · off " + off.passed + "/" + off.total;
        if (live && live.reboot_testnet && live.offline_ok) {
          txt += " · live " + (live.live_scenarios || 0);
        }
        return { state: "ok", text: txt };
      }
      return { state: "ok", text: base };
    }
    if (data.ok && (!data.core_available || data.core_aligned !== false)) {
      return { state: "ok", text: (data.passed || 0) + "/" + (data.total || 0) };
    }
    if (data.ok && data.core_available && data.core_aligned === false) {
      return { state: "warn", text: i18n("pages.mempool.parityCoreDrift") };
    }
    return { state: "fail", text: (data.passed || 0) + "/" + (data.total || 0) };
  }

  function compareProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (!data.core_available) {
      if (data.core_configured === false) {
        return { state: "skip", text: i18n("pages.features.coreCompareOptional") };
      }
      return { state: "skip", text: i18n("pages.features.coreCompareUnavailable") };
    }
    const verifyField = (data.fields || []).find((f) => f.name && String(f.name).indexOf("verifychain") === 0);
    const lagField = (data.fields || []).find((f) => f.name === "dogego_connect_lag");
    if (!data.chain_ok) return { state: "fail", text: i18n("pages.features.coreCompareMismatch") };
    if (verifyField && !verifyField.match) return { state: "warn", text: i18n("pages.features.coreCompareVerifyWarn") };
    if (data.connect_lag_ok === false || (lagField && !lagField.match)) {
      return { state: "warn", text: "connect lag" };
    }
    return { state: "ok", text: i18n("pages.features.coreCompareOk") };
  }

  function maintProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.ok) {
      if (data.ibd || (data.headers != null && data.blocks != null && data.blocks < data.headers)) {
        return { state: "warn", text: i18n("pages.features.coreMaintSyncing") };
      }
      if ((data.warnings || []).length) {
        return { state: "warn", text: i18n("pages.features.coreMaintWarn") };
      }
      return { state: "ok", text: i18n("pages.features.coreMaintOk") };
    }
    if ((data.issues || []).length) return { state: "fail", text: i18n("pages.features.coreMaintFail") };
    return { state: "warn", text: i18n("pages.features.coreMaintWarn") };
  }

  function ibdConvergenceProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "skip", text: i18n("pages.features.coreIbdConvergeSkipped") };
    if (data.ok) return { state: "ok", text: i18n("pages.features.coreIbdConvergeOk") };
    if ((data.issues || []).length) return { state: "fail", text: i18n("pages.features.coreIbdConvergeFail") };
    return { state: "warn", text: i18n("pages.features.coreIbdConvergeWarn") };
  }

  function addrmanProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "skip", text: i18n("pages.features.coreAddrmanSkipped") };
    if (data.ok) return { state: "ok", text: i18n("pages.features.coreAddrmanOk") };
    if ((data.issues || []).length) return { state: "fail", text: i18n("pages.features.coreAddrmanFail") };
    return { state: "warn", text: i18n("pages.features.coreAddrmanWarn") };
  }

  function resumeProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.ok) return { state: "ok", text: i18n("pages.features.coreResumeOk") };
    if ((data.issues || []).length) return { state: "fail", text: i18n("pages.features.coreResumeFail") };
    return { state: "warn", text: i18n("pages.features.coreResumeWarn") };
  }

  function connectProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    const maxLag = Number(data.connect_lag_max) > 0 ? Number(data.connect_lag_max) : 128;
    const lag = Number(data.connect_lag) || 0;
    const lagIssue = (data.issues || []).indexOf("connect_lag_above_threshold") >= 0;
    if (data.ibd && lag > maxLag) {
      const boost = formatConnectCatchUpBoostFromResume(data);
      return { state: "warn", text: boost ? i18n("pages.features.coreProbeConnectIbd") + " · " + boost : i18n("pages.features.coreProbeConnectIbd") };
    }
    if (!data.ibd && (lagIssue || lag > maxLag)) {
      return { state: "fail", text: i18n("pages.features.coreProbeConnectFail") };
    }
    return { state: "ok", text: i18n("pages.features.coreProbeConnectOk") };
  }

  function walletProbeMiniState(data) {
    if (!data || data.skipped) return { state: "skip", text: i18n("pages.features.coreWalletSkipped") };
    if (data.ok && !(data.warnings || []).length) return { state: "ok", text: i18n("pages.features.coreWalletOk") };
    if (data.ok) return { state: "warn", text: i18n("pages.features.coreWalletWarn") };
    return { state: "fail", text: i18n("pages.features.coreWalletFail") };
  }

  function bip152ProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (data.skipped) return { state: "warn", text: i18n("pages.features.coreBip152Skipped") };
    if (!data.ok) return { state: "fail", text: i18n("pages.features.coreBip152Fail") };
    if (data.ibd && !data.hb_negotiated) return { state: "warn", text: i18n("pages.features.coreMaintSyncing") };
    if (data.peer_count > 0 && !data.hb_negotiated) return { state: "warn", text: i18n("pages.features.coreBip152Warn") };
    return { state: "ok", text: i18n("pages.features.coreBip152Ok") };
  }

  function miningProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (!data.ok) return { state: "fail", text: i18n("pages.features.coreMiningFail") };
    if ((data.warnings || []).length) return { state: "warn", text: i18n("pages.features.coreMiningWarn") };
    return { state: "ok", text: i18n("pages.features.coreMiningOk") };
  }

  function pqProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (!data.ok) return { state: "fail", text: i18n("pages.features.corePqFail") };
    return { state: "ok", text: i18n("pages.features.corePqOk") };
  }

  function reindexProbeMiniState(data) {
    if (!data) return { state: "warn", text: "..." };
    if (!data.ok) return { state: "fail", text: i18n("pages.features.coreReindexFail") };
    if ((data.warnings || []).length) return { state: "warn", text: i18n("pages.features.coreReindexWarn") };
    if (data.ibd || (data.notes || []).some((n) => String(n).indexOf("catching_up") >= 0)) {
      return { state: "warn", text: i18n("pages.features.coreMaintSyncing") };
    }
    return { state: "ok", text: i18n("pages.features.coreReindexOk") };
  }

  function endToEndProbeMiniState(data) {
    if (!data || !data.steps) return { state: "warn", text: "..." };
    if (data.ok) return { state: "ok", text: i18n("pages.features.coreEndToEndOk") };
    const fail = (data.steps || []).find((s) => !s.skipped && !s.ok);
    return { state: "fail", text: fail ? fail.name : i18n("pages.features.coreEndToEndFail") };
  }

  function applyCoreStatusFromJSON(st) {
    if (!st) return;
    const oc = st.operator_cert;
    const card = $("ov-operator-cert-card");
    if (card && (st.core_rpc_configured || (oc && oc.total) || (st.mempool_offline_corpus && st.mempool_offline_corpus.total))) card.hidden = false;
    const rpcEl = $("ov-operator-cert-core-rpc");
    if (rpcEl) {
      if (st.core_rpc_addr) {
        rpcEl.textContent = i18n("pages.overview.coreStatusCoreRpc") + " " + st.core_rpc_addr +
          (st.core_rpc_configured ? "" : " (" + i18n("pages.features.live.coreRpcDefault") + ")");
        rpcEl.hidden = false;
      } else {
        rpcEl.hidden = true;
      }
    }
    const cacheEl = $("ov-operator-cert-cache");
    if (cacheEl) {
      if (st.probe_cache_fresh && st.probe_cache_age_sec != null) {
        cacheEl.textContent = i18n("pages.overview.coreStatusCached", { sec: st.probe_cache_age_sec });
        cacheEl.hidden = false;
      } else {
        cacheEl.textContent = i18n("pages.features.certLivePending");
        cacheEl.hidden = !st.core_rpc_configured;
      }
    }
    if (!oc || !oc.total) {
      applyOverviewMempoolCorpus(null, st);
      applySettingsCoreStatus(st);
      return;
    }
    applyOverviewOperatorCert({
      live_ok: !!oc.live_ok,
      solo_ok: !!oc.solo_ok,
      solo_pass: Number(oc.solo_pass) || 0,
      rows: Array.from({ length: Number(oc.total) || 0 }, (_, i) => ({
        web_probe: true,
        ok: i < (Number(oc.pass) || 0)
      }))
    });
    if (window.DogeGoSyncDock && window.DogeGoSyncDock.setOperatorCert) {
      const mp = mempoolProbeMetricsFromCoreStatus(st);
      window.DogeGoSyncDock.setOperatorCert({
        live_ok: !!oc.live_ok,
        solo_ok: !!oc.solo_ok,
        solo_pass: Number(oc.solo_pass) || 0,
        pass: Number(oc.pass) || 0,
        total: Number(oc.total) || 0,
        cached: !!st.probe_cache_fresh,
        corpus_ok: mp && mp.corpus_ok,
        corpus_passed: mp ? mp.corpus_passed : 0,
        corpus_total: mp ? mp.corpus_total : 0
      });
    }
    applyOverviewMempoolCorpus(null, st);
    applySettingsCoreStatus(st);
  }

  async function loadCoreStatus() {
    try {
      const r = await fetch("/api/core-status", { cache: "no-store" });
      if (!r.ok) return;
      applyCoreStatusFromJSON(await r.json());
    } catch (_) { /* */ }
  }

  function maybePollOverviewCoreStatus() {
    const now = Date.now();
    if (now - lastOvCoreStatusAt < OV_CORE_STATUS_MS) return;
    const ov = document.getElementById("panel-overview");
    if (!ov || !ov.classList.contains("active")) return;
    lastOvCoreStatusAt = now;
    void loadCoreStatus();
  }

  function fillCoreEndToEndUI(data) {
    if (!data) return;
    const pill = $("feat-core-e2e-pill");
    const summary = $("feat-core-e2e-summary");
    const stepsEl = $("feat-core-e2e-steps");
    if (pill) {
      if (data.ok) {
        pill.textContent = i18n("pages.features.coreE2eOk");
        pill.className = "p2p-health-pill ok";
      } else {
        pill.textContent = i18n("pages.features.coreE2eFail");
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      const steps = data.steps || [];
      const done = steps.filter((s) => s.skipped || s.ok).length;
      summary.textContent = done + "/" + steps.length + " steps" +
        (data.checked_at ? " · " + String(data.checked_at).replace("T", " ").replace("Z", " UTC") : "");
    }
    if (stepsEl && data.steps) {
      stepsEl.innerHTML = data.steps.map((s) => {
        const st = s.skipped ? "na" : (s.ok ? "live" : "partial");
        const statusTxt = s.skipped ? i18n("pages.features.coreE2eSkipped") : (s.ok ? i18n("pages.features.coreEndToEndOk") : i18n("pages.features.coreEndToEndFail"));
        const note = s.note ? "<p class=\"label\">" + escapeHtml(s.note) + "</p>" : "";
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(s.name || "") + "</h3>" +
          "<p class=\"label\">" + escapeHtml(statusTxt) + "</p>" + note + "</div></article>";
      }).join("");
    }
  }

  function fillCoreFieldEvidenceUI(data) {
    if (!data) return;
    const pill = $("feat-core-field-pill");
    const summary = $("feat-core-field-summary");
    const checks = $("feat-core-field-checks");
    const st = data.status || {};
    if (pill) {
      if (data.live_ok) {
        pill.textContent = i18n("pages.features.coreFieldEvidenceOk");
        pill.className = "p2p-health-pill ok";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreFieldEvidenceOffline");
        pill.className = "p2p-health-pill warming";
      } else {
        pill.textContent = i18n("pages.features.coreFieldEvidenceWarn");
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      const parts = [];
      if (st.chain_dir) parts.push(st.chain_dir);
      if (st.tip_height != null && st.tip_height >= 0) parts.push("tip " + st.tip_height);
      if (st.contiguous_raw != null && st.contiguous_raw >= 0) parts.push("stored " + st.contiguous_raw);
      if (data.notes && data.notes.indexOf("milestone_a_mainnet_only") >= 0) {
        parts.push(i18n("pages.features.coreFieldEvidenceMainnetOnly"));
      }
      summary.textContent = parts.join(" · ") || (data.hint ? String(data.hint).slice(0, 120) : "...");
    }
    if (checks && data.checks) {
      checks.innerHTML = data.checks.map((c) => {
        const stClass = c.status === "ok" ? "live" : (c.status === "skipped" ? "na" : "partial");
        let detail = c.note ? escapeHtml(c.note) : "";
        if (c.value != null) {
          const val = typeof c.value === "object" ? JSON.stringify(c.value) : String(c.value);
          detail = (detail ? detail + " · " : "") + "<span class=\"mono\">" + escapeHtml(val) + "</span>";
        }
        return "<article class=\"feat-row\"><div>" + statusPill(stClass) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
    }
  }

  async function loadCoreFieldEvidenceProbe() {
    const pill = $("feat-core-field-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-field-evidence-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreFieldEvidenceUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-field-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreEndToEndProbe() {
    const pill = $("feat-core-e2e-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-end-to-end-probe?refresh=1", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreEndToEndUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const stepsEl = $("feat-core-e2e-steps");
      if (stepsEl) stepsEl.innerHTML = "";
    }
  }

  function fillCoreBip152UI(data) {
    if (!data) return;
    const pill = $("feat-core-bip152-pill");
    const summary = $("feat-core-bip152-summary");
    const checks = $("feat-core-bip152-checks");
    const mini = bip152ProbeMiniState(data);
    if (pill) {
      pill.textContent = mini.text;
      pill.className = "p2p-health-pill " + (mini.state === "ok" ? "ok" : mini.state === "fail" ? "degraded" : "warming");
    }
    if (summary) {
      summary.textContent = "peers " + (data.peer_count != null ? data.peer_count : "?") +
        " · hb_to " + (data.hb_to_peers != null ? data.hb_to_peers : "?") +
        " · hb_from " + (data.hb_from_peers != null ? data.hb_from_peers : "?") +
        (data.core_available ? " · core hb " + (data.core_hb_negotiated ? "yes" : "no") : "") +
        (data.cmpct_relay && data.cmpct_relay.dogego_cmpct_reconstruct_ok > 0
          ? " · cmpct_ok " + data.cmpct_relay.dogego_cmpct_reconstruct_ok : "") +
        (data.ibd ? " · IBD" : "");
    }
    if (checks) {
      const rows = [];
      (data.issues || []).forEach((iss) => rows.push({ name: iss, status: "issue" }));
      (data.notes || []).forEach((n) => rows.push({ name: n, status: "ok" }));
      if (data.cmpct_relay_schema_ok) {
        rows.push({ name: "cmpct_relay_schema", status: "ok", value: "all dogego_cmpct_* counters present" });
      }
      if (data.cmpct_relay) {
        const cr = data.cmpct_relay;
        rows.push({
          name: "cmpct_relay_counters",
          status: "ok",
          note: "in=" + (cr.dogego_cmpct_in || 0) +
            " reconstruct_ok=" + (cr.dogego_cmpct_reconstruct_ok || 0) +
            " announced=" + (cr.dogego_cmpct_announced_out || 0) +
            " served=" + (cr.dogego_cmpct_served_getdata || 0)
        });
      }
      if (data.core_available) {
        rows.push({
          name: "core_getpeerinfo",
          status: "ok",
          note: "core peers " + (data.core_peer_count != null ? data.core_peer_count : "?") +
            " hb_to " + (data.core_hb_to_peers != null ? data.core_hb_to_peers : "?") +
            " hb_neg " + (data.core_hb_negotiated ? "yes" : "no")
        });
      }
      checks.innerHTML = rows.map((c) => {
        const st = c.status === "ok" ? "live" : "partial";
        const detail = c.note ? escapeHtml(c.note) : "";
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
    }
  }

  async function loadCoreBip152Probe() {
    const pill = $("feat-core-bip152-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-bip152-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreBip152UI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-bip152-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  function fillCoreMiningUI(data) {
    if (!data) return;
    const pill = $("feat-core-mining-pill");
    const summary = $("feat-core-mining-summary");
    const checks = $("feat-core-mining-checks");
    const mini = miningProbeMiniState(data);
    if (pill) {
      pill.textContent = mini.text;
      pill.className = "p2p-health-pill " + (mini.state === "ok" ? "ok" : mini.state === "fail" ? "degraded" : "warming");
    }
    if (summary) {
      summary.textContent = "blocks " + (data.blocks != null ? data.blocks : "?") +
        (data.aux_era ? " · aux era" : " · pre-aux") +
        (data.gbt_fields_ok ? " · gbt ok" : "") +
        (data.createaux_ok ? " · createaux ok" : data.createaux_skipped ? " · createaux skip" : "") +
        (data.core_aligned ? " · core aligned" : data.core_configured ? " · core optional" : "");
    }
    if (checks) {
      const rows = (data.checks || []).slice();
      (data.issues || []).forEach((iss) => rows.push({ name: iss, status: "issue" }));
      (data.notes || []).forEach((n) => rows.push({ name: n, status: "ok" }));
      checks.innerHTML = rows.map((c) => {
        const st = c.status === "ok" ? "live" : (c.status === "skipped" ? "partial" : "partial");
        const detail = c.note ? escapeHtml(c.note) : "";
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
    }
  }

  async function loadCoreMiningProbe() {
    const pill = $("feat-core-mining-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-mining-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreMiningUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-mining-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  function fillCorePQUI(data) {
    if (!data) return;
    const pill = $("feat-core-pq-pill");
    const summary = $("feat-core-pq-summary");
    const checks = $("feat-core-pq-checks");
    const mini = pqProbeMiniState(data);
    if (pill) {
      pill.textContent = mini.text;
      pill.className = "p2p-health-pill " + (mini.state === "ok" ? "ok" : mini.state === "fail" ? "degraded" : "warming");
    }
    if (summary) {
      const okCount = (data.checks || []).filter((c) => c.status === "ok").length;
      summary.textContent = (data.checks || []).length + " checks · " + okCount + " ok" +
        ((data.issues || []).length ? " · " + data.issues.length + " issue(s)" : "");
    }
    if (checks && data.checks) {
      checks.innerHTML = data.checks.map((c) => {
        const st = c.status === "ok" ? "live" : "partial";
        let detail = c.note ? escapeHtml(c.note) : "";
        if (c.value != null) {
          const val = typeof c.value === "object" ? JSON.stringify(c.value) : String(c.value);
          detail = (detail ? detail + " · " : "") + "<span class=\"mono\">" + escapeHtml(val) + "</span>";
        }
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
    }
  }

  async function loadCorePQProbe() {
    const pill = $("feat-core-pq-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-pq-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCorePQUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-pq-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  function fillCoreReindexUI(data) {
    if (!data) return;
    const pill = $("feat-core-reindex-pill");
    const summary = $("feat-core-reindex-summary");
    const checks = $("feat-core-reindex-checks");
    if (pill) {
      if (data.ok && !(data.warnings || []).length) {
        pill.textContent = i18n("pages.features.coreReindexOk");
        pill.className = "p2p-health-pill ok";
      } else if (data.ok) {
        pill.textContent = i18n("pages.features.coreReindexWarn");
        pill.className = "p2p-health-pill warming";
      } else {
        pill.textContent = i18n("pages.features.coreReindexFail");
        pill.className = "p2p-health-pill degraded";
      }
    }
    if (summary) {
      summary.textContent = "blocks " + (data.blocks != null ? data.blocks : "?") +
        (data.ibd ? " · IBD" : "") +
        (data.core_available ? " · Core compare" : "") +
        ((data.warnings || []).length ? " · " + (data.warnings || []).length + " warn" : "");
    }
    if (checks && data.checks) {
      checks.innerHTML = data.checks.map((c) => {
        const st = c.status === "ok" ? "live" : "partial";
        let detail = c.note ? escapeHtml(c.note) : "";
        if (c.value != null) {
          const val = typeof c.value === "object" ? JSON.stringify(c.value) : String(c.value);
          detail = (detail ? detail + " · " : "") + "<span class=\"mono\">" + escapeHtml(val) + "</span>";
        }
        return "<article class=\"feat-row\"><div>" + statusPill(st) + "</div><div>" +
          "<h3 class=\"mono\">" + escapeHtml(c.name || "") + "</h3>" +
          (detail ? "<p class=\"label\">" + detail + "</p>" : "") + "</div></article>";
      }).join("");
    }
  }

  async function loadCoreReindexProbe() {
    const pill = $("feat-core-reindex-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-reindex-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreReindexUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-reindex-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  function renderCoreProbeStrip(bundle) {
    if (!bundle) return;
    const checked = $("feat-core-probes-checked");
    if (checked) checked.textContent = bundle.checked_at ? bundle.checked_at.replace("T", " ").replace("Z", " UTC") : "...";
    const cmp = compareProbeMiniState(bundle.compare);
    setProbeMiniPill("feat-probe-compare-mini", cmp.state, cmp.text);
    const maint = maintProbeMiniState(bundle.maintenance);
    setProbeMiniPill("feat-probe-maint-mini", maint.state, maint.text);
    const resume = resumeProbeMiniState(bundle.restart_resume);
    setProbeMiniPill("feat-probe-resume-mini", resume.state, resume.text);
    const ibd = ibdConvergenceProbeMiniState(bundle.ibd_convergence);
    setProbeMiniPill("feat-probe-ibd-mini", ibd.state, ibd.text);
    const addrman = addrmanProbeMiniState(bundle.addrman);
    setProbeMiniPill("feat-probe-addrman-mini", addrman.state, addrman.text);
    const autostart = autostartProbeMiniState(bundle.autostart);
    setProbeMiniPill("feat-probe-autostart-mini", autostart.state, autostart.text);
    const founder = founderProbeMiniState(bundle.founder);
    setProbeMiniPill("feat-probe-founder-mini", founder.state, founder.text);
    const runner = runnerProbeMiniState(bundle.runner);
    setProbeMiniPill("feat-probe-runner-mini", runner.state, runner.text);
    const wf10 = workflow10ProbeMiniState(bundle.workflow10);
    setProbeMiniPill("feat-probe-workflow10-mini", wf10.state, wf10.text);
    const connect = connectProbeMiniState(bundle.restart_resume);
    setProbeMiniPill("feat-probe-connect-mini", connect.state, connect.text);
    const mp = mempoolProbeMiniState(bundle.mempool_parity);
    setProbeMiniPill("feat-probe-mempool-mini", mp.state, mp.text);
    const wal = walletProbeMiniState(bundle.wallet);
    setProbeMiniPill("feat-probe-wallet-mini", wal.state, wal.text);
    const reindex = reindexProbeMiniState(bundle.reindex);
    setProbeMiniPill("feat-probe-reindex-mini", reindex.state, reindex.text);
    const bip152 = bip152ProbeMiniState(bundle.bip152);
    setProbeMiniPill("feat-probe-bip152-mini", bip152.state, bip152.text);
    const mining = miningProbeMiniState(bundle.mining);
    setProbeMiniPill("feat-probe-mining-mini", mining.state, mining.text);
    const pq = pqProbeMiniState(bundle.pq);
    setProbeMiniPill("feat-probe-pq-mini", pq.state, pq.text);
    const e2e = endToEndProbeMiniState(bundle.end_to_end);
    setProbeMiniPill("feat-probe-e2e-mini", e2e.state, e2e.text);
  }

  function applyCoreProbeBundle(bundle, checkedAtFallback) {
    if (!bundle) return;
    renderCoreProbeStrip(bundle);
    const ts = bundle.checked_at || checkedAtFallback;
    if (ts) {
      const checked = $("feat-core-probes-checked");
      if (checked) checked.textContent = String(ts).replace("T", " ").replace("Z", " UTC");
    }
    if (bundle.compare) renderCoreCompare(bundle.compare);
    if (bundle.maintenance) fillCoreMaintenanceUI(bundle.maintenance);
    if (bundle.restart_resume) fillCoreRestartResumeUI(bundle.restart_resume);
    if (bundle.ibd_convergence) fillCoreIbdConvergenceUI(bundle.ibd_convergence);
    if (bundle.addrman) fillCoreAddrmanUI(bundle.addrman);
    if (bundle.autostart) fillCoreAutostartUI(bundle.autostart);
    if (bundle.founder) fillCoreFounderUI(bundle.founder);
    if (bundle.runner) fillCoreRunnerUI(bundle.runner);
    if (bundle.workflow10) fillCoreWorkflow10UI(bundle.workflow10);
    if (bundle.setup_parity) fillCoreSetupParityUI(bundle.setup_parity);
    if (bundle.mempool_parity) fillMempoolParityUI(bundle.mempool_parity);
    if (bundle.wallet) fillCoreWalletUI(bundle.wallet);
    if (bundle.reindex) fillCoreReindexUI(bundle.reindex);
    if (bundle.bip152) fillCoreBip152UI(bundle.bip152);
    if (bundle.mining) fillCoreMiningUI(bundle.mining);
    if (bundle.pq) fillCorePQUI(bundle.pq);
    if (bundle.end_to_end) fillCoreEndToEndUI(bundle.end_to_end);
  }

  function applyCoreProbePresetUI(path, data) {
    if (!data || !path) return;
    if (path.indexOf("core-operator-cert") >= 0) {
      if (!data.matrix_only) renderOperatorCert(data);
      if (data.probes) applyCoreProbeBundle(data.probes, data.checked_at);
      return;
    }
    if (path.indexOf("core-probes") >= 0) {
      applyCoreProbeBundle(data, data.checked_at);
      return;
    }
    if (path.indexOf("core-compare") >= 0) { renderCoreCompare(data); return; }
    if (path.indexOf("parity-probe") >= 0) { fillMempoolParityUI(data); return; }
    if (path.indexOf("stateful-status") >= 0) {
		fillMempoolParityUI({
        offline_corpus: data.offline_corpus,
        offline_stateful: data.offline_stateful,
        stateful_live: data.stateful_live,
        skipped: false,
        ok: true,
        passed: 0,
        total: 0
      });
      return;
    }
    if (path.indexOf("core-maintenance") >= 0) { fillCoreMaintenanceUI(data); return; }
    if (path.indexOf("core-restart-resume") >= 0) { fillCoreRestartResumeUI(data); return; }
    if (path.indexOf("core-ibd-convergence-probe") >= 0) { fillCoreIbdConvergenceUI(data); return; }
    if (path.indexOf("core-addrman-probe") >= 0) { fillCoreAddrmanUI(data); return; }
    if (path.indexOf("core-autostart-probe") >= 0) { fillCoreAutostartUI(data); return; }
    if (path.indexOf("core-founder-probe") >= 0) { fillCoreFounderUI(data); return; }
    if (path.indexOf("core-setup-parity") >= 0) {
      fillCoreSetupParityUI(data);
      return;
    }
    if (path.indexOf("core-runner-probes") >= 0) { fillCoreRunnerUI(data); return; }
    if (path.indexOf("core-workflow10-probe") >= 0) { fillCoreWorkflow10UI(data); return; }
    if (path.indexOf("core-wallet-probe") >= 0) { fillCoreWalletUI(data); return; }
    if (path.indexOf("core-reindex-probe") >= 0) { fillCoreReindexUI(data); return; }
    if (path.indexOf("core-bip152-probe") >= 0) { fillCoreBip152UI(data); return; }
    if (path.indexOf("core-mining-probe") >= 0) { fillCoreMiningUI(data); return; }
    if (path.indexOf("core-pq-probe") >= 0) { fillCorePQUI(data); return; }
    if (path.indexOf("core-end-to-end-probe") >= 0) { fillCoreEndToEndUI(data); return; }
    if (path.indexOf("core-field-evidence-probe") >= 0) { fillCoreFieldEvidenceUI(data); return; }
    if (path.indexOf("core-status") >= 0) { applyCoreStatusFromJSON(data); return; }
  }

  async function loadCoreProbes(forceRefresh) {
    const stripIds = ["feat-probe-compare-mini", "feat-probe-maint-mini", "feat-probe-resume-mini", "feat-probe-ibd-mini", "feat-probe-addrman-mini", "feat-probe-autostart-mini", "feat-probe-founder-mini", "feat-probe-runner-mini", "feat-probe-workflow10-mini", "feat-probe-connect-mini", "feat-probe-mempool-mini", "feat-probe-wallet-mini", "feat-probe-reindex-mini", "feat-probe-bip152-mini", "feat-probe-pq-mini", "feat-probe-e2e-mini"];
    stripIds.forEach((id) => setProbeMiniPill(id, "warn", "..."));
    const certUrl = (forceRefresh === false) ? "/api/core-operator-cert" : "/api/core-operator-cert?refresh=1";
    try {
      const r = await fetch(certUrl, { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const cert = await r.json();
      renderOperatorCert(cert);
      applyCoreProbeBundle(cert.probes || {}, cert.checked_at);
    } catch (e) {
      renderCoreCompare({ core_available: false, errors: [String(e.message || e)], fields: [] });
      stripIds.forEach((id) => setProbeMiniPill(id, "fail", String(e.message || e)));
    }
  }

  async function loadCoreAutostart() {
    const pill = $("feat-core-autostart-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-autostart-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreAutostartUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-autostart-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreWorkflow10() {
    const pill = $("feat-core-workflow10-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-workflow10-probe?skip_provision=1&mine_bootstrap=1", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreWorkflow10UI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-workflow10-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreRunner() {
    const pill = $("feat-core-runner-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-runner-probes?require_core=1", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreRunnerUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-runner-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreFounder() {
    const pill = $("feat-core-founder-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-founder-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreFounderUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-founder-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreIbdConvergence() {
    const pill = $("feat-core-ibd-converge-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-ibd-convergence-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreIbdConvergenceUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-ibd-converge-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreAddrman() {
    const pill = $("feat-core-addrman-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-addrman-probe", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreAddrmanUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-addrman-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreRestartResume() {
    const pill = $("feat-core-resume-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-restart-resume", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreRestartResumeUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-resume-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreMaintenance() {
    const pill = $("feat-core-maint-pill");
    if (pill) {
      pill.textContent = "...";
      pill.className = "p2p-health-pill warming";
    }
    try {
      const r = await fetch("/api/core-maintenance", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      fillCoreMaintenanceUI(await r.json());
    } catch (e) {
      if (pill) {
        pill.textContent = String(e.message || e);
        pill.className = "p2p-health-pill degraded";
      }
      const checks = $("feat-core-maint-checks");
      if (checks) checks.innerHTML = "";
    }
  }

  async function loadCoreCompare() {
    try {
      const r = await fetch("/api/core-compare", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      renderCoreCompare(await r.json());
    } catch (e) {
      renderCoreCompare({ core_available: false, errors: [String(e.message || e)], fields: [] });
    }
  }

  async function testCoreConnection() {
    const msgEl = $("st-core-test-msg");
    const btn = $("st-core-test");
    if (msgEl) {
      msgEl.hidden = false;
      msgEl.textContent = i18n("settings.coreTestBusy");
      msgEl.className = "label";
    }
    if (btn) btn.disabled = true;
    const body = {
      core_rpc_addr: $("st-core-rpc") ? $("st-core-rpc").value.trim() : "",
      core_rpc_user: $("st-core-rpc-user") ? $("st-core-rpc-user").value.trim() : "",
      core_rpc_password: $("st-core-rpc-pass") ? $("st-core-rpc-pass").value : ""
    };
    try {
      const r = await fetch("/api/core-test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
      });
      const data = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error((data.errors && data.errors[0]) || "HTTP " + r.status);
      if (msgEl) {
        if (data.ok && data.core_available) {
          const chain = data.chain || data.network || "";
          const blocks = data.blocks != null ? data.blocks.toLocaleString() : "?";
          msgEl.textContent = i18n("settings.coreTestOk") + " · " + chain + " · " + blocks + " blocks · " + (data.core_rpc_addr || "");
          msgEl.className = "label ok-inline";
          void loadCoreStatus();
        } else {
          const err = (data.errors && data.errors.join(" · ")) || data.hint || i18n("settings.coreTestFail");
          msgEl.textContent = i18n("settings.coreTestFail") + ": " + err;
          msgEl.className = "label err-inline";
        }
      }
    } catch (e) {
      if (msgEl) {
        msgEl.textContent = i18n("settings.coreTestFail") + ": " + String(e.message || e);
        msgEl.className = "label err-inline";
      }
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  async function testSignerConnection() {
    const msgEl = $("st-signer-test-msg");
    const btn = $("st-signer-test");
    if (msgEl) {
      msgEl.hidden = false;
      msgEl.textContent = i18n("settings.signerTestBusy");
      msgEl.className = "label";
    }
    if (btn) btn.disabled = true;
    const body = {
      signer_cmd: $("st-signer-cmd") ? $("st-signer-cmd").value.trim() : ""
    };
    try {
      const r = await fetch("/api/signer-test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
      });
      const data = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error((data.errors && data.errors[0]) || "HTTP " + r.status);
      if (msgEl) {
        if (data.ok && data.signer_configured) {
          const n = data.device_count != null ? data.device_count : 0;
          msgEl.textContent = i18n("settings.signerTestOk") + " · " + n + " device(s) · " + (data.signer_cmd || "");
          msgEl.className = "label ok-inline";
        } else if (data.signer_cmd && !data.signer_configured) {
          const err = (data.errors && data.errors.join(" · ")) || data.hint || i18n("settings.signerTestWarn");
          msgEl.textContent = i18n("settings.signerTestWarn") + ": " + err;
          msgEl.className = "label warn-inline";
        } else {
          const err = (data.errors && data.errors.join(" · ")) || data.hint || i18n("settings.signerTestFail");
          msgEl.textContent = i18n("settings.signerTestFail") + ": " + err;
          msgEl.className = "label err-inline";
        }
      }
    } catch (e) {
      if (msgEl) {
        msgEl.textContent = i18n("settings.signerTestFail") + ": " + String(e.message || e);
        msgEl.className = "label err-inline";
      }
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  function renderCapabilities(manifest) {
    if (!manifest) return;
    capabilitiesCache = manifest;
    const disc = $("feat-disclaimer");
    if (disc) {
      disc.textContent = manifest.disclaimer || "";
      disc.removeAttribute("data-doge-wait");
    }
    renderParitySummary(manifest.parity_summary);
    renderCoreGuidance(manifest.core_guidance);
    renderCertification(manifest.certification);
    renderCoreProbeAPIs(manifest.core_probe_apis);
    loadCoreProbes();
    const live = manifest.live || {};
    const liveEl = $("feat-live");
    if (liveEl) {
      const L = function (k) { return i18n("pages.features.live." + k); };
      liveEl.innerHTML = "<h2>" + escapeHtml(i18n("pages.features.liveTitle")) + "</h2><div class=\"live-strip\">" +
        "<span>" + L("mode") + " <strong>" + (live.node_mode || "...") + "</strong></span>" +
        "<span>" + L("network") + " <strong>" + (live.network || "...") + "</strong></span>" +
        "<span>" + L("p2p") + " <strong>" + (live.p2p_connectivity || "both") + "</strong></span>" +
        "<span>" + L("p2pHealth") + " <strong>" + p2pHealthLabel(live.health) + "</strong></span>" +
        "<span>" + L("peers") + " <strong>" + (live.connections_total != null ? live.connections_total : "...") + "</strong></span>" +
        "<span>" + L("listen") + " <strong>" + (live.listen_enabled === true ? i18n("common.on") : live.listen_enabled === false ? i18n("common.off") : "...") + "</strong></span>" +
        (live.upnp_mapped ? "<span>UPnP <strong>" + (live.upnp_external || "on") + "</strong></span>" : "") +
        (live.dgr_enabled ? "<span>DGR <strong>" + (live.dgr_inbound ? "inbound" : live.dgr_outbound ? "outbound" : "on") + "</strong></span>" : "") +
        (live.zmq_enabled ? "<span>ZMQ <strong>on</strong></span>" : "") +
        "<span>RPC <strong>" + rpcOverviewLabel(live) + "</strong></span>" +
        "<span>" + L("wallet") + " <strong>" + (live.wallet_enabled ? (live.wallet_rpc_ready ? i18n("common.on") : "loaded") : i18n("common.off")) + "</strong></span>" +
        (live.mining_active ? "<span>Mining <strong>" + i18n("common.on") + "</strong></span>" : "") +
        "<span>" + L("txIndex") + " <strong>" + (live.tx_index ? i18n("common.yes") : i18n("common.no")) + "</strong></span>" +
        "<span>" + L("rawBlocks") + " <strong>" + (live.raw_blocks ? i18n("common.yes") : i18n("common.no")) + "</strong></span>" +
        "<span>" + L("analytics") + " <strong>" + (live.embedded_analytics_sidecar ? i18n("common.yes") : i18n("common.no")) + "</strong></span>" +
        (live.relay_min_doge != null ? "<span>" + L("relayMin") + " <strong>" + formatRelayDOGE(live.relay_min_doge) + "</strong></span>" : "") +
        (live.package_ancestors != null ? "<span>" + L("pkgAnc") + " <strong>" + live.package_ancestors + "</strong></span>" : "") +
        (live.acceptdatacarrier != null ? "<span>OP_RETURN <strong>" + (live.acceptdatacarrier ? i18n("common.on") : i18n("common.off")) + "</strong></span>" : "") +
        (live.core_rpc_addr ? "<span>" + L("coreRpc") + " <strong class=\"mono\">" + escapeHtml(live.core_rpc_addr) + "</strong>" +
          (live.core_rpc_configured ? "" : " <span class=\"label\">(" + L("coreRpcDefault") + ")</span>") + "</span>" : "") +
        (live.operator_cert_total != null && live.operator_cert_total > 0
          ? "<span>" + L("operatorCert") + " <strong>" + (live.operator_cert_pass || 0) + "/" + live.operator_cert_total +
            (live.operator_cert_live_ok ? " ok" : "") +
            (live.operator_cert_solo_pass != null &&
              (live.operator_cert_solo_ok !== live.operator_cert_live_ok ||
                live.operator_cert_solo_pass !== live.operator_cert_pass)
              ? " · solo " + live.operator_cert_solo_pass + "/" + live.operator_cert_total
              : "") +
            "</strong></span>"
          : "") +
        (live.mempool_offline_corpus_total > 0
          ? "<span>" + L("mempoolCorpus") + " <strong>" + (live.mempool_offline_corpus_passed || 0) + "/" + live.mempool_offline_corpus_total +
            (live.mempool_parity_total ? " · " + (live.mempool_parity_passed || 0) + "/" + live.mempool_parity_total : "") +
            "</strong></span>"
          : "") +
        (live.dogego_utxo_bodies_aligned === false
          ? "<span>" + L("utxoReplay") + " <strong>" + escapeHtml(formatUtxoReplaySummary(live) || "...") + "</strong></span>"
          : "") +
        "</div>";
    }
    const roadEl = $("feat-roadmap-list");
    if (roadEl && manifest.roadmap && manifest.roadmap.length) {
      roadEl.innerHTML = manifest.roadmap.map((r) =>
        "<article class=\"roadmap-row" + (r.done ? " done" : "") + "\">" +
        "<div>" + statusPill(r.done ? "live" : "planned") + "</div>" +
        "<div><div class=\"roadmap-title\"><strong>" + escapeHtml(r.phase || "") + "</strong> - " + escapeHtml(r.title || "") + "</div>" +
        "<p class=\"label\">" + escapeHtml(r.summary || "") + "</p></div></article>"
      ).join("");
    }
    const gapsEl = $("feat-core-gaps-list");
    if (gapsEl) {
      gapsEl.innerHTML = "";
      gapsEl.hidden = true;
    }
    const root = $("feat-categories");
    if (root && manifest.categories) {
      const openLbl = i18n("pages.features.openTab");
      const corePrefix = i18n("pages.features.corePrefix");
      root.innerHTML = manifest.categories.map((cat) => {
        const rows = (cat.features || []).map((f) => {
          let open = "";
          if (f.ui_tab) open = "<button type=\"button\" class=\"btn btn-ghost feat-open\" data-goto-tab=\"" + f.ui_tab + "\">" + escapeHtml(openLbl) + "</button>";
          let core = f.core_note ? "<p class=\"core-note\">" + escapeHtml(corePrefix) + " " + escapeHtml(f.core_note) + "</p>" : "";
          return "<article class=\"feat-row\"><div>" + statusPill(f.status) + "</div><div><h3>" + escapeHtml(f.name) + "</h3><p>" + escapeHtml(f.summary) + "</p>" + core + "</div>" + open + "</article>";
        }).join("");
        return "<section class=\"feat-category card card-wide stack-gap\"><h2>" + escapeHtml(cat.title) + "</h2>" +
          (cat.blurb ? "<p class=\"cat-blurb\">" + escapeHtml(cat.blurb) + "</p>" : "") +
          "<div class=\"feat-list\">" + rows + "</div></section>";
      }).join("");
      root.querySelectorAll("[data-goto-tab]").forEach((b) => {
        b.addEventListener("click", () => showTab(b.getAttribute("data-goto-tab")));
      });
    }
    renderRPCMethods(manifest.rpc_methods || []);
  }

  function renderRPCMethods(methods) {
    const tbody = $("rpc-tbody");
    if (!tbody) return;
    const filter = ($("rpc-filter") && $("rpc-filter").value || "").trim().toLowerCase();
    const rows = methods.filter((m) => !filter || m.method.indexOf(filter) >= 0 || (m.help || "").toLowerCase().indexOf(filter) >= 0);
    const cnt = $("rpc-filter-count");
    if (cnt) cnt.textContent = rows.length + " / " + methods.length;
    tbody.innerHTML = rows.map((m) =>
      "<tr><td class=\"mono\">" + m.method + "</td><td>" + statusPill(m.class) + "</td><td>" + (m.help || "") + "</td></tr>"
    ).join("");
  }

  function renderDocManifest(manifest, cacheKey, titleId, subId, rootId, sectionPrefix) {
    if (!manifest) return;
    if (cacheKey === "docs") docsCache = manifest;
    const title = $(titleId);
    const sub = $(subId);
    if (title && manifest.title) title.textContent = manifest.title;
    if (sub && manifest.subtitle) sub.textContent = manifest.subtitle;
    const root = $(rootId);
    if (!root || !manifest.sections) return;
    root.innerHTML = manifest.sections.map((sec) => {
      let terms = "";
      if (sec.terms && sec.terms.length) {
        terms = "<dl class=\"glossary-dl\">" + sec.terms.map((t) =>
          "<dt>" + escapeHtml(t.term || "") + "</dt><dd>" + escapeHtml(t.explain || "") + "</dd>"
        ).join("") + "</dl>";
      }
      let links = "";
      if (sec.links && sec.links.length) {
        links = "<ul class=\"docs-link-list\">" + sec.links.map((l) =>
          "<li><button type=\"button\" class=\"btn btn-ghost btn-sm docs-open-md\" data-doc-path=\"" + escapeHtml(l.path || "") + "\">" +
          escapeHtml(l.label || l.path || "") + "</button> <code class=\"docs-path-tag\">" + escapeHtml(l.path || "") + "</code></li>"
        ).join("") + "</ul>";
      }
      const body = escapeHtml(sec.body || "").replace(/\n\n/g, "</p><p>").replace(/\n/g, "<br>");
      return "<section class=\"guide-section\" id=\"" + sectionPrefix + escapeHtml(sec.id || "") + "\">" +
        "<h2>" + escapeHtml(sec.title || "") + "</h2>" +
        "<p>" + body + "</p>" + terms + links + "</section>";
    }).join("");
  }

  function renderDocs(manifest) {
    renderDocManifest(manifest, "docs", "docs-title", "docs-subtitle", "docs-sections", "docs-");
    bindDocsMarkdownLinks();
    filterDocsSections();
    void enrichDocsDIPsSection();
  }

  async function enrichDocsDIPsSection() {
    const host = document.querySelector("#docs-dips");
    if (!host || host.querySelector(".dips-grid")) return;
    try {
      const r = await fetch("/api/dips", { cache: "no-store" });
      if (!r.ok) return;
      const data = await r.json();
      const entries = data.entries || [];
      if (!entries.length) return;
      const grid = document.createElement("div");
      grid.className = "dips-grid";
      entries.forEach((e) => {
        const card = document.createElement("button");
        card.type = "button";
        card.className = "dips-card";
        card.dataset.docPath = e.path || "";
        const status = escapeHtml(e.status || "");
        card.innerHTML =
          "<span class=\"dips-card-id\">" + escapeHtml(e.id || "") + "</span>" +
          "<span class=\"dips-card-title\">" + escapeHtml(e.title || "") + "</span>" +
          "<span class=\"dips-card-meta\">" +
          (e.bip ? "<span class=\"dips-chip\">" + escapeHtml(e.bip) + "</span>" : "") +
          "<span class=\"dips-chip dips-status-" + status.replace(/[^a-z0-9-]/gi, "") + "\">" + status + "</span>" +
          "</span>" +
          (e.summary ? "<span class=\"dips-card-sum\">" + escapeHtml(e.summary) + "</span>" : "");
        card.addEventListener("click", () => {
          if (e.path) openEmbeddedDoc(e.path);
        });
        grid.appendChild(card);
      });
      host.appendChild(grid);
    } catch (_) { /* ignore */ }
  }

  function ensureMarkedDocsOptions() {
    if (!window.marked || window.marked.__dogegoDocs) return;
    if (typeof window.marked.setOptions === "function") {
      window.marked.setOptions({
        gfm: true,
        headerIds: true,
        mangle: false,
      });
    }
    window.marked.__dogegoDocs = true;
  }

  function renderDocsMath(root) {
    if (!root || typeof window.renderMathInElement !== "function") return;
    try {
      window.renderMathInElement(root, {
        delimiters: [
          { left: "$$", right: "$$", display: true },
          { left: "\\[", right: "\\]", display: true },
          { left: "$", right: "$", display: false },
          { left: "\\(", right: "\\)", display: false },
        ],
        throwOnError: false,
        strict: "ignore",
      });
    } catch (_) { /* ignore */ }
  }

  function extractDocsMath(md) {
    const slots = [];
    let out = String(md || "").replace(/\$\$[\s\S]+?\$\$/g, (m) => {
      const i = slots.length;
      slots.push(m);
      return "@@DOGEGO_MATH_" + i + "@@";
    });
    out = out.replace(/(^|[^\\$])\$([^$\n]+?)\$/g, (_, pre, body) => {
      const i = slots.length;
      slots.push("$" + body + "$");
      return pre + "@@DOGEGO_MATH_" + i + "@@";
    });
    return { md: out, slots };
  }

  function restoreDocsMath(html, slots) {
    return String(html || "").replace(/@@DOGEGO_MATH_(\d+)@@/g, (_, i) => slots[+i] || "");
  }

  function parseDocsMarkdown(md) {
    ensureMarkedDocsOptions();
    const extracted = extractDocsMath(md);
    let html;
    if (window.marked && typeof window.marked.parse === "function") {
      html = window.marked.parse(extracted.md);
    } else {
      html = "<pre class=\"docs-md-fallback\">" + escapeHtml(md || "") + "</pre>";
    }
    return restoreDocsMath(html, extracted.slots);
  }

  function updateDocsCrumb(path) {
    const crumb = $("docs-md-crumb");
    if (!crumb) return;
    if (!path) {
      crumb.textContent = "";
      crumb.hidden = true;
      return;
    }
    crumb.hidden = false;
    crumb.textContent = path;
  }

  function renderDocsLoadError(body, err, data) {
    let html = "<div class=\"docs-md-error\">" +
      "<p class=\"err-inline\"><strong>Could not load this document.</strong></p>" +
      "<p class=\"label\">" + escapeHtml(String(err)) + "</p>";
    if (data && data.hint) {
      html += "<p class=\"label\">" + escapeHtml(data.hint) + "</p>";
    }
    if (data && data.path) {
      html += "<p class=\"label mono\">Path: " + escapeHtml(data.path) + "</p>";
    }
    html += "<p class=\"label\">Use the section links below, or close this viewer and pick another file.</p></div>";
    body.innerHTML = html;
  }

  function scrollDocAnchor(anchor) {
    if (!anchor || anchor === "#") return;
    const id = anchor.replace(/^#/, "");
    const body = $("docs-md-body");
    if (!body) return;
    const esc = typeof CSS !== "undefined" && CSS.escape ? CSS.escape(id) : id.replace(/[^a-zA-Z0-9_-]/g, "");
    const el = body.querySelector("#" + esc) || body.querySelector("[id=\"" + id + "\"]");
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  function bindMarkdownLinksIn(root, basePath, onInternal) {
    if (!root) return;
    root.querySelectorAll("a[href]").forEach((a) => {
      const href = (a.getAttribute("href") || "").trim();
      if (!href) return;
      if (/^https?:\/\//i.test(href) || /^mailto:/i.test(href)) {
        a.target = "_blank";
        a.rel = "noopener noreferrer";
        return;
      }
      a.addEventListener("click", (ev) => {
        ev.preventDefault();
        if (typeof onInternal === "function") {
          onInternal(basePath, href);
          return;
        }
        handleDocsMarkdownLink(basePath, href);
      });
    });
  }

  function bindDocsInlineLinks(basePath) {
    bindMarkdownLinksIn($("docs-md-body"), basePath);
  }

  async function handleDocsMarkdownLink(basePath, href) {
    if (href.startsWith("#")) {
      scrollDocAnchor(href);
      return;
    }
    try {
      const r = await fetch("/api/docs/resolve?base=" + encodeURIComponent(basePath) + "&href=" + encodeURIComponent(href), { cache: "no-store" });
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "HTTP " + r.status);
      if (data.external) {
        window.open(href, "_blank", "noopener,noreferrer");
        return;
      }
      await openEmbeddedDoc(data.path || "", data.anchor || "");
    } catch (e) {
      const body = $("docs-md-body");
      if (body) renderDocsLoadError(body, e.message || String(e), null);
    }
  }

  async function openEmbeddedDoc(rel, anchor) {
    const viewer = $("docs-md-viewer");
    const title = $("docs-md-title");
    const body = $("docs-md-body");
    if (!viewer || !body) return;
    viewer.hidden = false;
    if (title) title.textContent = rel;
    updateDocsCrumb(rel);
    wait(body, "Loading document…");
    try {
      const r = await fetch("/api/docs/md?path=" + encodeURIComponent(rel), { cache: "no-store" });
      const data = await r.json();
      if (!r.ok) {
        throw Object.assign(new Error(data.error || "HTTP " + r.status), { data: data });
      }
      const md = data.markdown || "";
      if (title && data.path) title.textContent = data.path;
      currentDocPath = data.path || rel;
      if (currentDocPath && docsPathHistory[docsPathHistory.length - 1] !== currentDocPath) {
        docsPathHistory.push(currentDocPath);
      }
      updateDocsCrumb(currentDocPath);
      body.innerHTML = parseDocsMarkdown(md);
      renderDocsMath(body);
      bindDocsInlineLinks(currentDocPath);
      if (anchor) scrollDocAnchor(anchor);
      viewer.scrollIntoView({ behavior: "smooth", block: "nearest" });
    } catch (e) {
      renderDocsLoadError(body, e.message || String(e), e.data || null);
    }
  }

  function docsViewerBack() {
    if (docsPathHistory.length > 1) {
      docsPathHistory.pop();
      const prev = docsPathHistory[docsPathHistory.length - 1];
      if (prev) {
        docsPathHistory.pop();
        openEmbeddedDoc(prev);
        return;
      }
    }
    docsViewerClose();
  }

  function docsViewerClose() {
    const v = $("docs-md-viewer");
    if (v) v.hidden = true;
    currentDocPath = "";
    docsPathHistory.length = 0;
    updateDocsCrumb("");
  }

  function bindDocsMarkdownLinks() {
    document.querySelectorAll(".docs-open-md").forEach((btn) => {
      btn.addEventListener("click", () => {
        const path = (btn.getAttribute("data-doc-path") || "").trim();
        if (/^https?:\/\//i.test(path)) {
          window.open(path, "_blank", "noopener,noreferrer");
          return;
        }
        docsPathHistory.length = 0;
        openEmbeddedDoc(path);
      });
    });
  }

  function filterDocsSections() {
    const q = ($("docs-search") && $("docs-search").value || "").trim().toLowerCase();
    document.querySelectorAll("#docs-sections .guide-section").forEach((sec) => {
      if (!q) {
        sec.hidden = false;
        return;
      }
      const text = (sec.textContent || "").toLowerCase();
      sec.hidden = text.indexOf(q) < 0;
    });
  }

  function escapeHtml(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }

  async function loadDocs() {
    if (docsCache) {
      renderDocs(docsCache);
      return;
    }
    const root = $("docs-sections");
    try {
      const r = await fetch("/api/docs", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      renderDocs(await r.json());
    } catch (e) {
      if (root) root.innerHTML = "<p class=\"label err-inline\">Failed to load documentation: " + escapeHtml(e.message) + "</p>";
    }
  }

  async function loadCapabilities() {
    try {
      const r = await fetch("/api/capabilities", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      renderCapabilities(await r.json());
      const matrix = await fetchCoreOperatorCert({ matrix: true });
      if (matrix) renderOperatorCert(matrix);
    } catch (e) {
      const disc = $("feat-disclaimer");
      if (disc) disc.textContent = "Failed to load capabilities: " + e.message;
    }
  }

  function shortDisplayAddr(addr) {
    const a = String(addr || "").trim();
    if (!a) return "...";
    if (a.length <= 22) return a;
    return a.slice(0, 10) + "…" + a.slice(-8);
  }

  function anAddressCell(addr) {
    const a = String(addr || "").trim();
    if (!a) return '<span class="an-addr-empty">-</span>';
    return (
      '<div class="an-addr-cell">' +
      '<button type="button" class="an-addr-pill bs-jump-addr-an" data-addr="' + escHtml(a) + '" title="' + escHtml(a) + '">' +
      '<span class="material-icons-round an-addr-icon" aria-hidden="true">account_balance_wallet</span>' +
      '<span class="an-addr-text">' + escHtml(shortDisplayAddr(a)) + "</span>" +
      '<span class="material-icons-round an-addr-open" aria-hidden="true">north_east</span>' +
      "</button>" +
      '<button type="button" class="an-addr-copy" data-copy="' + escHtml(a) + '" title="Copy address" aria-label="Copy address">' +
      '<span class="material-icons-round" aria-hidden="true">content_copy</span>' +
      "</button>" +
      "</div>"
    );
  }

  function bindAnAddrCells(root) {
    if (!root) return;
    root.querySelectorAll(".bs-jump-addr-an").forEach((btn) => {
      if (btn.dataset.addrBound === "1") return;
      btn.dataset.addrBound = "1";
      btn.addEventListener("click", () => {
        const a = btn.getAttribute("data-addr");
        if (a && window.DogeGoBlockStep && window.DogeGoBlockStep.openAddress) {
          window.DogeGoBlockStep.openAddress(a, true);
        }
      });
    });
    root.querySelectorAll(".an-addr-copy").forEach((btn) => {
      if (btn.dataset.copyBound === "1") return;
      btn.dataset.copyBound = "1";
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const t = btn.getAttribute("data-copy");
        if (!t || !navigator.clipboard) return;
        navigator.clipboard.writeText(t).then(() => {
          btn.classList.add("an-addr-copy-done");
          setTimeout(() => btn.classList.remove("an-addr-copy-done"), 1200);
        }).catch(() => {});
      });
    });
  }

  function fillTopUtxoHolders(rows) {
    const wrap = $("an-top-utxo-wrap");
    const tbody = $("an-top-utxo");
    const empty = $("an-top-utxo-empty");
    if (!wrap || !tbody) return;
    if (!rows || !rows.length) {
      wrap.hidden = true;
      if (empty) empty.hidden = false;
      return;
    }
    wrap.hidden = false;
    if (empty) empty.hidden = true;
    tbody.innerHTML = rows.map((r, i) => {
      const addr = r.address || "";
      const doge = r.doge != null ? Number(r.doge).toLocaleString(undefined, { maximumFractionDigits: 4 }) : "?";
      const utxos = r.utxo_count != null ? String(r.utxo_count) : "?";
      const first = r.first_tx_time || r.first_seen_time;
      const last = r.last_tx_time || r.last_seen_time;
      const firstTxt = first ? fmtDate(first) : "-";
      const lastTxt = last ? fmtDate(last) : "-";
      return (
        "<tr>" +
        '<td class="an-utxo-rank">#' + (i + 1) + "</td>" +
        '<td class="an-utxo-addr">' + anAddressCell(addr) + "</td>" +
        '<td class="an-utxo-amt">' + doge + "</td>" +
        '<td class="an-utxo-meta">' + utxos + "</td>" +
        '<td class="an-utxo-date">' + firstTxt + "</td>" +
        '<td class="an-utxo-date">' + lastTxt + "</td>" +
        "</tr>"
      );
    }).join("");
    bindAnAddrCells(wrap);
  }

  function fillAnalyticsStorage(j, summary) {
    const netEl = $("an-network");
    const diskEl = $("an-disk-total");
    const chipsEl = $("an-disk-chips");
    const s = summary || lastSummary || {};
    const st = (j && j.storage) || {};
    const lastSample = j && j.metric_timeline && j.metric_timeline.length
      ? j.metric_timeline[j.metric_timeline.length - 1]
      : null;
    if (netEl) {
      netEl.textContent = String(j.network || s.network || s.chain || "-").toLowerCase();
    }
    if (!diskEl) return;
    if (s.node_mode === "spv") {
      setUIPending(diskEl, false);
      diskEl.textContent = "SPV";
      renderMetricChips(chipsEl, [{ text: "No block bodies", tone: "muted" }]);
      return;
    }
    const chainTotal = pickNum(s.chain_bytes_total, st.chain_bytes_total, lastSample && lastSample.chain_data_bytes);
    setUIPending(diskEl, false);
    diskEl.textContent = chainTotal != null ? fmtBytes(chainTotal) : "...";
    const chips = [];
    const headersB = pickNum(s.headers_bytes, st.headers_bytes, lastSample && lastSample.headers_bytes);
    const rawB = pickNum(s.rawblocks_bytes, st.rawblocks_bytes, lastSample && lastSample.rawblocks_bytes);
    const txB = pickNum(s.txindex_bytes, st.txindex_bytes, lastSample && lastSample.txindex_bytes);
    if (headersB != null) chips.push({ text: "headers " + fmtBytes(headersB) });
    if (rawB != null) chips.push({ text: "rawblocks " + fmtBytes(rawB) });
    if (txB != null) chips.push({ text: "tx index " + fmtBytes(txB) });
    renderMetricChips(chipsEl, chips);
  }

  function updateOverviewTipLabel(s) {
    const label = $("ov-tip-label");
    const foot = $("ov-tip-foot");
    const spv = String(s && s.node_mode || "").toLowerCase() === "spv";
    if (label) label.textContent = spv ? "Header tip" : "Block tip";
    if (foot) foot.textContent = spv ? "Header chain height" : "Best connected block height";
  }

  function updateOverviewTipValue(s) {
    const tipEl = $("tip");
    if (!tipEl || !s) return;
    const spv = String(s.node_mode || "").toLowerCase() === "spv";
    setUIPending(tipEl, false);
    if (spv) {
      const h = Number(s.tip_height);
      if (isFinite(h)) setCompactStat(tipEl, h, { integer: true });
      else tipEl.textContent = String(s.tip_height ?? "...");
    } else {
      const active = chainActiveHeight(s);
      const h = active >= 0 ? active : Number(s.tip_height);
      if (isFinite(h)) setCompactStat(tipEl, h, { integer: true });
      else tipEl.textContent = String(s.tip_height ?? "...");
    }
  }

  function fillChainStats(cs) {
    if (!cs || cs.error) return;
    const hr = $("cs-hashrate");
    if (hr) hr.textContent = fmtHashrate(cs.estimated_network_hashrate_hs);
    const anHr = $("an-hashrate");
    if (anHr) {
      const hrTxt = fmtHashrate(cs.estimated_network_hashrate_hs);
      setUIPending(anHr, false);
      anHr.textContent = hrTxt;
      anHr.title = hrTxt;
    }
    const dt = $("cs-mean-dt");
    if (dt && cs.mean_header_delta_sec_last != null) {
      dt.textContent = cs.mean_header_delta_sec_last.toFixed(1) + " s";
    }
    const minted = $("an-minted");
    if (minted && cs.minted_in_scanned_raw_doge != null) {
      setUIPending(minted, false);
      setCompactStat(minted, cs.minted_in_scanned_raw_doge, {
        maximumFractionDigits: 0,
        compactFractionDigits: 1,
        suffix: " DOGE",
      });
    }
    const rows = cs.top_miners_by_payout_p2pkh || [];
    const wrap = $("cs-miners-wrap");
    const tbody = $("cs-miners");
    const empty = $("cs-miners-empty");
    const card = $("cs-miners-card");
    if (rows.length && tbody && wrap) {
      if (card) card.style.display = "";
      tbody.innerHTML = rows.map((r) => "<tr><td class=\"mono\">" + r.address + "</td><td>" + r.blocks + "</td></tr>").join("");
      wrap.style.display = "";
      if (empty) empty.style.display = "none";
      const canvas = $("chart-miners");
      if (canvas && typeof Chart !== "undefined") {
        const sig = rows.map((r) => r.address + ":" + r.blocks).join("|");
        if (sig !== sigMiners || !chartMiners) {
          sigMiners = sig;
          chartMiners = upsertChart(chartMiners, canvas, {
            type: "doughnut",
            data: {
              labels: rows.map((r) => r.address.slice(0, 10) + "…"),
              datasets: [{ data: rows.map((r) => r.blocks), backgroundColor: ["#c2a633", "#2563eb", "#16a34a", "#dc2626", "#7c3aed", "#0891b2"] }],
            },
            options: {
              responsive: true,
              maintainAspectRatio: false,
              animation: false,
              plugins: { legend: { position: "right", labels: { boxWidth: 10, font: { size: 10 } } } },
            },
          });
        }
      }
    } else if (empty) {
      empty.style.display = "";
      if (wrap) wrap.style.display = "none";
    }
    fitAnKpiStats();
  }

  function minerDistributionEmptyMessage(cs) {
    if (!cs) return "Waiting for chain stats…";
    if (cs.error) return "Could not load miner stats: " + cs.error;
    const hits = Number(cs.raw_blocks_in_window) || 0;
    const miss = Number(cs.raw_blocks_missing_in_window) || 0;
    const win = Number(cs.approx_window_blocks) || 0;
    if (hits > 0) return "";
    if (win === 0) return "No headers in the tip’s ~24h window yet.";
    if (miss > 0) {
      return "Raw block bodies for this tip’s ~24h window are still downloading (" + miss + " missing in scan).";
    }
    return "No coinbase payout addresses in the scanned ~24h window yet.";
  }

  function renderMetricTimelines(j) {
    const rangeH = Number($("an-timeline-range")?.value ?? 24);
    const samples = filterTimelineSamples(j.metric_timeline, rangeH);
    if (!samples || !samples.length || typeof Chart === "undefined") {
      setChartPending("an-disk-chart-wrap", false);
      setChartPending("an-mempool-chart-wrap", false);
      setChartPending("an-blocksize-chart-wrap", false);
      return;
    }

    const labels = timelineLabels(samples);
    const baseOpts = {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: { position: "bottom", labels: { boxWidth: 10, font: { size: 10 } } },
      },
      scales: {
        x: { ticks: { maxTicksLimit: 8, font: { size: 9 } } },
        y: { beginAtZero: true },
      },
    };

    const diskSig = samples.map((s) => s.recorded_unix + ":" + s.chain_data_bytes).join("|");
    const diskCanvas = $("chart-disk-size");
    if (diskCanvas && diskSig !== sigDisk) {
      sigDisk = diskSig;
      chartDiskSize = upsertChart(chartDiskSize, diskCanvas, {
        type: "line",
        data: {
          labels,
          datasets: [
            { label: "Total", data: samples.map((s) => s.chain_data_bytes), borderColor: "#c2a633", backgroundColor: "rgba(194,166,51,0.12)", fill: true, tension: 0.2, pointRadius: 0 },
            { label: "Headers", data: samples.map((s) => s.headers_bytes), borderColor: "#64748b", tension: 0.2, pointRadius: 0 },
            { label: "Raw blocks", data: samples.map((s) => s.rawblocks_bytes), borderColor: "#2563eb", tension: 0.2, pointRadius: 0 },
            { label: "Tx index", data: samples.map((s) => s.txindex_bytes), borderColor: "#16a34a", tension: 0.2, pointRadius: 0 },
          ],
        },
        options: {
          ...baseOpts,
          scales: {
            x: baseOpts.scales.x,
            y: {
              beginAtZero: true,
              ticks: { callback: (v) => fmtBytes(v) },
            },
          },
        },
      });
    }
    setChartPending("an-disk-chart-wrap", false);

    const mpSig = samples.map((s) => s.recorded_unix + ":" + s.mempool_txs + ":" + s.mempool_bytes).join("|");
    const mpCanvas = $("chart-mempool-size");
    if (mpCanvas && mpSig !== sigMempoolTimeline) {
      sigMempoolTimeline = mpSig;
      chartMempoolSize = upsertChart(chartMempoolSize, mpCanvas, {
        type: "line",
        data: {
          labels,
          datasets: [
            { label: "Transactions", data: samples.map((s) => s.mempool_txs), borderColor: "#2563eb", yAxisID: "y", tension: 0.2, pointRadius: 0 },
            { label: "Bytes", data: samples.map((s) => s.mempool_bytes), borderColor: "#c2a633", yAxisID: "y1", tension: 0.2, pointRadius: 0 },
          ],
        },
        options: {
          ...baseOpts,
          scales: {
            x: baseOpts.scales.x,
            y: { type: "linear", position: "left", beginAtZero: true, title: { display: true, text: "txs", font: { size: 10 } } },
            y1: {
              type: "linear",
              position: "right",
              beginAtZero: true,
              grid: { drawOnChartArea: false },
              ticks: { callback: (v) => fmtBytes(v) },
              title: { display: true, text: "size", font: { size: 10 } },
            },
          },
        },
      });
    }
    setChartPending("an-mempool-chart-wrap", false);

    const maxW = Number(j.max_block_weight) || 4000000;
    const refBytes = maxW / 4;
    const blkSig = samples.map((s) => s.recorded_unix + ":" + s.max_recent_block_bytes).join("|");
    const blkCanvas = $("chart-block-size");
    if (blkCanvas && blkSig !== sigBlockSize) {
      sigBlockSize = blkSig;
      const blockData = samples.map((s) => s.max_recent_block_bytes);
      const peak = Math.max(refBytes * 1.05, ...blockData, 0);
      chartBlockSize = upsertChart(chartBlockSize, blkCanvas, {
        type: "line",
        data: {
          labels,
          datasets: [
            { label: "Tip block (bytes)", data: blockData, borderColor: "#dc2626", backgroundColor: "rgba(220,38,38,0.1)", fill: true, tension: 0.2, pointRadius: 0 },
            { label: "~max serialized (weight÷4)", data: labels.map(() => refBytes), borderColor: "#94a3b8", borderDash: [6, 4], pointRadius: 0, fill: false },
          ],
        },
        options: {
          ...baseOpts,
          scales: {
            x: baseOpts.scales.x,
            y: {
              beginAtZero: true,
              suggestedMax: peak,
              ticks: { callback: (v) => fmtBytes(v) },
            },
          },
        },
      });
    }
    setChartPending("an-blocksize-chart-wrap", false);

    const note = $("an-block-limit-note");
    if (note) {
      note.textContent = "Dogecoin max block weight: " + maxW.toLocaleString() + " (~" + fmtBytes(refBytes) + " at weight÷4 reference)";
    }
  }

  function renderOverviewCharts(s, analytics) {
    if (typeof Chart === "undefined" || !s) return;

    const syncCanvas = $("chart-ov-sync");
    if (syncCanvas) {
      const tipH = Number(s.tip_height) || 0;
      const connected = Math.max(0, chainActiveHeight(s) + 1);
      const stored = Math.max(0, Number(s.contiguous_raw_height) + 1);
      const sig = tipH + "/" + connected + "/" + stored;
      const cap = $("ov-chart-sync-caption");
      if (cap) {
        cap.textContent = "Headers " + tipH.toLocaleString() + " · connected " + connected.toLocaleString() + " · stored " + stored.toLocaleString();
      }
      if (sig !== sigOvDashSync || !chartOvDashSync) {
        sigOvDashSync = sig;
        chartOvDashSync = upsertChart(chartOvDashSync, syncCanvas, {
          type: "bar",
          data: {
            labels: ["Headers", "Connected", "Stored"],
            datasets: [{ data: [tipH, connected, stored], backgroundColor: ["#64748b", "#c2a633", "#94a3b8"], borderRadius: 8 }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: modernChartPlugins(),
            scales: {
              x: { grid: { display: false }, ticks: { font: { family: CHART_FONT, size: 11 } } },
              y: { beginAtZero: true, grid: { color: chartColors.grid }, ticks: { font: { family: CHART_FONT, size: 10 } } },
            },
          },
        });
      }
    }

    const peersCanvas = $("chart-ov-peers");
    if (peersCanvas && sparkPeersOut.length >= 2 && sparkPeersIn.length >= 2) {
      const sig = sparkPeersOut.join(",") + "|" + sparkPeersIn.join(",");
      if (sig !== sigOvDashPeers || !chartOvDashPeers) {
        sigOvDashPeers = sig;
        const labels = sparkPeersOut.map((_, i) => String(i + 1));
        chartOvDashPeers = upsertChart(chartOvDashPeers, peersCanvas, {
          type: "line",
          data: {
            labels,
            datasets: [
              { label: "Outbound", data: sparkPeersOut.slice(), borderColor: chartColors.blue, backgroundColor: chartColors.blueFill, fill: true, tension: 0.35, pointRadius: 0 },
              { label: "Inbound", data: sparkPeersIn.slice(), borderColor: chartColors.green, backgroundColor: chartColors.greenFill, fill: true, tension: 0.35, pointRadius: 0 },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: {
              ...modernChartPlugins(),
              legend: { display: true, position: "bottom", labels: { boxWidth: 10, font: { size: 10 } } },
            },
            scales: {
              x: { display: false },
              y: { beginAtZero: true, grid: { color: chartColors.grid }, ticks: { stepSize: 1, font: { size: 10 } } },
            },
          },
        });
      }
    }

    const mpCanvas = $("chart-ov-mempool-timeline");
    const samples = analytics && analytics.metric_timeline ? filterTimelineSamples(analytics.metric_timeline, 24) : [];
    const mpCap = $("ov-chart-mempool-caption");
    if (mpCap) {
      if (samples.length) {
        const last = samples[samples.length - 1];
        mpCap.textContent = last.mempool_txs + " txs · " + fmtBytes(last.mempool_bytes);
      } else if (s.mempool_txs != null) {
        mpCap.textContent = String(s.mempool_txs) + " txs (live)";
      }
    }
    if (mpCanvas && samples.length >= 2) {
      const labels = timelineLabels(samples);
      const mpSig = samples.map((x) => x.recorded_unix + ":" + x.mempool_txs).join("|");
      if (mpSig !== sigOvDashMempoolTimeline || !chartOvDashMempoolTimeline) {
        sigOvDashMempoolTimeline = mpSig;
        chartOvDashMempoolTimeline = upsertChart(chartOvDashMempoolTimeline, mpCanvas, {
          type: "line",
          data: {
            labels,
            datasets: [
              { label: "Transactions", data: samples.map((x) => x.mempool_txs), borderColor: chartColors.blue, backgroundColor: chartColors.blueFill, fill: true, tension: 0.3, pointRadius: 0 },
              { label: "Size", data: samples.map((x) => x.mempool_bytes), borderColor: chartColors.accent, yAxisID: "y1", tension: 0.3, pointRadius: 0 },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            interaction: { mode: "index", intersect: false },
            plugins: {
              legend: { position: "bottom", labels: { boxWidth: 10, font: { size: 10 } } },
              tooltip: modernChartPlugins().tooltip,
            },
            scales: {
              x: { ticks: { maxTicksLimit: 6, font: { size: 9 } }, grid: { display: false } },
              y: { beginAtZero: true, position: "left", grid: { color: chartColors.grid }, title: { display: true, text: "txs", font: { size: 10 } } },
              y1: { beginAtZero: true, position: "right", grid: { drawOnChartArea: false }, ticks: { callback: (v) => fmtBytes(v) } },
            },
          },
        });
      }
    } else if (mpCanvas && sparkMempool.length >= 2) {
      const labels = sparkMempool.map((_, i) => String(i + 1));
      const mpSig = sparkMempool.join(",");
      if (mpSig !== sigOvDashMempoolTimeline || !chartOvDashMempoolTimeline) {
        sigOvDashMempoolTimeline = mpSig;
        chartOvDashMempoolTimeline = upsertChart(chartOvDashMempoolTimeline, mpCanvas, {
          type: "line",
          data: {
            labels,
            datasets: [{ label: "Mempool txs", data: sparkMempool.slice(), borderColor: chartColors.blue, backgroundColor: chartColors.blueFill, fill: true, tension: 0.35, pointRadius: 0 }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: modernChartPlugins(),
            scales: {
              x: { display: false },
              y: { beginAtZero: true, grid: { color: chartColors.grid } },
            },
          },
        });
      }
    }
  }

  function analyticsStoredBlockCount(j, summary) {
    const fromLive = Number(j && j.rawblocks_live_count);
    if (isFinite(fromLive) && fromLive > 0) return fromLive;
    const fromBodies = Number(j && j.stored_bodies_height);
    if (isFinite(fromBodies) && fromBodies >= 0) return fromBodies + 1;
    const csBodies = Number(j && j.chainstats && j.chainstats.stored_bodies_height);
    if (isFinite(csBodies) && csBodies >= 0) return csBodies + 1;
    if (summary && Number(summary.contiguous_raw_height) >= 0) {
      return Number(summary.contiguous_raw_height) + 1;
    }
    return isFinite(fromLive) && fromLive >= 0 ? fromLive : 0;
  }

  function analyticsSyncHeights(j, summary) {
    const s = summary || {};
    const tipH = Number(s.tip_height) || Number(j && j.headers_tip_height) || 0;
    let connected = -1;
    if (summary) connected = chainActiveHeight(summary);
    if (connected < 0 && j && j.chain_active_height != null) connected = Number(j.chain_active_height);
    if (connected < 0 && j && j.chainstats && j.chainstats.chain_active_height != null) {
      connected = Number(j.chainstats.chain_active_height);
    }
    connected = Math.max(0, connected + 1);
    let stored = analyticsStoredBlockCount(j, summary);
    if (!stored && summary && Number(s.contiguous_raw_height) >= 0) {
      stored = Number(s.contiguous_raw_height) + 1;
    }
    return { tipH, connected, stored };
  }

  function resizeAnalyticsCharts() {
    const charts = [chartSync, chartAnMiners, chartHeaderDt, chartDiskSize, chartMempoolSize, chartBlockSize, chartReorgDepth, chartReorgHour, chartReorgMiners];
    charts.forEach((c) => {
      if (c && typeof c.resize === "function") {
        try { c.resize(); } catch (_) {}
      }
    });
  }

  function resetAnalyticsChartSigs() {
    sigSync = "";
    sigAnMiners = "";
    sigHeaderDt = "";
    sigDisk = "";
    sigMempoolTimeline = "";
    sigBlockSize = "";
    sigReorgDepth = "";
    sigReorgHour = "";
    sigReorgMiners = "";
  }

  function renderAnalyticsCharts(j, summary) {
    const cs = j.chainstats;
    const canvasSync = $("chart-sync");
    if (canvasSync && typeof Chart !== "undefined") {
      const { tipH, connected, stored } = analyticsSyncHeights(j, summary);
      const sig = tipH + "/" + connected + "/" + stored;
      if (sig !== sigSync || !chartSync) {
        sigSync = sig;
        chartSync = upsertChart(chartSync, canvasSync, {
          type: "bar",
          data: {
            labels: ["Headers", "Connected", "Stored blocks"],
            datasets: [{ data: [tipH, connected, stored], backgroundColor: ["#64748b", "#c2a633", "#94a3b8"] }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: { legend: { display: false } },
            scales: { y: { beginAtZero: true } },
          },
        });
      }
      setChartPending("an-sync-chart-wrap", false);
    }

    const canvasAnMin = $("chart-an-miners");
    const minersEmpty = $("an-miners-empty");
    const minersCap = $("an-miners-caption");
    if (canvasAnMin && typeof Chart !== "undefined") {
      const rows = (cs && cs.top_miners_by_payout_p2pkh) || [];
      if (rows.length) {
        setChartPending("an-miners-chart-wrap", false);
        if (minersEmpty) {
          minersEmpty.hidden = true;
          minersEmpty.textContent = "";
        }
        if (minersCap) {
          const tipH = cs.miner_window_tip_height != null ? cs.miner_window_tip_height : "";
          const fb = cs.miner_window_fallback ? " (stored-body tip while headers catch up)" : "";
          minersCap.textContent = tipH !== ""
            ? ("Coinbase payout share near height " + Number(tipH).toLocaleString() + " (~24h header-time window)" + fb + ".")
            : "Coinbase payout share in the tip’s ~24h header-time window (local raw blocks).";
        }
        const sig = rows.map((r) => r.address + ":" + r.blocks).join("|");
        if (sig !== sigAnMiners || !chartAnMiners) {
          sigAnMiners = sig;
          chartAnMiners = upsertChart(chartAnMiners, canvasAnMin, {
            type: "bar",
            data: {
              labels: rows.map((r) => r.address.slice(0, 12) + "…"),
              datasets: [{ label: "Blocks", data: rows.map((r) => r.blocks), backgroundColor: "#c2a633" }],
            },
            options: {
              indexAxis: "y",
              responsive: true,
              maintainAspectRatio: false,
              animation: false,
              plugins: { legend: { display: false } },
            },
          });
        }
      } else {
        if (chartAnMiners) {
          sigAnMiners = "";
          chartAnMiners = destroyChart(chartAnMiners);
        }
        setChartPending("an-miners-chart-wrap", false);
        if (minersEmpty) {
          minersEmpty.hidden = false;
          minersEmpty.textContent = minerDistributionEmptyMessage(cs);
        }
      }
    }

    const canvasDt = $("chart-header-dt");
    if (canvasDt && typeof Chart !== "undefined" && cs && cs.mean_header_delta_sec_last != null) {
      const v = cs.mean_header_delta_sec_last;
      const sig = String(v);
      if (sig !== sigHeaderDt || !chartHeaderDt) {
        sigHeaderDt = sig;
        chartHeaderDt = upsertChart(chartHeaderDt, canvasDt, {
          type: "line",
          data: {
            labels: ["Target (60s)", "Observed avg"],
            datasets: [{ data: [60, v], borderColor: "#2563eb", backgroundColor: "rgba(37,99,235,0.15)", fill: true, tension: 0.3 }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: { legend: { display: false } },
            scales: { y: { beginAtZero: true, suggestedMax: Math.max(90, v * 1.2) } },
          },
        });
      }
      setChartPending("an-header-dt-wrap", false);
    } else {
      setChartPending("an-header-dt-wrap", false);
    }

    renderReorgAnalytics(j);
    void loadForksPanel(false);
  }

  function shortForkHash(h) {
    const s = String(h || "");
    if (s.length <= 16) return s || "-";
    return s.slice(0, 10) + "…" + s.slice(-6);
  }

  function forkStatusPill(status) {
    const st = String(status || "unknown");
    return "<span class=\"an-fork-pill an-fork-pill-" + escapeHtml(st) + "\">" + escapeHtml(st) + "</span>";
  }

  async function loadForksPanel(force) {
    const errEl = $("an-forks-err");
    if (errEl) errEl.hidden = true;
    try {
      const r = await fetch("/api/forks?ts=" + Date.now(), { cache: "no-store", credentials: "same-origin" });
      const j = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error((j && j.error) || ("HTTP " + r.status));
      renderForksPanel(j);
    } catch (e) {
      if (errEl) {
        errEl.hidden = false;
        errEl.textContent = String(e.message || e);
      }
    }
  }

  function renderForksPanel(j) {
    if (!j) return;
    const local = j.local_tip || {};
    const sum = j.peer_summary || {};
    const localEl = $("an-forks-local");
    if (localEl) {
      const h = local.height != null ? local.height : "-";
      localEl.textContent = String(h) + (local.hash ? " · " + shortForkHash(local.hash) : "");
      localEl.title = local.hash || "";
    }
    if ($("an-forks-aligned")) $("an-forks-aligned").textContent = String(sum.aligned || 0);
    if ($("an-forks-diverged")) $("an-forks-diverged").textContent = String(sum.diverged || 0);
    if ($("an-forks-ahead")) $("an-forks-ahead").textContent = String(sum.ahead || 0);

    const gloss = $("an-forks-glossary");
    if (gloss) {
      gloss.innerHTML = "";
      (j.glossary || []).forEach((g) => {
        const box = document.createElement("div");
        box.className = "an-fork-gloss";
        box.innerHTML = "<strong>" + escapeHtml(g.title || g.id) + "</strong><p>" + escapeHtml(g.body || "") + "</p>";
        gloss.appendChild(box);
      });
    }

    const peers = j.peer_tips || [];
    const peersEmpty = $("an-forks-peers-empty");
    const peersBody = $("an-forks-peers-tbody");
    if (peersBody) {
      peersBody.innerHTML = "";
      peers.forEach((p) => {
        const tr = document.createElement("tr");
        if (p.status === "diverged") tr.className = "an-fork-row-diverged";
        tr.innerHTML =
          "<td><code>" + escapeHtml(p.addr || "") + "</code><div class=\"label\">" + escapeHtml(p.subver || "") + "</div></td>" +
          "<td>" + forkStatusPill(p.status) + "</td>" +
          "<td>" + escapeHtml(String(p.synced_headers != null ? p.synced_headers : "-")) + "</td>" +
          "<td>" + escapeHtml(String(p.delta_height != null ? p.delta_height : "-")) + "</td>" +
          "<td class=\"mono-sm\" title=\"" + escapeHtml(p.tip_hash || "") + "\">" + escapeHtml(shortForkHash(p.tip_hash)) + "</td>" +
          "<td class=\"label\">" + escapeHtml(p.status_detail || "") + "</td>";
        peersBody.appendChild(tr);
      });
    }
    if (peersEmpty) peersEmpty.hidden = peers.length > 0;

    const deps = j.deployments || [];
    const depBody = $("an-forks-dep-tbody");
    if (depBody) {
      depBody.innerHTML = "";
      deps.sort((a, b) => String(a.name).localeCompare(String(b.name))).forEach((d) => {
        const tr = document.createElement("tr");
        const active = d.active === true || d.active === "true";
        let signal = "-";
        if (d.count != null && d.threshold != null) {
          signal = String(d.count) + " / " + String(d.threshold);
          if (d.elapsed != null && d.period != null) signal += " (" + d.elapsed + "/" + d.period + ")";
        } else if (d.activation_height != null || d.height != null) {
          signal = "height " + String(d.activation_height != null ? d.activation_height : d.height);
        }
        let notes = "";
        if (d.bip9_status) notes = "BIP9 " + d.bip9_status;
        if (d.bit != null) notes += (notes ? " · " : "") + "bit " + d.bit;
        if (d.possible === false) notes += (notes ? " · " : "") + "not possible this period";
        tr.innerHTML =
          "<td><strong>" + escapeHtml(d.name || "") + "</strong></td>" +
          "<td>" + escapeHtml(d.type || "-") + "</td>" +
          "<td>" + (active ? "<span class=\"an-fork-pill an-fork-pill-aligned\">active</span>" : "<span class=\"an-fork-pill an-fork-pill-behind\">inactive</span>") + "</td>" +
          "<td>" + escapeHtml(d.bip9_status || (d.type === "buried" ? "buried" : "-")) + "</td>" +
          "<td>" + escapeHtml(signal) + "</td>" +
          "<td class=\"label\">" + escapeHtml(notes || "Soft fork rule set at this tip") + "</td>";
        depBody.appendChild(tr);
      });
    }

    const reorgs = j.recent_reorgs || [];
    const reorgEmpty = $("an-forks-reorg-empty");
    const reorgBody = $("an-forks-reorg-tbody");
    const detail = $("an-forks-detail");
    if (reorgBody) {
      reorgBody.innerHTML = "";
      reorgs.slice().reverse().forEach((ev) => {
        const tr = document.createElement("tr");
        const aux = (Number(ev.displaced_auxpow_count) || 0) + (Number(ev.incoming_auxpow_count) || 0);
        tr.innerHTML =
          "<td>" + escapeHtml(fmtReorgWhen(ev.recorded_unix)) + "</td>" +
          "<td>" + escapeHtml(String(ev.fork_at != null ? ev.fork_at : "-")) + "</td>" +
          "<td>" + escapeHtml(String(ev.depth != null ? ev.depth : "-")) + "</td>" +
          "<td>" + escapeHtml(String(ev.old_tip_height != null ? ev.old_tip_height : "-")) + "</td>" +
          "<td class=\"mono-sm\">" + escapeHtml(ev.work_delta ? String(ev.work_delta).slice(0, 18) : "-") + "</td>" +
          "<td>" + escapeHtml(String(aux)) + "</td>" +
          "<td><button type=\"button\" class=\"btn btn-ghost btn-sm\">Detail</button></td>";
        const btn = tr.querySelector("button");
        if (btn && detail) {
          btn.addEventListener("click", () => {
            detail.hidden = false;
            detail.textContent = JSON.stringify(ev, null, 2);
          });
        }
        reorgBody.appendChild(tr);
      });
    }
    if (reorgEmpty) reorgEmpty.hidden = reorgs.length > 0;
  }

  function fmtReorgWhen(unix) {
    const n = Number(unix);
    if (!isFinite(n) || n <= 0) return "-";
    try {
      return new Date(n * 1000).toISOString().replace("T", " ").replace(/\.\d+Z$/, " UTC");
    } catch (_) {
      return String(unix);
    }
  }

  function topMinerEntries(map, limit) {
    const rows = Object.keys(map || {}).map((k) => ({ address: k, blocks: Number(map[k]) || 0 }));
    rows.sort((a, b) => b.blocks - a.blocks);
    return rows.slice(0, limit || 12);
  }

  function renderReorgAnalytics(j) {
    const events = (j && j.reorg_events) || [];
    const sum = (j && j.reorg_summary) || {};
    const totalEl = $("an-reorg-total");
    const maxEl = $("an-reorg-max-depth");
    const auxEl = $("an-reorg-auxpow");
    const lastEl = $("an-reorg-last");
    const empty = $("an-reorg-empty");
    const wrap = $("an-reorg-table-wrap");
    const tbody = $("an-reorg-tbody");
    const detail = $("an-reorg-detail");
    const minersEmpty = $("an-reorg-miners-empty");

    const total = Number(sum.total != null ? sum.total : events.length) || 0;
    if (totalEl) totalEl.textContent = String(total);
    if (maxEl) maxEl.textContent = String(sum.max_depth != null ? sum.max_depth : 0);
    if (auxEl) auxEl.textContent = String(sum.auxpow_involved != null ? sum.auxpow_involved : 0);
    if (lastEl) lastEl.textContent = sum.last_recorded_unix ? fmtReorgWhen(sum.last_recorded_unix) : "-";

    if (empty) empty.hidden = total > 0;
    if (wrap) wrap.hidden = total === 0;

    if (tbody) {
      tbody.innerHTML = "";
      const newestFirst = events.slice().reverse();
      newestFirst.forEach((ev) => {
        const tr = document.createElement("tr");
        const auxN = (Number(ev.displaced_auxpow_count) || 0) + (Number(ev.incoming_auxpow_count) || 0);
        const miners = Object.keys(ev.displaced_miner_counts || {});
        const minerLabel = miners.length
          ? miners.slice(0, 2).map((a) => a.slice(0, 10) + (a.length > 10 ? "…" : "")).join(", ") + (miners.length > 2 ? " +" + (miners.length - 2) : "")
          : "-";
        tr.innerHTML =
          "<td>" + (ev.seq != null ? ev.seq : "") + "</td>" +
          "<td class=\"mono\">" + fmtReorgWhen(ev.recorded_unix) + "</td>" +
          "<td>" + (ev.fork_at != null ? Number(ev.fork_at).toLocaleString() : "-") + "</td>" +
          "<td>" + (ev.depth != null ? ev.depth : "-") + "</td>" +
          "<td>" + (ev.old_tip_height != null ? Number(ev.old_tip_height).toLocaleString() : "-") + "</td>" +
          "<td>" + auxN + (ev.displaced_auxpow_count || ev.incoming_auxpow_count ? " (d" + (ev.displaced_auxpow_count || 0) + "/i" + (ev.incoming_auxpow_count || 0) + ")" : "") + "</td>" +
          "<td class=\"mono\">" + minerLabel + "</td>" +
          "<td><button type=\"button\" class=\"btn btn-ghost btn-sm an-reorg-detail-btn\">Detail</button></td>";
        const btn = tr.querySelector(".an-reorg-detail-btn");
        if (btn) {
          btn.addEventListener("click", () => {
            if (!detail) return;
            detail.hidden = false;
            detail.textContent = JSON.stringify(ev, null, 2);
            detail.scrollIntoView({ behavior: "smooth", block: "nearest" });
          });
        }
        tbody.appendChild(tr);
      });
    }

    if (typeof Chart === "undefined") return;

    const depthCanvas = $("chart-reorg-depth");
    if (depthCanvas) {
      const recent = events.slice(-40);
      const labels = recent.map((e) => "#" + (e.seq != null ? e.seq : "?"));
      const depths = recent.map((e) => Number(e.depth) || 0);
      const sig = labels.join("|") + ":" + depths.join(",");
      if (sig !== sigReorgDepth || !chartReorgDepth) {
        sigReorgDepth = sig;
        chartReorgDepth = upsertChart(chartReorgDepth, depthCanvas, {
          type: "bar",
          data: {
            labels,
            datasets: [{ label: "Depth", data: depths, backgroundColor: "#c2a633" }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: { legend: { display: false } },
            scales: { y: { beginAtZero: true, ticks: { precision: 0 } } },
          },
        });
      }
    }

    const hourCanvas = $("chart-reorg-hour");
    if (hourCanvas) {
      const byHour = sum.by_hour_utc || [];
      const hours = [];
      const counts = [];
      for (let h = 0; h < 24; h++) {
        hours.push(String(h).padStart(2, "0"));
        counts.push(Number(byHour[h]) || 0);
      }
      const sig = counts.join(",");
      if (sig !== sigReorgHour || !chartReorgHour) {
        sigReorgHour = sig;
        chartReorgHour = upsertChart(chartReorgHour, hourCanvas, {
          type: "bar",
          data: {
            labels: hours,
            datasets: [{ label: "Reorgs", data: counts, backgroundColor: "#64748b" }],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            plugins: { legend: { display: false } },
            scales: { y: { beginAtZero: true, ticks: { precision: 0 } } },
          },
        });
      }
    }

    const minerCanvas = $("chart-reorg-miners");
    if (minerCanvas) {
      const rows = topMinerEntries(sum.miner_on_displaced, 12);
      if (rows.length) {
        if (minersEmpty) minersEmpty.hidden = true;
        const sig = rows.map((r) => r.address + ":" + r.blocks).join("|");
        if (sig !== sigReorgMiners || !chartReorgMiners) {
          sigReorgMiners = sig;
          chartReorgMiners = upsertChart(chartReorgMiners, minerCanvas, {
            type: "bar",
            data: {
              labels: rows.map((r) => r.address.slice(0, 12) + (r.address.length > 12 ? "…" : "")),
              datasets: [{ label: "Displaced blocks", data: rows.map((r) => r.blocks), backgroundColor: "#9a8428" }],
            },
            options: {
              indexAxis: "y",
              responsive: true,
              maintainAspectRatio: false,
              animation: false,
              plugins: { legend: { display: false } },
            },
          });
        }
      } else {
        if (chartReorgMiners) {
          sigReorgMiners = "";
          chartReorgMiners = destroyChart(chartReorgMiners);
        }
        if (minersEmpty) minersEmpty.hidden = false;
      }
    }
  }

  function peerPingLabel(row) {
    const ping = Number(row.pingtime);
    if (isFinite(ping) && ping > 0) return ping.toFixed(3) + " s";
    const wait = Number(row.pingwait);
    if (isFinite(wait) && wait > 0) return "wait " + wait.toFixed(1) + " s";
    return "…";
  }

  function peerTrafficLabel(row) {
    const rx = Number(row.bytesrecv) || 0;
    const tx = Number(row.bytessent) || 0;
    return fmtBytes(rx) + " ↓ / " + fmtBytes(tx) + " ↑";
  }

  function peerDirectionLabel(row) {
    return row.inbound ? "Inbound" : "Outbound";
  }

  function peerRoleLabel(row) {
    const role = String(row.dogego_role || "").trim();
    if (role === "primary") return "Primary sync";
    if (role === "block-assist") return "Block assist";
    if (role === "relay") return row.inbound ? "Inbound relay" : "Outbound relay";
    return role || (row.inbound ? "Inbound" : "Outbound");
  }

  function peerFlagChips(row) {
    const chips = [];
    if (row.dogego_relay_cgnat) chips.push({ cls: "dgr", text: "DGR relay" });
    if (row.dogego_dgr_tunnel) chips.push({ cls: "dgr", text: "DGR tunnel" });
    if (row.dogego_role === "primary") chips.push({ cls: "primary", text: "Primary" });
    if (row.dogego_role === "block-assist") chips.push({ cls: "", text: "IBD assist" });
    if (row.addnode || row.whitelisted) chips.push({ cls: "", text: "addnode" });
    if (row.bip152_hb_to || row.bip152_hb_from) chips.push({ cls: "", text: "BIP152 HB" });
    if (row.dogego_addrbook_tried) chips.push({ cls: "", text: "addrbook" });
    if (Number(row.banscore) > 0) chips.push({ cls: "warn", text: "ban " + row.banscore });
    if (row.dogego_block_score != null) chips.push({ cls: "", text: "score " + row.dogego_block_score });
    if (row.dogego_ibd_lane != null) chips.push({ cls: "", text: "lane " + row.dogego_ibd_lane });
    return chips.map((c) => '<span class="an-peer-flag' + (c.cls ? " " + c.cls : "") + '">' + escapeHtml(c.text) + "</span>").join("");
  }

  function setPeersActionStatus(msg, isErr) {
    const el = $("an-peers-action-status");
    if (!el) return;
    el.textContent = msg || "";
    el.classList.toggle("err-inline", !!isErr);
  }

  async function peersAction(action, node) {
    const addr = String(node || "").trim();
    if (!addr) {
      setPeersActionStatus("Enter host:port first.", true);
      return false;
    }
    setPeersActionStatus(action + " " + addr + "…");
    try {
      const r = await fetch("/api/peers", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: action, node: addr }),
      });
      const data = await r.json().catch(() => ({}));
      if (!r.ok || data.ok === false) {
        throw new Error(data.error || ("HTTP " + r.status));
      }
      data._loadedAt = Date.now();
      lastAnalyticsPeersCache = data;
      renderAnalyticsPeers(data);
      setPeersActionStatus((action === "remove" ? "Removed " : action === "disconnect" ? "Disconnected " : "OK: ") + addr);
      return true;
    } catch (e) {
      setPeersActionStatus(String(e.message || e), true);
      return false;
    }
  }

  function renderAddedNodesList(data) {
    const wrap = $("an-added-nodes");
    const empty = $("an-added-empty");
    const lanWrap = $("st-lan-added-list");
    const rows = (data && data.added_nodes) || [];
    const list = Array.isArray(rows) ? rows : [];
    const html = list.map((row) => {
      const added = String((row && (row.addednode || row.address)) || "").trim();
      if (!added) return "";
      const connected = !!(row && row.connected);
      const connLbl = connected
        ? (i18n("pages.analytics.addedConnected") || "connected")
        : (i18n("pages.analytics.addedNotConnected") || "not connected");
      const removeLbl = i18n("pages.analytics.addedRemove") || "Remove";
      return "<div class=\"an-added-row\">" +
        "<code class=\"mono\">" + escapeHtml(added) + "</code>" +
        "<span class=\"label\">" + escapeHtml(connLbl) + "</span>" +
        "<button type=\"button\" class=\"btn btn-ghost btn-sm an-added-remove\" data-node=\"" + escapeHtml(added) + "\">" +
          "<span class=\"material-icons-round\" aria-hidden=\"true\">person_remove</span> " + escapeHtml(removeLbl) +
        "</button></div>";
    }).filter(Boolean).join("");
    if (wrap) {
      wrap.innerHTML = html;
      wrap.hidden = !html;
    }
    if (empty) empty.hidden = !!html;
    if (lanWrap) {
      lanWrap.innerHTML = html;
      lanWrap.hidden = !html;
    }
  }

  function renderAnalyticsPeers(data) {
    const summaryEl = $("an-peers-summary");
    const wrap = $("an-peers-wrap");
    const empty = $("an-peers-empty");
    const errEl = $("an-peers-err");
    if (!wrap) return;
    if (errEl) errEl.hidden = true;
    renderAddedNodesList(data);
    const peers = (data && data.peers) || [];
    const p2p = (data && data.p2p) || {};
    const dgr = (data && data.dgr) || {};
    if (summaryEl) {
      const parts = [];
      const mode = p2p.p2p_connectivity || p2p.p2p_mode || "";
      if (mode) parts.push("<span class=\"label\">" + escapeHtml(i18n("pages.analytics.peersDgrMode")) + " <strong>" + escapeHtml(String(mode)) + "</strong></span>");
      parts.push("<span class=\"label\">Out <strong>" + escapeHtml(String(data.connections_outbound != null ? data.connections_outbound : peers.filter((p) => !p.inbound).length)) + "</strong> · In <strong>" + escapeHtml(String(data.connections_inbound != null ? data.connections_inbound : peers.filter((p) => p.inbound).length)) + "</strong></span>");
      const addedN = Array.isArray(data.added_nodes) ? data.added_nodes.length : 0;
      if (addedN) parts.push("<span class=\"label\">Manual <strong>" + escapeHtml(String(addedN)) + "</strong></span>");
      if (dgr.enabled) {
        if (dgr.using_relay) {
          parts.push("<span class=\"p2p-health-pill ok\">" + escapeHtml(i18n("pages.analytics.peersDgrUsing")) + "</span>");
          if (dgr.active_relay) {
            parts.push("<span class=\"label\">" + escapeHtml(i18n("pages.analytics.peersDgrActive")) + " <strong class=\"mono\">" + escapeHtml(String(dgr.active_relay)) + "</strong></span>");
          }
        } else if (dgr.role) {
          parts.push("<span class=\"label\">DGR role <strong>" + escapeHtml(String(dgr.role)) + "</strong></span>");
        }
      }
      summaryEl.innerHTML = parts.join("");
      summaryEl.hidden = !parts.length;
    }
    if (!peers.length) {
      wrap.hidden = true;
      wrap.innerHTML = "";
      if (empty) empty.hidden = false;
      return;
    }
    if (empty) empty.hidden = true;
    wrap.hidden = false;
    const sorted = peers.slice().sort((a, b) => {
      const ai = a.inbound ? 1 : 0;
      const bi = b.inbound ? 1 : 0;
      if (ai !== bi) return bi - ai;
      return String(a.addr || "").localeCompare(String(b.addr || ""));
    });
    const discLbl = i18n("pages.analytics.peerDisconnect") || "Disconnect";
    wrap.innerHTML = sorted.map((row, idx) => {
      const ua = String(row.subver || "").trim();
      const note = String(row.dogego_note || "").trim();
      const dir = peerDirectionLabel(row);
      const dirCls = row.inbound ? "inbound" : "outbound";
      const addr = String(row.addr || "");
      const local = row.addrlocal ? String(row.addrlocal) : "";
      const flags = peerFlagChips(row);
      const isPrimary = row.dogego_role === "primary";
      const actions = (!isPrimary && addr)
        ? "<div class=\"an-peer-actions\"><button type=\"button\" class=\"btn btn-ghost btn-sm an-peer-disconnect\" data-addr=\"" + escapeHtml(addr) + "\">" +
            "<span class=\"material-icons-round\" aria-hidden=\"true\">link_off</span> " + escapeHtml(discLbl) +
          "</button></div>"
        : "";
      return "<article class=\"an-peer-card\">" +
        "<div class=\"an-peer-card-head\">" +
          "<span class=\"an-peer-dir " + dirCls + "\">" + escapeHtml(dir) + "</span>" +
          "<span class=\"an-peer-id label\">#" + escapeHtml(String(row.id != null ? row.id : idx + 1)) + "</span>" +
          "<span class=\"an-peer-ping\">" + escapeHtml(peerPingLabel(row)) + "</span>" +
        "</div>" +
        "<div class=\"an-peer-addr mono\" title=\"" + escapeHtml(addr) + "\">" + escapeHtml(addr) + "</div>" +
        (local ? "<div class=\"an-peer-local label\">local " + escapeHtml(local) + "</div>" : "") +
        "<div class=\"an-peer-stats\">" +
          "<div class=\"an-peer-stat\"><span class=\"label\">Role</span><strong>" + escapeHtml(peerRoleLabel(row)) + "</strong></div>" +
          "<div class=\"an-peer-stat\"><span class=\"label\">Type</span><strong class=\"mono\">" + escapeHtml(String(row.connection_type || "…")) + "</strong></div>" +
          "<div class=\"an-peer-stat\"><span class=\"label\">Start</span><strong>" + escapeHtml(row.startingheight != null ? Number(row.startingheight).toLocaleString() : "…") + "</strong></div>" +
          "<div class=\"an-peer-stat\"><span class=\"label\">Synced</span><strong>" + escapeHtml(row.synced_blocks != null ? Number(row.synced_blocks).toLocaleString() : "…") + "</strong></div>" +
          "<div class=\"an-peer-stat an-peer-stat-wide\"><span class=\"label\">Traffic</span><strong class=\"mono\">" + escapeHtml(peerTrafficLabel(row)) + "</strong></div>" +
        "</div>" +
        (note ? "<p class=\"an-peer-note label\">" + escapeHtml(note) + "</p>" : "") +
        (ua ? "<p class=\"an-peer-ua mono\" title=\"" + escapeHtml(ua) + "\">" + escapeHtml(ua) + "</p>" : "") +
        (flags ? "<div class=\"an-peer-flags\">" + flags + "</div>" : "") +
        actions +
        "</article>";
    }).join("");
  }

  async function loadAnalyticsPeers(force) {
    const now = Date.now();
    if (!force && lastAnalyticsPeersCache && now - (lastAnalyticsPeersCache._loadedAt || 0) < ANALYTICS_REFRESH_MS) {
      renderAnalyticsPeers(lastAnalyticsPeersCache);
      return;
    }
    const errEl = $("an-peers-err");
    try {
      const r = await fetch("/api/peers", { cache: "no-store", credentials: "same-origin" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const data = await r.json();
      data._loadedAt = now;
      lastAnalyticsPeersCache = data;
      renderAnalyticsPeers(data);
    } catch (e) {
      if (errEl) {
        errEl.hidden = false;
        errEl.textContent = i18n("pages.analytics.peersLoadFailed") + " (" + (e.message || e) + ")";
      }
    }
  }

  function renderAnalyticsDashboard(j, summary) {
    analyticsHydrated = true;
    const st = $("an-live-status");
    if (st) {
      st.textContent = j.analytics_db_exists
        ? "Pebble catalog · schema v" + j.schema_version
        : "Optional analytics catalog not created yet (node syncs normally without it)";
    }
    const tip = $("an-tip");
    if (tip) {
      const spv = summary && String(summary.node_mode || "").toLowerCase() === "spv";
      const tipLabel = $("an-tip-label");
      if (tipLabel) tipLabel.textContent = spv ? "Header tip" : "Block tip";
      setUIPending(tip, false);
      if (spv) {
        const h = Number(j.headers_tip_height);
        if (isFinite(h)) setCompactStat(tip, h, { integer: true });
        else tip.textContent = String(j.headers_tip_height ?? "...");
      } else {
        const active = summary ? chainActiveHeight(summary) : -1;
        const h = active >= 0 ? active : Number(j.chain_active_height ?? j.headers_tip_height);
        if (isFinite(h)) setCompactStat(tip, h, { integer: true });
        else tip.textContent = String(j.chain_active_height ?? j.headers_tip_height ?? "...");
      }
    }
    const raw = $("an-raw");
    if (raw) {
      setUIPending(raw, false);
      const stored = Number(analyticsStoredBlockCount(j, summary));
      if (isFinite(stored)) setCompactStat(raw, stored, { integer: true });
      else raw.textContent = String(analyticsStoredBlockCount(j, summary));
    }

    const dbSize = $("an-db-size");
    if (dbSize) {
      setUIPending(dbSize, false);
      dbSize.textContent = j.analytics_db_exists ? fmtBytes(j.analytics_db_bytes) : "Not created yet";
    }
    const dbPath = $("an-db-path");
    if (dbPath) {
      setUIPending(dbPath, false);
      dbPath.textContent = j.analytics_db_path || "…";
    }

    const maxW = Number(j.max_block_weight) || 4000000;
    const note = $("an-block-limit-note");
    if (note && (!j.metric_timeline || !j.metric_timeline.length)) {
      note.textContent = "Dogecoin max block weight: " + maxW.toLocaleString() + " (~" + fmtBytes(maxW / 4) + " at weight÷4 reference)";
    }

    if (summary) {
      const anMp = $("an-mempool");
      if (anMp && summary.mempool_txs != null) {
        setUIPending(anMp, false);
        setCompactStat(anMp, summary.mempool_txs, { integer: true, suffix: " tx" });
      }
      const anSync = $("an-sync-pct");
      if (anSync) {
        setUIPending(anSync, false);
        anSync.textContent = String(syncProgressPct(summary)) + "%";
      }
      const anTxProc = $("an-tx-processed");
      if (anTxProc) {
        const n = Number(summary.transactions_processed);
        setUIPending(anTxProc, false);
        if (isFinite(n) && n >= 0) setCompactStat(anTxProc, n, { integer: true, requireNonNeg: true });
        else anTxProc.textContent = "…";
      }
      const anNet = $("an-network");
      if (anNet) {
        setUIPending(anNet, false);
        anNet.textContent = (summary.network || summary.chain || "…").toString();
      }
    } else if (j && j.chainstats) {
      const tipH = Number(j.headers_tip_height);
      const active = Number(j.chain_active_height);
      const anSync = $("an-sync-pct");
      if (anSync && isFinite(tipH) && isFinite(active) && tipH > 0) {
        setUIPending(anSync, false);
        anSync.textContent = String(Math.min(100, Math.round((100 * (active + 1)) / (tipH + 1)))) + "%";
      }
    }

    renderMetricTimelines(j);

    const cs = j.chainstats;
    if (cs) fillChainStats(cs);
    fillAnalyticsStorage(j, summary);
    try {
      fillTopUtxoHolders(j.top_utxo_holders || []);
    } catch (e) {
      console.warn("analytics utxo holders:", e);
    }

    try {
      renderAnalyticsCharts(j, summary);
    } catch (e) {
      console.warn("analytics charts:", e);
    }

    const prog = $("an-progress-wrap");
    if (prog && j.index_progress && j.index_progress.length) {
      setUIPending(prog, false);
      prog.innerHTML = "<table class=\"data\"><thead><tr><th>Subsystem</th><th>Value</th><th>Updated</th></tr></thead><tbody>" +
        j.index_progress.map((r) => {
          const d = r.updated_unix ? new Date(r.updated_unix * 1000).toLocaleString() : "...";
          return "<tr><td>" + r.subsystem + "</td><td>" + r.last_height + "</td><td>" + d + "</td></tr>";
        }).join("") + "</tbody></table>";
    } else if (prog) {
      setUIPending(prog, false);
      prog.textContent = j.analytics_db_exists
        ? "No indexer checkpoints recorded yet."
        : "Indexer checkpoints appear after the optional analytics catalog is created.";
    }

    fitAnKpiStats();

    if (document.getElementById("panel-analytics")?.classList.contains("active")) {
      requestAnimationFrame(() => {
        resizeAnalyticsCharts();
        requestAnimationFrame(() => resizeAnalyticsCharts());
      });
    }

    if (lastAnalyticsPeersCache) renderAnalyticsPeers(lastAnalyticsPeersCache);

    const rawJson = $("an-live-json");
    if (rawJson && localStorage.getItem(LS_SUM) === "1") {
      rawJson.textContent = JSON.stringify(j, null, 2);
      rawJson.classList.add("show");
    }
  }

  async function loadAnalyticsPanel(force) {
    const now = Date.now();
    if (!force && now - lastAnalyticsLoadAt < ANALYTICS_REFRESH_MS) {
      if (lastAnalyticsJson) renderAnalyticsDashboard(lastAnalyticsJson, lastSummary);
      return;
    }
    if (!force && lastAnalyticsJson && lastSummary && lastSummary.from_disk_snapshot) {
      renderAnalyticsDashboard(lastAnalyticsJson, lastSummary);
      const st0 = $("an-live-status");
      if (st0) st0.textContent = "Showing cached analytics · refreshing…";
    }
    lastAnalyticsLoadAt = now;
    const st = $("an-live-status");
    if (st && !analyticsHydrated) wait(st, "Loading analytics…", { inline: true });
    setChartPending("an-miners-chart-wrap", !analyticsHydrated);
    setChartPending("an-sync-chart-wrap", !analyticsHydrated);
    setChartPending("an-header-dt-wrap", !analyticsHydrated);
    setChartPending("an-disk-chart-wrap", !analyticsHydrated);
    setChartPending("an-mempool-chart-wrap", !analyticsHydrated);
    setChartPending("an-blocksize-chart-wrap", !analyticsHydrated);
    const minersEmpty = $("an-miners-empty");
    if (minersEmpty && !analyticsHydrated) {
      minersEmpty.hidden = false;
      minersEmpty.textContent = "Loading miner distribution…";
    }
    try {
      if (!lastSummary) {
        try {
          const rs = await fetch("/api/summary", { cache: "no-store" });
          if (rs.ok) lastSummary = await rs.json();
        } catch (_) {}
      }
      const light = shouldDeferHeavyWalletAPI(lastSummary) ? "&light=1" : "";
      const r = await fetch("/api/analytics/summary?ts=" + now + light, { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const j = await r.json();
      lastAnalyticsJson = j;
      renderAnalyticsDashboard(j, lastSummary);
      void loadAnalyticsPeers(true);
    } catch (e) {
      if (st) st.textContent = "Failed to load analytics: " + e.message;
      setChartPending("an-miners-chart-wrap", false);
      if (minersEmpty) {
        minersEmpty.hidden = false;
        minersEmpty.textContent = "Could not load miner distribution (" + (e.message || e) + ").";
      }
    }
  }

  const SUBTAB_ATTR = {
    overview: "ov",
    settings: "st",
    mempool: "mp",
    send: "sd",
    receive: "rc",
    explorer: "ex",
  };

  function subTabAttr(group) {
    return SUBTAB_ATTR[group] || null;
  }

  function activateSubTab(group, subName) {
    const attr = subTabAttr(group);
    if (!attr || !subName) return;
    const tabs = document.querySelector('[data-subtabs="' + group + '"]');
    if (!tabs) return;
    tabs.querySelectorAll("button[data-" + attr + '-sub]').forEach((b) => {
      const on = b.dataset[attr + "Sub"] === subName;
      b.classList.toggle("active", on);
      b.setAttribute("aria-selected", on ? "true" : "false");
    });
    document.querySelectorAll("[data-" + attr + '-panel]').forEach((p) => {
      const on = p.dataset[attr + "Panel"] === subName;
      p.classList.toggle("active", on);
      p.hidden = !on;
    });
    if (group === "settings" && subName === "sync") void loadServicesPanel();
  }

  function initSubTabs() {
    document.querySelectorAll("[data-subtabs]").forEach((nav) => {
      const group = nav.dataset.subtabs;
      const attr = subTabAttr(group);
      if (!attr) return;
      nav.querySelectorAll("button[data-" + attr + '-sub]').forEach((btn) => {
        btn.addEventListener("click", async () => {
          activateSubTab(group, btn.dataset[attr + "Sub"]);
          if (group === "mempool" && btn.dataset.mpSub === "transactions") {
            try {
              const mp = await loadMempoolDetail();
              if (mp) fillMempoolPanel(mp);
            } catch (_) { /* */ }
          }
          if (group === "settings" && btn.dataset.stSub === "interface") {
            void loadTLSStatus();
          }
          if (group === "settings" && btn.dataset.stSub === "p2p") {
            refreshDGRLive();
            loadLanPeerHint();
          }
          if (group === "mempool" && btn.dataset.mpSub === "policy") {
            loadMempoolParityProbe();
          }
          if (group === "settings" && btn.dataset.stSub === "advanced") {
            void loadCoreStatus();
          }
          if (group === "settings" && btn.dataset.stSub === "tools") {
            void loadSettingsToolsPanel();
          }
          if (group === "receive" && btn.dataset.rcSub === "book") {
            void loadWalletAddressBook();
          }
        });
      });
    });
    document.querySelectorAll(".quick-nav-btn[data-st-sub]").forEach((b) => {
      b.addEventListener("click", () => {
        const sub = b.dataset.stSub;
        if (sub) setTimeout(() => activateSubTab("settings", sub), 0);
      });
    });
    document.querySelectorAll("[data-ov-jump]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const sub = btn.getAttribute("data-ov-jump");
        if (sub) activateSubTab("overview", sub);
      });
    });
  }

  window.DogeGoUI = { activateSubTab };

  function showTab(name, opts) {
    opts = opts || {};
    const n = HASH_ALIASES[name] || name;
    if (!TABS.includes(n)) return;
    if (document.body.classList.contains("mode-spv") && ["explorer", "analytics", "mempool"].includes(n)) {
      name = "overview";
    }
    const tab = HASH_ALIASES[name] || name;
    document.querySelectorAll(".nav-item, .bottom-nav-item").forEach((b) => {
      b.classList.toggle("active", b.dataset.tab === tab);
    });
    document.querySelectorAll(".panel").forEach((p) => {
      let on = p.id === "panel-" + tab;
      if (tab === "extensions") {
        const raw = (location.hash || "#extensions").replace(/^#/, "");
        const slash = raw.indexOf("/");
        const isDetail = slash > 0 && raw.startsWith("extensions/") && raw.slice(slash + 1).length > 0;
        on = isDetail ? p.id === "panel-extension-detail" : p.id === "panel-extensions";
      }
      p.classList.toggle("active", on);
    });
    setNavOpen(false);
    if (!opts.preserveHash) {
      const curHash = (location.hash || "").replace(/^#/, "");
      if (curHash !== tab && !curHash.startsWith(tab + "/")) {
        location.hash = tab;
      }
    }
    if (tab === "overview") void loadCoreStatus();
    if (tab === "analytics") {
      resetAnalyticsChartSigs();
      void loadAnalyticsPanel(true);
    }
    if (tab === "settings") {
      loadConfigForm();
      loadServicesPanel();
      refreshDGRLive();
      loadLanPeerHint();
      if (window.DogeGoControls) window.DogeGoControls.bindChoiceCards();
    }
    if (tab === "console") {
      loadLogs();
      loadRpcCookbook(false);
    }
    if (tab === "features") {
      if (!capabilitiesCache) loadCapabilities();
      else loadCoreProbes(false);
      loadCoreRunner();
      loadCoreWorkflow10();
    }
    if (tab === "docs") loadDocs();
    if (tab === "extensions") routeExtensionsFromHash();
    if (tab === "transactions") void refreshWalletTxHistory(true);
    if ((tab === "send" || tab === "receive") && (!lastWalletSnap || typeof lastWalletSnap.balance !== "number")) {
      void refreshWalletPanelAsync(refreshGen);
    }
    if (tab === "mempool") {
      if (lastSummary) refresh();
      loadMempoolDetail().then((mp) => { if (mp) fillMempoolPanel(mp); }).catch(() => {});
      const policyBtn = document.querySelector('[data-mp-sub="policy"]');
      if (policyBtn && policyBtn.classList.contains("active")) loadMempoolParityProbe();
    }
    if (tab === "blockstep" && window.DogeGoBlockStep) window.DogeGoBlockStep.onShow();
  }

  function applyPaymentDeepLink(raw) {
    const hash = (raw || location.hash || "").replace(/^#/, "");
    if (!hash.startsWith("send?")) return false;
    const params = new URLSearchParams(hash.slice(5));
    const to = params.get("to");
    if (!to) return false;
    showTab("send", { preserveHash: true });
    const toEl = $("send-to");
    const amtEl = $("send-amt");
    if (toEl) toEl.value = to;
    const amt = params.get("amount");
    if (amtEl && amt) amtEl.value = amt;
    const netWarn = params.get("net_warn");
    if (netWarn) {
      showSendResult(false, "Payment link is for " + netWarn + " but this node may use a different network. Verify the address before sending.");
    }
    validateSendForm();
    return true;
  }

  function routeFromHash() {
    const raw = (location.hash || "#overview").replace(/^#/, "");
    if (raw.startsWith("send?")) {
      applyPaymentDeepLink(raw);
      return;
    }
    const parts = raw.split("/");
    const tab = parts[0] || "overview";
    const sub = parts[1];
    showTab(tab, { preserveHash: !!(sub && (tab === "features" || tab === "docs" || tab === "extensions")) });
    if (sub && (tab === "overview" || tab === "settings")) activateSubTab(tab, sub);
    else if (sub && tab === "extensions") routeExtensionsFromHash();
    else if (sub && tab === "features" && sub.indexOf("feat-") === 0) {
      requestAnimationFrame(() => scrollToFeatAnchor(sub));
    } else if (sub && tab === "docs") {
      requestAnimationFrame(() => scrollToFeatAnchor("docs-" + sub));
    }
  }

  function escHtml(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }
  function shortHashEx(h) {
    if (!h || h.length < 16) return h || "...";
    return h.slice(0, 10) + "…" + h.slice(-8);
  }
  function fmtBlockTime(ts) {
    if (!ts) return "...";
    return new Date(ts * 1000).toLocaleString();
  }
  let lastExplorerSearch = { "ov-ex-out": null, "lk-ex-out": null };
  function explorerOutCanToggle(out) {
    return out && (out.id === "ov-ex-out" || out.id === "lk-ex-out");
  }
  function setLastExplorerSearch(out, data) {
    if (out && out.id) lastExplorerSearch[out.id] = data;
  }
  function getLastExplorerSearch(out) {
    return out && out.id ? lastExplorerSearch[out.id] : null;
  }

  function goBlockStepTx(txid) {
    if (!txid) return;
    location.hash = "blockstep/tx/" + encodeURIComponent(txid);
    showTab("blockstep");
    if (window.DogeGoBlockStep && window.DogeGoBlockStep.parseHashRoute) {
      window.DogeGoBlockStep.parseHashRoute();
    }
  }
  function goBlockStepBlock(height) {
    if (height == null || height === "") return;
    location.hash = "blockstep/block/" + height;
    showTab("blockstep");
    if (window.DogeGoBlockStep && window.DogeGoBlockStep.parseHashRoute) {
      window.DogeGoBlockStep.parseHashRoute();
    }
  }
  function goBlockStepAddress(address) {
    if (!address) return;
    location.hash = "blockstep/address/" + encodeURIComponent(address);
    showTab("blockstep");
    if (window.DogeGoBlockStep && window.DogeGoBlockStep.parseHashRoute) {
      window.DogeGoBlockStep.parseHashRoute();
    }
  }
  function toggleOvSearchRaw() {
    const out = $("ov-ex-out");
    if (!out || !getLastExplorerSearch(out)) return;
    const raw = out.dataset.raw === "1";
    renderExplorerSearchResult(getLastExplorerSearch(out), out, !raw);
  }
  function bindAddressHitRows(out) {
    out.querySelectorAll("[data-txid]").forEach((el) => {
      el.addEventListener("click", () => goBlockStepTx(el.getAttribute("data-txid")));
    });
    out.querySelectorAll(".ov-ex-blockstep").forEach((el) => {
      el.addEventListener("click", () => goBlockStepBlock(el.getAttribute("data-height")));
    });
    out.querySelectorAll(".ov-ex-open-addr").forEach((el) => {
      el.addEventListener("click", () => goBlockStepAddress(el.getAttribute("data-address")));
    });
    const rawBtn = out.querySelector("#ov-ex-show-raw");
    if (rawBtn) {
      rawBtn.addEventListener("click", () => renderExplorerSearchResult(getLastExplorerSearch(out), out, true));
    }
    const friendlyBtn = out.querySelector("#ov-ex-view-friendly");
    if (friendlyBtn) {
      friendlyBtn.addEventListener("click", () => renderExplorerSearchResult(getLastExplorerSearch(out), out, false));
    }
  }
  function ovExCopyBtn(text, label) {
    if (!text) return "";
    return (
      '<button type="button" class="ov-ex-copy" data-copy="' + escHtml(text) + '" title="Copy ' + escHtml(label || "value") + '" aria-label="Copy">' +
      '<span class="material-icons-round" aria-hidden="true">content_copy</span></button>'
    );
  }
  function ovExChip(icon, text, tone) {
    return (
      '<span class="ov-ex-chip' + (tone ? " ov-ex-chip-" + tone : "") + '">' +
      (icon ? '<span class="material-icons-round" aria-hidden="true">' + icon + "</span>" : "") +
      "<span>" + text + "</span></span>"
    );
  }
  function ovExHero(kind, title, subtitle) {
    const meta = {
      block: { icon: "view_module", tone: "block", label: "Block" },
      transaction: { icon: "receipt_long", tone: "tx", label: "Transaction" },
      address: { icon: "account_balance_wallet", tone: "addr", label: "Address" },
      none: { icon: "search_off", tone: "muted", label: "No match" },
    };
    const m = meta[kind] || meta.none;
    return (
      '<div class="ov-ex-hero ov-ex-hero-' + m.tone + '">' +
      '<div class="ov-ex-hero-icon"><span class="material-icons-round" aria-hidden="true">' + m.icon + "</span></div>" +
      '<div class="ov-ex-hero-body">' +
      '<span class="ov-ex-kind">' + escHtml(m.label) + "</span>" +
      "<h3>" + title + "</h3>" +
      (subtitle ? '<p class="ov-ex-hero-sub">' + subtitle + "</p>" : "") +
      "</div></div>"
    );
  }
  function bindOvExCopyButtons(out) {
    if (!out) return;
    out.querySelectorAll(".ov-ex-copy").forEach((btn) => {
      if (btn.dataset.copyBound === "1") return;
      btn.dataset.copyBound = "1";
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const t = btn.getAttribute("data-copy");
        if (!t || !navigator.clipboard) return;
        navigator.clipboard.writeText(t).then(() => {
          btn.classList.add("ov-ex-copy-done");
          setTimeout(() => btn.classList.remove("ov-ex-copy-done"), 1200);
        }).catch(() => {});
      });
    });
  }
  function ovExLoadingSkeleton() {
    return (
      '<div class="ov-ex-skeleton" aria-busy="true" aria-live="polite">' +
      '<div class="ov-ex-skel-hero"></div>' +
      '<div class="ov-ex-skel-row"></div>' +
      '<div class="ov-ex-skel-row short"></div>' +
      '<div class="ov-ex-skel-list"><div></div><div></div><div></div></div>' +
      "</div>"
    );
  }
  function renderAddressHitRows(rows, kind) {
    if (!rows || !rows.length) return "";
    let html = '<div class="ov-ex-hit-list">';
    rows.forEach((row) => {
      const doge = row.value_satoshi != null ? Number(row.value_satoshi) / 1e8 : null;
      const dogeStr = doge != null ? doge.toFixed(4) + " DOGE" : "";
      const icon = kind === "spend" ? "call_received" : "call_made";
      const label = kind === "spend" ? "Spent · vin " + row.vin : "Received · vout " + row.vout;
      html +=
        '<button type="button" class="ov-ex-hit-row" data-txid="' + escHtml(row.txid) + '">' +
        '<span class="ov-ex-hit-icon"><span class="material-icons-round" aria-hidden="true">' + icon + "</span></span>" +
        '<span class="ov-ex-hit-main">' +
        "<strong>" + escHtml(label) + "</strong>" +
        '<span class="ov-ex-hit-meta">Block #' + escHtml(row.height) + "</span>" +
        '<span class="mono ov-ex-hit-id">' + escHtml(shortHashEx(row.txid)) + "</span>" +
        "</span>" +
        (dogeStr ? '<span class="ov-ex-hit-amt">' + escHtml(dogeStr) + "</span>" : "") +
        '<span class="material-icons-round ov-ex-hit-chevron" aria-hidden="true">chevron_right</span>' +
        "</button>";
    });
    html += "</div>";
    return html;
  }
  function renderExplorerSearchResult(data, out, asJson) {
    if (!out) return;
    out.hidden = false;
    out.style.display = "block";
    if (explorerOutCanToggle(out)) setLastExplorerSearch(out, data);
    const canToggle = explorerOutCanToggle(out);
    if (asJson || !data || data.kind === "none" || !data.kind) {
      out.dataset.raw = asJson ? "1" : "0";
      if (asJson && data) {
        out.className = "ov-ex-results";
        out.innerHTML =
          (canToggle
            ? '<div class="ov-ex-toolbar"><button type="button" class="btn btn-ghost btn-sm" id="ov-ex-view-friendly">Show friendly view</button></div>'
            : "") +
          '<pre class="pre-out ov-ex-raw show">' + escHtml(JSON.stringify(data, null, 2)) + "</pre>";
        bindAddressHitRows(out);
        bindOvExCopyButtons(out);
        return;
      }
      if (data && data.kind === "none") {
        out.className = "ov-ex-results";
        out.innerHTML =
          ovExHero("none", "No results", escHtml(data.message || "Nothing matched that query on this node.")) +
          '<p class="ov-ex-empty-hint">Try a block height, 64-character hash or txid, or a Dogecoin address.</p>';
        return;
      }
      out.className = "pre-out ov-ex-results show";
      out.textContent = data ? JSON.stringify(data, null, 2) : "";
      return;
    }
    out.dataset.raw = "0";
    out.className = "ov-ex-results";
    if (data.kind === "block") {
      const b = data.block || {};
      const txs = data.transactions || [];
      const hashDisp = b.hash_display_hex || b.hash || "";
      const hashFull = hashDisp || b.hash_le_hex || "";
      let html = '<div class="ov-ex-panel">';
      html += '<div class="ov-ex-toolbar">';
      if (canToggle) {
        html += '<button type="button" class="btn btn-ghost btn-sm" id="ov-ex-show-raw"><span class="material-icons-round">data_object</span> Raw JSON</button>';
      }
      html +=
        '<button type="button" class="btn btn-primary btn-sm ov-ex-blockstep" data-height="' + escHtml(b.height) + '">' +
        '<span class="material-icons-round">open_in_new</span> BlockStep</button></div>';
      html += ovExHero("block", "Block #" + escHtml(b.height), escHtml(fmtBlockTime(b.time)));
      html += '<div class="ov-ex-chip-row">';
      html += ovExChip("layers", escHtml(b.tx_count != null ? b.tx_count : txs.length) + " transactions");
      html += b.has_raw_block ? ovExChip("check_circle", "Body on disk", "ok") : ovExChip("cloud_off", "Header only", "warn");
      if (b.connected_to_chain === false) html += ovExChip("sync", "Not connected yet", "warn");
      html += "</div>";
      html += '<div class="ov-ex-id-row"><span class="mono ov-ex-id">' + escHtml(shortHashEx(hashFull)) + "</span>" + ovExCopyBtn(hashFull, "block hash") + "</div>";
      if (b.coinbase_payout_addresses && b.coinbase_payout_addresses.length) {
        html += '<div class="ov-ex-note"><span class="material-icons-round">generating_tokens</span> Coinbase paid to <span class="mono">' + escHtml(b.coinbase_payout_addresses[0]) + "</span></div>";
      }
      if (data.transactions_note) {
        html += '<div class="ov-ex-alert warn">' + escHtml(data.transactions_note) + "</div>";
      }
      if (txs.length) {
        html += '<div class="ov-ex-section-head"><h4>Transactions</h4><span class="ov-ex-section-count">' + txs.length + "</span></div>";
        html += '<div class="ov-ex-hit-list">';
        txs.forEach((tx) => {
          const label = tx.is_coinbase ? "Coinbase reward" : "Transaction #" + tx.index;
          const icon = tx.is_coinbase ? "generating_tokens" : "receipt_long";
          html +=
            '<button type="button" class="ov-ex-hit-row" data-txid="' + escHtml(tx.txid) + '">' +
            '<span class="ov-ex-hit-icon"><span class="material-icons-round">' + icon + "</span></span>" +
            '<span class="ov-ex-hit-main"><strong>' + escHtml(label) + "</strong>" +
            '<span class="mono ov-ex-hit-id">' + escHtml(shortHashEx(tx.txid)) + "</span></span>" +
            (tx.total_doge != null ? '<span class="ov-ex-hit-amt">' + Number(tx.total_doge).toFixed(4) + " DOGE</span>" : "") +
            (tx.indexed === false ? '<span class="ov-ex-pill">not indexed</span>' : "") +
            '<span class="material-icons-round ov-ex-hit-chevron">chevron_right</span></button>';
        });
        html += "</div>";
      }
      html += "</div>";
      out.innerHTML = html;
      bindAddressHitRows(out);
      bindOvExCopyButtons(out);
      return;
    }
    if (data.kind === "transaction") {
      const tx = data.tx || {};
      const txid = tx.txid || data.query || "";
      const vins = tx.vin || [];
      const vouts = tx.vout || [];
      let outSum = 0;
      vouts.forEach((o) => { if (o && o.value != null) outSum += Number(o.value); });
      const conf = tx.confirmations;
      const src = data.source || "chain";
      let txHtml = '<div class="ov-ex-panel">';
      txHtml += '<div class="ov-ex-toolbar">';
      if (canToggle) {
        txHtml += '<button type="button" class="btn btn-ghost btn-sm" id="ov-ex-show-raw"><span class="material-icons-round">data_object</span> Raw JSON</button>';
      }
      txHtml +=
        '<button type="button" class="btn btn-primary btn-sm" id="ov-ex-open-tx">' +
        '<span class="material-icons-round">open_in_new</span> BlockStep</button></div>';
      txHtml += ovExHero("transaction", "Transaction", escHtml(src === "mempool" ? "Unconfirmed · local mempool" : "Confirmed on chain"));
      txHtml += '<div class="ov-ex-chip-row">';
      if (tx.height != null) txHtml += ovExChip("height", "Block #" + escHtml(tx.height));
      if (conf != null) txHtml += ovExChip("verified", conf + " confirmations", conf > 0 ? "ok" : "warn");
      txHtml += ovExChip("input", vins.length + " inputs");
      txHtml += ovExChip("output", vouts.length + " outputs");
      if (outSum > 0) txHtml += ovExChip("payments", outSum.toFixed(4) + " DOGE out");
      txHtml += "</div>";
      txHtml += '<div class="ov-ex-id-row"><span class="mono ov-ex-id">' + escHtml(txid) + "</span>" + ovExCopyBtn(txid, "txid") + "</div>";
      if (tx.blockhash) {
        txHtml += '<div class="ov-ex-note"><span class="material-icons-round">link</span> In block <span class="mono">' + escHtml(shortHashEx(tx.blockhash)) + "</span></div>";
      }
      txHtml += "</div>";
      out.innerHTML = txHtml;
      bindAddressHitRows(out);
      bindOvExCopyButtons(out);
      const btn = out.querySelector("#ov-ex-open-tx");
      if (btn) btn.addEventListener("click", () => goBlockStepTx(txid));
      return;
    }
    if (data.kind === "address") {
      const addr = (data.validate && data.validate.address) || data.query || "";
      const win = data.local_window || {};
      const bal = data.utxo_balance || {};
      let html = '<div class="ov-ex-panel">';
      html += '<div class="ov-ex-toolbar">';
      if (canToggle) {
        html += '<button type="button" class="btn btn-ghost btn-sm" id="ov-ex-show-raw"><span class="material-icons-round">data_object</span> Raw JSON</button>';
      }
      html +=
        '<button type="button" class="btn btn-primary btn-sm ov-ex-open-addr" data-address="' + escHtml(addr) + '">' +
        '<span class="material-icons-round">open_in_new</span> BlockStep</button></div>';
      html += ovExHero("address", "Address", "Local node lookup");
      if (bal.available) {
        html += '<div class="ov-ex-balance-card"><span class="ov-ex-balance-label">Confirmed balance</span>';
        html += '<div class="ov-ex-balance-val">' + Number(bal.total_doge || 0).toFixed(8) + " <small>DOGE</small></div>";
        if (bal.utxo_count != null) html += '<span class="ov-ex-balance-meta">' + escHtml(bal.utxo_count) + " UTXO</span>";
        if (bal.partial) html += '<span class="ov-ex-pill warn">cache may lag tip</span>';
        html += "</div>";
        if (bal.note) html += '<p class="field-hint">' + escHtml(String(bal.note)) + "</p>";
      } else if (bal.note) {
        html += '<p class="ov-ex-empty-hint">' + escHtml(String(bal.note)) + "</p>";
      } else if (bal.error) {
        html += '<p class="ov-ex-empty-hint">Balance unavailable: ' + escHtml(String(bal.error)) + "</p>";
      }
      html += '<div class="ov-ex-id-row"><span class="mono ov-ex-id">' + escHtml(addr) + "</span>" + ovExCopyBtn(addr, "address") + "</div>";
      if (win.total_received_doge_window != null || win.total_spent_doge_window != null) {
        html += '<div class="ov-ex-stat-grid">';
        if (win.total_received_doge_window != null) {
          html += '<div class="ov-ex-stat"><span class="ov-ex-stat-label">Received (window)</span><strong>' + Number(win.total_received_doge_window).toFixed(4) + " DOGE</strong></div>";
        }
        if (win.total_spent_doge_window != null && Number(win.total_spent_doge_window) > 0) {
          html += '<div class="ov-ex-stat"><span class="ov-ex-stat-label">Spent (window)</span><strong>' + Number(win.total_spent_doge_window).toFixed(4) + " DOGE</strong></div>";
        }
        if (win.raw_blocks_scanned != null) {
          html += '<div class="ov-ex-stat"><span class="ov-ex-stat-label">Blocks scanned</span><strong>' + escHtml(win.raw_blocks_scanned) + "</strong></div>";
        }
        html += "</div>";
      }
      if (data.local_window_error) {
        html += '<div class="ov-ex-alert warn">' + escHtml(data.local_window_error) + "</div>";
      }
      if (win.dogego_note) {
        html += '<p class="field-hint">' + escHtml(win.dogego_note) + "</p>";
      }
      const outs = win.matching_outputs || [];
      const spends = win.matching_spends || [];
      if (outs.length) {
        html += '<div class="ov-ex-section-head"><h4>Received outputs</h4><span class="ov-ex-section-count">' + outs.length + "</span></div>" + renderAddressHitRows(outs, "receive");
      }
      if (spends.length) {
        html += '<div class="ov-ex-section-head"><h4>Spent inputs</h4><span class="ov-ex-section-count">' + spends.length + "</span></div>" + renderAddressHitRows(spends, "spend");
      }
      if (!outs.length && !spends.length && !bal.available) {
        html += '<p class="ov-ex-empty-hint">No matching activity in stored blocks yet. Sync more of the chain or wait for indexing to catch up.</p>';
      }
      if (win.truncated) {
        html += '<p class="field-hint">Showing the first matches only - refine your search or open BlockStep for full history.</p>';
      }
      html += "</div>";
      out.innerHTML = html;
      bindAddressHitRows(out);
      bindOvExCopyButtons(out);
      return;
    }
    out.className = "pre-out ov-ex-results show";
    out.textContent = JSON.stringify(data, null, 2);
  }

  async function runExplorerSearch(q, outId) {
    const out = $(outId);
    if (!out) return;
    q = (q || "").trim();
    if (!q) return;
    out.hidden = false;
    out.style.display = "block";
    out.className = "ov-ex-results ov-ex-loading";
    out.innerHTML = ovExLoadingSkeleton();
    try {
      const r = await fetch("/api/explorer/search?q=" + encodeURIComponent(q), { cache: "no-store" });
      const data = await r.json();
      out.classList.remove("ov-ex-loading");
      renderExplorerSearchResult(data, out, false);
    } catch (e) {
      out.classList.remove("ov-ex-loading");
      out.className = "ov-ex-results";
      out.innerHTML = ovExHero("none", "Search failed", escHtml(String(e)));
    }
  }

  function formatLogLines(lines) {
    return (lines || []).map((l) => {
      const t = l.t || l.T || "";
      const cat = l.cat || l.Cat || "?";
      const msg = l.msg || l.Msg || "";
      return "[" + t + "] " + cat + ": " + msg;
    }).join("\n");
  }

  async function loadLogs() {
    const el = $("log-view");
    if (!el) return;
    try {
      const r = await fetchAPI("/api/logs?limit=1500", 10000);
      if (!r.ok) { el.textContent = "HTTP " + r.status; return; }
      const j = await r.json();
      const text = formatLogLines(j.lines);
      el.textContent = text || "(no log lines yet ... node activity will appear here as sync runs)";
      if ($("log-autoscroll") && $("log-autoscroll").checked) el.scrollTop = el.scrollHeight;
    } catch (e) {
      el.textContent = String(e);
    }
  }

  let lastOverviewLogAt = 0;
  async function loadOverviewLogTail(s) {
    const el = $("ov-log-tail");
    const wrap = $("ov-log-wrap");
    const toggle = $("ov-log-toggle");
    if (!el) return;
    const ibd = s && (s.ibd_active || s.initialblockdownload || isHeaderCatchUpPhase(s));
    if (!ibd) {
      if (toggle) toggle.hidden = true;
      if (wrap) wrap.hidden = true;
      return;
    }
    if (toggle) toggle.hidden = false;
    if (!overviewLogsVisible) {
      if (wrap) wrap.hidden = true;
      return;
    }
    const now = Date.now();
    if (now - lastOverviewLogAt < 2500) return;
    lastOverviewLogAt = now;
    try {
      const r = await fetchAPI("/api/logs?limit=12", 8000);
      if (!r.ok) return;
      const j = await r.json();
      const text = formatLogLines(j.lines);
      if (!text) {
        if (wrap) wrap.hidden = false;
        el.textContent = "Waiting for sync log lines… (open Console → Logs for full output)";
        return;
      }
      if (wrap) wrap.hidden = false;
      el.textContent = text;
    } catch (_) { /* */ }
  }

  function isBootPhase() {
    return Date.now() - PAGE_LOAD < BOOT_GRACE_MS;
  }

  function bootReadyToShow(s, wal) {
    if (!s) return false;
    const elapsed = Date.now() - PAGE_LOAD;
    const hasTip = s.tip_height != null && isFinite(Number(s.tip_height)) && Number(s.tip_height) >= 0;
    const syncOk = s.dogego_sync_ok === true;
    const ibd = !!(s.ibd_active || s.initialblockdownload);
    const health = String(s.dogego_sync_health || "");

    if (elapsed >= BOOT_MAX_OVERLAY_MS && hasTip) return true;

    if (!syncOk) {
      if (hasTip && ibd && elapsed > 8000) return true;
      if (hasTip && (health === "forward_ibd_starting" || health === "forward_ibd_active" || health === "syncing")) {
        return true;
      }
      return false;
    }

    if (s.wallet_enabled) {
      const w = wal || {};
      const hasAddr = !!(w.address || s.wallet_address);
      if (!hasAddr) {
        if (shouldDeferHeavyWalletAPI(s) || elapsed >= BOOT_MAX_OVERLAY_MS) return true;
        return false;
      }
    }
    return true;
  }

  function maybeHideBootOverlay(s, wal) {
    if (bootOverlayHidden || bootAppReady) return;
    if (!bootReadyToShow(s, wal)) return;
    bootAppReady = true;
    hideBootOverlay();
  }

  function setBootOverlayMessage(msg) {
    const m = $("boot-overlay-msg");
    if (m && msg) m.textContent = msg;
  }

  function hideBootOverlay() {
    if (bootOverlayHidden) return;
    bootOverlayHidden = true;
    if (bootMsgTimer) {
      clearInterval(bootMsgTimer);
      bootMsgTimer = null;
    }
    const ov = $("boot-overlay");
    if (!ov) {
      document.body.classList.remove("boot-loading");
      try { document.dispatchEvent(new CustomEvent("dogego:boot-ready")); } catch (_) {}
      return;
    }
    ov.classList.add("fade-out");
    document.body.classList.remove("boot-loading");
    try { document.dispatchEvent(new CustomEvent("dogego:boot-ready")); } catch (_) {}
    setTimeout(() => {
      ov.classList.remove("show", "fade-out");
      ov.hidden = true;
      ov.setAttribute("aria-busy", "false");
    }, 420);
  }

  function initBootOverlay() {
    document.body.classList.add("boot-loading");
    const waitEl = $("boot-overlay-wait");
    if (waitEl && window.DogeGoWait) window.DogeGoWait.mount(waitEl, "Much prepare. Very load.");
    bootMsgTimer = setInterval(() => {
      if (bootOverlayHidden) return;
      bootMsgIdx = (bootMsgIdx + 1) % BOOT_STATUS_MESSAGES.length;
      setBootOverlayMessage(BOOT_STATUS_MESSAGES[bootMsgIdx]);
    }, 3400);
    setTimeout(() => {
      if (bootOverlayHidden) return;
      if (lastSummary) maybeHideBootOverlay(lastSummary, lastWalletSnap);
    }, BOOT_MAX_OVERLAY_MS);
    setTimeout(() => {
      if (!bootOverlayHidden) {
        bootAppReady = true;
        hideBootOverlay();
      }
    }, BOOT_GRACE_MS);
  }

  function bootStatusMessage(e) {
    const msg = ((e && e.message) || "").toLowerCase();
    if (msg.includes("dashboard not ready") || msg.includes("live http")) {
      return "Node is still starting ... loading chain state and services…";
    }
    if (msg.includes("timeout") || msg.includes("aborted") || msg.includes("failed to fetch")) {
      return "Connecting to your local node ... retrying…";
    }
    return BOOT_STATUS_MESSAGES[bootMsgIdx % BOOT_STATUS_MESSAGES.length];
  }

  function renderMetricChips(el, chips) {
    if (!el) return;
    const items = (chips || []).filter((c) => c && c.text);
    if (!items.length) {
      el.innerHTML = "";
      el.hidden = true;
      return;
    }
    el.hidden = false;
    el.innerHTML = items
      .map((c) => '<span class="metric-chip' + (c.tone ? " metric-chip-" + c.tone : "") + '">' + escHtml(c.text) + "</span>")
      .join("");
  }

  function apiTimeoutMs() {
    if (Date.now() - PAGE_LOAD < BOOT_GRACE_MS) return 60000;
    return LOCAL_API_TIMEOUT_MS;
  }

  function friendlyAPIError(e) {
    if (!e) return "unknown error";
    const msg = (e.message || String(e)).toLowerCase();
    if (e.name === "TimeoutError" || msg.includes("timed out") || msg.includes("timeout")) {
      return "request timed out (node may be busy syncing)";
    }
    if (e.name === "AbortError" || msg.includes("aborted")) {
      return "request timed out (node may be busy syncing)";
    }
    if (msg.includes("failed to fetch") || msg.includes("networkerror")) {
      return "connection failed (is the node still starting?)";
    }
    return e.message || String(e);
  }

  function isTransientAPIError(e) {
    if (!e) return false;
    const msg = (e.message || "").toLowerCase();
    return e.name === "TimeoutError" || e.name === "AbortError" ||
      msg.includes("aborted") || msg.includes("timeout") ||
      msg.includes("failed to fetch") || msg.includes("network");
  }

  function showAPIError(e, streak) {
    const err = $("err");
    if (!err) return;
    const boot = isBootPhase();
    const transient = isTransientAPIError(e);
    const recentOk = lastApiSuccessAt > 0 && Date.now() - lastApiSuccessAt < 60000;
    const softStart =
      boot ||
      streak < API_FAIL_HARD_THRESHOLD ||
      recentOk ||
      ((e && e.message) || "").toLowerCase().includes("dashboard not ready");
    if (transient && softStart) {
      if (boot) {
        err.classList.remove("show");
        setBootOverlayMessage(bootStatusMessage(e));
        return;
      }
      if (txFlightBusy()) {
        err.textContent = "Send in progress ... wallet is busy. Dashboard will refresh when the send finishes.";
        err.className = "alert warn show";
        return;
      }
      err.textContent = "Node API slow to respond ... retrying. (" + friendlyAPIError(e) + ")";
      err.className = "alert warn show";
      return;
    }
    if (boot) {
      err.classList.remove("show");
      setBootOverlayMessage(bootStatusMessage(e));
      return;
    }
    hideBootOverlay();
    err.textContent = "Cannot reach node API ... " + friendlyAPIError(e);
    err.className = "alert err show";
  }

  function beginRefreshCycle() {
    refreshGen++;
    return { gen: refreshGen };
  }

  function fetchAPI(path, ms, init) {
    const timeout = ms == null ? apiTimeoutMs() : ms;
    const ctrl = new AbortController();
    const timer = setTimeout(() => {
      try {
        ctrl.abort(new DOMException("Request timed out", "TimeoutError"));
      } catch (_) {
        ctrl.abort();
      }
    }, timeout);
    const opts = Object.assign({ cache: "no-store", signal: ctrl.signal, credentials: "same-origin" }, init || {});
    if (window.DogeGoSecurity && window.DogeGoSecurity.guardFetch && path.indexOf("/api/wallet") === 0) {
      return window.DogeGoSecurity.guardFetch(path, opts).finally(() => clearTimeout(timer));
    }
    return fetch(path, opts).finally(() => clearTimeout(timer));
  }

  function pollIntervalMs(summary) {
    const lag = summaryConnectLag(summary);
    const ibd = summary && (summary.ibd_active || (Number(summary.blocks_behind_headers) || 0) > 64);
    if (ibd) return 1200;
    if (lag > 128) return 4000;
    if (lag > CONNECT_LAG_POLL_DEFER) return 2500;
    return POLL_MS;
  }

  function slowPollIntervalMs(summary) {
    const lag = summaryConnectLag(summary);
    const ibd = summary && (summary.ibd_active || (Number(summary.blocks_behind_headers) || 0) > 64);
    if (ibd) return SLOW_POLL_IBD_MS;
    if (lag > 128) return 20000;
    if (lag > CONNECT_LAG_POLL_DEFER) return 15000;
    return SLOW_POLL_MS;
  }

  function isTestnetNetwork(net) {
    const n = String(net || "").toLowerCase();
    return n === "testnet" || n === "reboottestnet";
  }

  function brandLogoForNetwork(net) {
    return isTestnetNetwork(net) ? BRAND_LOGO_TESTNET : BRAND_LOGO_MAINNET;
  }

  function applyNetworkBranding(summary) {
    const net = summary ? String(summary.network || summary.chain || "").toLowerCase() : "";
    const logo = brandLogoForNetwork(net);
    window.DogeGoLogo = logo;
    document.querySelectorAll("[data-brand-logo]").forEach((el) => {
      if (el.getAttribute("src") !== logo) el.setAttribute("src", logo);
    });
    const fav = $("brand-favicon");
    if (fav && fav.getAttribute("href") !== logo) fav.setAttribute("href", logo);
  }

  function initNetSwitcherFromSummary(s) {
    const peers = Array.isArray(s.peer_instances) ? s.peer_instances : [];
    peerInstancesCache = peers;
    const btn = $("title-net");
    const menu = $("net-switcher-menu");
    if (!btn) return;

    const net = String(s.network || s.chain || "").toLowerCase();
    btn.textContent = net || "...";
    btn.className = "badge net-switcher-btn " + (net === "testnet" ? "testnet" : net === "mainnet" ? "mainnet" : "unknown");

    if (!menu) return;
    const others = peers.filter((p) => !p.current);
    if (others.length === 0) {
      menu.hidden = true;
      btn.setAttribute("aria-expanded", "false");
      netSwitcherOpen = false;
      return;
    }
    menu.innerHTML = others.map((p) => {
      const label = p.label || p.network || "Node";
      const url = p.url || "";
      return '<button type="button" class="net-switcher-item" role="option" data-url="' + escapeHtml(url) + '">' + escapeHtml(label) + "</button>";
    }).join("");
  }

  function bindNetSwitcherOnce() {
    const root = $("net-switcher");
    const btn = $("title-net");
    const menu = $("net-switcher-menu");
    if (!root || !btn || !menu || root.dataset.bound) return;
    root.dataset.bound = "1";
    btn.addEventListener("click", (e) => {
      if (!peerInstancesCache.some((p) => !p.current)) return;
      e.stopPropagation();
      netSwitcherOpen = !netSwitcherOpen;
      menu.hidden = !netSwitcherOpen;
      btn.setAttribute("aria-expanded", netSwitcherOpen ? "true" : "false");
    });
    menu.addEventListener("click", (e) => {
      const item = e.target.closest(".net-switcher-item");
      if (!item) return;
      const url = item.getAttribute("data-url");
      if (url) window.location.href = url;
    });
    document.addEventListener("click", () => {
      if (!netSwitcherOpen) return;
      netSwitcherOpen = false;
      menu.hidden = true;
      btn.setAttribute("aria-expanded", "false");
    });
  }

  async function refresh() {
    if (refreshInFlight) return;
    refreshInFlight = true;
    let gen = 0;
    try {
      ({ gen } = beginRefreshCycle());
      if (window.DogeGoSecurity && window.DogeGoSecurity.refresh) {
        try {
          await window.DogeGoSecurity.refresh();
        } catch (_) { /* */ }
        if (gen !== refreshGen) return;
      }
      const rLive = await fetchAPI("/api/live", LIVE_API_TIMEOUT_MS);
      if (gen !== refreshGen) return;
      if (!rLive.ok) throw new Error("live HTTP " + rLive.status);
      const live = await rLive.json();
      if (!live.ok || !live.summary) throw new Error(live.summary_error || "dashboard not ready");
      const s = live.summary;
      const p2snap = live.p2p || null;
      lastSummary = s;
      if (live.analytics_summary && !lastAnalyticsJson) {
        lastAnalyticsJson = live.analytics_summary;
        try { renderAnalyticsDashboard(lastAnalyticsJson, s); } catch (_) { /* */ }
      }
      if (!s.from_disk_snapshot && !s.warming_up && !s.dogego_ui_loading) {
        persistSummarySnap(s);
      }
      applySummaryWalletStub(s);
      applyNodeMode(s);
      applyUpdateBanner(s);
      if (s.wallet_enabled) {
        walletTxCacheNetwork = String(s.network || s.chain || "").toLowerCase();
        if (!bootWalletHistoryReady && !walletTxHistory.loaded.length) {
          restoreWalletTxHistoryFromCache(walletTxCacheNetwork) || restoreWalletTxHistoryFromAnyCache();
        }
      }
      if (s.wallet_enabled && !shouldDeferWalletPoll(s)) {
        void refreshWalletPanelAsync(gen);
      }
      $("err") && $("err").classList.remove("show");

      const net = (s.network || s.chain || "").toLowerCase();
      applyNetworkBranding(s);
      initNetSwitcherFromSummary(s);
      if ($("chain")) $("chain").textContent = net || "...";
      if ($("title-ver")) $("title-ver").textContent = s.dogego_version || s.client_version || EMPTY;
      if ($("sidebar-ver")) $("sidebar-ver").textContent = s.dogego_version || s.client_version || EMPTY;
      const betaBadge = $("title-beta");
      if (betaBadge) betaBadge.hidden = s.dogego_beta === false;
      const badge = $("title-net");
      if (badge && !Array.isArray(s.peer_instances)) {
        badge.textContent = net || "...";
        badge.className = "badge net-switcher-btn " + (net === "testnet" ? "testnet" : net === "mainnet" ? "mainnet" : "unknown");
      }
      if ($("tip")) updateOverviewTipValue(s);
      updateOverviewTipLabel(s);
      clearOverviewMetricPending();
      summaryHydrated = true;
      const tipMini = $("ov-hero-tip-mini");
      if (tipMini) {
        const th = Number(s.tip_height);
        if (isFinite(th)) setCompactStat(tipMini, th, { integer: true, fit: false });
        else tipMini.textContent = "...";
      }
      if ($("headers")) {
        const hc = Number(s.header_count);
        if (isFinite(hc)) setCompactStat($("headers"), hc, { integer: true, fit: false });
        else $("headers").textContent = String(s.header_count ?? "...");
      }
      {
        const mp = Number(s.mempool_txs);
        if (isFinite(mp)) setCompactStat($("mempool"), mp, { integer: true });
        else setMetricText("mempool", String(s.mempool_txs ?? "..."));
      }
      const txProc = $("ov-tx-processed");
      if (txProc) {
        const n = Number(s.transactions_processed);
        setUIPending(txProc, false);
        if (isFinite(n) && n >= 0) setCompactStat(txProc, n, { integer: true, requireNonNeg: true });
        else txProc.textContent = "...";
      }
      const anTxProc = $("an-tx-processed");
      if (anTxProc && summaryHydrated) {
        const n = Number(s.transactions_processed);
        setUIPending(anTxProc, false);
        if (isFinite(n) && n >= 0) setCompactStat(anTxProc, n, { integer: true, requireNonNeg: true });
        else anTxProc.textContent = "-";
      }
      const pctOv = syncProgressPct(s);
      if ($("ov-metric-sync-pct")) $("ov-metric-sync-pct").textContent = String(pctOv);
      pushSpark(sparkTip, s.tip_height);
      pushSpark(sparkSync, pctOv);
      pushSpark(sparkMempool, s.mempool_txs);
      const pOut = Number(s.connections_out) || 0;
      const pIn = Number(s.connections_in) || 0;
      pushSpark(sparkPeers, pOut + pIn);
      pushSpark(sparkPeersOut, pOut);
      pushSpark(sparkPeersIn, pIn);
      chartOvTip = renderSpark($("ov-spark-tip"), chartOvTip, sparkTip, chartColors.accent, chartColors.accentFill);
      chartOvSync = renderSpark($("ov-spark-sync"), chartOvSync, sparkSync, chartColors.green, chartColors.greenFill);
      chartOvMempool = renderSpark($("ov-spark-mempool"), chartOvMempool, sparkMempool, chartColors.accent, chartColors.accentFill);
      chartOvPeers = renderSpark($("ov-spark-peers"), chartOvPeers, sparkPeers, chartColors.accent, chartColors.accentFill);
      const rp = s.relay_policy;
      if ($("ov-mp-relay") && rp) {
        let line =
          "Relay: min " + formatRelayDOGE(rp.effective_minrelay_doge) +
          " · incr " + formatRelayDOGE(rp.incrementalrelayfee_doge) +
          (rp.mempoolfullrbf ? " · full RBF" : "");
        const pkg = rp.package_policy;
        if (pkg) {
          line += " · pkg " + pkg.limitancestorcount + "/" + pkg.limitdescendantcount;
        }
        $("ov-mp-relay").textContent = line;
      }
      const storageWarn = s.storage_layout_warning;
      const storageAlert = $("ov-storage-alert");
      const storageAlertTxt = $("ov-storage-alert-text");
      if (storageAlert && storageAlertTxt) {
        if (storageWarn) {
          storageAlert.hidden = false;
          storageAlertTxt.textContent = storageWarn;
        } else {
          storageAlert.hidden = true;
          storageAlertTxt.textContent = "";
        }
      }
      const fwAlert = s.dogego_firewall_alert;
      const fwBox = $("ov-firewall-alert");
      const fwTxt = $("ov-firewall-alert-text");
      const fwCmds = $("ov-firewall-alert-cmds");
      const fwCopy = $("ov-firewall-alert-copy");
      const fwNotes = $("ov-firewall-alert-notes");
      if (fwBox && fwTxt) {
        const showFw = fwAlert && fwAlert.active && !fwAlert.dismissed;
        if (showFw) {
          fwBox.hidden = false;
          fwTxt.textContent = fwAlert.message || s.dogego_firewall_warning || "Allow DogeGo through the firewall for P2P sync.";
          if (fwCopy) {
            fwCopy.textContent = fwAlert.copy_hint || "Copy the OS-specific commands below into a terminal with admin rights:";
          }
          if (fwCmds) {
            const cmds = fwAlert.manual_commands;
            fwCmds.textContent = cmds && cmds.length ? cmds.join("\n") : "";
            fwCmds.hidden = !(cmds && cmds.length);
          }
          if (fwNotes) {
            const notes = fwAlert.manual_notes;
            if (notes && notes.length) {
              fwNotes.hidden = false;
              fwNotes.textContent = notes.join(" ");
            } else {
              fwNotes.hidden = true;
              fwNotes.textContent = "";
            }
          }
        } else {
          fwBox.hidden = true;
          fwTxt.textContent = "";
          if (fwCmds) fwCmds.textContent = "";
          if (fwNotes) { fwNotes.hidden = true; fwNotes.textContent = ""; }
        }
      }
      const ovStorageDetail = $("ov-storage-detail");
      if (ovStorageDetail) {
        renderMetricChips(ovStorageDetail, formatStorageRuntimeChips(s));
      }
      applyDiskOverview(s);
      const warns = s.chain_warnings;
      const warnEl = $("ov-chain-warn");
      if (warnEl) {
        if (warns && warns.length) {
          warnEl.hidden = false;
          warnEl.textContent = "Chain: " + warns.join("; ");
        } else {
          warnEl.hidden = true;
          warnEl.textContent = "";
        }
      }
      const chainAlert = $("ov-chain-alert");
      const chainAlertTxt = $("ov-chain-alert-text");
      if (chainAlert && chainAlertTxt) {
        if (warns && warns.length) {
          chainAlert.hidden = false;
          chainAlertTxt.textContent = warns.join(" ... ");
        } else {
          chainAlert.hidden = true;
          chainAlertTxt.textContent = "";
        }
      }
      if ($("rawblocks")) $("rawblocks").textContent = String(s.raw_blocks ?? "...");
      const peerShort = formatTopbarPeer(s, p2snap);
      const peerShortEl = $("peer-short");
      if (peerShortEl) {
        peerShortEl.textContent = peerShort;
        peerShortEl.title = peerShort !== "..." ? peerShort : "";
      }
      applySyncProgress(s);

      fillP2PCard(s, p2snap);
      if (live.mempool) {
        try { fillMempoolPanel(live.mempool); } catch (_) {}
      }
      renderOverviewCharts(s, lastAnalyticsJson);

      const now = Date.now();
      const runSlow = now - lastSlowPollAt >= slowPollIntervalMs(s);
      const lag = summaryConnectLag(s);
      const deferSlowExtras = lag > CONNECT_LAG_HEAVY_DEFER && !isPanelActive("overview") && !isPanelActive("send") && !isPanelActive("receive");
      const deferDuringSend = txFlightBusy();
      if (runSlow && !deferDuringSend) {
        lastSlowPollAt = now;
        const ibd = !!s.ibd_active || (Number(s.blocks_behind_headers) || 0) > 32;
        if (!deferSlowExtras) {
          const chainPath = ibd ? "/api/chainstats?light=1" : "/api/chainstats";
          const rStats = await fetchAPI(chainPath);
          if (gen !== refreshGen) return;
          if (rStats.ok) try { fillChainStats(await rStats.json()); } catch (_) {}
        }
        applySummaryWalletStub(s);
        if (!shouldDeferWalletPoll(s)) {
          void refreshWalletPanelAsync(gen);
        }
        if (!deferSlowExtras) {
          const rMin = await fetchAPI("/api/mining");
          if (gen !== refreshGen) return;
          if (rMin.ok) {
            const mn = await rMin.json();
            if ($("mine-in-conf")) $("mine-in-conf").textContent = mn.mine_in_config ? "on" : "off";
          }
          if (isPanelActive("settings")) {
            void loadServicesPanel();
          }
          if (s.embedded_analytics_sidecar) {
            const rAn = await fetchAPI("/api/analytics/summary");
            if (gen !== refreshGen) return;
            if (rAn.ok) {
              try {
                lastAnalyticsJson = await rAn.json();
                renderOverviewCharts(s, lastAnalyticsJson);
              } catch (_) {}
            }
          }
        }
      } else if (lastSummary) {
        updateSendUI(null, s);
      }

      if ($("chain-dir")) $("chain-dir").textContent = s.chain_data_dir || "...";
      if ($("base-dir")) $("base-dir").textContent = s.base_data_dir ? "Base: " + s.base_data_dir : "...";
      if ($("best")) $("best").textContent = s.best_hash || "...";
      if ($("peer")) $("peer").textContent = s.peer || "...";

      const showDbg = localStorage.getItem(LS_SUM) === "1";
      const dbgWrap = $("ov-debug-wrap");
      if (dbgWrap) dbgWrap.hidden = !showDbg;
      const rawSum = $("ov-raw-summary");
      if (rawSum) {
        if (showDbg) {
          rawSum.textContent = JSON.stringify(s, null, 2);
          rawSum.classList.add("show");
        } else {
          rawSum.textContent = "";
          rawSum.classList.remove("show");
        }
      }
      const p2el = $("p2p-snap");
      if (p2el) {
        if (showDbg && localStorage.getItem(LS_P2P) !== "0" && p2snap && p2snap.wired !== false) {
          p2el.textContent = JSON.stringify(p2snap, null, 2);
          p2el.classList.add("show");
        } else {
          p2el.textContent = "";
          p2el.classList.remove("show");
        }
      }

      const cout = s.connections_out != null ? String(s.connections_out) : "0";
      const cin = s.connections_in != null ? String(s.connections_in) : "0";
      ["ov-conn-out", "ov-conn-out-net", "ov-hero-peers-out"].forEach((id) => { if ($(id)) $(id).textContent = cout; });
      ["ov-conn-in", "ov-conn-in-net", "ov-hero-peers-in"].forEach((id) => { if ($(id)) $(id).textContent = cin; });
      if ($("ov-relay-note")) $("ov-relay-note").textContent = s.relay_note || "";
      if ($("dbg-relay-note")) $("dbg-relay-note").textContent = s.relay_note || "";

      if ($("rpc")) {
        const rpc = rpcSummaryLabel(s);
        $("rpc").textContent = rpc.text;
        $("rpc").classList.remove("rpc-off", "rpc-warmup", "rpc-starting");
        if (rpc.cls) $("rpc").classList.add(rpc.cls);
      }
      if ($("mine-req")) $("mine-req").textContent = s.mine_requested ? "on" : "off";

      const pan = document.getElementById("panel-analytics");
      if (pan && pan.classList.contains("active") && Date.now() - lastAnalyticsLoadAt >= ANALYTICS_REFRESH_MS) {
        void loadAnalyticsPanel(false);
      }
      maybePollOverviewCoreProbes();
      maybePollOverviewCoreStatus();
      apiFailStreak = 0;
      lastApiSuccessAt = Date.now();
      maybeHideBootOverlay(s, lastWalletSnap);
      const err = $("err");
      if (err) err.classList.remove("show");
    } catch (e) {
      if (gen !== refreshGen) return;
      if (e && e.name === "AbortError") return;
      apiFailStreak++;
      showAPIError(e, apiFailStreak);
    } finally {
      refreshInFlight = false;
    }
  }

  function stMsg(text, ok) {
    const m = $("st-msg");
    if (!m) return;
    m.textContent = text;
    m.className = ok ? "show ok" : "show bad";
  }

  const SERVICE_ACTION_LABELS = {
    start: "Start",
    stop: "Stop",
    restart: "Restart",
    pause: "Pause",
    resume: "Resume",
    clear: "Clear",
  };

  const SERVICE_ACTION_ICONS = {
    start: "play_arrow",
    stop: "stop",
    restart: "restart_alt",
    pause: "pause",
    resume: "play_arrow",
    clear: "delete_sweep",
  };

  function serviceActionButtonHtml(action) {
    const label = SERVICE_ACTION_LABELS[action] || action;
    const icon = SERVICE_ACTION_ICONS[action];
    const iconHtml = icon
      ? '<span class="material-icons-round" aria-hidden="true">' + icon + "</span> "
      : "";
    return iconHtml + label;
  }

  function serviceActionsForRow(s) {
    const running = !!s.running;
    return (s.actions || []).filter((a) => {
      switch (a) {
        case "start":
          return !running;
        case "stop":
        case "pause":
          return running;
        case "resume":
          return !running;
        case "restart":
          return running || s.id === "node";
        case "clear":
          return true;
        default:
          return true;
      }
    });
  }

  function serviceStatusPill(s) {
    const detail = String(s.detail || "").toLowerCase();
    if (s.id === "rpc" && !s.running && detail.indexOf("disabled") >= 0) {
      return '<span class="service-pill idle">disabled</span>';
    }
    if (s.id === "rpc" && s.running && (detail.indexOf("warming") >= 0 || detail.indexOf("starting") >= 0)) {
      return '<span class="service-pill warn">starting</span>';
    }
    if (s.running) return '<span class="service-pill on">running</span>';
    if (s.id === "mempool" && detail.indexOf("paused") >= 0) {
      return '<span class="service-pill warn">paused</span>';
    }
    return '<span class="service-pill off">stopped</span>';
  }

  function renderServicesList(services) {
    const el = $("st-services-list");
    if (!el) return;
    if (!services || !services.length) {
      el.innerHTML = "<p class=\"label\">Service control unavailable.</p>";
      return;
    }
    el.innerHTML = services.map((s) => {
      const status = serviceStatusPill(s);
      const actions = serviceActionsForRow(s).map((a) => {
        const danger = a === "stop" || a === "clear" || a === "pause";
        const primary = a === "start" || a === "resume" || a === "restart";
        let cls = "btn btn-sm";
        if (danger) cls += " btn-danger";
        else if (primary) cls += " btn-primary";
        else cls += " btn-secondary";
        return "<button type=\"button\" class=\"" + cls + "\" data-svc=\"" + escHtml(s.id) + "\" data-act=\"" + escHtml(a) + "\">" +
          serviceActionButtonHtml(a) + "</button>";
      }).join("");
      const note = s.restart_note ? "<p class=\"label service-note\">" + escHtml(s.restart_note) + "</p>" : "";
      const detail = s.detail ? "<p class=\"label service-detail\">" + escHtml(s.detail) + "</p>" : "";
      return "<div class=\"service-row\" data-service-id=\"" + escHtml(s.id) + "\">" +
        "<div class=\"service-head\"><strong>" + escHtml(s.label) + "</strong> " + status + "</div>" +
        detail +
        (actions ? "<div class=\"service-actions\">" + actions + "</div>" : "") +
        note + "</div>";
    }).join("");
    el.querySelectorAll("[data-svc]").forEach((btn) => {
      btn.addEventListener("click", () => runServiceAction(btn.dataset.svc, btn.dataset.act));
    });
  }

  async function loadServicesPanel() {
    try {
      const r = await fetch("/api/services", { cache: "no-store" });
      if (!r.ok) {
        renderServicesList([]);
        updateMiningSettingsPanel([]);
        return;
      }
      const data = await r.json();
      const services = data.services || [];
      renderServicesList(services);
      updateMiningSettingsPanel(services);
    } catch (_) {
      renderServicesList([]);
      updateMiningSettingsPanel([]);
    }
  }

  function findServiceRow(services, id) {
    return (services || []).find((s) => s.id === id) || null;
  }

  function updateMiningSettingsPanel(services) {
    const mining = findServiceRow(services, "mining");
    const statusEl = $("st-mining-status");
    const startBtn = $("st-mining-start");
    const stopBtn = $("st-mining-stop");
    const restartBtn = $("st-mining-restart");
    if (!mining) {
      if (statusEl) {
        statusEl.textContent = "Mining control unavailable (full node on reboot testnet required). Mainnet: use Console generatetoaddress or Tools → Mining.";
      }
      [startBtn, stopBtn, restartBtn].forEach((b) => { if (b) b.disabled = true; });
      return;
    }
    const parts = [];
    if (mining.detail) parts.push(mining.detail);
    if (!mining.can_start && !mining.can_stop && mining.restart_note) {
      parts.push(mining.restart_note);
    } else if (mining.running && mining.can_stop) {
      parts.push("Start is disabled while mining is running - use Stop or Restart for this run only.");
    } else if (!mining.running && mining.can_start) {
      parts.push("Start applies to this run only; save config + restart node to change the mine flag on boot.");
    }
    if (statusEl) statusEl.textContent = parts.join(" ");
    if (startBtn) startBtn.disabled = !mining.can_start;
    if (stopBtn) stopBtn.disabled = !mining.can_stop;
    if (restartBtn) restartBtn.disabled = !(mining.running || mining.can_stop);
  }

  function syncTopbarLockButton(wal) {
    const btn = $("topbar-lock");
    if (!btn) return;
    const walletEnc = !!(wal && wal.enabled !== false && wal.encrypted === true);
    btn.hidden = !walletEnc;
    if (!walletEnc) return;
    const walletLocked = wal.unlocked === false;
    const icon = btn.querySelector(".material-icons-round");
    if (walletLocked) {
      btn.title = "Wallet keys locked; click to enter passphrase";
      if (icon) icon.textContent = "lock";
    } else {
      btn.title = "Lock wallet keys";
      if (icon) icon.textContent = "lock_open";
    }
    btn.setAttribute("aria-label", btn.title);
  }

  function formatSettingsSaveWarning(kind, raw) {
    raw = String(raw || "").trim();
    if (!raw) return "";
    if (kind === "autostart") {
      const prefix = "Login autostart could not be registered:";
      if (raw.startsWith(prefix)) raw = raw.slice(prefix.length).trim();
      if (raw.startsWith("autostart:")) raw = raw.slice("autostart:".length).trim();
    }
    return raw;
  }

  function showSettingsRestartModal(opts) {
    opts = opts || {};
    const warnings = opts.warnings || {};
    const modal = $("settings-restart-modal");
    const notes = $("settings-restart-notes");
    const msgEl = $("settings-restart-msg");
    const promptEl = $("settings-restart-prompt");
    if (!modal) return;
    if (opts.variant === "manual") {
      const text = (window.DogeGoI18n && window.DogeGoI18n.t("settings.restartModalManualBody")) ||
        "DogeGo will restart and the dashboard will reconnect in a few seconds.";
      if (msgEl) msgEl.textContent = text;
    } else if (msgEl && window.DogeGoI18n) {
      msgEl.setAttribute("data-i18n", "settings.restartModalBody");
      window.DogeGoI18n.applyDOM(modal);
    }
    if (promptEl) {
      promptEl.hidden = false;
      if (window.DogeGoI18n) {
        promptEl.textContent = window.DogeGoI18n.t("settings.restartModalPrompt");
      }
    }
    const items = [];
    if (warnings && warnings.autostart) {
      const text = formatSettingsSaveWarning("autostart", warnings.autostart);
      const label = (window.DogeGoI18n && window.DogeGoI18n.t("settings.restartModalNoteAutostart")) || "Autostart";
      const hint = (window.DogeGoI18n && window.DogeGoI18n.t("settings.restartModalAutostartHint")) ||
        "Restart will not start DogeGo automatically at login until autostart is registered. Start DogeGo manually after restart, or run it once with administrator privileges to enable login autostart.";
      items.push("<div class=\"alert warn\" role=\"note\"><strong>" + escapeHtml(label) + "</strong><p>" + escapeHtml(text) + "</p><p class=\"label\">" + escapeHtml(hint) + "</p></div>");
    }
    if (warnings && warnings.uacomment) {
      const text = formatSettingsSaveWarning("tip", warnings.uacomment);
      const label = (window.DogeGoI18n && window.DogeGoI18n.t("settings.restartModalNoteTip")) || "User-agent tip";
      items.push("<div class=\"alert ok\" role=\"note\"><strong>" + escapeHtml(label) + "</strong>" + escapeHtml(text) + "</div>");
    }
    if (notes) {
      if (items.length) {
        notes.innerHTML = items.join("");
        notes.hidden = false;
      } else {
        notes.innerHTML = "";
        notes.hidden = true;
      }
    }
    modal.hidden = false;
  }

  function hideSettingsRestartModal() {
    const modal = $("settings-restart-modal");
    const notes = $("settings-restart-notes");
    const msgEl = $("settings-restart-msg");
    if (notes) {
      notes.innerHTML = "";
      notes.hidden = true;
    }
    if (msgEl && window.DogeGoI18n) {
      msgEl.setAttribute("data-i18n", "settings.restartModalBody");
      window.DogeGoI18n.applyDOM(modal);
    }
    if (modal) modal.hidden = true;
  }

  async function autostartWarningsForRestart() {
    const warnings = {};
    try {
      const r = await fetch("/api/autostart", { cache: "no-store" });
      if (!r.ok) return warnings;
      const data = await r.json();
      if (data.configured && !data.ok) {
        const detail = data.status && data.status.detail ? String(data.status.detail) : "";
        warnings.autostart = detail || "Login autostart is configured but not registered on this system.";
      }
    } catch (_) { /* */ }
    return warnings;
  }

  async function promptRestartNodeModal(extraWarnings) {
    const warnings = Object.assign({}, await autostartWarningsForRestart(), extraWarnings || {});
    showSettingsRestartModal({ variant: "manual", warnings: warnings });
  }

  async function restartNodeFromSettings() {
    stMsg("Restarting node…", true);
    try {
      const r = await fetch("/api/control/restart", { method: "POST", credentials: "same-origin" });
      if (!r.ok) {
        stMsg(await r.text().catch(() => "Restart failed"), false);
        return;
      }
      pollNodeAfterRestart();
    } catch (e) {
      stMsg(String(e), false);
    }
  }

  async function confirmRestartNodeFromSettings() {
    await promptRestartNodeModal();
  }

  async function stopNodeFromSettings() {
    if (!confirm("Stop DogeGo? The dashboard will disconnect.")) return;
    stMsg("Stopping node…", true);
    try {
      await fetch("/api/control/shutdown", { method: "POST" });
    } catch (_) {}
  }

  function pollNodeAfterRestart() {
    let attempts = 0;
    const maxAttempts = 60;
    stMsg("Restarting node… waiting for dashboard", true);
    const timer = setInterval(async () => {
      attempts += 1;
      try {
        const r = await fetch("/api/summary", { cache: "no-store", credentials: "same-origin" });
        if (r.ok) {
          clearInterval(timer);
          stMsg("Node restarted successfully.", true);
          void loadServicesPanel();
          refresh();
          setTimeout(() => { location.reload(); }, 400);
          return;
        }
      } catch (_) {}
      if (attempts >= maxAttempts) {
        clearInterval(timer);
        stMsg("Restart sent. Reload this page when the node is back.", true);
      }
    }, 1500);
  }

  async function runServiceAction(serviceId, action) {
    if (serviceId === "node") {
      if (action === "stop") {
        if (!confirm("Stop DogeGo? The dashboard will disconnect.")) return;
        await fetch("/api/control/shutdown", { method: "POST" });
        stMsg("Stopping node…", true);
        return;
      }
      if (action === "restart") {
        await promptRestartNodeModal();
        return;
      }
    }
    if (serviceId === "mining" && (action === "stop" || action === "restart")) {
      if (!confirm(action === "stop" ? "Stop background mining for this run?" : "Restart background mining?")) return;
    }
    try {
      const r = await fetch("/api/services", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ service: serviceId, action: action }),
      });
      if (!r.ok) {
        const errText = await r.text().catch(() => "");
        stMsg(errText || "Action failed (HTTP " + r.status + ")", false);
        return;
      }
      const data = await r.json().catch(() => ({}));
      const services = data.services || [];
      renderServicesList(services);
      updateMiningSettingsPanel(services);
      stMsg((SERVICE_ACTION_LABELS[action] || action) + " ... " + serviceId + " OK", true);
      refresh();
    } catch (e) {
      stMsg(String(e), false);
    }
  }

  function setNumInput(id, n) {
    const el = $(id);
    if (!el) return;
    el.value = n === 0 || n === undefined || n === null ? "" : String(n);
  }

  function formatStorageRuntimeChips(s) {
    if (!s) return [];
    const chips = [];
    if (s.native_contiguous_body_height != null && s.native_contiguous_body_height >= 0) {
      chips.push({ text: "bodies #" + s.native_contiguous_body_height });
    }
    if (s.native_raw_block_count != null) chips.push({ text: s.native_raw_block_count + " blocks" });
    if (s.native_tx_index) {
      const leg = s.native_txindex_legacy_files;
      const v2 = s.native_txindex_v2_files;
      let ix = "tx index";
      if (leg != null || v2 != null) ix += " " + (v2 || 0) + "v2";
      chips.push({ text: ix, tone: "accent" });
    } else if (s.native_tx_index === false) {
      chips.push({ text: "no tx index", tone: "muted" });
    }
    if (!chips.length) chips.push({ text: "native layout" });
    return chips;
  }

  function formatStorageRuntimeLine(s) {
    if (!s) return "...";
    const parts = [];
    if (s.native_contiguous_body_height != null && s.native_contiguous_body_height >= 0) {
      parts.push("bodies through " + s.native_contiguous_body_height);
    }
    if (s.native_raw_block_count != null) parts.push(s.native_raw_block_count + " blocks stored");
    if (s.native_tx_index) {
      let ix = "tx index";
      const leg = s.native_txindex_legacy_files;
      const v2 = s.native_txindex_v2_files;
      if (leg != null || v2 != null) ix += " (" + (v2 || 0) + " v2, " + (leg || 0) + " legacy)";
      parts.push(ix);
    } else if (s.native_tx_index === false) parts.push("no tx index");
    if (!parts.length) parts.push("native layout");
    return parts.join(" · ");
  }

  function applyDiskOverview(s) {
    const totalEl = $("ov-disk-total");
    const chipsEl = $("ov-disk-chips");
    const footEl = $("ov-disk-footnote");
    if (!totalEl) return;
    if (!s || s.node_mode === "spv") {
      totalEl.textContent = s && s.node_mode === "spv" ? "SPV" : "...";
      renderMetricChips(chipsEl, s && s.node_mode === "spv" ? [{ text: "No block bodies", tone: "muted" }] : []);
      if (footEl) footEl.hidden = true;
      return;
    }
    totalEl.textContent = s.chain_bytes_total != null ? fmtBytes(s.chain_bytes_total) : "...";
    const logical = Number(s.blocks_logical_bytes) || 0;
    const stored = Number(s.blocks_stored_payload_bytes) || 0;
    const layout = (s.block_layout || "perfile").toLowerCase();
    const zstdOn = !!s.block_zstd;
    const chips = [];
    if (s.headers_bytes != null) chips.push({ text: "headers " + fmtBytes(s.headers_bytes) });
    if (s.rawblocks_bytes != null) chips.push({ text: "rawblocks " + fmtBytes(s.rawblocks_bytes) });
    if (s.txindex_bytes != null) chips.push({ text: "tx index " + fmtBytes(s.txindex_bytes) });
    chips.push({ text: layout + (zstdOn ? " + zstd" : ""), tone: "accent" });
    if (logical > 0 && stored > 0) {
      chips.push({ text: fmtBytes(stored) + " on disk", tone: "accent" });
      chips.push({ text: fmtBytes(logical) + " wire" });
      const savings = Number(s.compression_savings_pct);
      const ratio = Number(s.compression_ratio);
      if (savings > 0.05) {
        chips.push({ text: savings.toFixed(1) + "% smaller", tone: "good" });
      } else if (ratio > 0 && ratio < 1) {
        chips.push({ text: (ratio * 100).toFixed(0) + "% of wire", tone: "good" });
      } else if (ratio >= 1) {
        chips.push({ text: "no compression gain", tone: "muted" });
      }
    } else if (s.rawblocks_bytes != null) {
      chips.push({ text: "scanning ratio…", tone: "muted" });
    }
    renderMetricChips(chipsEl, chips);
    if (footEl) {
      footEl.textContent = "Core uses blocks/ + chainstate/ (uncompressed blk, separate UTXO DB)";
      footEl.hidden = false;
    }
    const syncDisk = $("sync-disk-line");
    if (syncDisk) {
      if (!s || s.node_mode === "spv") {
        syncDisk.textContent = "";
      } else if (chips.length) {
        const chipText = chips.map((c) => c.text).join(" · ");
        syncDisk.textContent = "Disk: " + (s.chain_bytes_total != null ? fmtBytes(s.chain_bytes_total) + " chain · " : "") + chipText;
      } else if (s.chain_bytes_total != null) {
        syncDisk.textContent = "Disk: " + fmtBytes(s.chain_bytes_total) + " chain data";
      } else {
        syncDisk.textContent = "";
      }
    }
  }

  function updateSettingsModeUI() {
    const spv = $("st-mode") && $("st-mode").value === "spv";
    document.querySelectorAll(".settings-full-only").forEach((el) => {
      el.classList.toggle("is-spv", spv);
      if (el.matches('[data-st-sub="sync"]')) el.hidden = spv;
    });
    if (spv) {
      const syncBtn = document.querySelector('[data-subtabs="settings"] [data-st-sub="sync"]');
      if (syncBtn && syncBtn.classList.contains("active")) activateSubTab("settings", "general");
    }
  }

  function renderSettingsRuntime(rt) {
    settingsRuntime = rt || {};
    const el = $("st-runtime");
    if (!el || !rt) return;
    const mode = rt.node_mode || "full";
    const parts = [
      "This run: " + mode,
      rt.full_node ? "full node" : "SPV",
      rt.embedded_analytics_sidecar ? "analytics on" : "analytics off",
      rt.tx_index_enabled ? "tx index" : "no tx index",
    ];
    if (rt.mine_requested) parts.push("mine flag");
    if (rt.p2p_subversion) parts.push("wire UA active");
    el.textContent = parts.join(" · ");
  }

  function syncSettingsChoiceCards() {
    if (!window.DogeGoControls) return;
    ["st-network", "st-mode", "st-block-layout", "st-p2p", "st-firewall", "st-upnp"].forEach((id) => {
      window.DogeGoControls.syncChoiceCard(id);
    });
  }

  let uacommentPreviewTimer = 0;

  function uacommentPublishEnabled() {
    return $("st-uacomment-publish-tip") && $("st-uacomment-publish-tip").checked;
  }

  function buildUACommentPreviewBody() {
    const body = {
      uacomment: $("st-uacomment") ? $("st-uacomment").value.trim() : "",
      publish_tip: uacommentPublishEnabled(),
      datadir: $("st-datadir") ? $("st-datadir").value.trim() : "",
      network: $("st-network") ? $("st-network").value : "testnet",
      nowallet: !settingsWalletEnabled(),
    };
    if (body.publish_tip) {
      if (settingsWalletEnabled()) {
        body.uacomment_use_node_tip = true;
      } else if ($("st-uacomment-tip-addr")) {
        body.uacomment_tip_address = $("st-uacomment-tip-addr").value.trim();
      }
    }
    return body;
  }

  async function refreshUACommentPreview() {
    const subEl = $("st-uacomment-subversion-preview");
    const tipEl = $("st-uacomment-tip-preview");
    if (!subEl && !tipEl) return;
    try {
      const r = await fetch("/api/config/uacomment-preview", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify(buildUACommentPreviewBody()),
      });
      if (!r.ok) {
        const err = await r.text().catch(() => "");
        if (subEl) subEl.textContent = err ? String(err).trim() : "Preview unavailable";
        if (tipEl) tipEl.hidden = true;
        return;
      }
      const data = await r.json();
      if (subEl) {
        const sub = data.subversion || "";
        subEl.textContent = sub ? "Wire user-agent: " + sub : "Wire user-agent: (default)";
        subEl.title = sub;
      }
      if (tipEl) {
        if (data.tip_preview_error && uacommentPublishEnabled()) {
          tipEl.textContent = String(data.tip_preview_error);
          tipEl.hidden = false;
        } else if (data.tip_address && uacommentPublishEnabled()) {
          tipEl.textContent = "Tip address: " + data.tip_address;
          tipEl.hidden = false;
        } else {
          tipEl.hidden = true;
          tipEl.textContent = "";
        }
      }
    } catch (_) {
      if (subEl) subEl.textContent = "";
    }
  }

  function scheduleUACommentPreview() {
    if (uacommentPreviewTimer) clearTimeout(uacommentPreviewTimer);
    uacommentPreviewTimer = setTimeout(refreshUACommentPreview, 280);
  }

  function settingsWalletEnabled() {
    if ($("st-nowallet") && $("st-nowallet").checked) return false;
    return !($("st-wallet-enabled") && !$("st-wallet-enabled").checked);
  }

  function syncUACommentSettingsUI() {
    const publish = uacommentPublishEnabled();
    const walletOn = settingsWalletEnabled();
    const hint = $("st-uacomment-tip-hint");
    const manualWrap = $("st-uacomment-manual-wrap");
    if ($("st-uacomment-node-tip")) $("st-uacomment-node-tip").checked = walletOn;
    if (hint) hint.hidden = !publish || !walletOn;
    if (manualWrap) manualWrap.hidden = !publish || walletOn;
    scheduleUACommentPreview();
  }

  function applyConfigToForm(f) {
    dogegoSavedConfig = Object.assign({}, f);
    if ($("st-datadir")) $("st-datadir").value = f.datadir || "";
    if ($("st-peer")) $("st-peer").value = f.peer || "";
    if ($("st-network")) {
      const net = String(f.network || "testnet").toLowerCase();
      $("st-network").value = net === "mainnet" ? "mainnet" : "testnet";
    }
    if ($("st-rpc")) $("st-rpc").value = f.rpc || "";
    if ($("st-webui")) $("st-webui").value = f.webui || "";
    if ($("st-webui-remote-auth")) $("st-webui-remote-auth").checked = !!f.webui_remote_auth;
    if ($("st-mode")) $("st-mode").value = (f.node_mode || "full").toLowerCase() === "spv" ? "spv" : "full";
    const p2p = (f.p2p_connectivity || "both").toLowerCase();
    if ($("st-p2p") && ["both", "cgnat", "classic"].includes(p2p)) $("st-p2p").value = p2p;
    const fw = (f.firewall || "auto").toLowerCase();
    if ($("st-firewall") && ["auto", "always", "never"].includes(fw)) $("st-firewall").value = fw;
    const upnp = (f.upnp || "auto").toLowerCase();
    if ($("st-upnp") && ["auto", "enable", "disable"].includes(upnp)) $("st-upnp").value = upnp;
    setNumInput("st-maxout", f.maxoutbound);
    setNumInput("st-maxin", f.maxinbound);
    setNumInput("st-raw-backfill", f.rawblock_backfill);
    setNumInput("st-block-workers", f.block_sync_workers);
    setNumInput("st-maxtxfee", f.maxtxfee);
    setNumInput("st-maxmempool", f.maxmempool);
    setNumInput("st-dbcache", f.dbcache);
    setNumInput("st-mempoolexpiry", f.mempoolexpiry);
    setNumInput("st-maxorphantx", f.maxorphantx);
    setNumInput("st-minrelay", f.minrelaytxfee);
    setNumInput("st-incrementalrelay", f.incrementalrelayfee);
    setNumInput("st-harddust", f.harddustlimit);
    setNumInput("st-blockmaxweight", f.blockmaxweight);
    setNumInput("st-limit-anc", f.limitancestorcount);
    setNumInput("st-limit-desc", f.limitdescendantcount);
    setNumInput("st-limit-anc-kb", f.limitancestorsize);
    setNumInput("st-limit-desc-kb", f.limitdescendantsize);
    setNumInput("st-datacarrier-size", f.datacarriersize);
    if ($("st-accept-datacarrier")) {
      $("st-accept-datacarrier").checked = f.acceptdatacarrier !== false;
    }
    if ($("st-permit-bare-ms")) {
      $("st-permit-bare-ms").checked = f.permitbaremultisig !== false;
    }
    if ($("st-no-tx-index")) $("st-no-tx-index").checked = !!f.no_tx_index;
    if ($("st-block-layout")) {
      const lay = (f.block_storage_layout || "perfile").toLowerCase();
      $("st-block-layout").value = lay === "bundled" ? "bundled" : "perfile";
    }
    if ($("st-block-zstd")) $("st-block-zstd").checked = !!f.block_zstd;
    if ($("st-tx-embed")) $("st-tx-embed").checked = f.tx_index_embed_tx !== false;
    if ($("st-analytics")) $("st-analytics").checked = f.analytics_sidecar !== false;
    if ($("st-ibd-optimize")) $("st-ibd-optimize").checked = f.ibd_optimize !== false;
    if ($("st-nowebui")) $("st-nowebui").checked = !!f.nowebui;
    if ($("st-nobrowser")) $("st-nobrowser").checked = !!f.nobrowser;
    if ($("st-tray")) $("st-tray").checked = f.tray !== false;
    if ($("st-autostart")) {
      const as = String(f.autostart || "disable").toLowerCase();
      $("st-autostart").checked = as === "login";
    }
    if ($("st-nowallet")) $("st-nowallet").checked = !!f.nowallet;
    syncWalletEnabledToggle();
    if ($("st-unverified-mempool")) $("st-unverified-mempool").checked = !!f.allow_unverified_mempool;
    if ($("st-fullrbf")) $("st-fullrbf").checked = !!f.mempoolfullrbf;
    if ($("st-persistmempool")) {
      $("st-persistmempool").checked = f.persistmempool !== false;
    }
    if ($("st-rpc-cookie")) $("st-rpc-cookie").checked = !!f.rpc_cookie;
    if ($("st-webui-tls-local")) $("st-webui-tls-local").checked = !!f.webui_tls_local;
    if ($("st-rpc-tls-local")) $("st-rpc-tls-local").checked = !!f.rpc_tls_local;
    if ($("st-local-tls-trust-ca")) $("st-local-tls-trust-ca").checked = !!f.local_tls_trust_ca;
    if ($("mine-want")) $("mine-want").checked = !!f.mine;
    if ($("st-mining-addr")) $("st-mining-addr").value = f.miningaddress || "";
    if ($("st-uacomment")) $("st-uacomment").value = f.uacomment || "";
    if ($("st-uacomment-publish-tip")) {
      $("st-uacomment-publish-tip").checked = !!(f.uacomment_tip_address || f.uacomment_use_node_tip);
    }
    if ($("st-uacomment-node-tip")) $("st-uacomment-node-tip").checked = !!f.uacomment_use_node_tip;
    if ($("st-uacomment-tip-addr")) $("st-uacomment-tip-addr").value = f.uacomment_tip_address || "";
    const dgr = f.dogego_relay_cgnat || {};
    if ($("st-dgr-enabled")) $("st-dgr-enabled").checked = !!dgr.enabled;
    if ($("st-dgr-inbound")) $("st-dgr-inbound").checked = !!dgr.inbound_relay;
    if ($("st-dgr-outbound")) $("st-dgr-outbound").checked = !!dgr.outbound_relay;
    if ($("st-dgr-listen")) $("st-dgr-listen").value = dgr.listen || "";
    setNumInput("st-dgr-port", dgr.relay_port);
    if ($("st-dgr-seeds")) {
      $("st-dgr-seeds").value = Array.isArray(dgr.relay_seeds) ? dgr.relay_seeds.join("\n") : "";
    }
    if ($("st-dgr-dns")) $("st-dgr-dns").value = dgr.relay_dnsseed || "";
    setNumInput("st-dgr-max-clients", dgr.max_clients);
    setNumInput("st-dgr-max-conns", dgr.max_relay_conns);
    if ($("st-dgr-token")) $("st-dgr-token").value = dgr.auth_token || "";
    if ($("st-dgr-tls-pins")) {
      $("st-dgr-tls-pins").value = Array.isArray(dgr.relay_tls_pins) ? dgr.relay_tls_pins.join("\n") : "";
    }
    setNumInput("st-dgr-max-frames", dgr.max_session_frames_per_sec);
    setNumInput("st-dgr-max-p2p-proxy", dgr.max_p2p_proxy_per_sec);
    setNumInput("st-dgr-max-register", dgr.max_register_per_min);
    const adv = $("st-dgr-show-advanced");
    if (adv && !adv.checked) {
      const hasAdv = !!(dgr.relay_tls_pins && dgr.relay_tls_pins.length) ||
        dgr.max_session_frames_per_sec || dgr.max_p2p_proxy_per_sec || dgr.max_register_per_min;
      if (hasAdv) {
        adv.checked = true;
        const box = $("st-dgr-advanced");
        if (box) box.hidden = false;
      }
    }
    syncDGRRoleCards();
    updateDGRFieldsVisibility();
    suggestDGRForP2PMode();
    const p2pMode = (f.p2p_connectivity || "both").toLowerCase();
    fillDGRCard("st", dgrLiveCache || {}, { p2pMode: p2pMode, forceShow: !!dgr.enabled || p2pMode === "cgnat" || p2pMode === "both" });
    syncSettingsChoiceCards();
    if ($("st-rpc-user")) $("st-rpc-user").value = f.rpc_user || "";
    if ($("st-rpc-pass")) $("st-rpc-pass").value = f.rpc_password || "";
    if ($("st-core-rpc")) $("st-core-rpc").value = f.core_rpc_addr || "";
    if ($("st-core-rpc-user")) $("st-core-rpc-user").value = f.core_rpc_user || "";
    if ($("st-core-rpc-pass")) $("st-core-rpc-pass").value = f.core_rpc_password || "";
    if ($("st-signer-cmd")) $("st-signer-cmd").value = f.signer_cmd || "";
    if ($("st-zmq-hashblock")) $("st-zmq-hashblock").value = f.zmqpubhashblock || "";
    if ($("st-zmq-hashtx")) $("st-zmq-hashtx").value = f.zmqpubhashtx || "";
    if ($("st-zmq-rawblock")) $("st-zmq-rawblock").value = f.zmqpubrawblock || "";
    if ($("st-zmq-rawtx")) $("st-zmq-rawtx").value = f.zmqpubrawtx || "";
    updateSettingsModeUI();
    syncUACommentSettingsUI();
  }

  function buildConfigFromForm() {
    const maxOut = parseInt($("st-maxout") && $("st-maxout").value, 10);
    const maxIn = parseInt($("st-maxin") && $("st-maxin").value, 10);
    const rawBf = $("st-raw-backfill") && $("st-raw-backfill").value.trim();
    const blockW = parseInt($("st-block-workers") && $("st-block-workers").value, 10);
    const maxFee = parseInt($("st-maxtxfee") && $("st-maxtxfee").value, 10);
    const maxMem = parseInt($("st-maxmempool") && $("st-maxmempool").value, 10);
    const dbCache = parseInt($("st-dbcache") && $("st-dbcache").value, 10);
    const memExp = parseInt($("st-mempoolexpiry") && $("st-mempoolexpiry").value, 10);
    const maxOrph = parseInt($("st-maxorphantx") && $("st-maxorphantx").value, 10);
    const minRelay = parseInt($("st-minrelay") && $("st-minrelay").value, 10);
    const incrRelay = parseInt($("st-incrementalrelay") && $("st-incrementalrelay").value, 10);
    const hardDust = parseInt($("st-harddust") && $("st-harddust").value, 10);
    const blockWt = parseInt($("st-blockmaxweight") && $("st-blockmaxweight").value, 10);
    const limAnc = parseInt($("st-limit-anc") && $("st-limit-anc").value, 10);
    const limDesc = parseInt($("st-limit-desc") && $("st-limit-desc").value, 10);
    const limAncKb = parseInt($("st-limit-anc-kb") && $("st-limit-anc-kb").value, 10);
    const limDescKb = parseInt($("st-limit-desc-kb") && $("st-limit-desc-kb").value, 10);
    const dataSz = parseInt($("st-datacarrier-size") && $("st-datacarrier-size").value, 10);
    const body = Object.assign({}, dogegoSavedConfig, {
      datadir: $("st-datadir").value.trim(),
      peer: $("st-peer").value.trim(),
      network: $("st-network").value,
      rpc: $("st-rpc").value.trim(),
      webui: $("st-webui").value.trim(),
      webui_remote_auth: $("st-webui-remote-auth") && $("st-webui-remote-auth").checked,
      node_mode: $("st-mode").value,
      p2p_connectivity: $("st-p2p") ? $("st-p2p").value : "both",
      firewall: $("st-firewall") ? $("st-firewall").value : "auto",
      upnp: $("st-upnp") ? $("st-upnp").value : "auto",
      maxoutbound: isNaN(maxOut) ? 0 : maxOut,
      maxinbound: isNaN(maxIn) ? 0 : maxIn,
      block_sync_workers: isNaN(blockW) ? 0 : blockW,
      no_tx_index: $("st-no-tx-index") && $("st-no-tx-index").checked,
      block_storage_layout: $("st-block-layout") ? $("st-block-layout").value : "perfile",
      block_zstd: $("st-block-zstd") && $("st-block-zstd").checked,
      tx_index_embed_tx: !($("st-tx-embed") && $("st-tx-embed").checked === false),
      analytics_sidecar: !($("st-analytics") && $("st-analytics").checked === false),
      ibd_optimize: !($("st-ibd-optimize") && $("st-ibd-optimize").checked === false),
      nowebui: $("st-nowebui") && $("st-nowebui").checked,
      nobrowser: $("st-nobrowser") && $("st-nobrowser").checked,
      tray: $("st-tray") && $("st-tray").checked,
      autostart: ($("st-autostart") && $("st-autostart").checked) ? "login" : "disable",
      nowallet: !($("st-wallet-enabled") && $("st-wallet-enabled").checked),
      allow_unverified_mempool: $("st-unverified-mempool") && $("st-unverified-mempool").checked,
      mempoolfullrbf: $("st-fullrbf") && $("st-fullrbf").checked,
      rpc_cookie: $("st-rpc-cookie") && $("st-rpc-cookie").checked,
      webui_tls_local: $("st-webui-tls-local") && $("st-webui-tls-local").checked,
      rpc_tls_local: $("st-rpc-tls-local") && $("st-rpc-tls-local").checked,
      local_tls_trust_ca: $("st-local-tls-trust-ca") && $("st-local-tls-trust-ca").checked,
      mine: $("mine-want") && $("mine-want").checked,
      miningaddress: $("st-mining-addr") ? $("st-mining-addr").value.trim() : "",
      uacomment: $("st-uacomment") ? $("st-uacomment").value.trim() : "",
    });
    if ($("st-uacomment-publish-tip") && $("st-uacomment-publish-tip").checked) {
      if (settingsWalletEnabled()) {
        body.uacomment_use_node_tip = true;
      } else if ($("st-uacomment-tip-addr")) {
        body.uacomment_tip_address = $("st-uacomment-tip-addr").value.trim();
      }
    } else {
      body.uacomment_tip_address = "";
      body.uacomment_use_node_tip = false;
    }
    Object.assign(body, {
      rpc_user: $("st-rpc-user") ? $("st-rpc-user").value.trim() : "",
      rpc_password: $("st-rpc-pass") ? $("st-rpc-pass").value : "",
      core_rpc_addr: $("st-core-rpc") ? $("st-core-rpc").value.trim() : "",
      core_rpc_user: $("st-core-rpc-user") ? $("st-core-rpc-user").value.trim() : "",
      core_rpc_password: $("st-core-rpc-pass") ? $("st-core-rpc-pass").value : "",
      signer_cmd: $("st-signer-cmd") ? $("st-signer-cmd").value.trim() : "",
      zmqpubhashblock: $("st-zmq-hashblock") ? $("st-zmq-hashblock").value.trim() : "",
      zmqpubhashtx: $("st-zmq-hashtx") ? $("st-zmq-hashtx").value.trim() : "",
      zmqpubrawblock: $("st-zmq-rawblock") ? $("st-zmq-rawblock").value.trim() : "",
      zmqpubrawtx: $("st-zmq-rawtx") ? $("st-zmq-rawtx").value.trim() : "",
    });
    if (rawBf === "") delete body.rawblock_backfill;
    else body.rawblock_backfill = parseInt(rawBf, 10);
    if (!isNaN(maxFee) && maxFee > 0) body.maxtxfee = maxFee;
    else delete body.maxtxfee;
    if (!isNaN(maxMem) && maxMem > 0) body.maxmempool = maxMem;
    else delete body.maxmempool;
    if (!isNaN(dbCache) && dbCache > 0) body.dbcache = dbCache;
    else delete body.dbcache;
    if (!isNaN(memExp) && memExp > 0) body.mempoolexpiry = memExp;
    else delete body.mempoolexpiry;
    if (!isNaN(maxOrph) && maxOrph > 0) body.maxorphantx = maxOrph;
    else delete body.maxorphantx;
    if (!isNaN(minRelay) && minRelay > 0) body.minrelaytxfee = minRelay;
    else delete body.minrelaytxfee;
    if (!isNaN(incrRelay) && incrRelay > 0) body.incrementalrelayfee = incrRelay;
    else delete body.incrementalrelayfee;
    if (!isNaN(hardDust) && hardDust > 0) body.harddustlimit = hardDust;
    else delete body.harddustlimit;
    if (!isNaN(blockWt) && blockWt > 0) body.blockmaxweight = blockWt;
    else delete body.blockmaxweight;
    if (!isNaN(limAnc) && limAnc > 0) body.limitancestorcount = limAnc;
    else delete body.limitancestorcount;
    if (!isNaN(limDesc) && limDesc > 0) body.limitdescendantcount = limDesc;
    else delete body.limitdescendantcount;
    if (!isNaN(limAncKb) && limAncKb > 0) body.limitancestorsize = limAncKb;
    else delete body.limitancestorsize;
    if (!isNaN(limDescKb) && limDescKb > 0) body.limitdescendantsize = limDescKb;
    else delete body.limitdescendantsize;
    if (!isNaN(dataSz) && dataSz > 0) body.datacarriersize = dataSz;
    else delete body.datacarriersize;
    if ($("st-accept-datacarrier")) body.acceptdatacarrier = $("st-accept-datacarrier").checked;
    if ($("st-permit-bare-ms")) body.permitbaremultisig = $("st-permit-bare-ms").checked;
    if ($("st-persistmempool")) body.persistmempool = $("st-persistmempool").checked;
    const dgrPort = parseInt($("st-dgr-port") && $("st-dgr-port").value, 10);
    const dgrMaxCl = parseInt($("st-dgr-max-clients") && $("st-dgr-max-clients").value, 10);
    const dgrMaxCn = parseInt($("st-dgr-max-conns") && $("st-dgr-max-conns").value, 10);
    const dgrSeedsRaw = $("st-dgr-seeds") ? $("st-dgr-seeds").value : "";
    const dgrSeeds = dgrSeedsRaw.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const dgrDnsRaw = $("st-dgr-dns") ? $("st-dgr-dns").value : "";
    const dgrDnsHosts = dgrDnsRaw.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const dgrTlsRaw = $("st-dgr-tls-pins") ? $("st-dgr-tls-pins").value : "";
    const dgrTlsPins = dgrTlsRaw.split(/\r?\n/).map((s) => s.trim().toLowerCase()).filter(Boolean);
    const dgrMaxFrames = parseInt($("st-dgr-max-frames") && $("st-dgr-max-frames").value, 10);
    const dgrMaxP2P = parseInt($("st-dgr-max-p2p-proxy") && $("st-dgr-max-p2p-proxy").value, 10);
    const dgrMaxReg = parseInt($("st-dgr-max-register") && $("st-dgr-max-register").value, 10);
    const dgrEnabled = $("st-dgr-enabled") && $("st-dgr-enabled").checked;
    const dgrInbound = $("st-dgr-inbound") && $("st-dgr-inbound").checked;
    const dgrOutbound = $("st-dgr-outbound") && $("st-dgr-outbound").checked;
    body.dogego_relay_cgnat = {
      enabled: dgrEnabled || dgrInbound || dgrOutbound,
      inbound_relay: dgrInbound,
      outbound_relay: dgrOutbound,
      listen: $("st-dgr-listen") ? $("st-dgr-listen").value.trim() : "",
      relay_seeds: dgrSeeds,
      relay_dnsseed: dgrDnsHosts.join("\n"),
      auth_token: $("st-dgr-token") ? $("st-dgr-token").value : "",
    };
    if (dgrTlsPins.length) body.dogego_relay_cgnat.relay_tls_pins = dgrTlsPins;
    if (!isNaN(dgrPort) && dgrPort > 0) body.dogego_relay_cgnat.relay_port = dgrPort;
    if (!isNaN(dgrMaxCl) && dgrMaxCl > 0) body.dogego_relay_cgnat.max_clients = dgrMaxCl;
    if (!isNaN(dgrMaxCn) && dgrMaxCn > 0) body.dogego_relay_cgnat.max_relay_conns = dgrMaxCn;
    if (!isNaN(dgrMaxFrames) && dgrMaxFrames > 0) body.dogego_relay_cgnat.max_session_frames_per_sec = dgrMaxFrames;
    if (!isNaN(dgrMaxP2P) && dgrMaxP2P > 0) body.dogego_relay_cgnat.max_p2p_proxy_per_sec = dgrMaxP2P;
    if (!isNaN(dgrMaxReg) && dgrMaxReg > 0) body.dogego_relay_cgnat.max_register_per_min = dgrMaxReg;
    return body;
  }

  async function loadAutostartStatus() {
    const el = $("st-autostart-status");
    const verifyEl = $("st-autostart-verify");
    const probeLink = $("st-autostart-probe-link");
    if (!el) return;
    try {
      const r = await fetch("/api/autostart", { cache: "no-store" });
      if (!r.ok) {
        el.textContent = "";
        if (verifyEl) verifyEl.hidden = true;
        if (probeLink) probeLink.hidden = true;
        return;
      }
      const data = await r.json();
      const st = data.status || {};
      if (!st.supported) {
        el.textContent = "OS autostart is not available on this platform.";
        if (verifyEl) verifyEl.hidden = true;
        if (probeLink) probeLink.hidden = true;
        return;
      }
      const parts = [];
      if (st.method) parts.push(st.method);
      if (st.installed) parts.push("installed");
      else parts.push("not installed");
      if (st.detail) parts.push(st.detail);
      el.textContent = parts.join(" · ");
      if (verifyEl) {
        if (data.configured) {
          if (data.ok) {
            verifyEl.textContent = (data.verify && data.verify.warnings && data.verify.warnings.length)
              ? "Autostart verify: OK with warnings - run Features → OS login autostart probe"
              : "Autostart verify: OK";
            verifyEl.hidden = false;
          } else {
            verifyEl.textContent = "Autostart verify: failed - save config or re-run setup wizard, then check Features probe";
            verifyEl.hidden = false;
          }
        } else {
          verifyEl.textContent = "autostart=disable in config";
          verifyEl.hidden = false;
        }
      }
      if (probeLink) probeLink.hidden = false;
    } catch (_) {
      el.textContent = "";
      if (verifyEl) verifyEl.hidden = true;
      if (probeLink) probeLink.hidden = true;
    }
  }

  async function loadConfigForm() {
    try {
      const r = await fetch("/api/config", { cache: "no-store" });
      if (!r.ok) return;
      const c = await r.json();
      if ($("st-confpath")) $("st-confpath").textContent = c.path ? "Config: " + c.path : "";
      applyConfigToForm(c.config || c);
      renderSettingsRuntime(c.runtime);
      loadAutostartStatus();
      void loadTLSStatus();
    } catch (_) {}
  }

  function isLoopbackBind(addr) {
    const a = String(addr || "").trim().toLowerCase();
    if (!a) return true;
    if (a.startsWith("0.0.0.0") || a === "::" || a.startsWith("[::]")) return false;
    let host = a;
    if (a.startsWith("[")) {
      const end = a.indexOf("]");
      host = end > 1 ? a.slice(1, end) : a;
    } else if (a.includes(":") && a.includes(".")) {
      host = a.slice(0, a.indexOf(":"));
    } else if (a.includes(":") && a.indexOf(":") === a.lastIndexOf(":")) {
      host = a.slice(0, a.indexOf(":"));
    }
    return host === "127.0.0.1" || host === "localhost" || host === "::1";
  }

  function updateMainnetEncryptionBanners(wal) {
    const need = !!(wal && wal.mainnet_encryption_required);
    ["recv-mainnet-encrypt-banner", "send-mainnet-encrypt-banner"].forEach((id) => {
      const el = $(id);
      if (el) el.hidden = !need;
    });
  }

  function securityChecklistRow(ok, text) {
    const icon = ok ? "check_circle" : "warning";
    const cls = ok ? "ok" : "warn";
    return "<li class=\"" + cls + "\"><span class=\"material-icons-round\" aria-hidden=\"true\">" + icon + "</span><span>" + escapeHtml(text) + "</span></li>";
  }

  function updateSecurityChecklist(wal) {
    const list = $("st-security-checklist");
    if (!list) return;
    const cfg = dogegoSavedConfig || {};
    const tls = lastTLSCache || {};
    const rows = [];
    const onMainnet = String((wal && wal.network) || cfg.network || "").toLowerCase() === "mainnet";
    if (onMainnet && wal && wal.enabled !== false) {
      const encOk = !!wal.encrypted;
      rows.push(securityChecklistRow(encOk, i18n(encOk ? "settings.securityChecklistEncryptOk" : "settings.securityChecklistEncryptWarn")));
    }
    const rpcAuth = !!(cfg.rpc_cookie || (cfg.rpc_user && String(cfg.rpc_user).trim()));
    rows.push(securityChecklistRow(rpcAuth || !String(cfg.rpc || "").trim(), i18n(rpcAuth ? "settings.securityChecklistRpcAuthOk" : "settings.securityChecklistRpcAuthWarn")));
    const rpcLoop = isLoopbackBind(cfg.rpc);
    rows.push(securityChecklistRow(rpcLoop, i18n(rpcLoop ? "settings.securityChecklistRpcLoopOk" : "settings.securityChecklistRpcLoopWarn")));
    const webLoop = isLoopbackBind(cfg.webui);
    rows.push(securityChecklistRow(webLoop, i18n(webLoop ? "settings.securityChecklistWebLoopOk" : "settings.securityChecklistWebLoopWarn")));
    if (!webLoop) {
      const remoteAuth = !!cfg.webui_remote_auth;
      rows.push(securityChecklistRow(remoteAuth, i18n(remoteAuth ? "settings.securityChecklistRemoteAuthOk" : "settings.securityChecklistRemoteAuthWarn")));
    }
    const httpsOn = !!(tls.webui_https || tls.rpc_https);
    rows.push(securityChecklistRow(httpsOn, i18n(httpsOn ? "settings.securityChecklistTlsOk" : "settings.securityChecklistTlsInfo")));
    list.innerHTML = rows.join("");
  }

  function updateDashboardTLSBanner(st) {
    const banner = $("dash-tls-cert-banner");
    if (!banner) return;
    if (location.protocol !== "https:") {
      banner.hidden = true;
      scheduleDashboardBannerStackSync();
      return;
    }
    try {
      if (localStorage.getItem("dogego_tls_cert_banner_dismissed") === "1") {
        banner.hidden = true;
        scheduleDashboardBannerStackSync();
        return;
      }
    } catch (_) { /* */ }
    const tlsOn = !!(st && (st.webui_tls_local || st.webui_https || st.rpc_tls_local || st.rpc_https));
    banner.hidden = !tlsOn;
    scheduleDashboardBannerStackSync();
  }

  function renderTLSStatus(st) {
    lastTLSCache = st || null;
    updateDashboardTLSBanner(st);
    const statusEl = $("st-tls-status");
    const caPathEl = $("st-tls-ca-path");
    const trustBtn = $("st-tls-trust-btn");
    if (!statusEl) return;
    if (!st || (!st.webui_tls_local && !st.rpc_tls_local && !st.webui_https && !st.rpc_https)) {
      statusEl.textContent = i18n("settings.localTlsStatusOff");
      if (caPathEl) caPathEl.hidden = true;
      if (trustBtn) trustBtn.hidden = true;
      updateSecurityChecklist(lastWalletSnap);
      return;
    }
    if (st.local_ca_trusted) {
      statusEl.textContent = i18n("settings.localTlsStatusOn");
    } else {
      statusEl.textContent = i18n("settings.localTlsStatusUntrusted");
    }
    if (caPathEl) {
      if (st.local_ca_path) {
        caPathEl.hidden = false;
        caPathEl.textContent = "CA: " + st.local_ca_path;
      } else {
        caPathEl.hidden = true;
      }
    }
    if (trustBtn) {
      trustBtn.hidden = !st.local_ca_path || !!st.local_ca_trusted;
    }
    updateSecurityChecklist(lastWalletSnap);
  }

  async function loadTLSStatus() {
    try {
      const r = await fetch("/api/tls/status", { credentials: "same-origin", cache: "no-store" });
      if (!r.ok) return;
      renderTLSStatus(await r.json());
    } catch (_) { /* */ }
  }

  async function trustLocalTLS_CA() {
    const msg = $("st-tls-trust-hint");
    const trustBtn = $("st-tls-trust-btn");
    if (trustBtn) trustBtn.disabled = true;
    try {
      const r = await fetch("/api/tls/trust-ca", { method: "POST", credentials: "same-origin" });
      const j = await r.json().catch(() => ({}));
      if (msg) {
        msg.textContent = j.ok ? (j.detail || i18n("settings.localTlsTrustOk")) : (j.error || j.detail || i18n("settings.localTlsTrustFail"));
      }
      await loadTLSStatus();
    } catch (e) {
      if (msg) msg.textContent = i18n("settings.localTlsTrustFail") + ": " + String(e);
    } finally {
      if (trustBtn) trustBtn.disabled = false;
    }
  }

  initSubTabs();
  const mpFilter = $("mp-tx-filter");
  if (mpFilter) {
    mpFilter.addEventListener("input", () => {
      mpTxFilter = mpFilter.value;
      const tbody = $("mp-tx-body");
      if (tbody) {
        tbody.innerHTML = renderMempoolTxRows(mpTxCache, mpTxFilter);
        bindMempoolTxRows();
      }
    });
  }

  function scrollToFeatAnchor(id) {
    const el = $(id);
    if (!el) return;
    requestAnimationFrame(() => el.scrollIntoView({ behavior: "smooth", block: "start" }));
  }

  document.querySelectorAll(".nav-item, .bottom-nav-item, .quick-nav-btn").forEach((b) => {
    b.addEventListener("click", () => {
      if (b.id === "bottom-nav-menu" || b.classList.contains("bottom-nav-menu")) return;
      const tab = b.dataset.tab;
      const scroll = b.dataset.featScroll;
      if (scroll) {
        showTab(tab || "features", { preserveHash: true });
        location.hash = (tab || "features") + "/" + scroll;
        scrollToFeatAnchor(scroll);
        return;
      }
      if (!tab) return;
      if (tab === "extensions" && b.id === "nav-ext-catalog") {
        showExtensionCatalogView();
        showTab(tab, { preserveHash: true });
        return;
      }
      const sub = b.dataset.ovSub || b.dataset.stSub;
      if (sub) {
        showTab(tab);
        activateSubTab(tab === "settings" ? "settings" : "overview", sub);
        location.hash = tab + "/" + sub;
      } else {
        showTab(tab);
      }
    });
  });

  $("rpc-filter") && $("rpc-filter").addEventListener("input", () => {
    if (capabilitiesCache) renderRPCMethods(capabilitiesCache.rpc_methods || []);
  });

  $("nav-backdrop") && $("nav-backdrop").addEventListener("click", () => setNavOpen(false));
  initSidebarCollapse();

  function walletAddrForRPC() {
    const s = lastSummary;
    return (s && s.wallet_address) || "";
  }

  function substituteRPCParams(params) {
    const list = params || [];
    if (!list.length) return list;
    const raw = JSON.stringify(list);
    if (raw.indexOf('"WALLET"') < 0) return list;
    const addr = walletAddrForRPC() || "YOUR_P2PKH_ADDRESS";
    return JSON.parse(raw.replace(/"WALLET"/g, JSON.stringify(addr)));
  }

  function formatRPCParams(params) {
    return JSON.stringify(substituteRPCParams(params || []), null, 2);
  }

  function applyRPCForm(method, params) {
    if ($("rpc-method")) $("rpc-method").value = method || "";
    if ($("rpc-params")) $("rpc-params").value = formatRPCParams(params);
  }

  function applyRPCPreset(preset) {
    let params = preset.params || [];
    if (preset.wallet) params = substituteRPCParams(params);
    applyRPCForm(preset.method, params);
  }

  function openRpcTutorialDoc() {
    showTab("docs", { preserveHash: true });
    docsPathHistory.length = 0;
    void openEmbeddedDoc("docs/RPC_CONSOLE_TUTORIAL.md");
  }

  function applyCookbookEntry(entry) {
    if (!entry || !entry.method) return;
    applyRPCForm(entry.method, entry.params || []);
    const methodEl = $("rpc-method");
    if (methodEl) {
      methodEl.scrollIntoView({ behavior: "smooth", block: "center" });
      methodEl.focus();
    }
  }

  function populateRpcMethodDatalist(entries) {
    const dl = $("rpc-method-list");
    if (!dl) return;
    dl.innerHTML = (entries || []).map((e) =>
      "<option value=\"" + escapeHtml(e.method) + "\">" + escapeHtml(e.summary || e.method) + "</option>"
    ).join("");
  }

  function applyRpcMethodFromCookbook(method) {
    const m = (method || "").trim();
    if (!m || !rpcCookbookCache) return;
    const ent = rpcCookbookCache.find((x) => x.method === m);
    if (ent) applyCookbookEntry(ent);
  }

  function renderRpcCookbookList(entries, filter) {
    const list = $("rpc-cookbook-list");
    const countEl = $("rpc-cookbook-count");
    if (!list) return;
    const q = (filter || "").trim().toLowerCase();
    const rows = (entries || []).filter((e) => {
      if (!q) return true;
      const hay = (e.method + " " + (e.summary || "") + " " + (e.help || "")).toLowerCase();
      return hay.indexOf(q) >= 0;
    });
    if (countEl) {
      countEl.textContent = rows.length + " / " + (entries ? entries.length : 0) + " methods";
    }
    if (!rows.length) {
      list.innerHTML = "<p class=\"label\">No methods match.</p>";
      return;
    }
    list.innerHTML = rows.map((e) => {
      const sum = escapeHtml(e.summary || e.help || "");
      const curl = escapeHtml(e.curl || "");
      const cli = escapeHtml(e.cli || "");
      return "<article class=\"rpc-cookbook-row\">" +
        "<h3>" + escapeHtml(e.method) + "</h3>" +
        (sum ? "<p class=\"label\">" + sum + "</p>" : "") +
        (curl ? "<p class=\"label\">curl</p><pre>" + curl + "</pre>" : "") +
        (cli ? "<p class=\"label\">CLI</p><pre>" + cli + "</pre>" : "") +
        "<div class=\"rpc-cookbook-actions\">" +
        "<button type=\"button\" class=\"btn btn-ghost btn-sm rpc-cookbook-use\" data-method=\"" + escapeHtml(e.method) + "\">Use in Console</button>" +
        "</div></article>";
    }).join("");
    list.querySelectorAll(".rpc-cookbook-use").forEach((btn) => {
      btn.addEventListener("click", () => {
        const m = btn.getAttribute("data-method");
        const ent = (entries || []).find((x) => x.method === m);
        applyCookbookEntry(ent);
      });
    });
  }

  async function loadRpcCookbook(force) {
    const list = $("rpc-cookbook-list");
    if (!list) return;
    if (rpcCookbookCache && !force) {
      populateRpcMethodDatalist(rpcCookbookCache);
      renderRpcCookbookList(rpcCookbookCache, ($("rpc-cookbook-search") && $("rpc-cookbook-search").value) || "");
      return;
    }
    wait(list, "Loading cookbook…", { compact: true });
    try {
      const r = await fetch("/api/rpc/cookbook", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const data = await r.json();
      rpcCookbookCache = data.entries || [];
      populateRpcMethodDatalist(rpcCookbookCache);
      renderRpcCookbookList(rpcCookbookCache, ($("rpc-cookbook-search") && $("rpc-cookbook-search").value) || "");
    } catch (e) {
      list.innerHTML = "<p class=\"label\">Cookbook load failed: " + escapeHtml(String(e.message || e)) + "</p>";
    }
  }

  function initRpcCookbook() {
    $("rpc-open-tutorial-doc") && $("rpc-open-tutorial-doc").addEventListener("click", openRpcTutorialDoc);
    $("rpc-cookbook-search") && $("rpc-cookbook-search").addEventListener("input", () => {
      renderRpcCookbookList(rpcCookbookCache || [], $("rpc-cookbook-search").value);
    });
  }

  const RPC_TOOL_GROUPS = [
    { id: "wallet", label: "Wallet", match: (m) => /^(getwallet|wallet|encrypt|listaddress|getaddress|sendto|dump|import|sign|keypool|fundraw|bump|psbt|setlabel|getaccount|lockunspent|listunspent|getreceived|listlabels|abandon)/.test(m) },
    { id: "chain", label: "Chain & blocks", match: (m) => /^get(block|raw|txout|chaint|best|difficulty|mining|network)/.test(m) || ["validateaddress", "preciousblock", "invalidateblock", "reconsiderblock", "waitforblock", "waitforblockheight", "scanblocks", "gettxoutproof", "verifytxoutproof"].indexOf(m) >= 0 },
    { id: "mining", label: "Mining", match: (m) => /^(getmining|generate|submitblock|getblocktemplate|createaux|getaux|prioritisetransaction)/.test(m) },
    { id: "mempool", label: "Mempool & transactions", match: (m) => /^(sendraw|testmempool|getrawmempool|getmempool|submitpackage|decoderaw|createraw)/.test(m) },
    { id: "peers", label: "Peers & network", match: (m) => /^(getpeer|addnode|disconnect|getconnection|getnetwork|setban|listbanned|ping|getnode|help)/.test(m) },
    { id: "maintenance", label: "Maintenance & index", match: (m) => /^(reindex|saveutxo|prune|stop|uptime|getindex|verifychain|loadtxoutset)/.test(m) },
  ];

  function rpcToolGroupForMethod(method) {
    for (const g of RPC_TOOL_GROUPS) {
      if (g.match(method)) return g.id;
    }
    return "other";
  }

  function renderSettingsToolsPanel(entries, filter) {
    const root = $("st-tools-groups");
    const countEl = $("st-tools-count");
    if (!root) return;
    const q = (filter || "").trim().toLowerCase();
    const filtered = (entries || []).filter((e) => {
      if (!q) return true;
      const hay = (e.method + " " + (e.summary || "") + " " + (e.help || "")).toLowerCase();
      return hay.indexOf(q) >= 0;
    });
    if (countEl) countEl.textContent = filtered.length + " method(s)";
    if (!filtered.length) {
      root.innerHTML = "<p class=\"label\">No methods match.</p>";
      return;
    }
    const buckets = {};
    RPC_TOOL_GROUPS.forEach((g) => { buckets[g.id] = { meta: g, items: [] }; });
    buckets.other = { meta: { id: "other", label: "Other" }, items: [] };
    filtered.forEach((e) => {
      const gid = rpcToolGroupForMethod(e.method);
      (buckets[gid] || buckets.other).items.push(e);
    });
    root.innerHTML = RPC_TOOL_GROUPS.concat([{ id: "other", label: "Other" }]).map((g) => {
      const bucket = buckets[g.id];
      if (!bucket || !bucket.items.length) return "";
      const rows = bucket.items.map((e) => {
        const paramsJson = escapeHtml(formatRPCParams(e.params || []));
        const sum = escapeHtml(e.summary || e.help || "");
        return "<article class=\"rpc-tool-row\" data-method=\"" + escapeHtml(e.method) + "\">" +
          "<div class=\"rpc-tool-head\"><strong>" + escapeHtml(e.method) + "</strong>" +
          (sum ? "<span class=\"label\">" + sum + "</span>" : "") + "</div>" +
          "<label class=\"label\">Parameters (JSON array)</label>" +
          "<textarea class=\"rpc-tool-params mono\" rows=\"2\">" + paramsJson + "</textarea>" +
          "<div class=\"settings-actions settings-actions-inline\">" +
          "<button type=\"button\" class=\"btn btn-primary btn-sm rpc-tool-run\">Run</button>" +
          "<button type=\"button\" class=\"btn btn-ghost btn-sm rpc-tool-console\">Open in Console</button>" +
          "</div>" +
          "<pre class=\"pre-out rpc-tool-result\" aria-live=\"polite\"></pre>" +
          "</article>";
      }).join("");
      return "<details class=\"rpc-tool-group\" open><summary><strong>" + escapeHtml(bucket.meta.label) + "</strong> <span class=\"label\">(" + bucket.items.length + ")</span></summary>" +
        "<div class=\"rpc-tool-group-body\">" + rows + "</div></details>";
    }).join("");
    root.querySelectorAll(".rpc-tool-run").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const row = btn.closest(".rpc-tool-row");
        const method = row && row.dataset.method;
        const ta = row && row.querySelector(".rpc-tool-params");
        const resultEl = row && row.querySelector(".rpc-tool-result");
        if (!method || !ta) return;
        let params;
        try {
          params = JSON.parse(ta.value.trim() || "[]");
          if (!Array.isArray(params)) throw new Error("params must be a JSON array");
        } catch (e) {
          if (resultEl) {
            resultEl.textContent = String(e);
            resultEl.classList.add("show", "rpc-tool-result-err");
          }
          return;
        }
        await runSettingsRPC(method, params, null, resultEl, "Running " + method + "…");
      });
    });
    root.querySelectorAll(".rpc-tool-console").forEach((btn) => {
      btn.addEventListener("click", () => {
        const row = btn.closest(".rpc-tool-row");
        const method = row && row.dataset.method;
        const ta = row && row.querySelector(".rpc-tool-params");
        if (!method) return;
        let params = [];
        try { params = JSON.parse((ta && ta.value.trim()) || "[]"); } catch (_) {}
        applyRPCForm(method, params);
        showTab("console");
      });
    });
  }

  const extNotices = [];

  function clearExtWarnNotices() {
    for (let i = extNotices.length - 1; i >= 0; i--) {
      if (extNotices[i].kind === "warn") extNotices.splice(i, 1);
    }
    renderExtNotices();
  }

  function pushExtNotice(kind, title, detail) {
    const id = "extn-" + Date.now() + "-" + Math.random().toString(36).slice(2, 8);
    if (kind === "ok" || kind === "err") {
      // Drop in-progress warn rows so success/error replaces the spinner notice.
      for (let i = extNotices.length - 1; i >= 0; i--) {
        if (extNotices[i].kind === "warn") extNotices.splice(i, 1);
      }
    }
    extNotices.unshift({ id, kind, title, detail: detail || "" });
    while (extNotices.length > 4) extNotices.pop();
    renderExtNotices();
    if (kind === "ok") {
      setTimeout(() => dismissExtNotice(id), 3500);
    } else if (kind === "err") {
      setTimeout(() => dismissExtNotice(id), 12000);
    }
  }

  function dismissExtNotice(id) {
    const i = extNotices.findIndex((n) => n.id === id);
    if (i >= 0) extNotices.splice(i, 1);
    renderExtNotices();
  }

  function renderExtNotices() {
    const bar = $("ext-notice-bar");
    if (!bar) return;
    if (!extNotices.length) {
      bar.hidden = true;
      bar.innerHTML = "";
      return;
    }
    bar.hidden = false;
    bar.innerHTML = extNotices.map((n) => {
      const cls = n.kind === "ok" ? "ok" : n.kind === "warn" ? "warn" : "err";
      const icon = n.kind === "ok" ? "check_circle" : n.kind === "warn" ? "hourglass_top" : "error";
      return '<div class="ext-notice-row ' + cls + '" data-notice-id="' + escapeHtml(n.id) + '">' +
        '<span class="material-icons-round ext-notice-icon" aria-hidden="true">' + icon + "</span>" +
        '<div class="ext-notice-text"><strong>' + escapeHtml(n.title) + "</strong>" +
        (n.detail ? '<span class="ext-notice-detail">' + escapeHtml(n.detail) + "</span>" : "") +
        "</div>" +
        '<button type="button" class="ext-notice-dismiss" data-notice-id="' + escapeHtml(n.id) + '" aria-label="Dismiss">' +
        '<span class="material-icons-round">close</span></button></div>';
    }).join("");
  }

  function extRpcFullName(extId, inner) {
    return "dogego_ext_" + String(extId || "").replace(/\./g, "_") + "_" + inner;
  }

  function extToolsFromRpcMethods(ext) {
    const prefix = "dogego_ext_" + String(ext.id || "").replace(/\./g, "_") + "_";
    const skip = { info: 1, ui_status: 1 };
    return (ext.rpc_methods || []).map((full) => {
      const inner = full.indexOf(prefix) === 0 ? full.slice(prefix.length) : full.split("_").pop();
      if (!inner || skip[inner]) return null;
      return { id: inner, label: inner.replace(/_/g, " "), method: inner, icon: "play_arrow" };
    }).filter(Boolean);
  }

  async function runExtensionRPC(ext, method, params, resultEl) {
    const extId = ext && ext.id;
    if (!extId || !method) return;
    const full = extRpcFullName(extId, method);
    if (resultEl) resultEl.textContent = "Running " + full + "…";
    try {
      const r = await fetch("/api/rpc", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ method: full, params: params || [] }),
      });
      const body = await r.json().catch(() => ({}));
      if (body.error) throw new Error(typeof body.error === "string" ? body.error : (body.error.message || JSON.stringify(body.error)));
      const out = body.result != null ? body.result : body;
      const text = typeof out === "string" ? out : JSON.stringify(out, null, 2);
      if (resultEl) resultEl.textContent = text;
      return out;
    } catch (e) {
      if (resultEl) resultEl.textContent = String(e.message || e);
      throw e;
    }
  }

  function isExtToggleField(field, el) {
    const t = (field && field.type) || "";
    if (t === "checkbox" || t === "switch") return true;
    return !!(el && el.type === "checkbox");
  }

  async function runExtensionTool(ext, tool, form) {
    const extId = ext && ext.id;
    if (!extId || !tool || !tool.method) return;
    const params = Array.isArray(tool.params) ? tool.params.slice() : [];
    if (!params.length) {
      const jsonFields = { proof_json: 1, payload: 1, payload_json: 1, backup_json: 1 };
      const asObject = tool.params_as === "object" || tool.method === "setconfig" || tool.method === "inscribe" || tool.method === "putasset" || tool.method === "importbackup";
      const obj = {};
      (tool.fields || []).forEach((field) => {
        const el = form.querySelector("[name=\"" + field.name + "\"]");
        let raw = "";
        let boolVal = false;
        if (el) {
          if (isExtToggleField(field, el)) {
            boolVal = !!el.checked;
            raw = boolVal ? "true" : "false";
          } else {
            raw = (el.value || "").trim();
          }
        }
        if (jsonFields[field.name]) {
          if (!raw) return;
          try {
            const parsed = JSON.parse(raw);
            if (asObject) obj[field.name] = parsed;
            else params.push(parsed);
          } catch (_) {
            throw new Error("Invalid JSON in " + (field.label || field.name));
          }
          return;
        }
        if (isExtToggleField(field, el)) {
          if (asObject) obj[field.name] = boolVal;
          else params.push(boolVal);
          return;
        }
        if (field.type === "number") {
          let n = 0;
          if (raw === "" && field.default != null) n = Number(field.default);
          else if (raw !== "") n = Number(raw);
          if (asObject) obj[field.name] = n;
          else params.push(n);
          return;
        }
        if (asObject) obj[field.name] = raw;
        else params.push(raw);
      });
      if (asObject) params.push(obj);
    }
    const resultEl = form.querySelector(".ext-tool-result") || form.closest(".ext-console") && form.closest(".ext-console").querySelector(".ext-console-output");
    await runExtensionRPC(ext, tool.method, params, resultEl);
  }

  function collectExtensionFields(host, fields) {
    const obj = {};
    (fields || []).forEach((field) => {
      const el = host.querySelector("[name=\"" + field.name + "\"]");
      if (!el) return;
      if (isExtToggleField(field, el)) {
        obj[field.name] = !!el.checked;
        return;
      }
      const raw = (el.value || "").trim();
      if (field.type === "number") {
        obj[field.name] = raw === "" ? (field.default != null ? Number(field.default) : 0) : Number(raw);
        return;
      }
      obj[field.name] = raw;
    });
    return obj;
  }

  function renderExtensionField(field) {
    const wrap = document.createElement("div");
    wrap.className = "ext-tool-field";
    const lab = document.createElement("label");
    lab.textContent = field.label || field.name;
    let input;
    if (field.type === "textarea") {
      input = document.createElement("textarea");
      input.rows = field.rows || 4;
    } else if (field.type === "select") {
      wrap.className = "ext-tool-field ext-choice-field";
      const title = document.createElement("div");
      title.className = "ext-choice-label";
      title.textContent = field.label || field.name;
      const hidden = document.createElement("input");
      hidden.type = "hidden";
      hidden.name = field.name;
      hidden.className = "ext-choice-value";
      const opts = field.options || [];
      let initial = field.default != null ? String(field.default) : "";
      if (!initial && opts.length) {
        const first = opts[0];
        initial = String(first.value != null ? first.value : first);
      }
      hidden.value = initial;
      const grid = document.createElement("div");
      grid.className = "ext-choice-grid";
      grid.setAttribute("role", "radiogroup");
      grid.setAttribute("aria-label", field.label || field.name);
      opts.forEach((opt) => {
        const value = String(opt.value != null ? opt.value : opt);
        const label = opt.label != null ? String(opt.label) : value;
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "ext-choice-option" + (value === initial ? " active" : "");
        btn.dataset.value = value;
        btn.setAttribute("role", "radio");
        btn.setAttribute("aria-checked", value === initial ? "true" : "false");
        if (opt.icon) {
          btn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(opt.icon) + "</span><span class=\"ext-choice-option-text\">" + escapeHtml(label) + "</span>";
        } else {
          btn.innerHTML = "<span class=\"ext-choice-option-text\">" + escapeHtml(label) + "</span>";
        }
        btn.addEventListener("click", () => {
          hidden.value = value;
          grid.querySelectorAll(".ext-choice-option").forEach((b) => {
            const on = b.dataset.value === value;
            b.classList.toggle("active", on);
            b.setAttribute("aria-checked", on ? "true" : "false");
          });
        });
        grid.appendChild(btn);
      });
      wrap.append(title, hidden, grid);
      if (field.hint) {
        const hint = document.createElement("p");
        hint.className = "field-hint";
        hint.textContent = field.hint;
        wrap.appendChild(hint);
      }
      return wrap;
    } else if (field.type === "checkbox" || field.type === "switch") {
      wrap.className = "ext-tool-field ext-switch-field";
      input = document.createElement("input");
      input.type = "checkbox";
      input.className = "ext-switch-input";
      const def = field.default;
      input.checked = def === true || def === "true" || def === "1";
      input.name = field.name;
      const sw = document.createElement("label");
      sw.className = "ext-switch";
      const track = document.createElement("span");
      track.className = "ext-switch-track";
      track.setAttribute("aria-hidden", "true");
      track.innerHTML = "<span class=\"ext-switch-thumb\"></span>";
      const text = document.createElement("span");
      text.className = "ext-switch-label";
      text.textContent = field.label || field.name;
      sw.append(input, track, text);
      wrap.appendChild(sw);
      if (field.hint) {
        const hint = document.createElement("p");
        hint.className = "field-hint";
        hint.textContent = field.hint;
        wrap.appendChild(hint);
      }
      return wrap;
    } else {
      input = document.createElement("input");
      input.type = field.type === "number" ? "number" : "text";
      input.className = "ext-modern-input";
    }
    input.name = field.name;
    if (field.placeholder && input.placeholder !== undefined) input.placeholder = field.placeholder;
    if (field.default != null) input.value = field.default;
    if (field.type === "textarea" || field.type === "text" || field.type === "number" || !field.type) {
      input.classList.add("ext-modern-input");
    }
    lab.appendChild(input);
    wrap.appendChild(lab);
    if (field.hint) {
      const hint = document.createElement("p");
      hint.className = "field-hint";
      hint.textContent = field.hint;
      wrap.appendChild(hint);
    }
    return wrap;
  }

  function syncExtensionChoiceField(host, fieldName, value) {
    if (!host || !fieldName) return;
    const hidden = host.querySelector("input.ext-choice-value[name=\"" + fieldName + "\"]");
    if (!hidden) return;
    const v = value != null ? String(value) : "";
    hidden.value = v;
    const grid = hidden.parentElement && hidden.parentElement.querySelector(".ext-choice-grid");
    if (!grid) return;
    grid.querySelectorAll(".ext-choice-option").forEach((b) => {
      const on = b.dataset.value === v;
      b.classList.toggle("active", on);
      b.setAttribute("aria-checked", on ? "true" : "false");
    });
  }

  function renderExtensionQuickActions(container, ext, actions, consoleOut) {
    if (!container || !actions || !actions.length) return;
    const row = document.createElement("div");
    row.className = "ext-quick-actions";
    actions.forEach((action) => {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "btn btn-ghost btn-sm ext-quick-action";
      btn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(action.icon || "play_arrow") + "</span> " + escapeHtml(action.label || action.method);
      btn.addEventListener("click", () => {
        void runExtensionRPC(ext, action.method, action.params || [], consoleOut).then((out) => {
          if (action.method === "info" && out && out.ui && container.closest(".ext-detail-page")) {
            const dashHost = container.closest(".ext-detail-page").querySelector(".ext-panel-dash-host");
            if (dashHost) renderExtensionDashboard(dashHost, ext, out);
          }
        }).catch(() => {});
      });
      row.appendChild(btn);
    });
    container.appendChild(row);
  }

  function renderExtensionCommandPicker(container, ext, tools, consoleOut) {
    if (!container || !ext || !tools || !tools.length) return;
    const wrap = document.createElement("div");
    wrap.className = "ext-command-picker";
    const sel = document.createElement("select");
    sel.className = "ext-command-select";
    sel.innerHTML = "<option value=\"\">Select RPC command…</option>";
    tools.forEach((tool, i) => {
      const opt = document.createElement("option");
      opt.value = String(i);
      opt.textContent = tool.label || tool.method;
      sel.appendChild(opt);
    });
    const run = document.createElement("button");
    run.type = "button";
    run.className = "btn btn-primary btn-sm";
    run.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">play_arrow</span> Run";
    run.addEventListener("click", () => {
      const idx = sel.value;
      if (idx === "") return;
      const tool = tools[Number(idx)];
      const card = container.parentElement && container.parentElement.querySelector("[data-tool-id=\"" + (tool.id || tool.method) + "\"]");
      if (card) {
        void runExtensionTool(ext, tool, card).catch(() => {});
      } else {
        void runExtensionRPC(ext, tool.method, tool.params || [], consoleOut).catch(() => {});
      }
    });
    wrap.append(sel, run);
    container.appendChild(wrap);
  }

  function renderExtensionTools(container, ext, tools, panelUi) {
    if (!container) return;
    container.innerHTML = "";
    if (!ext || !ext.enabled) {
      container.hidden = true;
      return;
    }
    let list = tools && tools.length ? tools : extToolsFromRpcMethods(ext);
    const ui = panelUi || {};
    if (!list.length && !(ui.quick_actions || []).length) {
      container.hidden = true;
      return;
    }
    container.hidden = false;
    const consoleBox = document.createElement("div");
    consoleBox.className = "ext-console card";
    const consoleHead = document.createElement("div");
    consoleHead.className = "ext-console-head";
    consoleHead.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">terminal</span> <strong>Extension console</strong>";
    const consoleOut = document.createElement("pre");
    consoleOut.className = "ext-console-output mono-sm";
    consoleOut.textContent = "Run a quick action or select a command below. Results appear here.";
    consoleBox.append(consoleHead, consoleOut);
    if ((ui.quick_actions || []).length) {
      renderExtensionQuickActions(consoleBox, ext, ui.quick_actions, consoleOut);
    }
    if (list.length) {
      renderExtensionCommandPicker(consoleBox, ext, list, consoleOut);
    }
    container.appendChild(consoleBox);
    if (!list.length) return;
    const title = document.createElement("p");
    title.className = "label ext-tools-title";
    title.textContent = "Command forms";
    container.appendChild(title);
    list.forEach((tool) => {
      const card = document.createElement("div");
      card.className = "ext-tool-card";
      card.dataset.toolId = tool.id || tool.method || "";
      const head = document.createElement("div");
      head.className = "ext-tool-head";
      head.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(tool.icon || "bolt") + "</span> " + escapeHtml(tool.label || tool.method);
      card.appendChild(head);
      if (tool.hint) {
        const hint = document.createElement("p");
        hint.className = "field-hint";
        hint.textContent = tool.hint;
        card.appendChild(hint);
      }
      const fields = document.createElement("div");
      fields.className = "ext-tool-fields";
      if ((tool.fields || []).length) {
        tool.fields.forEach((field) => {
          fields.appendChild(renderExtensionField(field));
        });
      }
      card.appendChild(fields);
      const run = document.createElement("button");
      run.type = "button";
      run.className = "btn btn-primary btn-sm";
      run.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">play_arrow</span> Run";
      const pre = document.createElement("pre");
      pre.className = "ext-tool-result mono-sm";
      run.addEventListener("click", () => {
        void runExtensionTool(ext, tool, card).then((out) => {
          if (consoleOut && pre.textContent) consoleOut.textContent = pre.textContent;
        }).catch(() => {
          if (consoleOut && pre.textContent) consoleOut.textContent = pre.textContent;
        });
      });
      card.append(run, pre);
      container.appendChild(card);
    });
  }

  const EXT_PERM_META = {
    chain_read: { label: "Chain", icon: "link" },
    chain_index: { label: "Indexer", icon: "storage" },
    datadir_write: { label: "Data", icon: "folder" },
    rpc_register: { label: "RPC", icon: "terminal" },
    ui_panel: { label: "Panel", icon: "dashboard" },
    p2p_extension: { label: "P2P", icon: "hub" },
    wallet_rpc: { label: "Wallet RPC", icon: "account_balance_wallet" },
  };

  function extIconUrl(ext) {
    if (ext && ext.id) return "/api/extensions/icon?id=" + encodeURIComponent(ext.id);
    return "/api/extensions/icon?id=" + encodeURIComponent("dogego.zkl2");
  }

  function extStateLabel(ext) {
    if (ext.enabled) return { text: "Enabled", cls: "on", icon: "check_circle" };
    if (ext.installed) return { text: "Installed", cls: "idle", icon: "inventory_2" };
    return { text: "Available", cls: "off", icon: "cloud_download" };
  }

  function extBadgeRow(items, kind) {
    const row = document.createElement("div");
    row.className = "ext-badges";
    (items || []).forEach((item) => {
      const meta = kind === "perm" ? (EXT_PERM_META[item] || { label: item, icon: "extension" }) : { label: item, icon: "bolt" };
      const chip = document.createElement("span");
      chip.className = "ext-badge";
      chip.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + meta.icon + "</span> " + escapeHtml(meta.label);
      row.appendChild(chip);
    });
    return row;
  }

  async function fetchWithTimeout(url, options, ms) {
    const timeoutMs = ms || 12000;
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), timeoutMs);
    try {
      return await fetch(url, Object.assign({}, options || {}, { signal: ctrl.signal }));
    } catch (e) {
      if (e && e.name === "AbortError") throw new Error("Request timed out after " + timeoutMs + "ms");
      throw e;
    } finally {
      clearTimeout(timer);
    }
  }

  async function renderMarkdownInto(el, md, basePath) {
    if (!el) return;
    el.innerHTML = parseDocsMarkdown(md || "");
    renderDocsMath(el);
    bindMarkdownLinksIn(el, basePath, async (base, href) => {
      if (href.startsWith("#")) {
        const id = href.replace(/^#/, "");
        const esc = typeof CSS !== "undefined" && CSS.escape ? CSS.escape(id) : id.replace(/[^a-zA-Z0-9_-]/g, "");
        const target = el.querySelector("#" + esc) || el.querySelector("[id=\"" + id + "\"]");
        if (target) target.scrollIntoView({ behavior: "smooth", block: "start" });
        return;
      }
      try {
        const r = await fetch("/api/docs/resolve?base=" + encodeURIComponent(base) + "&href=" + encodeURIComponent(href), { cache: "no-store" });
        const data = await r.json().catch(() => ({}));
        if (!r.ok) throw new Error(data.error || "HTTP " + r.status);
        if (data.external) {
          window.open(href, "_blank", "noopener,noreferrer");
          return;
        }
        showTab("docs", { preserveHash: true });
        docsPathHistory.length = 0;
        await openEmbeddedDoc(data.path || "", data.anchor || "");
      } catch (e) {
        el.insertAdjacentHTML("afterbegin",
          "<p class=\"label err-inline\">Could not open link: " + escapeHtml(e.message || String(e)) + "</p>");
      }
    });
  }

  async function loadExtensionDocsInto(el, ext) {
    if (!el) return;
    const extId = ext && ext.id;
    const docsPath = ext && ext.docs_path;
    if (!extId && !docsPath) {
      el.innerHTML = "<p class=\"field-hint\">No documentation available for this extension.</p>";
      return;
    }
    el.innerHTML = "<p class=\"label\">Loading documentation…</p>";
    try {
      const q = extId
        ? ("id=" + encodeURIComponent(extId))
        : ("path=" + encodeURIComponent(docsPath));
      const r = await fetchWithTimeout("/api/extensions/docs?" + q, { cache: "no-store", credentials: "same-origin" });
      const body = await r.json().catch(() => ({}));
      if (!r.ok || body.error) throw new Error(body.error || ("HTTP " + r.status));
      await renderMarkdownInto(el, body.markdown || "", body.path || docsPath || "");
    } catch (e) {
      el.innerHTML = "<p class=\"label\">" + escapeHtml(String(e.message || e)) + "</p>";
    }
  }

  function unwrapExtensionPanelInfo(body) {
    let info = body && body.result != null ? body.result : body;
    // JSON-RPC / manager wrappers may nest { result: { result: { ui } } }.
    for (let i = 0; i < 4; i++) {
      if (!info || typeof info !== "object" || Array.isArray(info)) break;
      if (info.ui && typeof info.ui === "object") break;
      if (info.result && typeof info.result === "object" && !Array.isArray(info.result)) {
        info = info.result;
        continue;
      }
      break;
    }
    return info;
  }

  function extensionOffersUIPanel(ext) {
    if (!ext) return false;
    if (ext.ui_panel) return true;
    const perms = ext.permissions || [];
    const caps = ext.capabilities || [];
    return perms.indexOf("ui_panel") >= 0 || caps.indexOf("ui_panel") >= 0;
  }

  async function loadExtensionPanelInto(dashEl, advancedEl, toolsEl, ext) {
    if (!dashEl || !ext || !ext.id) return null;
    dashEl.innerHTML = "<p class=\"label\">Loading panel…</p>";
    if (advancedEl) advancedEl.textContent = "";
    try {
      const r = await fetchWithTimeout("/api/extensions/panel?id=" + encodeURIComponent(ext.id), { credentials: "same-origin", cache: "no-store" }, 15000);
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(extensionApiError(body) || ("HTTP " + r.status));
      if (body.error) throw new Error(extensionApiError(body));
      const info = unwrapExtensionPanelInfo(body);
      if (!info || typeof info !== "object") throw new Error("Empty panel response");
      renderExtensionDashboard(dashEl, ext, info);
      if (advancedEl) advancedEl.textContent = JSON.stringify(info, null, 2);
      if (toolsEl) {
        toolsEl.hidden = true;
        toolsEl.innerHTML = "";
      }
      return info;
    } catch (e) {
      // Still show a Menu shell so ui_panel extensions are usable if status RPC fails.
      const fallbackInfo = {
        ui: {
          panel_title: (ext && ext.name) || (ext && ext.id) || "Extension",
          subtitle: "Panel RPC unavailable: " + String(e.message || e) + ". Menu below uses declared RPC methods.",
        },
      };
      try {
        renderExtensionDashboard(dashEl, ext, fallbackInfo);
      } catch (e2) {
        dashEl.innerHTML = "<p class=\"label\">Panel unavailable: " + escapeHtml(String(e.message || e)) + "</p>";
      }
      if (advancedEl) advancedEl.textContent = String(e.message || e);
      if (toolsEl) {
        toolsEl.hidden = true;
        toolsEl.innerHTML = "";
      }
      return null;
    }
  }

  function extDetailDisclosure(title, open) {
    const det = document.createElement("details");
    det.className = "ui-disclosure ext-detail-disclosure";
    if (open) det.open = true;
    const sum = document.createElement("summary");
    sum.innerHTML = "<span class=\"material-icons-round disclosure-chevron\" aria-hidden=\"true\">expand_more</span> " + escapeHtml(title);
    const body = document.createElement("div");
    body.className = "ui-disclosure-body";
    det.append(sum, body);
    return { det, body };
  }

  function renderExtStatusChips(host, chips) {
    if (!host || !chips || !chips.length) return;
    const row = document.createElement("div");
    row.className = "ext-status-chips";
    chips.forEach((chip) => {
      const el = document.createElement("div");
      el.className = "ext-status-chip tone-" + escapeHtml(chip.tone || "neutral");
      const icon = chip.icon ? "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(chip.icon) + "</span>" : "";
      el.innerHTML = icon +
        "<span class=\"ext-status-chip-body\"><span class=\"ext-status-chip-label\">" + escapeHtml(chip.label || "") + "</span>" +
        "<span class=\"ext-status-chip-value\">" + escapeHtml(chip.value != null ? String(chip.value) : "") + "</span></span>";
      row.appendChild(el);
    });
    host.appendChild(row);
  }

  function renderExtensionDashboard(host, ext, info) {
    if (!host) return;
    host.innerHTML = "";
    let ui = (info && info.ui && typeof info.ui === "object" && !Array.isArray(info.ui)) ? info.ui : {};
    const dash = document.createElement("div");
    dash.className = "ext-dash";
    if (ui.panel_title) {
      const title = document.createElement("h3");
      title.className = "ext-detail-section-title";
      title.textContent = ui.panel_title;
      dash.appendChild(title);
    }
    if (ui.subtitle) {
      const sub = document.createElement("p");
      sub.className = "ext-dash-subtitle";
      sub.textContent = ui.subtitle;
      dash.appendChild(sub);
    } else if (ui.summary) {
      const lead = document.createElement("p");
      lead.className = "ext-dash-summary";
      lead.textContent = ui.summary;
      dash.appendChild(lead);
    }
    if (ui.status_chips) renderExtStatusChips(dash, ui.status_chips);

    ui = ensureExtensionWorkspaceUI(ui, ext);
    const hasNav = Array.isArray(ui.nav) && ui.nav.length > 0;
    const hasSections = ui.sections && typeof ui.sections === "object" && Object.keys(ui.sections).length > 0;
    if (ui && (hasNav || hasSections)) {
      renderExtensionWorkspace(dash, ext, info, ui);
      host.appendChild(dash);
      return;
    }

    (ui.widgets || []).forEach((w) => renderExtWidget(dash, w, ext));
    if (!ui.widgets || !ui.widgets.length) {
      const fallback = document.createElement("pre");
      fallback.className = "mono-sm ext-panel-json";
      fallback.textContent = JSON.stringify(info, null, 2);
      dash.appendChild(fallback);
    }
    host.appendChild(dash);
  }

  // Promote flat panels to the same Menu + sections shell. Extensions never inject HTML;
  // the host only renders allowlisted JSON fields as escaped text / known widgets.
  function ensureExtensionWorkspaceUI(ui, ext) {
    if (!ui || typeof ui !== "object" || Array.isArray(ui)) return {};
    const hasNav = Array.isArray(ui.nav) && ui.nav.length > 0;
    const hasSections = ui.sections && typeof ui.sections === "object" && !Array.isArray(ui.sections) && Object.keys(ui.sections).length > 0;
    if (hasNav || hasSections) {
      const out = Object.assign({}, ui);
      if (!Array.isArray(out.nav) && hasSections) {
        out.nav = Object.keys(ui.sections).map((id) => ({
          id: id,
          label: (ui.sections[id] && ui.sections[id].title) || id,
          icon: "extension",
        }));
      }
      if (!out.layout) out.layout = "workspace";
      return out;
    }
    const sections = {};
    const nav = [];
    sections.home = {
      title: "Overview",
      lead: ui.subtitle || ui.summary || "Status at a glance. Use the menu for tools.",
      widgets: Array.isArray(ui.widgets) ? ui.widgets : [],
      quick_actions: Array.isArray(ui.quick_actions) ? ui.quick_actions : [
        { id: "refresh", label: "Refresh", method: "info", icon: "refresh" },
      ],
    };
    nav.push({ id: "home", label: "Home", icon: "home" });
    if (Array.isArray(ui.tools) && ui.tools.length) {
      sections.tools = {
        title: "Tools",
        lead: "Host-rendered forms (safe text fields only).",
        tools: ui.tools,
      };
      nav.push({ id: "tools", label: "Tools", icon: "construction" });
    } else if (ext && (ext.rpc_methods || []).length) {
      const derived = extToolsFromRpcMethods(ext).filter((t) => t.method !== "info" && t.method !== "ui_status");
      if (derived.length) {
        sections.tools = {
          title: "Tools",
          lead: "Generated from this extension's RPC methods.",
          tools: derived,
        };
        nav.push({ id: "tools", label: "Tools", icon: "construction" });
      }
    }
    sections.settings = {
      title: "Settings & security",
      lead: "Extensions add WebUI only through this JSON panel (no HTML/JS). Optional wallet_rpc uses allowlisted methods after dashboard unlock. Put custom toggles in your own Settings section.",
      quick_actions: [
        { id: "refresh", label: "Refresh", method: "info", icon: "refresh" },
      ],
    };
    nav.push({ id: "settings", label: "Settings", icon: "tune" });
    return Object.assign({}, ui, {
      layout: "workspace",
      nav: nav,
      sections: sections,
      panel_title: ui.panel_title || (ext && (ext.name || ext.id)) || "Extension",
    });
  }

  function renderExtWidget(host, w, ext) {
    if (!w || !w.type) return;
    // Allowlist only - never render extension-supplied HTML.
    if (w.type === "stats") renderExtStatsWidget(host, w);
    else if (w.type === "proof_list") renderExtProofListWidget(host, w, ext);
    else if (w.type === "item_list") renderExtItemListWidget(host, w);
    else if (w.type === "table") renderExtTableWidget(host, w, ext);
    else if (w.type === "metric_chart") renderExtMetricChartWidget(host, w);
    else if (w.type === "callout") renderExtCalloutWidget(host, w);
    else if (w.type === "progress") renderExtProgressWidget(host, w);
  }

  function renderExtCalloutWidget(host, widget) {
    const box = document.createElement("div");
    box.className = "ext-callout tone-" + escapeHtml(widget.tone || "neutral");
    box.innerHTML =
      "<span class=\"material-icons-round ext-callout-icon\" aria-hidden=\"true\">" + escapeHtml(widget.icon || "info") + "</span>" +
      "<div class=\"ext-callout-body\">" +
      (widget.title ? "<strong class=\"ext-callout-title\">" + escapeHtml(widget.title) + "</strong>" : "") +
      (widget.body ? "<p class=\"ext-callout-text\">" + escapeHtml(widget.body) + "</p>" : "") +
      "</div>";
    host.appendChild(box);
  }

  function renderExtProgressWidget(host, widget) {
    const wrap = document.createElement("div");
    wrap.className = "ext-progress";
    const label = document.createElement("div");
    label.className = "ext-progress-label";
    const pct = Math.max(0, Math.min(100, Number(widget.percent != null ? widget.percent : 0) || 0));
    label.innerHTML = "<span>" + escapeHtml(widget.label || "Progress") + "</span><span>" + escapeHtml(widget.value != null ? String(widget.value) : (Math.round(pct) + "%")) + "</span>";
    const track = document.createElement("div");
    track.className = "ext-progress-track";
    const fill = document.createElement("div");
    fill.className = "ext-progress-fill";
    fill.style.width = pct + "%";
    track.appendChild(fill);
    wrap.append(label, track);
    host.appendChild(wrap);
  }

  function renderExtMetricChartWidget(host, widget) {
    const wrap = document.createElement("div");
    wrap.className = "ext-metric-chart card";
    if (widget.title) {
      const h = document.createElement("h4");
      h.className = "ext-proof-list-title";
      h.textContent = widget.title;
      wrap.appendChild(h);
    }
    if (widget.lead) {
      const p = document.createElement("p");
      p.className = "field-hint";
      p.textContent = widget.lead;
      wrap.appendChild(p);
    }
    const canvas = document.createElement("canvas");
    canvas.className = "ext-metric-canvas";
    canvas.height = 140;
    wrap.appendChild(canvas);
    host.appendChild(wrap);
    if (typeof Chart === "undefined") {
      const fallback = document.createElement("p");
      fallback.className = "field-hint";
      fallback.textContent = "Charts unavailable in this build.";
      wrap.appendChild(fallback);
      return;
    }
    const labels = widget.labels || [];
    const series = widget.series || [];
    const datasets = series.map((s, i) => {
      const color = s.color || (["#c2a633", "#3b82f6", "#16a34a", "#ea580c"][i % 4]);
      return {
        label: s.label || ("Series " + (i + 1)),
        data: s.data || [],
        borderColor: color,
        backgroundColor: (widget.chart === "bar") ? color : (color.length === 7 ? color + "33" : color),
        fill: widget.chart !== "bar",
        tension: 0.35,
        pointRadius: labels.length > 24 ? 0 : 3,
        borderWidth: 2,
      };
    });
    try {
      // eslint-disable-next-line no-new
      new Chart(canvas.getContext("2d"), {
        type: widget.chart === "bar" ? "bar" : "line",
        data: { labels: labels, datasets: datasets },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: Object.assign({}, typeof modernChartPlugins === "function" ? modernChartPlugins() : {}, {
            legend: { display: datasets.length > 1, position: "bottom", labels: { boxWidth: 10, font: { size: 11 } } },
          }),
          scales: {
            x: { ticks: { maxTicksLimit: 8, font: { size: 10 } }, grid: { display: false } },
            y: { beginAtZero: true, ticks: { font: { size: 10 } }, grid: { color: "rgba(92,101,112,0.12)" } },
          },
        },
      });
    } catch (_) { /* ignore chart errors */ }
  }

  function renderExtTableWidget(host, widget, ext) {
    const wrap = document.createElement("div");
    wrap.className = "ext-table-wrap";
    const head = document.createElement("div");
    head.className = "ext-table-head";
    const title = document.createElement("h4");
    title.className = "ext-proof-list-title";
    title.textContent = widget.title || "Table";
    head.appendChild(title);
    const pageSize = Math.max(5, Math.min(100, Number(widget.page_size) || 20));
    let rows = Array.isArray(widget.rows) ? widget.rows.slice() : [];
    const columns = widget.columns || [];
    let shown = 0;
    let filter = "";

    const search = document.createElement("input");
    search.type = "search";
    search.className = "ext-modern-input ext-table-search";
    search.placeholder = widget.search_placeholder || "Search…";
    search.setAttribute("aria-label", "Search table");
    if (widget.search === false) search.hidden = true;
    head.appendChild(search);
    wrap.appendChild(head);

    const scroller = document.createElement("div");
    scroller.className = "ext-table-scroll";
    const table = document.createElement("table");
    table.className = "ext-table";
    const thead = document.createElement("thead");
    const hr = document.createElement("tr");
    if (!columns.length && rows[0]) {
      Object.keys(rows[0]).forEach((k) => {
        if (k === "_id") return;
        columns.push({ key: k, label: k.replace(/_/g, " ") });
      });
    }
    columns.forEach((c) => {
      const th = document.createElement("th");
      th.textContent = c.label || c.key;
      hr.appendChild(th);
    });
    thead.appendChild(hr);
    const tbody = document.createElement("tbody");
    table.append(thead, tbody);
    scroller.appendChild(table);
    wrap.appendChild(scroller);

    const foot = document.createElement("div");
    foot.className = "ext-table-foot";
    const meta = document.createElement("span");
    meta.className = "field-hint";
    const moreBtn = document.createElement("button");
    moreBtn.type = "button";
    moreBtn.className = "btn btn-ghost btn-sm";
    moreBtn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">expand_more</span> Show more";
    foot.append(meta, moreBtn);
    wrap.appendChild(foot);

    function filteredRows() {
      const q = filter.trim().toLowerCase();
      if (!q) return rows;
      return rows.filter((row) => {
        return columns.some((c) => String(row[c.key] != null ? row[c.key] : "").toLowerCase().indexOf(q) >= 0);
      });
    }

    function paint(reset) {
      if (reset) {
        tbody.innerHTML = "";
        shown = 0;
      }
      const list = filteredRows();
      if (!list.length) {
        tbody.innerHTML = "<tr><td colspan=\"" + Math.max(1, columns.length) + "\" class=\"ext-table-empty\">No matching rows.</td></tr>";
        meta.textContent = "0 rows";
        moreBtn.hidden = true;
        return;
      }
      const next = list.slice(shown, shown + pageSize);
      next.forEach((row) => {
        const tr = document.createElement("tr");
        columns.forEach((c) => {
          const td = document.createElement("td");
          const val = row[c.key];
          td.textContent = val != null ? String(val) : "";
          if (c.mono) td.className = "mono-sm";
          tr.appendChild(td);
        });
        tbody.appendChild(tr);
      });
      shown += next.length;
      meta.textContent = "Showing " + shown + " of " + list.length;
      moreBtn.hidden = shown >= list.length;
    }

    moreBtn.addEventListener("click", () => paint(false));
    let searchTimer = null;
    search.addEventListener("input", () => {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(() => {
        filter = search.value || "";
        paint(true);
      }, 120);
    });

    if (widget.load_more && widget.load_more.method && ext) {
      const loadRemote = document.createElement("button");
      loadRemote.type = "button";
      loadRemote.className = "btn btn-ghost btn-sm";
      loadRemote.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">cloud_download</span> Load more from node";
      loadRemote.addEventListener("click", () => {
        void (async () => {
          loadRemote.disabled = true;
          try {
            const lm = widget.load_more;
            const params = Array.isArray(lm.params) ? lm.params.slice() : [];
            if (lm.limit_param_index != null) {
              params[lm.limit_param_index] = Math.max(rows.length + pageSize, Number(params[lm.limit_param_index]) || pageSize);
            }
            const method = extRpcFullName(ext.id, lm.method);
            const r = await fetch("/api/rpc", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              credentials: "same-origin",
              body: JSON.stringify({ method, params }),
            });
            const body = await r.json().catch(() => ({}));
            if (body.error) throw new Error(typeof body.error === "string" ? body.error : JSON.stringify(body.error));
            let extra = body.result;
            if (lm.map_rows === "tokens" && Array.isArray(extra)) {
              extra = extra.map((t) => ({
                tick: t.tick || t.Tick || "",
                mints: t.mint_count != null ? t.mint_count : (t.MintCount != null ? t.MintCount : ""),
                transfers: t.transfer_count != null ? t.transfer_count : (t.TransferCount != null ? t.TransferCount : ""),
                max: t.max || t.Max || "",
              }));
            } else if (lm.map_rows === "proofs" && Array.isArray(extra)) {
              extra = extra.map((p) => ({
                proof_hash: shortHashEx(p.proof_hash || p.proofHash || ""),
                height: p.block_height != null ? p.block_height : "",
                tx: shortHashEx(p.transaction_id || p.transactionId || ""),
              }));
            }
            if (Array.isArray(extra)) {
              const seen = {};
              rows.forEach((r0) => { seen[JSON.stringify(r0)] = 1; });
              extra.forEach((r0) => {
                const k = JSON.stringify(r0);
                if (!seen[k]) {
                  seen[k] = 1;
                  rows.push(r0);
                }
              });
              paint(true);
            }
          } catch (e) {
            meta.textContent = String(e.message || e);
          } finally {
            loadRemote.disabled = false;
          }
        })();
      });
      foot.appendChild(loadRemote);
    }

    paint(true);
    host.appendChild(wrap);
  }

  function renderExtItemListWidget(host, widget) {
    const wrap = document.createElement("div");
    wrap.className = "ext-item-list";
    const head = document.createElement("div");
    head.className = "ext-table-head";
    const title = document.createElement("h4");
    title.className = "ext-proof-list-title";
    title.textContent = widget.title || "Items";
    head.appendChild(title);
    const pageSize = Math.max(5, Math.min(80, Number(widget.page_size) || 15));
    const items = widget.items || [];
    let shown = 0;
    let filter = "";
    const search = document.createElement("input");
    search.type = "search";
    search.className = "ext-modern-input ext-table-search";
    search.placeholder = "Search…";
    if (items.length > 8) head.appendChild(search);
    wrap.appendChild(head);
    const listHost = document.createElement("div");
    listHost.className = "ext-item-list-body";
    wrap.appendChild(listHost);
    const moreBtn = document.createElement("button");
    moreBtn.type = "button";
    moreBtn.className = "btn btn-ghost btn-sm";
    moreBtn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">expand_more</span> Show more";
    wrap.appendChild(moreBtn);

    function filtered() {
      const q = filter.trim().toLowerCase();
      if (!q) return items;
      return items.filter((it) => {
        const hay = ((it.title || "") + " " + (it.meta || "") + " " + (it.id || "")).toLowerCase();
        return hay.indexOf(q) >= 0;
      });
    }

    function paint(reset) {
      if (reset) {
        listHost.innerHTML = "";
        shown = 0;
      }
      const list = filtered();
      if (!list.length) {
        listHost.innerHTML = "<p class=\"field-hint\">Nothing indexed yet.</p>";
        moreBtn.hidden = true;
        return;
      }
      list.slice(shown, shown + pageSize).forEach((it) => {
        const row = document.createElement("div");
        row.className = "ext-item-row";
        row.innerHTML = "<strong>" + escapeHtml(it.title || it.id || "") + "</strong>" +
          (it.meta ? "<span class=\"ext-item-meta\">" + escapeHtml(String(it.meta)) + "</span>" : "");
        listHost.appendChild(row);
      });
      shown = Math.min(list.length, shown + pageSize);
      moreBtn.hidden = shown >= list.length;
    }
    moreBtn.addEventListener("click", () => paint(false));
    search.addEventListener("input", () => {
      filter = search.value || "";
      paint(true);
    });
    paint(true);
    host.appendChild(wrap);
  }

  function renderExtensionWorkspace(dash, ext, info, ui) {
    const shell = document.createElement("div");
    shell.className = "ext-workspace";
    const nav = document.createElement("nav");
    nav.className = "ext-workspace-nav";
    nav.setAttribute("aria-label", "Extension sections");
    const navTop = document.createElement("div");
    navTop.className = "ext-workspace-nav-top";
    const navLabel = document.createElement("div");
    navLabel.className = "ext-workspace-nav-label";
    navLabel.textContent = "Menu";
    const navToggle = document.createElement("button");
    navToggle.type = "button";
    navToggle.className = "ext-workspace-nav-toggle btn btn-ghost btn-sm";
    navToggle.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">menu</span> Menu";
    navToggle.setAttribute("aria-expanded", "true");
    navTop.append(navLabel, navToggle);
    nav.appendChild(navTop);
    const navBtns = document.createElement("div");
    navBtns.className = "ext-workspace-nav-btns";
    nav.appendChild(navBtns);
    const main = document.createElement("div");
    main.className = "ext-workspace-main";
    const consoleBox = document.createElement("details");
    consoleBox.className = "ext-console card ext-workspace-console";
    consoleBox.open = window.matchMedia && !window.matchMedia("(max-width: 720px)").matches;
    consoleBox.innerHTML = "<summary class=\"ext-console-head\"><span class=\"material-icons-round\" aria-hidden=\"true\">terminal</span> <strong>Results</strong><span class=\"ext-console-hint\">hide / show</span></summary>";
    const consoleOut = document.createElement("pre");
    consoleOut.className = "ext-console-output mono-sm";
    consoleOut.textContent = "Pick a menu item. Quick actions and wizards print results here.";
    consoleBox.appendChild(consoleOut);

    const menuKey = "dogego_ext_menu_" + (ext && ext.id ? ext.id : "x");
    const collapsedPref = sessionStorage.getItem(menuKey);
    if (collapsedPref === "1" || (collapsedPref == null && window.matchMedia && window.matchMedia("(max-width: 720px)").matches)) {
      shell.classList.add("nav-collapsed");
      navToggle.setAttribute("aria-expanded", "false");
    }
    navToggle.addEventListener("click", () => {
      const collapsed = shell.classList.toggle("nav-collapsed");
      navToggle.setAttribute("aria-expanded", collapsed ? "false" : "true");
      sessionStorage.setItem(menuKey, collapsed ? "1" : "0");
    });

    const sections = ui.sections || {};
    const navItems = ui.nav || Object.keys(sections).map((id) => ({ id: id, label: (sections[id] && sections[id].title) || id, icon: "extension" }));
    const storageKey = "dogego_ext_section_" + (ext && ext.id ? ext.id : "x");
    let active = sessionStorage.getItem(storageKey) || (navItems[0] && navItems[0].id) || "home";
    if (!sections[active] && navItems[0]) active = navItems[0].id;

    function showSection(id) {
      active = id;
      sessionStorage.setItem(storageKey, id);
      navBtns.querySelectorAll(".ext-workspace-nav-btn").forEach((b) => {
        b.classList.toggle("active", b.dataset.sectionId === id);
      });
      main.innerHTML = "";
      const sec = sections[id] || {};
      if (sec.title) {
        const h = document.createElement("h4");
        h.className = "ext-workspace-title";
        h.textContent = sec.title;
        main.appendChild(h);
      }
      if (sec.lead) {
        const p = document.createElement("p");
        p.className = "ext-dash-subtitle";
        p.textContent = sec.lead;
        main.appendChild(p);
      }
      (sec.widgets || []).forEach((w) => renderExtWidget(main, w, ext));
      if ((sec.quick_actions || []).length) {
        renderExtensionQuickActions(main, ext, sec.quick_actions, consoleOut);
      }
      (sec.wizards || []).forEach((wiz) => renderExtensionWizard(main, ext, wiz, consoleOut));
      if ((sec.tools || []).length) {
        const toolsHost = document.createElement("div");
        toolsHost.className = "ext-section-tools";
        main.appendChild(toolsHost);
        renderExtensionToolsInline(toolsHost, ext, sec.tools, consoleOut);
      }
      if (!sec.widgets && !sec.tools && !sec.wizards && !sec.quick_actions) {
        const hint = document.createElement("p");
        hint.className = "field-hint";
        hint.textContent = "This section has no tools yet.";
        main.appendChild(hint);
      }
      if (window.matchMedia && window.matchMedia("(max-width: 720px)").matches) {
        shell.classList.add("nav-collapsed");
        navToggle.setAttribute("aria-expanded", "false");
        sessionStorage.setItem(menuKey, "1");
      }
    }

    navItems.forEach((item) => {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "ext-workspace-nav-btn";
      btn.dataset.sectionId = item.id;
      btn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(item.icon || "circle") + "</span><span>" + escapeHtml(item.label || item.id) + "</span>";
      btn.addEventListener("click", () => showSection(item.id));
      navBtns.appendChild(btn);
    });

    shell.append(nav, main);
    dash.append(shell, consoleBox);
    showSection(active);
  }

  function renderExtensionToolsInline(container, ext, tools, consoleOut) {
    (tools || []).forEach((tool) => {
      const card = document.createElement("details");
      card.className = "ext-tool-card ext-tool-card-collapsible";
      card.open = !(tool.collapsed === true || tool.advanced === true);
      card.dataset.toolId = tool.id || tool.method || "";
      const head = document.createElement("summary");
      head.className = "ext-tool-head";
      head.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(tool.icon || "bolt") + "</span> " + escapeHtml(tool.label || tool.method);
      card.appendChild(head);
      if (tool.hint) {
        const hint = document.createElement("p");
        hint.className = "field-hint";
        hint.textContent = tool.hint;
        card.appendChild(hint);
      }
      const fields = document.createElement("div");
      fields.className = "ext-tool-fields";
      (tool.fields || []).forEach((field) => fields.appendChild(renderExtensionField(field)));
      card.appendChild(fields);
      const run = document.createElement("button");
      run.type = "button";
      run.className = "btn btn-primary btn-sm";
      run.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">play_arrow</span> " + escapeHtml(tool.run_label || "Run");
      const pre = document.createElement("pre");
      pre.className = "ext-tool-result mono-sm";
      run.addEventListener("click", () => {
        void runExtensionTool(ext, tool, card).then(() => {
          if (consoleOut && pre.textContent) {
            consoleOut.textContent = pre.textContent;
            const det = consoleOut.closest("details");
            if (det) det.open = true;
          }
        }).catch(() => {
          if (consoleOut && pre.textContent) {
            consoleOut.textContent = pre.textContent;
            const det = consoleOut.closest("details");
            if (det) det.open = true;
          }
        });
      });
      card.append(run, pre);
      container.appendChild(card);
    });
  }

  function renderExtensionWizard(host, ext, wiz, consoleOut) {
    if (!wiz || !wiz.method) return;
    const card = document.createElement("div");
    card.className = "ext-wizard card";
    const steps = wiz.steps || [];
    let stepIdx = 0;
    const head = document.createElement("div");
    head.className = "ext-wizard-head";
    head.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(wiz.icon || "auto_awesome") + "</span> <strong>" + escapeHtml(wiz.title || "Wizard") + "</strong>";
    const stepper = document.createElement("div");
    stepper.className = "ext-wizard-stepper";
    const body = document.createElement("div");
    body.className = "ext-wizard-body";
    const actions = document.createElement("div");
    actions.className = "ext-wizard-actions";
    const pre = document.createElement("pre");
    pre.className = "ext-tool-result mono-sm";
    const collected = {};

    function paint() {
      stepper.innerHTML = "";
      steps.forEach((s, i) => {
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "ext-wizard-step" + (i === stepIdx ? " active" : "") + (i < stepIdx ? " done" : "");
        chip.textContent = (i + 1) + ". " + (s.title || s.id || ("Step " + (i + 1)));
        chip.addEventListener("click", () => {
          if (i <= stepIdx) {
            saveCurrent();
            stepIdx = i;
            paint();
          }
        });
        stepper.appendChild(chip);
      });
      body.innerHTML = "";
      const step = steps[stepIdx] || {};
      if (step.hint) {
        const hint = document.createElement("p");
        hint.className = "field-hint";
        hint.textContent = step.hint;
        body.appendChild(hint);
      }
      const fields = document.createElement("div");
      fields.className = "ext-tool-fields";
      fields.dataset.wizardStep = String(stepIdx);
      (step.fields || []).forEach((field) => {
        const el = renderExtensionField(field);
        const input = el.querySelector("[name]");
        if (input && collected[field.name] != null) {
          if (input.type === "checkbox") input.checked = !!collected[field.name];
          else {
            input.value = collected[field.name];
            if (field.type === "select" || input.classList.contains("ext-choice-value")) {
              syncExtensionChoiceField(el, field.name, collected[field.name]);
            }
          }
        }
        fields.appendChild(el);
      });
      body.appendChild(fields);
      actions.innerHTML = "";
      if (stepIdx > 0) {
        const back = document.createElement("button");
        back.type = "button";
        back.className = "btn btn-ghost btn-sm";
        back.textContent = "Back";
        back.addEventListener("click", () => {
          saveCurrent();
          stepIdx--;
          paint();
        });
        actions.appendChild(back);
      }
      const next = document.createElement("button");
      next.type = "button";
      next.className = "btn btn-primary btn-sm";
      const last = stepIdx >= steps.length - 1;
      next.innerHTML = last
        ? ("<span class=\"material-icons-round\" aria-hidden=\"true\">play_arrow</span> " + escapeHtml(wiz.finish_label || "Finish"))
        : "Next";
      next.addEventListener("click", () => {
        saveCurrent();
        if (!last) {
          stepIdx++;
          paint();
          return;
        }
        void runExtensionRPC(ext, wiz.method, [collected], pre).then((out) => {
          if (consoleOut) consoleOut.textContent = typeof out === "string" ? out : JSON.stringify(out, null, 2);
        }).catch((e) => {
          if (consoleOut) consoleOut.textContent = String(e.message || e);
        });
      });
      actions.appendChild(next);
    }

    function saveCurrent() {
      const fieldsHost = body.querySelector(".ext-tool-fields");
      if (!fieldsHost) return;
      const step = steps[stepIdx] || {};
      Object.assign(collected, collectExtensionFields(fieldsHost, step.fields || []));
    }

    card.append(head, stepper, body, actions, pre);
    host.appendChild(card);
    paint();
  }

  function renderExtStatsWidget(host, widget) {
    const grid = document.createElement("div");
    grid.className = "ext-dash-stats";
    (widget.items || []).forEach((item) => {
      const card = document.createElement("div");
      card.className = "ext-dash-stat";
      const lab = document.createElement("div");
      lab.className = "ext-dash-stat-label";
      lab.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + escapeHtml(item.icon || "insights") + "</span> " + escapeHtml(item.label || "");
      const val = document.createElement("div");
      const text = item.value != null ? String(item.value) : "";
      val.className = "ext-dash-stat-value" + (text.length > 12 ? " ext-dash-stat-value-long" : "");
      val.textContent = text;
      card.append(lab, val);
      grid.appendChild(card);
    });
    host.appendChild(grid);
  }

  function renderExtProofListWidget(host, widget, ext) {
    const wrap = document.createElement("div");
    wrap.className = "ext-proof-list";
    const title = document.createElement("h4");
    title.className = "ext-proof-list-title";
    title.textContent = widget.title || "Proofs";
    wrap.appendChild(title);
    const proofs = widget.proofs || [];
    if (!proofs.length) {
      const empty = document.createElement("p");
      empty.className = "field-hint";
      empty.textContent = "No proofs indexed yet.";
      wrap.appendChild(empty);
    } else {
      proofs.forEach((p) => {
        wrap.appendChild(renderExtProofCard(p, ext));
      });
    }
    if (ext && ext.id === "dogego.zkl2" && proofs.length) {
      const more = document.createElement("button");
      more.type = "button";
      more.className = "btn btn-ghost btn-sm ext-proof-load-more";
      more.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">refresh</span> Load more proofs";
      more.addEventListener("click", () => {
        void loadMoreZkl2Proofs(wrap, ext, proofs.length);
      });
      wrap.appendChild(more);
    }
    host.appendChild(wrap);
  }

  function renderExtProofCard(p, ext) {
    const card = document.createElement("article");
    card.className = "ext-proof-card";
    const main = document.createElement("div");
    main.className = "ext-proof-card-main";
    const hash = document.createElement("div");
    hash.className = "ext-proof-card-hash";
    hash.textContent = shortHashEx(p.proof_hash || p.proofHash || "");
    hash.title = p.proof_hash || p.proofHash || "";
    const meta = document.createElement("div");
    meta.className = "ext-proof-card-meta";
    const txid = p.transaction_id || p.transactionId || "";
    if (txid) {
      const tx = document.createElement("span");
      tx.textContent = "Tx " + shortHashEx(txid);
      tx.title = txid;
      meta.appendChild(tx);
    }
    if (p.block_height != null) {
      const bh = document.createElement("span");
      bh.textContent = "Height " + p.block_height;
      meta.appendChild(bh);
    }
    if (p.created_timestamp) {
      const ts = document.createElement("span");
      ts.textContent = fmtBlockTime(p.created_timestamp);
      meta.appendChild(ts);
    }
    main.append(hash, meta);
    const actions = document.createElement("div");
    actions.className = "ext-proof-card-actions";
    if (txid) {
      const ex = document.createElement("button");
      ex.type = "button";
      ex.className = "btn btn-ghost btn-sm";
      ex.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">open_in_new</span>";
      ex.title = "Open transaction";
      ex.addEventListener("click", () => {
        location.hash = "explorer";
        showTab("explorer");
        const q = $("lk-ex-q") || $("ov-ex-q");
        if (q) q.value = txid;
      });
      actions.appendChild(ex);
    }
    card.append(main, actions);
    return card;
  }

  async function loadMoreZkl2Proofs(wrap, ext, offset) {
    const btn = wrap.querySelector(".ext-proof-load-more");
    if (btn) btn.disabled = true;
    try {
      const method = extRpcFullName(ext.id, "listproofs");
      const r = await fetch("/api/rpc", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ method, params: ["", Math.max(50, offset + 20)] }),
      });
      const body = await r.json().catch(() => ({}));
      if (body.error) throw new Error(typeof body.error === "string" ? body.error : JSON.stringify(body.error));
      const list = Array.isArray(body.result) ? body.result : [];
      list.slice(offset).forEach((p) => wrap.insertBefore(renderExtProofCard(p, ext), btn));
      if (btn) btn.remove();
    } catch (e) {
      if (btn) {
        btn.disabled = false;
        btn.textContent = String(e.message || e);
      }
    }
  }

  function setExtensionPanelsVisible(mode) {
    const catalogPanel = $("panel-extensions");
    const detailPanel = $("panel-extension-detail");
    if (catalogPanel) catalogPanel.classList.toggle("active", mode === "catalog");
    if (detailPanel) detailPanel.classList.toggle("active", mode === "detail");
  }

  function showExtensionCatalogView(opts) {
    opts = opts || {};
    setExtensionPanelsVisible("catalog");
    document.querySelectorAll(".nav-item-sub[data-ext-nav-id]").forEach((b) => b.classList.remove("active"));
    if (!opts.preserveHash) {
      const cur = (location.hash || "").replace(/^#/, "");
      if (cur.startsWith("extensions/")) location.hash = "extensions";
    }
  }

  function showExtensionDetailView(extId, opts) {
    opts = opts || {};
    if (!extId) {
      showExtensionCatalogView(opts);
      return;
    }
    setExtensionPanelsVisible("detail");
    document.querySelectorAll(".nav-item-sub[data-ext-nav-id]").forEach((b) => {
      b.classList.toggle("active", b.dataset.extNavId === extId);
    });
    if (!opts.preserveHash) location.hash = "extensions/" + extId;
    void loadExtensionDetailPage(extId);
  }

  function routeExtensionsFromHash() {
    const raw = (location.hash || "#extensions").replace(/^#/, "");
    const slash = raw.indexOf("/");
    if (slash > 0 && raw.startsWith("extensions/")) {
      showExtensionDetailView(decodeURIComponent(raw.slice(slash + 1)), { preserveHash: true });
      return;
    }
    showExtensionCatalogView({ preserveHash: true });
    void loadExtensionsCatalog();
  }

  function setExtNavSubmenuOpen(open) {
    extNavSubmenuOpen = !!open;
    const submenu = $("nav-ext-submenu");
    const fold = $("nav-ext-fold");
    if (submenu) submenu.hidden = !extNavSubmenuOpen;
    if (fold) {
      fold.setAttribute("aria-expanded", extNavSubmenuOpen ? "true" : "false");
      fold.classList.toggle("open", extNavSubmenuOpen);
    }
    try {
      sessionStorage.setItem("dogego_ext_nav_open", extNavSubmenuOpen ? "1" : "0");
    } catch (_) { /* ignore */ }
  }

  function updateExtUpdatesBadge(rows) {
    const catalogBtn = $("nav-ext-catalog");
    if (!catalogBtn) return;
    let badge = catalogBtn.querySelector(".nav-ext-update-badge");
    const n = (rows || []).filter((e) => e && e.update_available).length;
    if (n <= 0) {
      if (badge) badge.remove();
      catalogBtn.removeAttribute("data-updates");
      return;
    }
    if (!badge) {
      badge = document.createElement("span");
      badge.className = "nav-ext-update-badge";
      catalogBtn.appendChild(badge);
    }
    badge.textContent = n > 9 ? "9+" : String(n);
    catalogBtn.setAttribute("data-updates", String(n));
    badge.title = n + " extension update" + (n === 1 ? "" : "s") + " available";
  }

  function renderExtensionNavSubmenu(rows) {
    const submenu = $("nav-ext-submenu");
    const fold = $("nav-ext-fold");
    if (!submenu) return;
    const enabled = (rows || extensionsCatalogCache || []).filter((e) => e && e.enabled);
    updateExtUpdatesBadge(rows || extensionsCatalogCache || []);
    submenu.innerHTML = "";
    enabled.forEach((ext) => {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "nav-item nav-item-sub";
      btn.dataset.extNavId = ext.id;
      const icon = document.createElement("img");
      icon.className = "nav-ext-sub-icon";
      icon.src = extIconUrl(ext);
      icon.alt = "";
      icon.loading = "lazy";
      icon.onerror = function () {
        icon.replaceWith(Object.assign(document.createElement("span"), {
          className: "material-icons-round nav-ext-sub-fallback",
          textContent: "extension",
        }));
      };
      const label = document.createElement("span");
      label.className = "nav-label";
      label.textContent = ext.name || ext.id;
      btn.append(icon, label);
      if (ext.update_available) {
        const tip = document.createElement("span");
        tip.className = "nav-ext-sub-update";
        tip.title = "Update available";
        tip.textContent = "↑";
        btn.appendChild(tip);
      }
      btn.addEventListener("click", () => {
        showTab("extensions");
        showExtensionDetailView(ext.id);
      });
      submenu.appendChild(btn);
    });
    if (fold) fold.hidden = enabled.length === 0;
    if (enabled.length && sessionStorage.getItem("dogego_ext_nav_open") === "1") {
      setExtNavSubmenuOpen(true);
    } else if (!enabled.length) {
      setExtNavSubmenuOpen(false);
    }
  }

  function findExtensionRow(extId) {
    return (extensionsCatalogCache || []).find((r) => r && r.id === extId) || null;
  }

  async function loadExtensionDetailPage(extId) {
    const root = $("ext-detail-root");
    if (!root) return;
    let ext = findExtensionRow(extId);
    if (!ext) {
      await loadExtensionsCatalog(false);
      ext = findExtensionRow(extId);
    }
    if (!ext) {
      root.innerHTML = "<p class=\"label\">Extension not found.</p>";
      return;
    }
    wait(root, "Loading extension…", { compact: true });
    root.innerHTML = "";
    const wrap = document.createElement("div");
    wrap.className = "ext-detail-page card";
    wrap.dataset.extId = ext.id || "";
    const hero = document.createElement("div");
    hero.className = "ext-detail-hero";
    const iconWrap = document.createElement("div");
    iconWrap.className = "ext-detail-icon";
    const img = document.createElement("img");
    img.src = extIconUrl(ext);
    img.alt = "";
    img.onerror = function () {
      img.replaceWith(Object.assign(document.createElement("span"), {
        className: "material-icons-round ext-card-icon-fallback",
        textContent: "extension",
      }));
    };
    iconWrap.appendChild(img);
    const titleBlock = document.createElement("div");
    titleBlock.className = "ext-detail-titleblock";
    const h1 = document.createElement("h1");
    h1.className = "ext-detail-title";
    h1.textContent = ext.name || ext.id;
    const meta = document.createElement("p");
    meta.className = "ext-detail-meta";
    const catVer = ext.version || "?";
    const instVer = ext.installed_version || (ext.installed ? catVer : "");
    let verBit = "catalog v" + catVer;
    if (ext.installed && instVer) {
      verBit = "installed v" + instVer + (instVer !== String(catVer) ? " · catalog v" + catVer : "");
    }
    meta.textContent = ext.id + " · " + verBit + (ext.author ? " · " + ext.author : "");
    titleBlock.append(h1, meta);
    if (ext.update_available) {
      const upd = document.createElement("p");
      upd.className = "ext-update-pill";
      upd.textContent = "Update available from catalog";
      titleBlock.appendChild(upd);
    }
    const st = extStateLabel(ext);
    const pill = document.createElement("span");
    pill.className = "service-pill ext-state-pill " + st.cls;
    pill.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + st.icon + "</span> " + st.text;
    hero.append(iconWrap, titleBlock, pill);
    if (ext.description) {
      const desc = document.createElement("p");
      desc.className = "ext-detail-desc";
      desc.textContent = ext.description;
      hero.appendChild(desc);
    }
    const actions = document.createElement("div");
    actions.className = "ext-detail-actions";
    if (!ext.enabled) {
      const en = document.createElement("button");
      en.type = "button";
      en.className = "btn btn-primary btn-sm";
      if (ext.installed || ext.builtin) {
        en.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">play_arrow</span> Enable";
        en.addEventListener("click", () => void extAction("enable", ext.id));
      } else if (ext.download_url || ext.repository) {
        en.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">download</span> Install";
        en.addEventListener("click", () => void extAction("install", ext.id));
      }
      actions.appendChild(en);
    } else {
      const dis = document.createElement("button");
      dis.type = "button";
      dis.className = "btn btn-ghost btn-sm";
      dis.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">pause</span> Disable";
      dis.addEventListener("click", () => void extAction("disable", ext.id));
      actions.appendChild(dis);
    }
    // Uninstall removes the installed package directory. For built-in modules this does not
    // remove compiled code, it only removes the installed extension bundle from disk.
    if (ext.installed && !ext.enabled) {
      const un = document.createElement("button");
      un.type = "button";
      un.className = "btn btn-ghost btn-sm ext-uninstall";
      un.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">delete</span> Uninstall";
      un.addEventListener("click", () => {
        if (!confirm("Remove extension " + ext.id + "?")) return;
        void extAction("uninstall", ext.id);
      });
      actions.appendChild(un);
    }
    if (ext.update_available && (ext.download_url || (ext.downloads && Object.keys(ext.downloads).length) || ext.repository)) {
      const updBtn = document.createElement("button");
      updBtn.type = "button";
      updBtn.className = "btn btn-primary btn-sm";
      updBtn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">system_update</span> Update";
      updBtn.title = "Fetch from GitHub catalog. Extension databases and settings under data/ are preserved.";
      updBtn.addEventListener("click", () => void extAction("update", ext.id));
      actions.appendChild(updBtn);
    }
    hero.appendChild(actions);
    wrap.appendChild(hero);
    if ((ext.permissions || []).length || (ext.capabilities || []).length) {
      const badges = document.createElement("div");
      badges.className = "ext-detail-badges";
      if ((ext.permissions || []).length) badges.appendChild(extBadgeRow(ext.permissions, "perm"));
      if ((ext.capabilities || []).length) badges.appendChild(extBadgeRow(ext.capabilities, "cap"));
      wrap.appendChild(badges);
    }
    // Only ui_panel extensions have /api/extensions/panel. RPC-only extensions render a console
    // directly from rpc_methods without calling the panel endpoint.
    if (ext.enabled && extensionOffersUIPanel(ext)) {
      const workspaceCard = document.createElement("div");
      workspaceCard.className = "card ext-workspace-card";
      const wsHead = document.createElement("div");
      wsHead.className = "ext-workspace-card-head";
      wsHead.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">dashboard</span> <div><strong>Extension workspace</strong><p class=\"field-hint\">Use the menu on the left (Home, Tokens, Mint / Deploy, Settings, …). Settings configures wallet RPC for this extension.</p></div>";
      const dashHost = document.createElement("div");
      dashHost.className = "ext-panel-dash-host";
      const advWrap = extDetailDisclosure("Advanced (raw JSON)", false);
      const advPre = document.createElement("pre");
      advPre.className = "mono-sm ext-panel-json ext-detail-panel-json";
      advWrap.body.appendChild(advPre);
      const toolsBox = document.createElement("div");
      toolsBox.className = "ext-tools-box";
      toolsBox.hidden = true;
      workspaceCard.append(wsHead, dashHost, toolsBox, advWrap.det);
      wrap.appendChild(workspaceCard);
      void loadExtensionPanelInto(dashHost, advPre, toolsBox, ext);
    } else if (ext.enabled && (ext.rpc_methods || []).length) {
      const toolsWrap = extDetailDisclosure("Extension console", true);
      const toolsBox = document.createElement("div");
      toolsBox.className = "ext-tools-box";
      toolsWrap.body.appendChild(toolsBox);
      wrap.appendChild(toolsWrap.det);
      renderExtensionTools(toolsBox, ext, [], { quick_actions: extToolsFromRpcMethods(ext).map((t) => ({ id: t.id, label: t.label, method: t.method, icon: t.icon })) });
    }
    if ((ext.rpc_methods || []).length) {
      const rpcWrap = extDetailDisclosure("RPC reference", false);
      const rpcBadges = extBadgeRow(ext.rpc_methods, "cap");
      rpcBadges.classList.add("ext-rpc-badges");
      rpcWrap.body.appendChild(rpcBadges);
      const consoleBtn = document.createElement("button");
      consoleBtn.type = "button";
      consoleBtn.className = "btn btn-ghost btn-sm";
      consoleBtn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">terminal</span> Open Console";
      consoleBtn.addEventListener("click", () => {
        const first = (ext.rpc_methods || [])[0];
        if (first && $("rpc-method")) $("rpc-method").value = first;
        showTab("console");
      });
      rpcWrap.body.appendChild(consoleBtn);
      wrap.appendChild(rpcWrap.det);
    }
    const docWrap = extDetailDisclosure("Documentation", !(ext.enabled && extensionOffersUIPanel(ext)));
    const docBox = document.createElement("div");
    docBox.className = "ext-docs markdown-body ext-detail-docs";
    docWrap.body.appendChild(docBox);
    wrap.appendChild(docWrap.det);
    root.appendChild(wrap);
    void loadExtensionDocsInto(docBox, ext);
  }

  function setExtensionCardExpanded(card, open) {
    if (!card) return;
    card.classList.toggle("expanded", !!open);
    const expandBtn = card.querySelector(".ext-card-expand-btn");
    if (expandBtn) {
      expandBtn.setAttribute("aria-expanded", open ? "true" : "false");
      expandBtn.innerHTML = open
        ? "<span class=\"material-icons-round\" aria-hidden=\"true\">expand_less</span> <span data-i18n=\"pages.extensions.showLess\">Show less</span>"
        : "<span class=\"material-icons-round\" aria-hidden=\"true\">expand_more</span> <span data-i18n=\"pages.extensions.showMore\">Show more</span>";
    }
    if (window.DogeGoI18n && window.DogeGoI18n.apply) window.DogeGoI18n.apply(card);
  }

  function collapseExtensionCards(except) {
    document.querySelectorAll(".ext-card.expanded").forEach((c) => {
      if (c !== except) setExtensionCardExpanded(c, false);
    });
  }

  function renderExtensionCard(ext) {
    const slot = document.createElement("div");
    slot.className = "ext-card-slot";
    const card = document.createElement("article");
    card.className = "ext-card";
    card.dataset.extId = ext.id || "";
    const head = document.createElement("div");
    head.className = "ext-card-head";
    const iconWrap = document.createElement("div");
    iconWrap.className = "ext-card-icon";
    const img = document.createElement("img");
    img.src = extIconUrl(ext);
    img.alt = "";
    img.loading = "lazy";
    img.onerror = function () {
      img.replaceWith(Object.assign(document.createElement("span"), {
        className: "material-icons-round ext-card-icon-fallback",
        textContent: "extension",
      }));
    };
    iconWrap.appendChild(img);
    const titleBlock = document.createElement("div");
    titleBlock.className = "ext-card-titleblock";
    const title = document.createElement("h3");
    title.className = "ext-card-title";
    title.textContent = ext.name || ext.id || "Extension";
    const meta = document.createElement("p");
    meta.className = "ext-card-meta";
    const catVer = ext.version || "?";
    const instVer = ext.installed_version || (ext.installed ? catVer : "");
    let verLine = "catalog v" + catVer;
    if (ext.installed && instVer) verLine = "installed v" + instVer + (instVer !== catVer ? " · catalog v" + catVer : "");
    meta.textContent = verLine + (ext.author ? " · " + ext.author : "") + (ext.builtin ? " · built-in" : "");
    if (ext.update_available) {
      const upd = document.createElement("span");
      upd.className = "ext-update-pill";
      upd.textContent = "Update available";
      titleBlock.append(title, meta, upd);
    } else {
      titleBlock.append(title, meta);
    }
    const st = extStateLabel(ext);
    const pill = document.createElement("span");
    pill.className = "service-pill ext-state-pill " + st.cls;
    pill.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">" + st.icon + "</span> " + st.text;
    head.append(iconWrap, titleBlock, pill);
    const desc = document.createElement("p");
    desc.className = "ext-card-desc";
    desc.textContent = ext.description || "";
    const badges = document.createElement("div");
    badges.className = "ext-card-badges";
    if ((ext.permissions || []).length) badges.appendChild(extBadgeRow(ext.permissions, "perm"));
    if ((ext.capabilities || []).length) badges.appendChild(extBadgeRow(ext.capabilities, "cap"));
    const body = document.createElement("div");
    body.className = "ext-card-body";
    body.append(desc, badges);
    const expandRow = document.createElement("div");
    expandRow.className = "ext-card-expand";
    const expandBtn = document.createElement("button");
    expandBtn.type = "button";
    expandBtn.className = "btn btn-ghost btn-sm ext-card-expand-btn";
    expandBtn.setAttribute("aria-expanded", "false");
    expandBtn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">expand_more</span> <span data-i18n=\"pages.extensions.showMore\">Show more</span>";
    expandBtn.addEventListener("click", (ev) => {
      ev.stopPropagation();
      const open = !card.classList.contains("expanded");
      collapseExtensionCards(card);
      setExtensionCardExpanded(card, open);
    });
    expandRow.appendChild(expandBtn);
    requestAnimationFrame(() => {
      const needsExpand = desc.scrollHeight > desc.clientHeight + 2
        || badges.scrollHeight > badges.clientHeight + 2
        || ((ext.permissions || []).length + (ext.capabilities || []).length) > 4;
      if (!needsExpand && !(ext.description && ext.description.length > 120)) {
        expandRow.hidden = true;
      }
    });
    const actions = document.createElement("div");
    actions.className = "ext-card-actions";
    if (!ext.enabled) {
      const en = document.createElement("button");
      en.type = "button";
      en.className = "btn btn-primary btn-sm";
      if (ext.installed || ext.builtin) {
        en.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">play_arrow</span> Enable";
        en.addEventListener("click", () => void extAction("enable", ext.id));
      } else if (ext.download_url || ext.repository) {
        en.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">download</span> Install";
        en.addEventListener("click", () => void extAction("install", ext.id));
      } else {
        en.className = "btn btn-ghost btn-sm";
        en.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">upload_file</span> Install zip";
        en.addEventListener("click", () => {
          const zipInput = $("ext-zip-input");
          if (zipInput) zipInput.click();
        });
      }
      actions.appendChild(en);
    } else {
      const dis = document.createElement("button");
      dis.type = "button";
      dis.className = "btn btn-ghost btn-sm";
      dis.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">pause</span> Disable";
      dis.addEventListener("click", () => void extAction("disable", ext.id));
      actions.appendChild(dis);
    }
    if (ext.installed && !ext.enabled) {
      const un = document.createElement("button");
      un.type = "button";
      un.className = "btn btn-ghost btn-sm ext-uninstall";
      un.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">delete</span> Uninstall";
      un.addEventListener("click", () => {
        if (!confirm("Remove extension " + ext.id + "?")) return;
        void extAction("uninstall", ext.id);
      });
      actions.appendChild(un);
    }
    if (ext.update_available && (ext.download_url || (ext.downloads && Object.keys(ext.downloads).length) || ext.repository)) {
      const updBtn = document.createElement("button");
      updBtn.type = "button";
      updBtn.className = "btn btn-primary btn-sm";
      updBtn.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">system_update</span> Update";
      updBtn.title = "Fetch the catalog package from GitHub. Extension data and settings are preserved.";
      updBtn.addEventListener("click", () => void extAction("update", ext.id));
      actions.appendChild(updBtn);
    }
    if (ext.enabled) {
      const open = document.createElement("button");
      open.type = "button";
      open.className = "btn btn-ghost btn-sm";
      open.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">open_in_new</span> Open";
      open.addEventListener("click", () => {
        showTab("extensions");
        showExtensionDetailView(ext.id);
      });
      actions.appendChild(open);
    }
    card.append(head, body, expandRow, actions);
    slot.appendChild(card);
    return slot;
  }

  let extDevManualLoaded = false;
  async function loadExtensionDevManual() {
    const el = $("ext-dev-manual");
    if (!el || extDevManualLoaded) return;
    extDevManualLoaded = true;
    wait(el, "Loading developer manual…", { compact: true });
    try {
      const r = await fetchWithTimeout("/api/extensions/docs?path=" + encodeURIComponent("extensions/catalog/BUILDING.md"), { cache: "no-store", credentials: "same-origin" });
      const body = await r.json().catch(() => ({}));
      if (!r.ok || body.error) throw new Error(body.error || ("HTTP " + r.status));
      await renderMarkdownInto(el, body.markdown || "", body.path || "extensions/catalog/BUILDING.md");
    } catch (e) {
      el.innerHTML = "<p class=\"label\">" + escapeHtml(String(e.message || e)) + "</p>";
    }
  }

  async function loadExtensionCatalogSources() {
    const list = $("ext-sources-list");
    if (!list) return;
    list.innerHTML = "<li class=\"ext-source-loading\"><span class=\"material-icons-round\" aria-hidden=\"true\">sync</span> Loading sources…</li>";
    try {
      const r = await fetchWithTimeout("/api/extensions/catalog-sources", { cache: "no-store", credentials: "same-origin" }, 10000);
      const body = await r.json().catch(() => ({}));
      const sources = (body.result && body.result.sources) || body.sources || [];
      list.innerHTML = "";
      if (!sources.length) {
        list.innerHTML = "<li class=\"field-hint\">No catalog sources configured.</li>";
        return;
      }
      sources.forEach((url) => {
        const li = document.createElement("li");
        li.className = "ext-source-item";
        const link = document.createElement("a");
        link.href = url;
        link.target = "_blank";
        link.rel = "noopener noreferrer";
        link.textContent = url;
        const rm = document.createElement("button");
        rm.type = "button";
        rm.className = "btn btn-ghost btn-sm";
        rm.title = "Remove source";
        rm.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">close</span>";
        rm.addEventListener("click", async () => {
          await fetch("/api/extensions/catalog-sources?url=" + encodeURIComponent(url), { method: "DELETE", credentials: "same-origin" });
          void loadExtensionCatalogSources();
          void loadExtensionsCatalog(true);
        });
        li.append(link, rm);
        list.appendChild(li);
      });
    } catch (e) {
      list.innerHTML = "<li class=\"label\">" + escapeHtml(String(e.message || e)) + "</li>";
    }
  }

  async function loadExtensionsCatalog(forceRefresh) {
    const list = $("ext-list");
    const status = $("ext-status");
    if (!list) return;
    void loadExtensionCatalogSources();
    void loadExtensionDevManual();
    wait(list, "Loading extensions…", { compact: true });
    try {
      const url = "/api/extensions/catalog" + (forceRefresh ? "?refresh=1" : "");
      const r = await fetchWithTimeout(url, { cache: "no-store", credentials: "same-origin" }, forceRefresh ? 25000 : 12000);
      const body = await r.json().catch(() => ({}));
      if (!r.ok) {
        const errMsg = extensionApiError(body) || ("HTTP " + r.status);
        throw new Error(errMsg);
      }
      if (body.error) {
        throw new Error(extensionApiError(body));
      }
      let rows = (body.result && body.result.catalog) || body.catalog || [];
      if (!rows.length) {
        const lr = await fetch("/api/extensions", { cache: "no-store", credentials: "same-origin" });
        const lbody = await lr.json().catch(() => ({}));
        const local = (lbody.result && lbody.result.extensions) || lbody.extensions || [];
        if (local.length) rows = local;
      }
      extensionsCatalogCache = rows;
      renderExtensionNavSubmenu(rows);
      if (status) {
        status.textContent = rows.length
          ? rows.length + " extension(s). Install from catalog or zip, then enable."
          : "No extensions listed. Install a zip or add a catalog source.";
      }
      list.removeAttribute("data-doge-wait");
      list.classList.remove("doge-wait-host");
      list.innerHTML = "";
      if (!rows.length) {
        list.innerHTML = "<p class=\"label\">No extensions in catalog. Use Install zip or add a catalog source.</p>";
        return;
      }
      rows.forEach((ext) => list.appendChild(renderExtensionCard(ext)));
    } catch (e) {
      list.removeAttribute("data-doge-wait");
      list.classList.remove("doge-wait-host");
      list.innerHTML = "<p class=\"label\">Failed to load extensions: " + escapeHtml(String(e.message || e)) + "</p>";
    }
  }

  function extensionApiError(body) {
    if (!body || !body.error) return "";
    const e = body.error;
    if (typeof e === "string") {
      if (e === "wallet_locked") {
        if (window.DogeGoWalletPassphrase) {
          window.DogeGoWalletPassphrase.openUnlock({
            message: "Enter your wallet passphrase to use extensions that require wallet_rpc (separate from dashboard PIN).",
          });
        }
        return "Wallet is locked. Enter your wallet passphrase (Settings → Wallet or Unlock wallet on Overview).";
      }
      if (e === "Extensions not available" || (typeof e === "string" && e.indexOf("Extensions not available") >= 0)) {
        return "Extension host not in this process. Stop all dogego.exe in Task Manager, rebuild (go build ./cmd/dogego), run dogego node from that binary, wait until sync UI loads, then retry.";
      }
      return e;
    }
    return e.message || JSON.stringify(e);
  }

  function setExtensionCardBusy(id, busy, label) {
    document.querySelectorAll("[data-ext-id=\"" + CSS.escape(id) + "\"]").forEach((el) => {
      el.classList.toggle("ext-card-busy", !!busy);
      el.querySelectorAll("button").forEach((btn) => { btn.disabled = !!busy; });
      const pill = el.querySelector(".ext-state-pill");
      if (pill && busy && label) {
        pill.innerHTML = "<span class=\"material-icons-round\" aria-hidden=\"true\">hourglass_top</span> " + escapeHtml(label);
      }
    });
  }

  async function extAction(kind, id) {
    const paths = {
      enable: "/api/extensions/enable",
      disable: "/api/extensions/disable",
      install: "/api/extensions/install",
      uninstall: "/api/extensions/uninstall",
      update: "/api/extensions/update",
    };
    const path = paths[kind];
    if (!path) return;
    const labels = { enable: "Enable", disable: "Disable", install: "Install", uninstall: "Uninstall", update: "Update" };
    const busyLabels = {
      enable: "Starting extension (first run may take 1-3 min on Windows)...",
      install: "Installing (large universal zip; may take a few minutes)…",
      update: "Updating from catalog (data/settings preserved)…",
      disable: "Stopping…",
      uninstall: "Removing…",
    };
    const slowMs = kind === "enable" || kind === "install" || kind === "update" ? 180000 : 60000;
    pushExtNotice("warn", labels[kind] + " " + id, busyLabels[kind] || "Working…");
    setExtensionCardBusy(id, true, busyLabels[kind] || "Working…");
    try {
      const r = await fetchWithTimeout(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify(kind === "uninstall" ? { id, remove_data: true } : { id }),
      }, slowMs);
      const body = await r.json().catch(() => ({}));
      if (!r.ok || body.error) throw new Error(extensionApiError(body) || ("HTTP " + r.status));
      clearExtWarnNotices();
      // Keep local cache in sync immediately so detail/catalog buttons update even if a refresh races.
      patchExtensionCacheAfterAction(kind, id, body);
      await loadExtensionsCatalog(true);
      const viewingId = currentExtensionDetailId();
      if (kind === "uninstall") {
        showExtensionCatalogView();
        showTab("extensions", { preserveHash: true });
      } else if (kind === "enable" || kind === "update" || kind === "install" || kind === "disable") {
        // Stay on (or open) the detail page so Enable/Disable/Uninstall buttons refresh correctly.
        setExtNavSubmenuOpen(kind === "enable" || kind === "update" || kind === "install");
        showTab("extensions");
        showExtensionDetailView(id);
      } else if (viewingId === id) {
        showExtensionDetailView(id, { preserveHash: true });
      }
      pushExtNotice("ok", labels[kind] + " " + id, "Completed successfully.");
    } catch (e) {
      pushExtNotice("err", labels[kind] + " " + id + " failed", String(e.message || e));
      await loadExtensionsCatalog(true).catch(() => {});
      const viewingId = currentExtensionDetailId();
      if (viewingId === id && kind !== "uninstall") {
        showExtensionDetailView(id, { preserveHash: true });
      }
    } finally {
      setExtensionCardBusy(id, false);
    }
  }

  function currentExtensionDetailId() {
    const detail = $("panel-extension-detail");
    if (detail && detail.classList.contains("active")) {
      const fromDom = detail.querySelector("[data-ext-id]");
      if (fromDom && fromDom.dataset.extId) return fromDom.dataset.extId;
    }
    const raw = (location.hash || "").replace(/^#/, "");
    if (raw.startsWith("extensions/")) {
      return decodeURIComponent(raw.slice("extensions/".length));
    }
    return "";
  }

  function patchExtensionCacheAfterAction(kind, id, body) {
    const rows = extensionsCatalogCache || [];
    const idx = rows.findIndex((r) => r && r.id === id);
    if (kind === "uninstall") {
      if (idx >= 0) {
        rows[idx] = Object.assign({}, rows[idx], {
          enabled: false,
          installed: false,
          status: "available",
          update_available: false,
        });
      }
      return;
    }
    if (idx < 0) return;
    const next = Object.assign({}, rows[idx]);
    if (kind === "disable") {
      next.enabled = false;
      next.status = next.installed ? "installed" : "available";
    } else if (kind === "enable") {
      next.enabled = true;
      next.installed = true;
      next.status = "running";
    } else if (kind === "install" || kind === "update") {
      next.installed = true;
      if (body && body.result && body.result.version) next.installed_version = body.result.version;
    }
    rows[idx] = next;
    extensionsCatalogCache = rows;
  }

  function openExtensionPage(extId) {
    showTab("extensions");
    showExtensionDetailView(extId);
  }

  async function loadSettingsToolsPanel(force) {
    const root = $("st-tools-groups");
    if (!root) return;
    if (rpcCookbookCache && !force) {
      renderSettingsToolsPanel(rpcCookbookCache, ($("st-tools-search") && $("st-tools-search").value) || "");
      return;
    }
    wait(root, "Loading RPC catalog…", { compact: true });
    try {
      const r = await fetch("/api/rpc/cookbook", { cache: "no-store" });
      if (!r.ok) throw new Error("HTTP " + r.status);
      const data = await r.json();
      rpcCookbookCache = data.entries || [];
      renderSettingsToolsPanel(rpcCookbookCache, ($("st-tools-search") && $("st-tools-search").value) || "");
    } catch (e) {
      root.innerHTML = "<p class=\"label\">Failed to load RPC catalog: " + escapeHtml(String(e.message || e)) + "</p>";
    }
  }

  $("st-tools-search") && $("st-tools-search").addEventListener("input", () => {
    renderSettingsToolsPanel(rpcCookbookCache || [], $("st-tools-search").value);
  });

  async function runRPCConsole() {
    const method = ($("rpc-method") && $("rpc-method").value.trim()) || "";
    const raw = ($("rpc-params") && $("rpc-params").value.trim()) || "[]";
    const status = $("rpc-status");
    const out = $("rpc-out");
    if (!method) return;
    let params;
    try {
      params = JSON.parse(raw);
      if (!Array.isArray(params)) throw new Error("params must be a JSON array");
    } catch (e) {
      if (status) status.textContent = String(e);
      return;
    }
    if (status) wait(status, "Running RPC…", { inline: true });
    try {
      const r = await fetch("/api/rpc", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ method, params }),
        credentials: "same-origin",
        cache: "no-store",
      });
      const body = await r.json();
      if (out) {
        out.textContent = JSON.stringify(body, null, 2);
        out.classList.add("show");
      }
      if (status) status.textContent = body.error ? "RPC error" : "OK";
      if (!body.error && (method === "dogego_recoverheaders" || method === "generatetoaddress" || method === "generate")) {
        refresh();
      }
    } catch (e) {
      if (status) status.textContent = String(e);
    }
  }

  function initRPCConsole() {
    const row = $("rpc-presets");
    if (!row) return;
    RPC_PRESETS.forEach((p) => {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "btn btn-ghost btn-sm";
      b.textContent = p.label;
      b.addEventListener("click", () => applyRPCPreset(p));
      row.appendChild(b);
    });
    $("rpc-run") && $("rpc-run").addEventListener("click", runRPCConsole);
    $("rpc-method") && $("rpc-method").addEventListener("change", () => {
      applyRpcMethodFromCookbook($("rpc-method").value);
    });
    $("rpc-params") && $("rpc-params").addEventListener("keydown", (e) => {
      if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        runRPCConsole();
      }
    });
  }

  async function runCoreProbePreset(preset) {
    const out = $("rpc-out");
    const status = $("rpc-status");
    if (!out) return;
    out.textContent = "...";
    if (status) status.textContent = preset.label + "…";
    try {
      const r = await fetch(preset.path, { cache: "no-store" });
      const text = await r.text();
      let pretty = text;
      try { pretty = JSON.stringify(JSON.parse(text), null, 2); } catch (_) { /* */ }
      out.textContent = pretty;
      if (status) status.textContent = preset.label + " · HTTP " + r.status;
      if (r.ok) {
        try { applyCoreProbePresetUI(preset.path, JSON.parse(text)); } catch (_) { /* */ }
      }
    } catch (e) {
      out.textContent = String(e.message || e);
      if (status) status.textContent = "failed";
    }
  }

  function initCoreProbeConsole() {
    const row = $("core-probe-presets");
    if (!row) return;
    CORE_PROBE_PRESETS.forEach((p) => {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "btn btn-ghost btn-sm";
      b.textContent = p.label;
      b.addEventListener("click", () => runCoreProbePreset(p));
      row.appendChild(b);
    });
  }

  function decorateButtonsWithIcons() {
    const map = [
      { sel: "#ov-ex-clear", icon: "clear" },
      { sel: "#st-nav-reset", icon: "restart_alt" },
      { sel: "#st-sec-disable", icon: "lock_reset" },
      { sel: "#st-sec-save", icon: "save" },
      { sel: "#docs-md-close", icon: "close" },
      { sel: "#ov-header-recover-btn", icon: "restore" },
      { sel: "#ov-log-toggle", icon: "terminal" },
      { sel: "#st-reindextx", icon: "manage_search" },
      { sel: "#st-reindexfilters", icon: "filter_alt" },
    ];
    map.forEach((entry) => {
      const b = $(entry.sel.replace(/^#/, ""));
      if (!b) return;
      if (b.querySelector(".material-icons-round")) return;
      b.innerHTML = '<span class="material-icons-round" aria-hidden="true">' + entry.icon + "</span> " + b.innerHTML;
    });
    document.querySelectorAll("button.btn, button.quick-nav-btn").forEach((b) => {
      if (b.querySelector(".material-icons-round")) return;
      b.innerHTML = '<span class="material-icons-round" aria-hidden="true">arrow_forward</span> ' + b.innerHTML;
    });
  }

  function bindExplorerSearch(inputId, triggerId, outId, formId) {
    const input = $(inputId);
    const run = () => runExplorerSearch(input && input.value, outId);
    const form = formId ? $(formId) : null;
    if (form) {
      form.addEventListener("submit", (e) => {
        e.preventDefault();
        run();
      });
    }
    const btn = $(triggerId);
    if (btn && btn.type !== "submit") btn.addEventListener("click", (e) => { e.preventDefault(); run(); });
    if (input) {
      input.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          run();
        }
      });
    }
  }
  bindExplorerSearch("ov-ex-q", "ov-ex-go", "ov-ex-out", "ov-ex-form");
  bindExplorerSearch("lk-ex-q", "lk-ex-go", "lk-ex-out", "lk-ex-form");
  $("ov-header-recover-btn") && $("ov-header-recover-btn").addEventListener("click", recoverHeaderJournal);

  $("ov-ex-clear") && $("ov-ex-clear").addEventListener("click", () => {
    if ($("ov-ex-q")) $("ov-ex-q").value = "";
    const o = $("ov-ex-out");
    if (o) { o.innerHTML = ""; o.textContent = ""; o.hidden = true; o.classList.remove("show"); }
  });

  initRPCConsole();
  initRpcCookbook();
  initCoreProbeConsole();
  $("docs-search") && $("docs-search").addEventListener("input", filterDocsSections);
  $("docs-md-back") && $("docs-md-back").addEventListener("click", docsViewerBack);
  $("docs-md-close") && $("docs-md-close").addEventListener("click", docsViewerClose);
  async function runSettingsRPC(method, params, confirmText, msgEl, runningText) {
    if (confirmText && !confirm(confirmText)) return;
    if (msgEl) {
      msgEl.textContent = runningText || "Running…";
      msgEl.classList.add("show");
      msgEl.classList.remove("rpc-tool-result-err");
    }
    try {
      const r = await fetch("/api/rpc", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ method, params }),
        credentials: "same-origin",
      });
      const body = await r.json();
      if (body.error) {
        if (msgEl) {
          msgEl.textContent = body.error.message || JSON.stringify(body.error);
          msgEl.classList.add("show", "rpc-tool-result-err");
        }
        return;
      }
      if (msgEl) {
        const text = typeof body.result === "string" ? body.result : JSON.stringify(body.result, null, 2);
        msgEl.textContent = text;
        msgEl.classList.add("show");
        msgEl.classList.remove("rpc-tool-result-err");
      }
    } catch (e) {
      if (msgEl) {
        msgEl.textContent = String(e);
        msgEl.classList.add("show", "rpc-tool-result-err");
      }
    }
  }
  $("st-reindextx") && $("st-reindextx").addEventListener("click", async () => {
    const clear = $("st-reindextx-clear") && $("st-reindextx-clear").checked;
    await runSettingsRPC(
      "reindextx",
      [clear],
      "Rebuild transaction index from raw blocks? This can take a long time on mainnet.",
      $("st-reindextx-msg"),
      "Running reindextx…"
    );
  });
  $("st-reindexfilters") && $("st-reindexfilters").addEventListener("click", async () => {
    await runSettingsRPC(
      "reindexblockfilters",
      [],
      "Rebuild BIP158 block filters from raw blocks? Requires tx index; can take a long time on mainnet.",
      $("st-reindexfilters-msg"),
      "Running reindexblockfilters…"
    );
  });
  $("log-refresh") && $("log-refresh").addEventListener("click", () => {
    wait($("log-view"), "Fetching fresh logs…", { compact: true });
    loadLogs();
  });
  $("an-refresh-now") && $("an-refresh-now").addEventListener("click", () => {
    wait($("an-live-status"), "Refreshing analytics…", { inline: true });
    void loadAnalyticsPanel(true);
    void loadAnalyticsPeers(true);
  });
  $("an-timeline-range") && $("an-timeline-range").addEventListener("change", () => {
    sigDisk = "";
    sigMempoolTimeline = "";
    sigBlockSize = "";
    if (lastAnalyticsJson) renderAnalyticsDashboard(lastAnalyticsJson, lastSummary);
  });

  $("st-show-summary") && $("st-show-summary").addEventListener("change", (e) => {
    localStorage.setItem(LS_SUM, e.target.checked ? "1" : "0");
    refresh();
  });

  $("st-mode") && $("st-mode").addEventListener("change", updateSettingsModeUI);
  ["st-uacomment", "st-uacomment-tip-addr", "st-datadir", "st-network"].forEach((id) => {
    const el = $(id);
    if (el) el.addEventListener("input", scheduleUACommentPreview);
  });
  ["st-uacomment-publish-tip", "st-wallet-enabled", "st-nowallet"].forEach((id) => {
    const el = $(id);
    if (el) el.addEventListener("change", syncUACommentSettingsUI);
  });
  $("st-node-restart") && $("st-node-restart").addEventListener("click", () => { void confirmRestartNodeFromSettings(); });
  $("st-node-stop") && $("st-node-stop").addEventListener("click", () => { void stopNodeFromSettings(); });
  ["st-mining-start", "st-mining-stop", "st-mining-restart"].forEach((id) => {
    const el = $(id);
    if (!el) return;
    el.addEventListener("click", () => {
      const act = id.replace("st-mining-", "");
      void runServiceAction("mining", act);
    });
  });
  $("st-wallet-encrypt-btn") && $("st-wallet-encrypt-btn").addEventListener("click", async () => {
    const pass = ($("st-wallet-encrypt-pass") && $("st-wallet-encrypt-pass").value) || "";
    const confirm = ($("st-wallet-encrypt-confirm") && $("st-wallet-encrypt-confirm").value) || "";
    const msgEl = $("st-wallet-encrypt-msg");
    if (msgEl) wait(msgEl, "Encrypting…", { inline: true });
    try {
      const r = await fetch("/api/wallet/encrypt", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ passphrase: pass, confirm: confirm }),
      });
      const body = await r.json();
      if (!r.ok) {
        if (msgEl) msgEl.textContent = body.error || "Encrypt failed";
        return;
      }
      if (msgEl) msgEl.textContent = body.message || "Wallet encrypted.";
      if ($("st-wallet-encrypt-pass")) $("st-wallet-encrypt-pass").value = "";
      if ($("st-wallet-encrypt-confirm")) $("st-wallet-encrypt-confirm").value = "";
      refresh();
    } catch (e) {
      if (msgEl) msgEl.textContent = String(e);
    }
  });
  $("st-wallet-pass-change-btn") && $("st-wallet-pass-change-btn").addEventListener("click", async () => {
    const msgEl = $("st-wallet-encrypt-msg");
    if (msgEl) wait(msgEl, "Updating passphrase…", { inline: true });
    try {
      const r = await fetch("/api/wallet/passphrase-change", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({
          old_passphrase: ($("st-wallet-pass-old") && $("st-wallet-pass-old").value) || "",
          new_passphrase: ($("st-wallet-pass-new") && $("st-wallet-pass-new").value) || "",
          confirm: ($("st-wallet-pass-confirm") && $("st-wallet-pass-confirm").value) || "",
        }),
      });
      const body = await r.json();
      if (!r.ok) {
        if (msgEl) msgEl.textContent = body.error || "Passphrase change failed";
        if (body.wallet_locked && window.DogeGoWalletPassphrase) window.DogeGoWalletPassphrase.openUnlock();
        return;
      }
      if (msgEl) msgEl.textContent = body.message || "Passphrase changed.";
      ["st-wallet-pass-old", "st-wallet-pass-new", "st-wallet-pass-confirm"].forEach((id) => {
        const el = $(id);
        if (el) el.value = "";
      });
      refresh();
    } catch (e) {
      if (msgEl) msgEl.textContent = String(e);
    }
  });
  $("st-save") && $("st-save").addEventListener("click", async () => {
    const body = buildConfigFromForm();
    const r = await fetch("/api/config", { method: "POST", headers: { "Content-Type": "application/json" }, credentials: "same-origin", body: JSON.stringify(body) });
    if (r.ok) {
      dogegoSavedConfig = body;
      const warnings = {};
      try {
        const data = await r.json();
        if (data && data.autostart_warning) warnings.autostart = data.autostart_warning;
        if (data && data.uacomment_warning) warnings.uacomment = data.uacomment_warning;
      } catch (_) {}
      stMsg("Saved to dogecoinconf.json.", true);
      loadAutostartStatus();
      loadCoreCompare();
      loadConfigForm();
      showSettingsRestartModal({ warnings: warnings });
    } else {
      const errText = await r.text().catch(() => "");
      stMsg((errText || "Save failed") + " (HTTP " + r.status + ")", false);
    }
  });
  $("settings-restart-now") && $("settings-restart-now").addEventListener("click", () => {
    hideSettingsRestartModal();
    void restartNodeFromSettings();
  });
  $("settings-restart-later") && $("settings-restart-later").addEventListener("click", hideSettingsRestartModal);
  $("settings-restart-backdrop") && $("settings-restart-backdrop").addEventListener("click", hideSettingsRestartModal);
  $("st-tls-trust-btn") && $("st-tls-trust-btn").addEventListener("click", () => { void trustLocalTLS_CA(); });
  $("st-core-test") && $("st-core-test").addEventListener("click", testCoreConnection);
  $("st-signer-test") && $("st-signer-test").addEventListener("click", testSignerConnection);
  $("copy-addr") && $("copy-addr").addEventListener("click", async () => {
    const a = $("recv-addr") && $("recv-addr").textContent;
    if (!a || a === EMPTY || a.startsWith("Wallet")) return;
    try {
      await navigator.clipboard.writeText(a.trim());
      const done = $("copy-done");
      if (done) {
        done.hidden = false;
        setTimeout(() => { done.hidden = true; }, 2200);
      }
    } catch (_) { /* */ }
  });

  function refreshWalletAddressBookIfOpen() {
    invalidateWalletAddressBook();
    if (isReceiveBookTabActive()) void loadWalletAddressBook(true);
  }

  $("wallet-ab-search-form") && $("wallet-ab-search-form").addEventListener("submit", (ev) => ev.preventDefault());

  $("wallet-ab-filter") && $("wallet-ab-filter").addEventListener("input", () => {
    const clearBtn = $("wallet-ab-filter-clear");
    if (clearBtn) clearBtn.hidden = !($("wallet-ab-filter").value.trim());
    clearTimeout(walletAbFilterTimer);
    walletAbFilterTimer = setTimeout(() => {
      if (walletAddressBookLoaded) renderAddressBookTable(walletAddressBookRows);
    }, 120);
  });

  $("wallet-ab-filter-clear") && $("wallet-ab-filter-clear").addEventListener("click", () => {
    const inp = $("wallet-ab-filter");
    if (inp) inp.value = "";
    const clearBtn = $("wallet-ab-filter-clear");
    if (clearBtn) clearBtn.hidden = true;
    if (walletAddressBookLoaded) renderAddressBookTable(walletAddressBookRows);
  });

  $("wallet-ab-type") && $("wallet-ab-type").addEventListener("change", () => {
    if (walletAddressBookLoaded) renderAddressBookTable(walletAddressBookRows);
  });

  function walletAddressWarmupError(err) {
    const msg = String((err && err.message) || err || "").toLowerCase();
    return msg.includes("not implemented") || msg.includes("not ready") || msg.includes("still starting");
  }

  async function walletAddressPost(path, body, attempt) {
    const r = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
      credentials: "same-origin",
    });
    const data = await r.json();
    if (!r.ok) {
      const err = (data && data.error && (data.error.message || data.error)) || data.error || r.statusText;
      const e = new Error(typeof err === "string" ? err : JSON.stringify(err));
      if (walletAddressWarmupError(e) && attempt < 4) {
        await new Promise((resolve) => setTimeout(resolve, 1500));
        return walletAddressPost(path, body, attempt + 1);
      }
      throw e;
    }
    return data.result != null ? data.result : data;
  }

  let walletAbNewAddr = "";
  let walletAbNewModalBusy = false;

  function closeWalletAbNewModal() {
    const hadAddr = !!walletAbNewAddr;
    const modal = $("wallet-ab-new-modal");
    if (modal) modal.hidden = true;
    walletAbNewAddr = "";
    const loading = $("wallet-ab-new-loading");
    const content = $("wallet-ab-new-content");
    const labelInp = $("wallet-ab-new-label");
    const doneBtn = $("wallet-ab-new-done");
    const errEl = $("wallet-ab-new-err");
    if (content) content.hidden = true;
    if (labelInp) labelInp.value = "";
    if (doneBtn) doneBtn.disabled = true;
    if (errEl) { errEl.hidden = true; errEl.textContent = ""; }
    if (loading) {
      loading.hidden = false;
      wait(loading, i18n("pages.receive.abNewGenerating"), { compact: true });
    }
    if (hadAddr) {
      refreshWalletAddressBookIfOpen();
      refresh();
    }
  }

  async function finishWalletAbNewModal() {
    const doneBtn = $("wallet-ab-new-done");
    const labelInp = $("wallet-ab-new-label");
    const errEl = $("wallet-ab-new-err");
    const label = labelInp ? labelInp.value.trim() : "";
    if (doneBtn) doneBtn.disabled = true;
    try {
      if (walletAbNewAddr && label) {
        await walletAddressPost("/api/wallet/address/label", { address: walletAbNewAddr, label: label });
      }
      closeWalletAbNewModal();
    } catch (e) {
      if (errEl) {
        errEl.textContent = String(e.message || e);
        errEl.hidden = false;
      }
      if (doneBtn) doneBtn.disabled = false;
    }
  }

  async function openWalletAbNewModal() {
    if (walletAbNewModalBusy) return;
    walletAbNewModalBusy = true;
    const modal = $("wallet-ab-new-modal");
    const loading = $("wallet-ab-new-loading");
    const content = $("wallet-ab-new-content");
    const addrEl = $("wallet-ab-new-addr");
    const doneBtn = $("wallet-ab-new-done");
    const labelInp = $("wallet-ab-new-label");
    const errEl = $("wallet-ab-new-err");
    if (errEl) { errEl.hidden = true; errEl.textContent = ""; }
    if (content) content.hidden = true;
    if (doneBtn) doneBtn.disabled = true;
    if (labelInp) labelInp.value = "";
    if (addrEl) addrEl.textContent = "…";
    if (loading) {
      loading.hidden = false;
      wait(loading, i18n("pages.receive.abNewGenerating"), { compact: true });
    }
    if (modal) modal.hidden = false;
    try {
      const result = await walletAddressPost("/api/wallet/address/new", {}, 0);
      walletAbNewAddr = typeof result === "string" ? result : (result && result.address) || String(result || "");
      if (addrEl) addrEl.textContent = walletAbNewAddr;
      if (loading) loading.hidden = true;
      if (content) content.hidden = false;
      if (doneBtn) doneBtn.disabled = false;
      if (labelInp) labelInp.focus();
    } catch (e) {
      if (loading) loading.hidden = true;
      alert(i18n("pages.receive.abGenerateFailed") + " " + String(e.message || e));
      closeWalletAbNewModal();
    } finally {
      walletAbNewModalBusy = false;
    }
  }

  $("wallet-ab-new") && $("wallet-ab-new").addEventListener("click", () => {
    void openWalletAbNewModal();
  });
  $("wallet-keypool-refill-btn") && $("wallet-keypool-refill-btn").addEventListener("click", async () => {
    const btn = $("wallet-keypool-refill-btn");
    if (!btn || btn.disabled) return;
    btn.disabled = true;
    const labelEl = btn.querySelector("span:last-child");
    const prev = labelEl ? labelEl.textContent : "";
    try {
      if (labelEl) labelEl.textContent = i18n("pages.receive.keypoolRefilling");
      const data = await walletKeypoolRefill();
      if (labelEl) {
        labelEl.textContent = i18n("pages.receive.keypoolRefillOk") +
          (data && data.keypool_size != null ? " · keypool " + data.keypool_size : "");
      }
      refresh();
      refreshWalletAddressBookIfOpen();
    } catch (e) {
      if (labelEl) labelEl.textContent = String(e);
    } finally {
      window.setTimeout(() => {
        if (labelEl) labelEl.textContent = prev || i18n("pages.receive.keypoolRefill");
        updateKeypoolRefillButton(lastSummary, lastWalletSnap);
      }, 2500);
    }
  });

  $("wallet-ab-new-cancel") && $("wallet-ab-new-cancel").addEventListener("click", closeWalletAbNewModal);
  $("wallet-ab-new-backdrop") && $("wallet-ab-new-backdrop").addEventListener("click", closeWalletAbNewModal);
  $("wallet-ab-new-done") && $("wallet-ab-new-done").addEventListener("click", () => void finishWalletAbNewModal());
  $("wallet-ab-new-copy") && $("wallet-ab-new-copy").addEventListener("click", () => {
    void copyAddressBookText(walletAbNewAddr, $("wallet-ab-new-copy"));
  });

  $("wallet-ab-new-label") && $("wallet-ab-new-label").addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      ev.preventDefault();
      const doneBtn = $("wallet-ab-new-done");
      if (doneBtn && !doneBtn.disabled) void finishWalletAbNewModal();
    }
  });

  $("wallet-address-list") && $("wallet-address-list").addEventListener("click", async (ev) => {
    const copyBtn = ev.target.closest(".wallet-ab-copy");
    if (copyBtn) {
      ev.preventDefault();
      await copyAddressBookText(copyBtn.getAttribute("data-copy") || "", copyBtn);
      return;
    }
    const editBtn = ev.target.closest(".wallet-ab-label-edit");
    if (editBtn) {
      ev.preventDefault();
      showAddressBookLabelEditor(editBtn.getAttribute("data-addr") || "", editBtn.getAttribute("data-label") || "");
      return;
    }
    const cancelBtn = ev.target.closest(".wallet-ab-label-cancel");
    if (cancelBtn) {
      ev.preventDefault();
      renderAddressBookTable(walletAddressBookRows);
    }
  });

  $("wallet-address-list") && $("wallet-address-list").addEventListener("submit", async (ev) => {
    const form = ev.target.closest(".wallet-ab-label-form");
    if (!form) return;
    ev.preventDefault();
    const addr = form.getAttribute("data-addr") || "";
    const input = form.querySelector(".wallet-ab-label-input");
    const label = input ? input.value.trim() : "";
    try {
      await saveAddressBookLabel(addr, label);
    } catch (e) {
      alert(String(e.message || e));
      renderAddressBookTable(walletAddressBookRows);
    }
  });

  function formatSendError(body, amt) {
    const code = body && body.code;
    const msg = (body && body.error) || "Send failed";
    const wal = lastWalletSnap || {};
    const imm = Number(wal.immature_balance) || 0;
    const bal = Number(wal.balance) || 0;
    if (code === -13 || (body && body.wallet_locked)) {
      return "Wallet is locked. Enter your wallet passphrase to spend (Core walletpassphrase, or use Unlock & send).";
    }
    if (code === -6 && (body && body.immature_only || (imm > 0 && bal < 1e-8))) {
      return (
        "Mining rewards are not spendable yet (coinbase maturity ~240 blocks on testnet). Spendable: " +
        formatDOGE(bal, 4) +
        " DOGE; immature: " +
        formatDOGE(imm, 4) +
        " DOGE. Raising the fee rate will not help until coinbases mature." +
        (code != null ? " (code " + code + ")" : "")
      );
    }
    if (body && body.fee_hint) {
      return body.fee_hint + (code != null ? " (code " + code + ")" : "");
    }
    if (code === -6) {
      let hint = "Not enough spendable UTXOs for amount + network fee.";
      if (imm > 0 && bal < amt) {
        hint = "Your balance is mostly immature mining rewards (~240 confirmations required on testnet). Available: " + formatDOGE(bal, 4) + " DOGE; immature: " + formatDOGE(imm, 4) + " DOGE.";
      } else if (bal > 0 && amt > bal) {
        hint = "Amount exceeds spendable balance (" + formatDOGE(bal, 4) + " DOGE). Leave room for the network fee or use Max.";
      }
      return hint + " (code -6)";
    }
    return msg + (code != null ? " (code " + code + ")" : "");
  }

  function sendErrorHTML(body, amt) {
    const text = formatSendError(body, amt);
    if (!body || body.suggested_fee_rate == null || body.immature_only) {
      return null;
    }
    const rate = Number(body.suggested_fee_rate);
    const est = body.estimated_fee_doge != null ? formatDOGE(body.estimated_fee_doge, 4) : estimateSendFeeDOGE(rate).toFixed(4);
    pendingSendRetry = { rate: rate };
    return (
      "<p>" + escHtml(text) + "</p>" +
      '<button type="button" class="btn btn-secondary" id="send-apply-fee-retry">Use ~' +
      formatDOGE(rate, 6) + "/kB (~" + est + " DOGE) and send again</button>"
    );
  }

  document.addEventListener("click", (ev) => {
    const btn = ev.target.closest("#send-apply-fee-retry");
    if (!btn || !pendingSendRetry) return;
    const feeEl = $("send-fee-rate");
    if (feeEl) {
      feeEl.value = String(pendingSendRetry.rate);
      feeEl.disabled = false;
      updateSendFeeEstimate();
    }
    pendingSendRetry = null;
    const sendBtn = $("send-btn");
    if (sendBtn && !sendBtn.disabled) sendBtn.click();
  });

  let sendConfirmTimer = null;
  let sendConfirmSeconds = 0;

  function closeSendConfirmModal() {
    const modal = $("send-confirm-modal");
    if (modal) modal.hidden = true;
    if (sendConfirmTimer) {
      clearInterval(sendConfirmTimer);
      sendConfirmTimer = null;
    }
    sendConfirmSeconds = 0;
    const arm = $("send-confirm-arm");
    const go = $("send-confirm-go");
    if (arm) arm.checked = false;
    if (go) go.disabled = true;
  }

  function refreshSendConfirmButton() {
    const arm = $("send-confirm-arm");
    const go = $("send-confirm-go");
    const cd = $("send-confirm-countdown");
    const ready = sendConfirmSeconds <= 0 && arm && arm.checked;
    if (go) go.disabled = !ready;
    if (cd) {
      if (sendConfirmSeconds > 0) {
        cd.textContent = "Confirm unlocks in " + sendConfirmSeconds + "s…";
        cd.hidden = false;
      } else if (!arm || !arm.checked) {
        cd.textContent = "";
        cd.hidden = true;
      } else {
        cd.textContent = "";
        cd.hidden = true;
      }
    }
  }

  function openSendConfirmModal(dest, amt) {
    const modal = $("send-confirm-modal");
    if (!modal) return;
    const toEl = $("send-confirm-to");
    const amtEl = $("send-confirm-amt");
    const feeEl = $("send-confirm-fee");
    if (toEl) toEl.textContent = dest;
    if (amtEl) amtEl.textContent = formatDOGE(amt, 8) + " DOGE";
    if (feeEl) feeEl.textContent = formatDOGE(estimateSendFeeDOGE(currentSendFeeRate()), 8) + " DOGE";
    const arm = $("send-confirm-arm");
    if (arm) arm.checked = false;
    sendConfirmSeconds = 3;
    refreshSendConfirmButton();
    modal.hidden = false;
    void loadSendUtxos(true);
    if (sendConfirmTimer) clearInterval(sendConfirmTimer);
    sendConfirmTimer = setInterval(() => {
      sendConfirmSeconds -= 1;
      refreshSendConfirmButton();
      if (sendConfirmSeconds <= 0) {
        clearInterval(sendConfirmTimer);
        sendConfirmTimer = null;
      }
    }, 1000);
  }

  async function executeWalletSend(dest, amt) {
    showSendResult(true, "Signing and broadcasting…");
    const btn = $("send-btn");
    if (btn) btn.disabled = true;
    let pendingId = null;
    if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackSigning) {
      pendingId = window.DogeGoTxFlight.trackSigning({ amount: amt, address: dest });
    }
    const payload = { address: dest, amount: amt };
    const feeCustom = $("send-fee-rate") && $("send-fee-rate").value;
    const feeNum = parseFloat(feeCustom);
    if (isFinite(feeNum) && feeNum > 0) payload.fee_rate = feeNum;
    if ($("send-subtract-fee") && $("send-subtract-fee").checked) {
      payload.subtract_fee_from_amount = true;
    }
    const inputs = selectedSendUtxos();
    if (inputs.length) payload.inputs = inputs;
    const pqHex = $("send-pq-commit") && $("send-pq-commit").value.trim();
    const carrierMode = document.querySelector('input[name="send-pq-mode"][value="carrier"]');
    if (carrierMode && carrierMode.checked) {
      payload.pq_mode = "carrier";
      payload.pq_tag = $("send-pq-tag") ? $("send-pq-tag").value : "FLC1";
    } else if (pqHex) {
      payload.pq_tag = $("send-pq-tag") ? $("send-pq-tag").value : "FLC1";
      payload.pq_commitment = pqHex.replace(/^0x/i, "");
    }
    const sendTimer = setTimeout(() => {
      if (pendingId && window.DogeGoTxFlight && window.DogeGoTxFlight.updateSigningFlight) {
        window.DogeGoTxFlight.updateSigningFlight(pendingId, "Still working (coin select / broadcast)…");
      }
    }, 4000);
    try {
      const r = await fetchAPI("/api/wallet/send", 120000, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const body = await r.json().catch(() => ({}));
      if (r.ok && body.txid) {
        if (pendingId && window.DogeGoTxFlight && window.DogeGoTxFlight.dismiss) {
          window.DogeGoTxFlight.dismiss(pendingId);
        }
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackSend) {
          window.DogeGoTxFlight.trackSend({
            txid: body.txid,
            hex: body.hex,
            amount: amt,
            address: dest,
            status: body.status || "broadcasting",
            broadcast_error: body.broadcast_error,
          });
        }
        patchWalletTxFromSendResponse(body);
        const st = body.status === "mempool" ? "accepted to mempool" : "broadcast in progress";
        showSendResult(true, "Transaction submitted (" + st + ").\nTrack live status in the bar at the top.\nTxid: " + body.txid);
        if ($("send-amt")) $("send-amt").value = "";
        document.querySelectorAll(".send-utxo-cb:checked").forEach((cb) => { cb.checked = false; });
        refresh();
        if (isPanelActive("transactions")) {
          void loadWalletTxHistoryPage(true);
        }
      } else if (isRetryableSendError(null, body, r.status)) {
        const qid = enqueueSend(payload);
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackQueued) {
          window.DogeGoTxFlight.trackQueued(pendingId || qid, { amount: amt, address: dest });
        } else if (pendingId && window.DogeGoTxFlight && window.DogeGoTxFlight.dismiss) {
          window.DogeGoTxFlight.dismiss(pendingId);
        }
        showSendResult(true, "Send queued. It will broadcast automatically when the node is ready.");
        void processSendQueue();
      } else {
        if (body && (body.code === -13 || body.wallet_locked)) {
          const unlocked = await ensureWalletUnlockedForSend();
          if (unlocked) {
            return executeWalletSend(dest, amt);
          }
        }
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackFailure) {
          window.DogeGoTxFlight.trackFailure(pendingId, {
            amount: amt,
            address: dest,
            error: formatSendError(body, amt),
          });
        } else if (pendingId && window.DogeGoTxFlight && window.DogeGoTxFlight.dismiss) {
          window.DogeGoTxFlight.dismiss(pendingId);
        }
        const html = sendErrorHTML(body, amt);
        if (html) showSendResult(false, "", html);
        else showSendResult(false, formatSendError(body, amt));
      }
    } catch (e) {
      if (isRetryableSendError(e, null, 0)) {
        const qid = enqueueSend(payload);
        if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackQueued) {
          window.DogeGoTxFlight.trackQueued(pendingId || qid, { amount: amt, address: dest });
        } else if (pendingId && window.DogeGoTxFlight && window.DogeGoTxFlight.dismiss) {
          window.DogeGoTxFlight.dismiss(pendingId);
        }
        showSendResult(true, "Send queued. It will broadcast automatically when the node is ready.");
        void processSendQueue();
      } else if (window.DogeGoTxFlight && window.DogeGoTxFlight.trackFailure) {
        window.DogeGoTxFlight.trackFailure(pendingId, {
          amount: amt,
          address: dest,
          error: friendlyAPIError(e),
        });
      } else if (pendingId && window.DogeGoTxFlight && window.DogeGoTxFlight.dismiss) {
        window.DogeGoTxFlight.dismiss(pendingId);
      }
      showSendResult(false, friendlyAPIError(e));
    } finally {
      clearTimeout(sendTimer);
      validateSendForm();
      void refreshWalletPanelAsync(refreshGen);
    }
  }

  $("send-confirm-cancel") && $("send-confirm-cancel").addEventListener("click", closeSendConfirmModal);
  $("send-confirm-backdrop") && $("send-confirm-backdrop").addEventListener("click", closeSendConfirmModal);
  $("send-confirm-arm") && $("send-confirm-arm").addEventListener("change", refreshSendConfirmButton);
  $("send-confirm-go") && $("send-confirm-go").addEventListener("click", () => {
    const dest = $("send-to") && $("send-to").value.trim();
    const amt = parseFloat($("send-amt") && $("send-amt").value);
    closeSendConfirmModal();
    if (dest && isFinite(amt) && amt > 0) void executeWalletSend(dest, amt);
  });

  $("send-btn") && $("send-btn").addEventListener("click", () => {
    const dest = $("send-to") && $("send-to").value.trim();
    const amtRaw = $("send-amt") && $("send-amt").value;
    const amt = parseFloat(amtRaw);
    if (!dest || !isFinite(amt) || amt <= 0) {
      showSendResult(false, "Enter a valid address and positive amount.");
      return;
    }
    void (async () => {
      const ok = await ensureWalletUnlockedForSend();
      if (!ok) return;
      openSendConfirmModal(dest, amt);
    })();
  });

  $("send-max-btn") && $("send-max-btn").addEventListener("click", () => {
    const wal = lastWalletSnap;
    if (!wal || typeof wal.balance !== "number") return;
    const subtract = $("send-subtract-fee") && $("send-subtract-fee").checked;
    const fee = estimateSendFeeDOGE(currentSendFeeRate());
    const max = subtract ? wal.balance : Math.max(0, wal.balance - fee);
    if ($("send-amt")) $("send-amt").value = max > 0 ? String(Number(max.toFixed(8))) : "0";
    validateSendForm();
  });

  ["send-to", "send-amt", "send-fee-rate"].forEach((id) => {
    const el = $(id);
    if (el) el.addEventListener("input", validateSendForm);
  });
  $("send-subtract-fee") && $("send-subtract-fee").addEventListener("change", validateSendForm);
  $("tx-history-filter") && $("tx-history-filter").addEventListener("input", () => {
    clearTimeout(txHistoryFilterTimer);
    txHistoryFilterTimer = setTimeout(() => rerenderWalletTxHistory(), 300);
  });
  $("tx-history-clear") && $("tx-history-clear").addEventListener("click", () => {
    const inp = $("tx-history-filter");
    if (inp) inp.value = "";
    rerenderWalletTxHistory();
    if (inp) inp.focus();
  });
  document.querySelectorAll(".wallet-tx-type-filter").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".wallet-tx-type-filter").forEach((b) => {
        b.classList.remove("active");
        b.setAttribute("aria-selected", "false");
      });
      btn.classList.add("active");
      btn.setAttribute("aria-selected", "true");
      lastTxTypeFilter = btn.getAttribute("data-tx-type") || "all";
      rerenderWalletTxHistory();
    });
  });
  $("tx-history-form") && $("tx-history-form").addEventListener("submit", (e) => e.preventDefault());
  $("tx-history-export") && $("tx-history-export").addEventListener("click", exportWalletTxHistoryCSV);
  $("wallet-tx-load-all") && $("wallet-tx-load-all").addEventListener("click", () => void loadAllWalletTxHistory());
  $("wallet-tx-sheet-close") && $("wallet-tx-sheet-close").addEventListener("click", closeWalletTxSheet);
  $("wallet-tx-sheet-backdrop") && $("wallet-tx-sheet-backdrop").addEventListener("click", closeWalletTxSheet);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && $("wallet-tx-sheet") && !$("wallet-tx-sheet").hidden) closeWalletTxSheet();
  });
  document.querySelector('[data-subtabs="send"]') && document.querySelector('[data-subtabs="send"]').addEventListener("click", (ev) => {
    const btn = ev.target.closest("button[data-sd-sub]");
    if (btn && btn.getAttribute("data-sd-sub") === "advanced") loadSendUtxos(true);
  });

  async function fetchExplorer(path, outId) {
    const out = $(outId);
    if (!out) return;
    out.classList.add("show");
    wait(out, "Fetching from your node…");
    try {
      const r = await fetch(path, { cache: "no-store" });
      out.textContent = await r.text();
      try { out.textContent = JSON.stringify(JSON.parse(out.textContent), null, 2); } catch (_) {}
    } catch (e) {
      out.textContent = String(e);
    }
  }

  $("lk-h-go") && $("lk-h-go").addEventListener("click", () => {
    const h = $("lk-h-height") && $("lk-h-height").value;
    const hash = $("lk-h-hash") && $("lk-h-hash").value.trim();
    let url = "/api/explorer/header?";
    if (hash) url += "hash=" + encodeURIComponent(hash);
    else url += "height=" + encodeURIComponent(h || "0");
    fetchExplorer(url, "lk-h-out");
  });
  $("lk-b-go") && $("lk-b-go").addEventListener("click", () => {
    const h = $("lk-b-height") && $("lk-b-height").value;
    const hash = $("lk-b-hash") && $("lk-b-hash").value.trim();
    let url = "/api/explorer/block?";
    if (hash) url += "hash=" + encodeURIComponent(hash);
    else url += "height=" + encodeURIComponent(h || "0");
    fetchExplorer(url, "lk-b-out");
  });
  $("lk-tx-go") && $("lk-tx-go").addEventListener("click", () => {
    const id = $("lk-txid") && $("lk-txid").value.trim();
    fetchExplorer("/api/explorer/tx?txid=" + encodeURIComponent(id), "lk-tx-out");
  });
  $("lk-decode-go") && $("lk-decode-go").addEventListener("click", () => {
    const hex = $("lk-hex") && $("lk-hex").value.trim();
    fetchExplorer("/api/explorer/decode?hex=" + encodeURIComponent(hex), "lk-decode-out");
  });

  async function walletRPC(method, params) {
    const r = await fetch("/api/rpc", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ method, params: params || [] }),
      credentials: "same-origin",
    });
    const body = await r.json();
    if (body.error) throw new Error(body.error.message || JSON.stringify(body.error));
    return body.result;
  }

  function pqFilterValue() {
    const el = document.querySelector('input[name="pq-filter"]:checked');
    return el ? el.value : "all";
  }

  async function scanPQCommitments() {
    const out = $("pq-scan-out");
    if (!out) return;
    wait(out, "Scanning mempool for PQ tags…");
    const filter = pqFilterValue();
    try {
      const r = await fetch("/api/mempool?limit=200", { cache: "no-store" });
      const mp = await r.json();
      const txs = Array.isArray(mp.transactions) ? mp.transactions : [];
      const hits = [];
      for (const row of txs) {
        const txid = row.txid;
        if (!txid) continue;
        let tx;
        try {
          tx = await walletRPC("getrawtransaction", [txid, true]);
        } catch (_) { continue; }
        const vouts = tx && tx.vout;
        if (!Array.isArray(vouts)) continue;
        for (let i = 0; i < vouts.length; i++) {
          const spk = vouts[i].scriptPubKey;
          const hex = spk && (spk.hex || spk.script);
          if (!hex || !/^6a/i.test(hex)) continue;
          let valid = false;
          let detail = null;
          try {
            detail = await walletRPC("dogego_verifypqcommitment", [hex]);
            valid = !!(detail && detail.valid === true);
          } catch (e) {
            detail = { valid: false, error: String(e) };
          }
          if (filter === "valid" && !valid) continue;
          if (filter === "invalid" && valid) continue;
          hits.push({ txid, vout: i, valid, detail, tag: detail && detail.tag });
        }
      }
      if (!hits.length) {
        out.innerHTML = "<p class=\"label\">No PQ commitments found in mempool sample.</p>";
        return;
      }
      out.innerHTML = hits.map((h) => {
        const cls = h.valid ? "pq-hit-valid" : "pq-hit-invalid";
        const status = h.valid ? "valid" : "invalid";
        const tag = h.tag ? " · " + h.tag : "";
        return "<div class=\"pq-hit " + cls + "\"><span class=\"mono\">" + h.txid + "</span> vout " + h.vout +
          " · <strong>" + status + "</strong>" + tag +
          " <button type=\"button\" class=\"btn btn-ghost btn-sm quick-nav-btn\" data-tab=\"explorer\">Open</button></div>";
      }).join("");
    } catch (e) {
      out.innerHTML = "<p class=\"alert err show\">" + String(e) + "</p>";
    }
  }

  $("pq-scan-btn") && $("pq-scan-btn").addEventListener("click", scanPQCommitments);

  $("import-wif-btn") && $("import-wif-btn").addEventListener("click", async () => {
    const out = $("import-wif-out");
    const wif = $("import-wif") && $("import-wif").value.trim();
    const label = $("import-label") && $("import-label").value.trim();
    if (!out || !wif) return;
    out.classList.add("show");
    wait(out, "Importing private key…", { compact: true });
    try {
      const params = label ? [wif, label, true] : [wif, "", true];
      await walletRPC("importprivkey", params);
      out.textContent = i18n("pages.receive.importWifOk");
      if ($("import-wif")) $("import-wif").value = "";
      refresh();
      refreshWalletAddressBookIfOpen();
    } catch (e) {
      out.textContent = String(e);
    }
  });

  function walletDatPoolSuffix(p) {
    if (!p || !p.pool_count) return "";
    let s = " pool=" + p.pool_count;
    if (p.pool_pubkeys) s += " pool_pubkeys=" + p.pool_pubkeys;
    if (p.pool_keys_matched) s += " pool_keys_matched=" + p.pool_keys_matched;
    if (p.pool_keys_unmatched) s += " pool_keys_unmatched=" + p.pool_keys_unmatched;
    if (p.pool_indices_replayed != null) s += " pool_indices_replayed=" + p.pool_indices_replayed;
    if (p.pool_core_indices_stored) s += " pool_core_indices_stored=" + p.pool_core_indices_stored;
    if (p.keypool_refill_size) s += " keypool_refill_size=" + p.keypool_refill_size;
    if (p.pool_entries && p.pool_entries.length) s += " pool_entries=" + p.pool_entries.length;
    if (p.pool_entries_truncated) s += " pool_entries_truncated";
    if (p.pool_index_min != null && p.pool_index_max != null) {
      s += p.pool_index_min === p.pool_index_max
        ? " pool_idx=" + p.pool_index_min
        : " pool_idx=" + p.pool_index_min + "-" + p.pool_index_max;
    }
    return s;
  }

  async function walletImportExtended(type, body) {
    const r = await fetch("/api/wallet/import", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify(body),
    });
    const data = await r.json();
    if (!r.ok) {
      const err = (data && data.error && data.error.message) || (data && data.error) || r.statusText;
      throw new Error(typeof err === "string" ? err : JSON.stringify(err));
    }
    return data.result || data;
  }

  function isReceiveBookTabActive() {
    const panel = document.querySelector('[data-rc-panel="book"]');
    return !!(panel && !panel.hidden);
  }

  function invalidateWalletAddressBook() {
    walletAddressBookSig = "";
  }

  function addressBookPathIndex(path) {
    const m = String(path || "").match(/\/(\d+)$/);
    return m ? parseInt(m[1], 10) : 999999;
  }

  function addressBookTypeRank(row) {
    if (row.isnodetip) return 1;
    if (row.hdpath) return row.ischange ? 2 : 0;
    if (row.watchonly) return 3;
    if (row.cosigner) return 4;
    return 5;
  }

  function sortAddressBookRows(rows) {
    return rows.slice().sort((a, b) => {
      const ta = addressBookTypeRank(a);
      const tb = addressBookTypeRank(b);
      if (ta !== tb) return ta - tb;
      const ia = addressBookPathIndex(a.hdpath);
      const ib = addressBookPathIndex(b.hdpath);
      if (ia !== ib) return ia - ib;
      const pa = a.hdpath || "";
      const pb = b.hdpath || "";
      if (pa !== pb) return pa < pb ? -1 : 1;
      return String(a.address || "").localeCompare(String(b.address || ""));
    });
  }

  function addressBookRowTags(row) {
    const tags = [];
    if (row.hdpath) {
      tags.push(row.hdpath);
      if (row.isnodetip) tags.push("node tip");
      else tags.push(row.ischange ? "change" : "receive");
    }
    if (row.iskeypool) tags.push("keypool");
    if (row.hd_keypool_core_index != null && row.hd_keypool_core_index !== "") {
      tags.push("core pool #" + row.hd_keypool_core_index);
    }
    if (row.watchonly) tags.push("watch");
    if (row.cosigner) tags.push("cosigner");
    return tags.join(" · ");
  }

  function addressBookFilterText() {
    const el = $("wallet-ab-filter");
    return el ? el.value.trim().toLowerCase() : "";
  }

  function addressBookTypeFilter() {
    const el = $("wallet-ab-type");
    return el ? el.value : "all";
  }

  function addressBookRowMatchesType(row, type) {
    if (!type || type === "all") return true;
    if (type === "receive") return !!row.hdpath && !row.ischange && !row.isnodetip;
    if (type === "nodetip") return !!row.isnodetip;
    if (type === "change") return !!row.ischange;
    if (type === "watch") return !!row.watchonly;
    if (type === "cosigner") return !!row.cosigner;
    if (type === "keypool") return !!row.iskeypool;
    return true;
  }

  function addressBookRowMatches(row, q) {
    if (!q) return true;
    const hay = [
      row.address,
      row.hdpath,
      row.label,
      row.ischange ? "change" : "receive",
      row.isnodetip ? "node tip" : "",
      row.watchonly ? "watch" : "",
      row.cosigner ? "cosigner" : "",
      row.iskeypool ? "keypool" : "",
      row.hd_keypool_core_index != null ? "core pool " + row.hd_keypool_core_index : "",
    ].filter(Boolean).join(" ").toLowerCase();
    return hay.indexOf(q) >= 0;
  }

  function addressBookRowsSignature(rows) {
    return JSON.stringify(rows.map((r) => ({
      a: r.address,
      p: r.hdpath,
      c: !!r.ischange,
      nt: !!r.isnodetip,
      l: r.label || "",
      w: !!r.watchonly,
      cs: !!r.cosigner,
      k: !!r.iskeypool,
      ci: r.hd_keypool_core_index,
    })));
  }

  function renderAddressBookTable(rows) {
    const el = $("wallet-address-list");
    if (!el) return;
    const q = addressBookFilterText();
    const type = addressBookTypeFilter();
    const filtered = sortAddressBookRows(rows).filter((r) => addressBookRowMatches(r, q) && addressBookRowMatchesType(r, type));
    if (!rows.length) {
      el.className = "label";
      el.textContent = i18n("pages.receive.abEmpty");
      return;
    }
    if (!filtered.length) {
      el.className = "label";
      el.textContent = i18n("pages.receive.abNoMatch");
      return;
    }
    el.className = "wallet-address-table-wrap";
    const copyLabel = i18n("pages.receive.abCopy");
    const editTitle = i18n("pages.receive.abEditLabel");
    el.innerHTML = "<table class=\"wallet-address-table\"><thead><tr><th>" + escapeHtml(i18n("pages.receive.abColAddress")) + "</th><th>" + escapeHtml(i18n("pages.receive.abColPath")) + "</th><th>" + escapeHtml(i18n("pages.receive.abColLabel")) + "</th></tr></thead><tbody>" +
      filtered.map((row) => {
        const addr = row.address || "";
        const tags = addressBookRowTags(row);
        const label = row.label || "";
        return "<tr data-ab-addr=\"" + escapeHtml(addr) + "\">" +
          "<td><div class=\"wallet-ab-addr-cell\"><span class=\"mono\">" + escapeHtml(addr) + "</span>" +
          "<button type=\"button\" class=\"btn btn-ghost btn-sm wallet-ab-copy\" data-copy=\"" + escapeHtml(addr) + "\" title=\"" + escapeHtml(copyLabel) + "\">" +
          "<span class=\"material-icons-round\" aria-hidden=\"true\">content_copy</span> " + escapeHtml(copyLabel) + "</button></div></td>" +
          "<td>" + escapeHtml(tags) + "</td>" +
          "<td class=\"wallet-ab-label-cell\"><div class=\"wallet-ab-label-view\">" +
          "<span class=\"wallet-ab-label-text\">" + escapeHtml(label) + "</span>" +
          "<button type=\"button\" class=\"btn btn-ghost btn-sm wallet-ab-label-edit\" data-addr=\"" + escapeHtml(addr) + "\" data-label=\"" + escapeHtml(label) + "\" title=\"" + escapeHtml(editTitle) + "\">" +
          "<span class=\"material-icons-round\" aria-hidden=\"true\">edit</span></button></div></td></tr>";
      }).join("") + "</tbody></table>";
  }

  async function copyAddressBookText(text, btn) {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      if (btn) {
        btn.classList.add("copied");
        const icon = btn.querySelector(".material-icons-round");
        if (icon) icon.textContent = "check";
        setTimeout(() => {
          btn.classList.remove("copied");
          if (icon) icon.textContent = "content_copy";
        }, 2000);
      }
    } catch (_) { /* */ }
  }

  function showAddressBookLabelEditor(addr, label) {
    const el = $("wallet-address-list");
    if (!el) return;
    const row = el.querySelector('tr[data-ab-addr="' + CSS.escape(addr) + '"]');
    if (!row) return;
    const cell = row.querySelector(".wallet-ab-label-cell");
    if (!cell) return;
    cell.innerHTML =
      "<form class=\"wallet-ab-label-form\" data-addr=\"" + escapeHtml(addr) + "\">" +
      "<input type=\"text\" class=\"wallet-ab-label-input modern-input\" value=\"" + escapeHtml(label) + "\" autocomplete=\"off\" maxlength=\"64\" />" +
      "<button type=\"submit\" class=\"btn btn-primary btn-sm\">" + escapeHtml(i18n("pages.receive.abSaveLabel")) + "</button>" +
      "<button type=\"button\" class=\"btn btn-ghost btn-sm wallet-ab-label-cancel\">" + escapeHtml(i18n("pages.receive.abCancelLabel")) + "</button>" +
      "</form>";
    const input = cell.querySelector(".wallet-ab-label-input");
    if (input) {
      input.focus();
      input.select();
    }
  }

  async function saveAddressBookLabel(addr, label) {
    await walletAddressPost("/api/wallet/address/label", { address: addr, label: label || "" });
    const row = walletAddressBookRows.find((r) => r.address === addr);
    if (row) row.label = label || "";
    walletAddressBookSig = addressBookRowsSignature(walletAddressBookRows);
    renderAddressBookTable(walletAddressBookRows);
    void loadWalletAddressLabels();
  }

  async function loadWalletAddressLabels() {
    const list = $("wallet-ab-labels");
    if (!list) return;
    try {
      const r = await fetchAPI("/api/wallet/labels");
      if (!r.ok) return;
      const labels = await r.json();
      if (!Array.isArray(labels)) return;
      list.innerHTML = labels.map((label) => {
        const text = String(label || "");
        return text ? "<option value=\"" + escapeHtml(text) + "\"></option>" : "";
      }).join("");
    } catch (_) { /* ignore */ }
  }

  async function loadWalletAddressBook(force) {
    const el = $("wallet-address-list");
    if (!el || !isReceiveBookTabActive()) return;
    if (walletAddressBookInFlight) return;
    const firstLoad = !walletAddressBookLoaded;
    if (firstLoad) {
      el.className = "label";
      wait(el, "Loading addresses…", { compact: true });
    }
    walletAddressBookInFlight = true;
    try {
      const r = await fetchAPI("/api/wallet/addresses");
      if (!r.ok) {
        el.textContent = "Wallet addresses unavailable.";
        el.className = "label";
        return;
      }
      const rows = await r.json();
      if (!Array.isArray(rows)) {
        el.textContent = "Wallet addresses unavailable.";
        el.className = "label";
        return;
      }
      const sig = addressBookRowsSignature(rows);
      if (!force && sig === walletAddressBookSig && walletAddressBookLoaded) {
        renderAddressBookTable(walletAddressBookRows);
        return;
      }
      walletAddressBookRows = rows;
      walletAddressBookSig = sig;
      walletAddressBookLoaded = true;
      el.removeAttribute("data-doge-wait");
      el.classList.remove("doge-wait-host");
      renderAddressBookTable(rows);
      void loadWalletAddressLabels();
    } catch (e) {
      el.textContent = String(e);
      el.className = "label";
    } finally {
      walletAddressBookInFlight = false;
    }
  }

  $("import-mnemonic-btn") && $("import-mnemonic-btn").addEventListener("click", async () => {
    const out = $("import-ext-out");
    const mnemonic = $("import-mnemonic") && $("import-mnemonic").value.trim();
    if (!out || !mnemonic) return;
    if (!window.confirm("Importing a mnemonic replaces the HD wallet on this node. Continue?")) return;
    out.classList.add("show");
    wait(out, "Importing mnemonic…", { compact: true });
    try {
      const res = await walletImportExtended("mnemonic", {
        type: "mnemonic",
        mnemonic: mnemonic,
        passphrase: ($("import-mnemonic-pass") && $("import-mnemonic-pass").value) || "",
        rescan: true,
      });
      out.textContent = i18n("pages.receive.importAddrOk", { address: (res && res.address) || "…" });
      refresh();
      refreshWalletAddressBookIfOpen();
    } catch (e) {
      out.textContent = String(e);
    }
  });

  $("import-bip38-btn") && $("import-bip38-btn").addEventListener("click", async () => {
    const out = $("import-ext-out");
    const key = $("import-bip38") && $("import-bip38").value.trim();
    const pass = $("import-bip38-pass") && $("import-bip38-pass").value;
    if (!out || !key) return;
    if (!window.confirm("BIP38 import replaces the spend key with the decrypted paper wallet. Continue?")) return;
    out.classList.add("show");
    wait(out, "Decrypting BIP38 key…", { compact: true });
    try {
      const res = await walletImportExtended("bip38", {
        type: "bip38",
        bip38: key,
        passphrase: pass || "",
        rescan: true,
      });
      out.textContent = i18n("pages.receive.importAddrOkShort", { address: (res && res.address) || "…" });
      refresh();
      refreshWalletAddressBookIfOpen();
    } catch (e) {
      out.textContent = String(e);
    }
  });

  $("import-walletdat-probe-btn") && $("import-walletdat-probe-btn").addEventListener("click", async () => {
    const out = $("import-walletdat-out");
    const path = $("import-walletdat-path") && $("import-walletdat-path").value.trim();
    if (!out || !path) return;
    out.classList.add("show");
    wait(out, i18n("pages.receive.walletDatProbing"), { compact: true });
    try {
      const r = await fetch("/api/wallet/probe-walletdat?path=" + encodeURIComponent(path), { credentials: "same-origin" });
      const data = await r.json();
      if (!r.ok) throw new Error((data && data.error && data.error.message) || r.statusText);
      const p = data.result || data;
      const summary = [];
      if (p && typeof p === "object" && !Array.isArray(p)) {
        summary.push(
          "is_bdb=" + p.is_bdb +
          " encrypted=" + p.encrypted +
          " keys=" + (p.key_count != null ? p.key_count : "?") +
          " encrypted_keys=" + (p.encrypted_keys != null ? p.encrypted_keys : 0) +
          walletDatPoolSuffix(p) +
          " can_import=" + p.can_import
        );
        if (p.needs_passphrase) {
          summary.push(i18n("pages.receive.walletDatProbeNeedsPassphrase"));
        }
        if (p.hint) summary.push(String(p.hint));
        if (p.pool_unmatched_hint) summary.push(String(p.pool_unmatched_hint));
      }
      out.textContent = (summary.length ? summary.join("\n\n") + "\n\n" : "") + JSON.stringify(p, null, 2);
    } catch (e) {
      out.textContent = String(e);
    }
  });

  $("import-walletdat-btn") && $("import-walletdat-btn").addEventListener("click", async () => {
    const out = $("import-walletdat-out");
    const path = $("import-walletdat-path") && $("import-walletdat-path").value.trim();
    const viaCore = $("import-walletdat-via-core") && $("import-walletdat-via-core").checked;
    const passEl = $("import-walletdat-pass");
    const passphrase = passEl && passEl.value ? passEl.value : "";
    if (!out || !path) return;
    if (!window.confirm("Import keys from Core wallet.dat into this DogeGo wallet?")) return;
    out.classList.add("show");
    wait(out, i18n("pages.receive.walletDatImporting"), { compact: true });
    try {
      const payload = {
        type: "walletdat",
        path: path,
        via_core_rpc: !!viaCore,
      };
      if (passphrase) payload.passphrase = passphrase;
      const res = await walletImportExtended("walletdat", payload);
      const parts = ["Imported"];
      if (res && res.keys_imported != null) parts.push("keys_imported=" + res.keys_imported);
      if (res) parts.push(walletDatPoolSuffix(res).trim());
      if (res && res.keypool_hint) parts.push(String(res.keypool_hint));
      if (res && res.pool_unmatched_hint) parts.push(String(res.pool_unmatched_hint));
      if (res && res.keypool_refill_size) parts.push("keypool_refill_size=" + res.keypool_refill_size);
      if (res && res.via_native_bdb) parts.push("via_native_bdb");
      if (res && res.via_core_rpc) parts.push("via_core_rpc");
      out.textContent = parts.filter(Boolean).join(" · ") + "\n\n" + JSON.stringify(res, null, 2);
      if (passEl) passEl.value = "";
      refresh();
      refreshWalletAddressBookIfOpen();
    } catch (e) {
      out.textContent = String(e);
    }
  });

  $("msg-sign-btn") && $("msg-sign-btn").addEventListener("click", async () => {
    const outWrap = $("msg-sign-wrap");
    const out = $("msg-sign-out");
    const errOut = $("msg-sign-err");
    const msg = $("msg-sign-text") && $("msg-sign-text").value;
    if (errOut) { errOut.classList.remove("show"); errOut.textContent = ""; }
    if (outWrap) outWrap.hidden = true;
    if (!out) return;
    if (errOut) {
      errOut.classList.add("show");
      wait(errOut, "Signing message…", { compact: true });
    }
    try {
      const addr = $("recv-addr") && $("recv-addr").textContent.trim();
      if (!addr || addr === "..." || addr.indexOf("disabled") >= 0) throw new Error("No wallet address");
      const sig = await walletRPC("signmessage", [addr, msg]);
      if (errOut) { errOut.classList.remove("show"); errOut.textContent = ""; }
      out.textContent = sig;
      if (outWrap) outWrap.hidden = false;
    } catch (e) {
      if (errOut) { errOut.classList.add("show"); errOut.textContent = String(e); }
    }
  });

  $("copy-sig") && $("copy-sig").addEventListener("click", async () => {
    const sig = $("msg-sign-out") && $("msg-sign-out").textContent;
    if (!sig || !sig.trim()) return;
    try {
      await navigator.clipboard.writeText(sig.trim());
      const done = $("copy-sig-done");
      if (done) {
        done.hidden = false;
        setTimeout(() => { done.hidden = true; }, 2200);
      }
    } catch (_) { /* */ }
  });

  $("msg-verify-btn") && $("msg-verify-btn").addEventListener("click", async () => {
    const out = $("msg-verify-out");
    if (!out) return;
    const addr = $("msg-verify-addr") && $("msg-verify-addr").value.trim();
    const sig = $("msg-verify-sig") && $("msg-verify-sig").value.trim();
    const msg = $("msg-verify-text") && $("msg-verify-text").value;
    out.classList.add("show");
    wait(out, i18n("pages.receive.verifyRunning"), { compact: true });
    try {
      const ok = await walletRPC("verifymessage", [addr, sig, msg]);
      out.textContent = ok ? i18n("pages.receive.verifyValid") : i18n("pages.receive.verifyInvalid");
    } catch (e) {
      out.textContent = String(e);
    }
  });

  const LS_BACKUP_REMIND = "dogego_backup_remind_dismiss";
  const LS_BACKUP_LAST = "dogego_backup_last_download";
  const BACKUP_INTERVAL_MS = 30 * 86400000;
  let rotateArchivePath = "";
  function maybeShowBackupReminder() {
    const el = $("wallet-backup-remind");
    if (!el) return;
    try {
      const dismissUntil = Number(localStorage.getItem(LS_BACKUP_REMIND) || 0);
      if (Date.now() < dismissUntil) {
        el.hidden = true;
        return;
      }
      const lastBackup = Number(localStorage.getItem(LS_BACKUP_LAST) || 0);
      const stale = !lastBackup || (Date.now() - lastBackup >= BACKUP_INTERVAL_MS);
      el.hidden = !stale;
    } catch (_) {
      el.hidden = false;
    }
  }
  function setRotateOut(text, isErr) {
    const out = $("wallet-rotate-out");
    if (!out) return;
    out.classList.add("show");
    out.textContent = text;
    if (isErr) out.classList.add("wallet-result-err");
    else out.classList.remove("wallet-result-err");
  }
  async function walletRotate(action, extra) {
    const r = await fetch("/api/wallet/rotate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify(Object.assign({ action: action }, extra || {})),
    });
    const body = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(body.error || ("HTTP " + r.status));
    return body;
  }
  $("wallet-backup-download") && $("wallet-backup-download").addEventListener("click", async () => {
    try {
      const r = await fetch("/api/wallet/backup/download", { credentials: "same-origin" });
      if (!r.ok) throw new Error("Download failed (HTTP " + r.status + ")");
      const blob = await r.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "dogego-wallet-backup.json";
      a.click();
      URL.revokeObjectURL(url);
      try {
        localStorage.setItem(LS_BACKUP_LAST, String(Date.now()));
      } catch (_) { /* ignore */ }
      maybeShowBackupReminder();
      setRotateOut("Backup downloaded. Store the file offline in a safe place.", false);
      const panel = $("wallet-rotate-panel");
      if (panel) panel.hidden = false;
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-backup-rotate-toggle") && $("wallet-backup-rotate-toggle").addEventListener("click", () => {
    const panel = $("wallet-rotate-panel");
    if (panel) panel.hidden = !panel.hidden;
  });
  $("wallet-rotate-prepare") && $("wallet-rotate-prepare").addEventListener("click", async () => {
    try {
      const body = await walletRotate("prepare");
      setRotateOut("New sweep address: " + body.new_address + "\nBalance: " + body.balance_doge + " DOGE\n" + (body.note || ""), false);
      ["wallet-rotate-sweep", "wallet-rotate-verify"].forEach((id) => { const b = $(id); if (b) b.disabled = false; });
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-rotate-sweep") && $("wallet-rotate-sweep").addEventListener("click", async () => {
    try {
      const body = await walletRotate("sweep");
      setRotateOut("Sweep broadcast\ntxid: " + body.txid + "\n" + (body.note || ""), false);
      const v = $("wallet-rotate-verify");
      if (v) v.disabled = false;
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-rotate-verify") && $("wallet-rotate-verify").addEventListener("click", async () => {
    try {
      const body = await walletRotate("verify");
      const lines = "Balance: " + body.balance_doge + " DOGE · spendable UTXOs: " + body.spendable_utxos;
      if (body.ready_to_finalize) {
        setRotateOut(lines + "\nReady to finalize - old wallet appears empty.", false);
        const f = $("wallet-rotate-finalize");
        if (f) f.disabled = false;
      } else {
        setRotateOut(lines + "\nWait for the sweep to confirm, then verify again.", false);
      }
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-rotate-finalize") && $("wallet-rotate-finalize").addEventListener("click", async () => {
    try {
      const body = await walletRotate("finalize");
      rotateArchivePath = body.archive_path || "";
      setRotateOut("Rotation complete.\nNew address: " + body.new_address + "\nOld wallet archived at:\n" + rotateArchivePath + "\n\nDelete the archive only after you confirm funds on the new address.", false);
      const rm = $("wallet-rotate-remove-archive");
      if (rm) rm.hidden = !rotateArchivePath;
      refresh();
      refreshWalletAddressBookIfOpen();
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-rotate-cancel") && $("wallet-rotate-cancel").addEventListener("click", async () => {
    try {
      await walletRotate("cancel");
      rotateArchivePath = "";
      setRotateOut("Rotation cancelled.", false);
      ["wallet-rotate-sweep", "wallet-rotate-verify", "wallet-rotate-finalize"].forEach((id) => { const b = $(id); if (b) b.disabled = true; });
      const rm = $("wallet-rotate-remove-archive");
      if (rm) rm.hidden = true;
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-rotate-remove-archive") && $("wallet-rotate-remove-archive").addEventListener("click", async () => {
    if (!rotateArchivePath) return;
    if (!window.confirm("Delete the archived pre-rotation wallet file from disk? Only do this after confirming funds on your new address.")) return;
    try {
      await walletRotate("remove_archive", { archive_path: rotateArchivePath });
      setRotateOut("Old wallet archive deleted.", false);
      rotateArchivePath = "";
      const rm = $("wallet-rotate-remove-archive");
      if (rm) rm.hidden = true;
    } catch (e) {
      setRotateOut(String(e), true);
    }
  });
  $("wallet-backup-dismiss") && $("wallet-backup-dismiss").addEventListener("click", () => {
    localStorage.setItem(LS_BACKUP_REMIND, String(Date.now() + 30 * 86400000));
    const el = $("wallet-backup-remind");
    if (el) el.hidden = true;
  });
  maybeShowBackupReminder();

  window.addEventListener("hashchange", routeFromHash);

  if ($("st-show-summary")) $("st-show-summary").checked = localStorage.getItem(LS_SUM) === "1";
  function afterI18n() {
    if (window.DogeGoI18n) window.DogeGoI18n.applyDOM(document);
    if (walletAddressBookLoaded) renderAddressBookTable(walletAddressBookRows);
    if (window.DogeGoPrefs) window.DogeGoPrefs.init();
    if (window.DogeGoControls) window.DogeGoControls.bindChoiceCards();
    if (capabilitiesCache) renderCapabilities(capabilitiesCache);
  }
  if (window.DogeGoI18n) {
    window.DogeGoI18n.ready().then(afterI18n);
    document.addEventListener("dogego:locale", afterI18n);
  } else if (window.DogeGoPrefs) {
    window.DogeGoPrefs.init();
  }
  if (window.DogeGoSecurity) {
    window.DogeGoSecurity.init().then(() => {
      window.DogeGoSecurity.onUnlock(() => refresh());
    });
  }
  function resetSidebarNav() {
    if (window.DogeGoPrefs?.resetNav) window.DogeGoPrefs.resetNav();
  }
  $("st-nav-reset") && $("st-nav-reset").addEventListener("click", resetSidebarNav);
  $("st-wallet-enabled") && $("st-wallet-enabled").addEventListener("change", (e) => {
    if ($("st-nowallet")) $("st-nowallet").checked = !e.target.checked;
    updateWalletSettingsPanel(lastWalletSnap, lastSummary);
  });
  $("st-pq-commitments") && $("st-pq-commitments").addEventListener("change", async (e) => {
    const ok = await saveWalletFlag("pq_commitments_enabled", e.target.checked);
    if (ok) refresh();
  });
  $("st-pq-carrier") && $("st-pq-carrier").addEventListener("change", async (e) => {
    const ok = await saveWalletFlag("pq_carrier_enabled", e.target.checked);
    if (ok) refresh();
  });
  document.querySelectorAll('input[name="send-pq-mode"]').forEach((el) => {
    el.addEventListener("change", () => updateSendUI(lastWalletSnap, lastSummary));
  });
  $("st-avoid-reuse") && $("st-avoid-reuse").addEventListener("change", async (e) => {
    const ok = await saveWalletFlag("avoid_reuse", e.target.checked);
    if (ok) refresh();
  });
  $("st-wallet-rescan-btn") && $("st-wallet-rescan-btn").addEventListener("click", () => startWalletRescan(false));
  $("st-wallet-rescan-full-btn") && $("st-wallet-rescan-full-btn").addEventListener("click", () => startWalletRescan(true));
  $("st-wallet-unlock-keys") && $("st-wallet-unlock-keys").addEventListener("click", async () => {
    if (!window.DogeGoWalletPassphrase || !window.DogeGoWalletPassphrase.promptUnlock) return;
    const ok = await window.DogeGoWalletPassphrase.promptUnlock({
      message: "Enter your wallet passphrase to unlock spend keys (Core walletpassphrase).",
    });
    if (ok) {
      await refreshWalletPanelAsync(refreshGen);
      validateSendForm();
    }
  });
  $("ov-wallet-unlock-btn") && $("ov-wallet-unlock-btn").addEventListener("click", async () => {
    if (!window.DogeGoWalletPassphrase || !window.DogeGoWalletPassphrase.promptUnlock) return;
    const ok = await window.DogeGoWalletPassphrase.promptUnlock({
      message: "Enter your wallet passphrase to unlock spend keys (Core walletpassphrase).",
    });
    if (ok) {
      await refreshWalletPanelAsync(refreshGen);
      validateSendForm();
    }
  });
  $("st-wallet-lock-keys") && $("st-wallet-lock-keys").addEventListener("click", async () => {
    const btn = $("st-wallet-lock-keys");
    if (btn) btn.disabled = true;
    try {
      if (window.DogeGoWalletPassphrase && window.DogeGoWalletPassphrase.lock) {
        await window.DogeGoWalletPassphrase.lock();
      }
      await refreshWalletPanelAsync(refreshGen);
      validateSendForm();
    } catch (e) {
      const msg = $("st-wallet-flags-msg");
      if (msg) msg.textContent = e.message || String(e);
    } finally {
      if (btn) btn.disabled = false;
    }
  });
  $("st-p2p") && $("st-p2p").addEventListener("change", () => {
    syncDGRWithP2PMode();
    const p2p = $("st-p2p").value;
    fillDGRCard("st", dgrLiveCache || {}, { p2pMode: p2p, forceShow: p2p === "cgnat" || p2p === "both" || ($("st-dgr-enabled") && $("st-dgr-enabled").checked) });
  });
  document.querySelectorAll("[data-dgr-role]").forEach((btn) => {
    btn.addEventListener("click", () => {
      applyDGRRole(btn.getAttribute("data-dgr-role") || "off", true);
    });
  });
  $("st-dgr-show-advanced") && $("st-dgr-show-advanced").addEventListener("change", (e) => {
    const box = $("st-dgr-advanced");
    if (box) box.hidden = !e.target.checked;
  });
  ["st-dgr-enabled", "st-dgr-inbound", "st-dgr-outbound"].forEach((id) => {
    $(id) && $(id).addEventListener("change", () => {
      dgrUserTouched = true;
      syncDGRRoleCards();
      updateDGRFieldsVisibility();
    });
  });
  ["st-dgr-seeds", "st-dgr-dns", "st-dgr-tls-pins"].forEach((id) => {
    $(id) && $(id).addEventListener("input", () => { dgrUserTouched = true; });
  });
  $("st-dgr-copy-server-cert") && $("st-dgr-copy-server-cert").addEventListener("click", () => {
    const fp = dgrLiveCache && dgrLiveCache.server_cert_sha256;
    if (fp) copyClipboard(fp);
  });
  $("st-dgr-use-server-cert") && $("st-dgr-use-server-cert").addEventListener("click", () => {
    const fp = dgrLiveCache && dgrLiveCache.server_cert_sha256;
    const ta = $("st-dgr-tls-pins");
    if (!fp || !ta) return;
    const lines = ta.value.split(/\r?\n/).map((s) => s.trim().toLowerCase()).filter(Boolean);
    if (!lines.includes(fp)) lines.unshift(fp);
    ta.value = lines.join("\n");
    dgrUserTouched = true;
  });
  $("st-lan-copy") && $("st-lan-copy").addEventListener("click", () => {
    const t = $("st-lan-share") && $("st-lan-share").textContent;
    copyClipboard(t, $("st-lan-status"));
  });
  $("st-lan-add") && $("st-lan-add").addEventListener("click", () => { void addLanPeerNow(); });
  $("an-peer-add") && $("an-peer-add").addEventListener("click", () => {
    const input = $("an-peer-addr");
    void peersAction("add", input && input.value);
  });
  $("an-peer-onetry") && $("an-peer-onetry").addEventListener("click", () => {
    const input = $("an-peer-addr");
    void peersAction("onetry", input && input.value);
  });
  document.addEventListener("click", (ev) => {
    const rem = ev.target && ev.target.closest && ev.target.closest(".an-added-remove");
    if (rem) {
      const node = rem.getAttribute("data-node");
      if (node && window.confirm("Remove manual peer " + node + "?")) {
        void peersAction("remove", node);
      }
      return;
    }
    const disc = ev.target && ev.target.closest && ev.target.closest(".an-peer-disconnect");
    if (disc) {
      const addr = disc.getAttribute("data-addr");
      if (addr) void peersAction("disconnect", addr);
    }
  });
  $("ov-lan-pair") && $("ov-lan-pair").addEventListener("click", () => {
    setTimeout(() => {
      const card = $("st-lan-peer-card");
      if (card) card.scrollIntoView({ behavior: "smooth", block: "start" });
      loadLanPeerHint();
    }, 50);
  });
  loadConfigForm();
  loadServicesPanel();
  decorateButtonsWithIcons();
  loadCapabilities();
  $("feat-core-compare-refresh") && $("feat-core-compare-refresh").addEventListener("click", loadCoreCompare);
  $("feat-core-maint-refresh") && $("feat-core-maint-refresh").addEventListener("click", loadCoreMaintenance);
  $("feat-core-resume-refresh") && $("feat-core-resume-refresh").addEventListener("click", loadCoreRestartResume);
  $("feat-core-ibd-converge-refresh") && $("feat-core-ibd-converge-refresh").addEventListener("click", loadCoreIbdConvergence);
  $("feat-core-addrman-refresh") && $("feat-core-addrman-refresh").addEventListener("click", loadCoreAddrman);
  $("feat-core-autostart-refresh") && $("feat-core-autostart-refresh").addEventListener("click", loadCoreAutostart);
  $("feat-core-founder-refresh") && $("feat-core-founder-refresh").addEventListener("click", loadCoreFounder);
  $("feat-core-runner-refresh") && $("feat-core-runner-refresh").addEventListener("click", loadCoreRunner);
  $("feat-core-workflow10-refresh") && $("feat-core-workflow10-refresh").addEventListener("click", loadCoreWorkflow10);
  $("feat-core-wallet-refresh") && $("feat-core-wallet-refresh").addEventListener("click", loadCoreWalletProbe);
  $("feat-core-reindex-refresh") && $("feat-core-reindex-refresh").addEventListener("click", loadCoreReindexProbe);
  $("feat-core-bip152-refresh") && $("feat-core-bip152-refresh").addEventListener("click", loadCoreBip152Probe);
  $("feat-core-mining-refresh") && $("feat-core-mining-refresh").addEventListener("click", loadCoreMiningProbe);
  $("feat-core-pq-refresh") && $("feat-core-pq-refresh").addEventListener("click", loadCorePQProbe);
  $("feat-core-field-refresh") && $("feat-core-field-refresh").addEventListener("click", loadCoreFieldEvidenceProbe);
  $("feat-core-e2e-refresh") && $("feat-core-e2e-refresh").addEventListener("click", loadCoreEndToEndProbe);
  $("feat-core-probes-refresh") && $("feat-core-probes-refresh").addEventListener("click", () => loadCoreProbes(true));
  bindProbeStripMiniPills();
  $("ov-operator-cert-run") && $("ov-operator-cert-run").addEventListener("click", () => {
    showTab("features");
    loadCoreProbes(true);
  });
  $("mp-parity-run") && $("mp-parity-run").addEventListener("click", loadMempoolParityProbe);
  $("feat-mp-parity-run") && $("feat-mp-parity-run").addEventListener("click", loadMempoolParityProbe);
  $("update-banner-dismiss") && $("update-banner-dismiss").addEventListener("click", async () => {
    try {
      await fetch("/api/update/dismiss", { method: "POST", credentials: "same-origin" });
      const el = $("update-banner");
      if (el) el.hidden = true;
      scheduleDashboardBannerStackSync();
      await refresh();
    } catch (_) { /* ignore */ }
  });
  $("ov-firewall-dismiss") && $("ov-firewall-dismiss").addEventListener("click", async () => {
    try {
      await fetch("/api/firewall/dismiss", { method: "POST", credentials: "same-origin" });
      const el = $("ov-firewall-alert");
      if (el) el.hidden = true;
      await refresh();
    } catch (_) { /* ignore */ }
  });
  $("update-banner-download") && $("update-banner-download").addEventListener("click", async () => {
    const btn = $("update-banner-download");
    if (btn) btn.disabled = true;
    try {
      const r = await fetch("/api/update/download", { method: "POST", credentials: "same-origin" });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error || ("HTTP " + r.status));
      const detail = $("update-banner-detail");
      if (detail) {
        let text = "Downloaded to " + (body.path || "updates folder") + ". ";
        if (body.sha256) text += "SHA256 " + body.sha256 + ". ";
        text += body.note || "Stop DogeGo, replace the binary, restart.";
        detail.textContent = text;
      }
    } catch (e) {
      const detail = $("update-banner-detail");
      if (detail) detail.textContent = String(e);
    } finally {
      if (btn) btn.disabled = false;
    }
  });
  $("update-banner-apply") && $("update-banner-apply").addEventListener("click", async () => {
    const btn = $("update-banner-apply");
    if (btn) btn.disabled = true;
    const detail = $("update-banner-detail");
    if (detail) detail.textContent = "Installing update. The node will restart…";
    try {
      const r = await fetch("/api/update/apply", { method: "POST", credentials: "same-origin" });
      const body = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(body.error || body.message || ("HTTP " + r.status));
      if (detail) detail.textContent = body.note || "Restarting into the new version…";
      let tries = 0;
      const poll = setInterval(async () => {
        tries++;
        try {
          const s = await fetch("/api/summary", { credentials: "same-origin" });
          if (s.ok) {
            clearInterval(poll);
            location.reload();
          }
        } catch (_) { /* restarting */ }
        if (tries > 120) clearInterval(poll);
      }, 1000);
    } catch (e) {
      if (detail) detail.textContent = String(e);
      if (btn) btn.disabled = false;
    }
  });
  function currentWebUISection() {
    const raw = (location.hash || "#overview").replace(/^#/, "");
    if (!raw) return "overview";
    return raw;
  }

  function dogeGoVersionLabel() {
    const el = $("sidebar-ver") || $("peer-short");
    const t = el && el.textContent ? el.textContent.trim() : "";
    if (t && t !== "..." && t !== "…") return t;
    return "unknown";
  }

  function openDogeGoGitHubIssue(opts) {
    opts = opts || {};
    const section = opts.section || currentWebUISection();
    const version = opts.version || dogeGoVersionLabel();
    const surface = opts.surface || "WebUI";
    const title = opts.title || ("[Bug] DogeGo " + version + " · " + section);
    const body = [
      "## Environment",
      "",
      "- **DogeGo version:** " + version,
      "- **Surface:** " + surface,
      "- **WebUI section / CLI context:** `" + section + "`",
      "- **OS:** " + (navigator.platform || "unknown"),
      "- **Browser:** " + (navigator.userAgent || "n/a"),
      "",
      "## What happened",
      "",
      "<!-- Describe the bug, missing feature, or engagement note -->",
      "",
      "## Steps to reproduce",
      "",
      "1. ",
      "2. ",
      "",
      "## Expected behavior",
      "",
      "",
      "## Extra details",
      "",
      "<!-- logs, screenshots, RPC errors -->",
      "",
    ].join("\n");
    const url = "https://github.com/qlpqlp/dogego/issues/new?title=" +
      encodeURIComponent(title) + "&body=" + encodeURIComponent(body);
    window.open(url, "_blank", "noopener,noreferrer");
  }

  $("topbar-feedback") && $("topbar-feedback").addEventListener("click", () => {
    openDogeGoGitHubIssue();
  });
  $("ext-refresh") && $("ext-refresh").addEventListener("click", () => void loadExtensionsCatalog(true));
  document.addEventListener("pointerdown", (ev) => {
    const open = document.querySelector(".ext-card.expanded");
    if (!open) return;
    if (open.contains(ev.target)) return;
    setExtensionCardExpanded(open, false);
  });
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape") return;
    collapseExtensionCards();
  });
  $("ext-notice-bar") && $("ext-notice-bar").addEventListener("click", (ev) => {
    const btn = ev.target.closest(".ext-notice-dismiss");
    if (!btn) return;
    dismissExtNotice(btn.getAttribute("data-notice-id"));
  });
  $("ext-detail-back") && $("ext-detail-back").addEventListener("click", () => {
    showExtensionCatalogView();
    showTab("extensions", { preserveHash: true });
    void loadExtensionsCatalog(false);
  });
  $("nav-ext-fold") && $("nav-ext-fold").addEventListener("click", (ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    setExtNavSubmenuOpen(!extNavSubmenuOpen);
  });
  $("nav-ext-catalog") && $("nav-ext-catalog").addEventListener("click", () => {
    showExtensionCatalogView();
  });
  $("ext-source-add") && $("ext-source-add").addEventListener("click", async () => {
    const input = $("ext-source-input");
    const url = (input && input.value.trim()) || "";
    if (!url) return;
    pushExtNotice("warn", "Adding catalog source", url);
    try {
      const r = await fetch("/api/extensions/catalog-sources", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ url }),
      });
      const body = await r.json().catch(() => ({}));
      if (!r.ok || body.error) throw new Error(extensionApiError(body) || ("HTTP " + r.status));
      if (input) input.value = "";
      pushExtNotice("ok", "Catalog source added", url);
      void loadExtensionCatalogSources();
      void loadExtensionsCatalog(true);
    } catch (e) {
      pushExtNotice("err", "Could not add catalog source", String(e.message || e));
    }
  });
  $("ext-zip-input") && $("ext-zip-input").addEventListener("change", async (ev) => {
    const input = ev.target;
    const file = input && input.files && input.files[0];
    if (!file) return;
    const list = $("ext-list");
    if (list) wait(list, "Installing " + file.name + "…", { compact: true });
    pushExtNotice("warn", "Installing extension", file.name);
    try {
      const fd = new FormData();
      fd.append("zip", file);
      const r = await fetch("/api/extensions/install", { method: "POST", body: fd, credentials: "same-origin" });
      const body = await r.json().catch(() => ({}));
      if (!r.ok || body.error) throw new Error(extensionApiError(body) || ("HTTP " + r.status));
      const installed = body.result || body;
      const id = (installed && installed.id) || file.name;
      pushExtNotice("ok", "Installed " + id, "Click Enable on the extension card.");
      await loadExtensionsCatalog(true);
    } catch (e) {
      pushExtNotice("err", "Install failed", String(e.message || e));
      if (list) {
        list.removeAttribute("data-doge-wait");
        list.classList.remove("doge-wait-host");
        list.innerHTML = "<p class=\"label\">" + escapeHtml(String(e.message || e)) + "</p>";
      }
    }
    input.value = "";
  });
  $("st-update-check") && $("st-update-check").addEventListener("click", () => settingsUpdateAction("/api/update/check", $("st-update-check")));
  $("st-update-download") && $("st-update-download").addEventListener("click", () => settingsUpdateAction("/api/update/download", $("st-update-download")));
  $("st-update-apply") && $("st-update-apply").addEventListener("click", () => settingsUpdateAction("/api/update/apply", $("st-update-apply")));
  $("st-update-dismiss") && $("st-update-dismiss").addEventListener("click", async () => {
    try {
      await fetch("/api/update/dismiss", { method: "POST", credentials: "same-origin" });
      await refresh();
    } catch (_) { /* ignore */ }
  });
  initBootOverlay();
  scheduleDashboardBannerStackSync();
  (function initCodeReviewBanner() {
    const LS_KEY = "dogego_code_review_banner_v1";
    const banner = $("code-review-banner");
    if (!banner) return;
    try {
      if (localStorage.getItem(LS_KEY) === "1") {
        banner.hidden = true;
        scheduleDashboardBannerStackSync();
        return;
      }
    } catch (_) { /* */ }
    banner.hidden = false;
    scheduleDashboardBannerStackSync();
    const dismiss = $("code-review-banner-dismiss");
    if (!dismiss) return;
    dismiss.addEventListener("click", () => {
      try { localStorage.setItem(LS_KEY, "1"); } catch (_) { /* */ }
      banner.hidden = true;
      scheduleDashboardBannerStackSync();
    });
  })();
  if (location.protocol === "https:") void loadTLSStatus();
  $("dash-tls-cert-dismiss") && $("dash-tls-cert-dismiss").addEventListener("click", () => {
    try { localStorage.setItem("dogego_tls_cert_banner_dismissed", "1"); } catch (_) { /* */ }
    const banner = $("dash-tls-cert-banner");
    if (banner) banner.hidden = true;
    scheduleDashboardBannerStackSync();
  });
  window.addEventListener("resize", scheduleDashboardBannerStackSync);
  document.addEventListener("dogego:locale", scheduleDashboardBannerStackSync);
  restoreWalletTxHistoryFromAnyCache();
  hydrateSummarySnapFromLocalStorage();
  document.addEventListener("dogego-tx-settled", (ev) => {
    const d = ev && ev.detail ? ev.detail : {};
    void onTxSettled(d.txid);
  });
  routeFromHash();
  bindNetSwitcherOnce();
  refresh();
  (function schedulePoll() {
    setTimeout(() => {
      refresh().finally(() => {
        refreshDGRLive();
        schedulePoll();
      });
    }, pollIntervalMs(lastSummary));
  })();
  setInterval(() => {
    const pan = document.getElementById("panel-console");
    if (!pan || !pan.classList.contains("active")) return;
    const ms = lastSummary && (lastSummary.ibd_active || isHeaderCatchUpPhase(lastSummary)) ? 800 : POLL_MS;
    loadLogs();
    void ms;
  }, 800);
  window.DogeGoApplyUtxoReplayUI = applyUtxoReplayUI;
  window.DogeGoRefresh = refresh;
  window.DogeGoLastWallet = () => lastWalletSnap;
  window.DogeGoSyncTopbarLock = () => syncTopbarLockButton(lastWalletSnap);
})();
