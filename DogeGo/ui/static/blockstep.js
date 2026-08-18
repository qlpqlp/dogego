/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* BlockStep ... interactive chain timeline explorer */
(function (global) {
  const $ = (id) => document.getElementById(id);
  const API = "/api/blockstep";
  const GENESIS_TS = 1386325540;
  let meta = null;
  let metaWaiters = null;
  let currentHeight = 0;
  let currentTime = 0;
  let historyStack = [];
  let view = "timeline";
  let loadedRoute = "";
  let routeLoading = "";
  let timelineSamples = [];
  let timelineLoadGen = 0;
  const BS_TX_PAGE = 40;
  const BS_ADDR_PAGE = 40;
  let blockTxScroll = { height: -1, offset: 0, total: 0, loading: false, observer: null };
  let addrScroll = { address: "", recvOffset: 0, recvTotal: 0, spendOffset: 0, spendTotal: 0, data: null, loading: false, observer: null, hintTxid: "", hintVout: "" };

  function curHash() {
    return (location.hash || "").replace(/^#/, "");
  }

  function setHashIfNeeded(route) {
    if (curHash() !== route) location.hash = route;
  }

  function routeAlreadyShowing(route, expectView) {
    return loadedRoute === route && view === expectView && routeLoading !== route;
  }

  function waitLoad(el, msg) {
    if (!el) return;
    if (global.DogeGoWait) global.DogeGoWait.set(el, msg);
    else el.innerHTML = "<p class=\"bs-loading\">" + (msg || "Loading…") + "</p>";
  }
  function esc(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  function matIcon(name, extraClass) {
    const cls = "material-icons-round" + (extraClass ? " " + extraClass : "");
    return '<span class="' + cls + '" aria-hidden="true">' + esc(name || "help_outline") + "</span>";
  }
  function availIcon(av, fallback) {
    return (av && (av.icon || av.emoji)) || fallback;
  }
  function flowIcon(row, fallback) {
    return row && (row.icon || row.emoji) ? (row.icon || row.emoji) : fallback;
  }
  function fmtDate(ts) {
    if (!ts) return "...";
    const d = new Date(ts * 1000);
    return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
  }
  function fmtTime(ts) {
    if (!ts) return "";
    return new Date(ts * 1000).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  }
  function shortHash(h) {
    if (!h || h.length < 16) return h || "...";
    return h.slice(0, 8) + "…" + h.slice(-8);
  }
  function shortAddr(a) {
    if (!a || a.length < 12) return a || "...";
    return a.slice(0, 6) + "…" + a.slice(-6);
  }

  function attrEsc(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  async function copyClipboard(text, hintEl) {
    if (!text) return false;
    try {
      await navigator.clipboard.writeText(String(text).trim());
      if (hintEl) {
        const prevTitle = hintEl.getAttribute("title") || "";
        hintEl.classList.add("copied");
        hintEl.setAttribute("title", "Copied!");
        if (hintEl.querySelector(".bs-copy-label")) {
          const lbl = hintEl.querySelector(".bs-copy-label");
          const prevLbl = lbl.textContent;
          lbl.textContent = "Copied!";
          setTimeout(() => {
            lbl.textContent = prevLbl;
            hintEl.classList.remove("copied");
            hintEl.setAttribute("title", prevTitle);
          }, 1600);
        } else {
          setTimeout(() => {
            hintEl.classList.remove("copied");
            hintEl.setAttribute("title", prevTitle);
          }, 1600);
        }
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  function copyBtnHtml(text, label) {
    if (!text) return "";
    const lbl = label || "Copy";
    return (
      '<button type="button" class="btn btn-ghost btn-sm bs-copy-btn" data-copy="' + attrEsc(text) + '" title="' + esc(lbl) + '" aria-label="' + esc(lbl) + '">' +
      matIcon("content_copy", "bs-copy-icon") +
      '<span class="bs-copy-label">Copy</span></button>'
    );
  }

  function copyRowHtml(text, className, copyLabel) {
    if (!text) return "";
    return (
      '<div class="bs-copy-row bs-copy-row-hero">' +
      '<div class="bs-copy-row-value">' +
      '<span class="' + (className || "bs-mono") + '" title="' + attrEsc(text) + '">' + esc(text) + "</span>" +
      "</div>" +
      copyBtnHtml(text, copyLabel || "Copy") +
      "</div>"
    );
  }

  function opReturnInlineHtml(out) {
    const kind = out.output_kind || "";
    if (kind !== "op_return" && kind !== "pq_commitment" && kind !== "pq_carrier") return "";
    const hx = out.script_hex || "";
    const payload = out.op_return_payload || "";
    const parts = [];
    if (payload) parts.push(copyBtnHtml(payload, "Copy payload"));
    if (hx) parts.push(copyBtnHtml(hx, "Copy script"));
    if (out.pq_commitment) parts.push(copyBtnHtml(out.pq_commitment, "Copy commitment"));
    if (!parts.length) return "";
    const decodeId = "bs-op-dec-" + String(out.index != null ? out.index : 0);
    let decodeInner = "";
    if (out.pq_commitment) {
      decodeInner =
        '<dl class="bs-op-decode-dl">' +
        "<dt>Tag</dt><dd class=\"bs-mono\">" + esc(out.pq_tag || "") + "</dd>" +
        (out.pq_scheme ? "<dt>Scheme</dt><dd>" + esc(out.pq_scheme) + "</dd>" : "") +
        '<dt>Commitment</dt><dd class="bs-mono">' + esc(out.pq_commitment) + "</dd>" +
        "</dl>";
    } else if (payload) {
      decodeInner = '<p class="bs-op-decode-body"><span class="bs-mono">' + esc(payload) + "</span></p>";
    } else if (out.asm) {
      decodeInner = '<p class="bs-mono bs-op-decode-body">' + esc(out.asm) + "</p>";
    }
    parts.push(
      '<button type="button" class="btn btn-ghost btn-sm bs-op-decode-btn" data-decode-target="' + esc(decodeId) + '" title="Decode">' +
      matIcon("code", "bs-op-decode-icon") + "<span>Decode</span></button>"
    );
    return (
      '<div class="bs-op-inline">' +
      '<div class="bs-op-inline-actions">' + parts.join("") + "</div>" +
      '<div class="bs-op-decode-panel bs-op-decode-inline" id="' + esc(decodeId) + '" hidden>' + decodeInner + "</div>" +
      "</div>"
    );
  }

  function pqChipHtml(tag, title, extra) {
    if (!tag && !extra) return "";
    return (
      '<span class="bs-pq-chip" title="' + esc(title || "Post-quantum") + '">' +
      matIcon("verified_user", "bs-pq-chip-icon") +
      " PQ" + (tag ? " · " + esc(tag) : "") +
      (extra || "") +
      "</span>"
    );
  }

  function pqCarrierLinksHtml(carrier) {
    if (!carrier) return "";
    let links = "";
    if (carrier.txc_txid) {
      links += '<button type="button" class="btn btn-ghost btn-sm bs-pq-carrier-jump" data-jump-tx="' + esc(carrier.txc_txid) + '">TX_C</button>';
    }
    if (carrier.txr_txid) {
      links += '<button type="button" class="btn btn-ghost btn-sm bs-pq-carrier-jump" data-jump-tx="' + esc(carrier.txr_txid) + '">TX_R</button>';
    }
    return links;
  }

  function bindOpReturnDecodeButtons(root) {
    if (!root) return;
    root.querySelectorAll(".bs-op-decode-btn").forEach((btn) => {
      if (btn.dataset.decodeBound === "1") return;
      btn.dataset.decodeBound = "1";
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const id = btn.getAttribute("data-decode-target");
        const panel = id ? document.getElementById(id) : null;
        if (!panel) return;
        const open = panel.hidden;
        panel.hidden = !open;
        btn.classList.toggle("is-open", open);
      });
    });
  }

  function pqCarrierBannerHtml(carrier) {
    if (!carrier || !carrier.role) return "";
    const role = String(carrier.role);
    const links = pqCarrierLinksHtml(carrier);
    const status = carrier.reveal_status === "confirmed"
      ? "Carrier reveal confirmed on-chain"
      : "TX_R reveal pending (carrier outputs not spent yet)";
    return (
      '<div class="bs-pq-banner-wrap">' +
      '<p class="bs-pq-banner bs-pq-carrier-banner">' +
      matIcon("verified_user", "bs-pq-banner-icon") +
      "<span><strong>PQ carrier</strong> · " + esc(role.toUpperCase()) +
      (carrier.pq_tag ? " · " + esc(carrier.pq_tag) : "") +
      " - " + esc(status) + "</span></p>" +
      (links ? '<div class="bs-pq-carrier-links">' + links + "</div>" : "") +
      "</div>"
    );
  }

  function bindCopyButtons(root) {
    if (!root) return;
    root.querySelectorAll(".bs-copy-btn").forEach((btn) => {
      if (btn.dataset.copyBound === "1") return;
      btn.dataset.copyBound = "1";
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const t = btn.getAttribute("data-copy");
        if (t) copyClipboard(t, btn);
      });
    });
  }

  async function api(path) {
    const r = await fetch(API + path, { cache: "no-store", credentials: "same-origin" });
    const data = await r.json();
    if (!r.ok && data.error) throw new Error(data.error);
    return data;
  }

  function setView(v) {
    view = v;
    ["timeline", "block", "tx", "address"].forEach((name) => {
      const el = $("bs-view-" + name);
      if (el) el.hidden = name !== v;
    });
    const back = $("bs-back-btn");
    if (back) back.hidden = historyStack.length === 0;
  }

  function pushHistory(state) {
    historyStack.push(state);
  }

  function popHistory() {
    if (!historyStack.length) return;
    const prev = historyStack.pop();
    restoreState(prev);
  }

  function restoreState(st) {
    if (!st) return;
    if (st.view === "timeline") {
      currentHeight = st.height || currentHeight;
      currentTime = st.time || currentTime;
      setView("timeline");
      renderTimeline();
      return;
    }
    if (st.view === "block") return openBlock(st.height, false);
    if (st.view === "tx") return openTx(st.txid, false);
    if (st.view === "address") return openAddress(st.address, false);
  }

  function notIndexedCard(icon, title, msg, tip) {
    return (
      '<div class="bs-missing-card">' +
      '<span class="bs-missing-icon">' + matIcon(icon || "info") + "</span>" +
      "<h3>" + esc(title) + "</h3>" +
      "<p>" + esc(msg) + "</p>" +
      (tip ? '<p class="bs-missing-tip">' + esc(tip) + "</p>" : "") +
      "</div>"
    );
  }

  function navigableHeight() {
    if (!meta) return 0;
    if (meta.navigable_height != null) return meta.navigable_height;
    if (meta.contiguous_bodies != null && meta.contiguous_bodies >= 0) return meta.contiguous_bodies;
    return meta.header_tip_height || 0;
  }

  function navigableTime() {
    if (!meta) return GENESIS_TS;
    if (meta.navigable_time) return meta.navigable_time;
    if (meta.header_tip_time) return meta.header_tip_time;
    return meta.timeline_end || Math.floor(Date.now() / 1000);
  }

  function timelineStart() {
    return (meta && meta.timeline_start) || GENESIS_TS;
  }

  function timelineEnd() {
    if (!meta) return Math.floor(Date.now() / 1000);
    const navT = navigableTime();
    if (navT > 0) return navT;
    return meta.timeline_end || Math.floor(Date.now() / 1000);
  }

  function yearLabel(ts) {
    return new Date(ts * 1000).getFullYear().toString();
  }

  function yearMarkers(from, to, maxLabels) {
    if (!isFinite(from) || !isFinite(to) || to <= from) {
      return [{ label: yearLabel(from), pct: 50 }];
    }
    const y0 = new Date(from * 1000).getFullYear();
    const y1 = new Date(to * 1000).getFullYear();
    if (y0 === y1) return [{ label: String(y0), pct: 0 }, { label: String(y0), pct: 100 }];
    const span = to - from;
    const out = [];
    const step = Math.max(1, Math.ceil((y1 - y0 + 1) / (maxLabels || 5)));
    for (let y = y0; y <= y1; y += step) {
      const ts = Math.floor(new Date(y, 0, 1).getTime() / 1000);
      const clamped = Math.max(from, Math.min(to, ts));
      const pct = ((clamped - from) / span) * 100;
      out.push({ label: String(y), pct });
    }
    const endPct = 100;
    if (!out.length || out[out.length - 1].pct < endPct - 2) {
      out.push({ label: String(y1), pct: endPct });
    }
    return out;
  }

  function fmtRange(from, to) {
    return fmtDate(from) + " → " + fmtDate(to);
  }

  function updateIndexNote() {
    const note = $("bs-index-note");
    if (!note) return;
    if (!meta) {
      if (note && global.DogeGoWait) global.DogeGoWait.set(note, "Checking what your node has locally…", { inline: true });
      else if (note) note.textContent = "Checking what your node has locally…";
      return;
    }
    const tip = meta.header_tip_height ?? 0;
    const nav = navigableHeight();
    const parts = [];
    if (tip > nav) {
      parts.push(
        "Explore blocks through #" + Number(nav).toLocaleString() +
        " (" + Number(tip).toLocaleString() + " headers on disk). Sync continues in the background."
      );
    }
    if (meta.indexing_note) parts.push(meta.indexing_note);
    note.textContent = parts.join(" ") || "Slide through time or open a block to explore.";
  }

  function applyMeta(m) {
    meta = m;
    const navH = navigableHeight();
    const navT = navigableTime();
    if (currentHeight == null || currentHeight === undefined || currentHeight > navH) currentHeight = navH;
    if (!currentTime || currentTime > navT) currentTime = navT;
    if (currentTime < timelineStart()) currentTime = timelineStart();
    updateIndexNote();
  }

  function bootstrapFallbackMeta() {
    if (meta) return;
    const now = Math.floor(Date.now() / 1000);
    meta = {
      timeline_start: GENESIS_TS,
      timeline_end: now,
      navigable_height: 0,
      navigable_time: GENESIS_TS,
      header_tip_height: 0,
    };
    currentHeight = 0;
    currentTime = GENESIS_TS;
    updateIndexNote();
  }

  async function loadMeta() {
    const data = await api("/meta");
    applyMeta(data);
    return data;
  }

  function ensureMeta(done) {
    if (meta) {
      done();
      return;
    }
    if (metaWaiters) {
      metaWaiters.push(done);
      return;
    }
    metaWaiters = [done];
    updateIndexNote();
    loadMeta()
      .then(() => {
        const waiters = metaWaiters || [];
        metaWaiters = null;
        waiters.forEach((fn) => fn());
      })
      .catch((e) => {
        bootstrapFallbackMeta();
        const note = $("bs-index-note");
        if (note) note.textContent += " Status unavailable: " + e.message;
        const waiters = metaWaiters || [];
        metaWaiters = null;
        waiters.forEach((fn) => fn());
      });
  }

  function renderTimeline() {
    const from = timelineStart();
    const to = timelineEnd();
    const slider = $("bs-time-slider");
    if (slider) {
      slider.min = String(from);
      slider.max = String(to);
      const val = currentTime || to;
      slider.value = String(Math.max(from, Math.min(to, val)));
      currentTime = parseInt(slider.value, 10);
    }
    $("bs-time-label") && ($("bs-time-label").textContent = fmtDate(currentTime) + " · " + fmtTime(currentTime));
    $("bs-height-label") && ($("bs-height-label").textContent = "Block #" + Number(currentHeight).toLocaleString());
    const rangeEl = $("bs-range-label");
    if (rangeEl) {
      const tip = meta && meta.header_tip_height != null ? Number(meta.header_tip_height) : null;
      let hint = "Synced range · " + fmtRange(from, to);
      if (tip != null && navigableHeight() < tip) {
        hint += " (headers at #" + tip.toLocaleString() + ")";
      }
      rangeEl.textContent = hint;
    }
    void loadTimelineSamples(from, to);
    drawTimelineTrack(from, to);
  }

  async function loadTimelineSamples(from, to) {
    const gen = ++timelineLoadGen;
    try {
      const data = await api("/timeline?from=" + from + "&to=" + to + "&points=16");
      if (gen !== timelineLoadGen) return;
      timelineSamples = Array.isArray(data.points) ? data.points : [];
      drawTimelineTrack(from, to);
    } catch (_) {
      if (gen === timelineLoadGen) timelineSamples = [];
    }
  }

  function drawTimelineTrack(from, to) {
    const track = $("bs-timeline-track");
    if (!track) return;
    const span = to - from;
    const pct = span > 0 ? ((currentTime - from) / span) * 100 : 100;
    const markers = yearMarkers(from, to, 5);
    let dots = "";
    timelineSamples.forEach((pt) => {
      const t = Number(pt.time);
      if (!isFinite(t) || t < from || t > to) return;
      const dp = span > 0 ? ((t - from) / span) * 100 : 0;
      const cls = pt.has_raw_body ? "bs-tl-dot has-body" : "bs-tl-dot header-only";
      dots += '<button type="button" class="' + cls + '" style="left:' + dp.toFixed(2) + '%" data-ts="' + t + '" title="Block #' + Number(pt.height).toLocaleString() + '"></button>';
    });
    let years = "";
    markers.forEach((m) => {
      years += '<span class="bs-tl-year" style="left:' + m.pct.toFixed(1) + '%">' + esc(m.label) + "</span>";
    });
    track.innerHTML =
      '<div class="bs-timeline-modern">' +
      '<div class="bs-tl-rail">' +
      '<div class="bs-tl-synced" style="width:' + Math.max(0, Math.min(100, pct)) + '%"></div>' +
      dots +
      '<button type="button" class="bs-tl-thumb" style="left:' + Math.max(0, Math.min(100, pct)) + '%" aria-label="Current position"></button>' +
      "</div>" +
      '<div class="bs-tl-years">' + years + "</div>" +
      "</div>";
    track.querySelectorAll(".bs-tl-dot").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const ts = parseInt(btn.getAttribute("data-ts"), 10);
        if (isFinite(ts)) {
          pushHistory({ view, height: currentHeight, time: currentTime });
          syncHeightFromTime(ts).catch(() => {});
        }
      });
    });
  }

  async function syncHeightFromTime(ts) {
    const data = await api("/at-time?ts=" + encodeURIComponent(ts));
    currentTime = data.time || ts;
    let h = data.height;
    const maxH = meta ? navigableHeight() : h;
    if (h > maxH) {
      h = maxH;
      if (meta && h >= 0) {
        const hdr = await api("/at-time?ts=" + encodeURIComponent(navigableTime()));
        currentTime = hdr.time || navigableTime();
      }
    }
    currentHeight = h;
    renderTimeline();
  }

  function renderBlockTxCard(tx) {
    const typeHtml = tx.is_coinbase
      ? matIcon("generating_tokens", "bs-tx-type-icon") + " Coinbase"
      : "Transaction";
    const idxBadge = tx.indexed === false ? '<span class="bs-badge-warn">not indexed</span>' : "";
    return (
      '<div class="bs-tx-card-wrap">' +
      '<button type="button" class="bs-tx-card" data-txid="' + esc(tx.txid) + '">' +
      '<span class="bs-tx-type">' + typeHtml + "</span>" +
      "<strong>" + esc(shortHash(tx.txid)) + "</strong>" +
      "<span>" + (tx.total_doge != null ? tx.total_doge.toFixed(4) + " DOGE" : "") + "</span>" +
      idxBadge +
      "</button>" +
      copyBtnHtml(tx.txid, "Copy transaction id") +
      "</div>"
    );
  }

  function bindBlockTxCards(root) {
    if (!root) return;
    root.querySelectorAll(".bs-tx-card[data-txid]").forEach((btn) => {
      if (btn.dataset.bound) return;
      btn.dataset.bound = "1";
      btn.addEventListener("click", () => openTx(btn.getAttribute("data-txid"), true));
    });
    bindCopyButtons(root);
  }

  function teardownBlockTxScroll() {
    if (blockTxScroll.observer) {
      blockTxScroll.observer.disconnect();
      blockTxScroll.observer = null;
    }
    blockTxScroll = { height: -1, offset: 0, total: 0, loading: false, observer: null };
  }

  function setupBlockTxScroll(body, height, route) {
    if (blockTxScroll.observer) blockTxScroll.observer.disconnect();
    const sentinel = body.querySelector("#bs-block-tx-sentinel");
    if (!sentinel || blockTxScroll.offset >= blockTxScroll.total) return;
    const scrollRoot = body;
    const hasMore = () => blockTxScroll.offset < blockTxScroll.total;
    blockTxScroll.observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) void loadMoreBlockTx(body, height, route);
      },
      { root: scrollRoot, rootMargin: "120px", threshold: 0.01 }
    );
    blockTxScroll.observer.observe(sentinel);
    if (!body.dataset.blockScrollBound) {
      body.dataset.blockScrollBound = "1";
      body.addEventListener(
        "scroll",
        () => {
          if (!blockTxScroll.loading && hasMore()) {
            const rect = sentinel.getBoundingClientRect();
            const rootRect = scrollRoot.getBoundingClientRect();
            if (rect.top < rootRect.bottom + 160) void loadMoreBlockTx(body, height, route);
          }
        },
        { passive: true }
      );
    }
  }

  async function loadMoreBlockTx(body, height, route) {
    if (blockTxScroll.loading || blockTxScroll.offset >= blockTxScroll.total) return;
    blockTxScroll.loading = true;
    const sentinel = body.querySelector("#bs-block-tx-sentinel");
    if (sentinel) sentinel.classList.add("is-loading");
    try {
      const data = await api("/block?height=" + encodeURIComponent(height) + "&tx_offset=" + blockTxScroll.offset + "&tx_limit=" + BS_TX_PAGE);
      if (routeLoading !== route) return;
      const grid = body.querySelector("#bs-block-tx-grid");
      const txs = data.transactions || [];
      if (grid && txs.length) {
        grid.insertAdjacentHTML("beforeend", txs.map(renderBlockTxCard).join(""));
        bindBlockTxCards(grid);
      }
      if (data.transaction_count != null) blockTxScroll.total = Number(data.transaction_count);
      if (txs.length > 0) {
        blockTxScroll.offset += txs.length;
      } else if (blockTxScroll.offset < blockTxScroll.total) {
        blockTxScroll.total = blockTxScroll.offset;
      }
      if (sentinel) {
        sentinel.classList.remove("is-loading");
        if (blockTxScroll.offset >= blockTxScroll.total) {
          sentinel.remove();
          if (blockTxScroll.observer) blockTxScroll.observer.disconnect();
        }
      }
    } catch (_) {
      if (sentinel) {
        sentinel.classList.remove("is-loading");
        sentinel.textContent = "Could not load more";
      }
    } finally {
      blockTxScroll.loading = false;
    }
  }

  async function openBlock(height, push) {
    const route = "blockstep/block/" + height;
    if (routeLoading === route) return;
    if (!push && routeAlreadyShowing(route, "block") && currentHeight === height) return;
    if (push) pushHistory({ view: view, height: currentHeight, time: currentTime });
    routeLoading = route;
    teardownBlockTxScroll();
    currentHeight = height;
    setView("block");
    const body = $("bs-block-body");
    if (body) waitLoad(body, "Loading block…");
    try {
      const data = await api("/block?height=" + encodeURIComponent(height) + "&tx_offset=0&tx_limit=" + BS_TX_PAGE);
      if (routeLoading !== route) return;
      const b = data.block || {};
      const av = data.availability || {};
      let html =
        '<div class="bs-hero-card">' +
        '<span class="bs-hero-icon">' + matIcon(availIcon(av, "view_module")) + "</span>" +
        "<h2>Block #" + esc(b.height) + "</h2>" +
        "<p>" + esc(fmtDate(b.time)) + " · " + esc(shortHash(b.hash)) + "</p>" +
        '<p class="bs-status-pill bs-status-' + esc(av.status || "partial") + '">' + esc(av.status || "") + "</p>" +
        "</div>";
      if (!b.has_raw_block) {
        html += notIndexedCard(
          "cloud_download",
          "Block still syncing",
          data.dogego_blockstep_note || "This block header exists but the body is not on disk yet.",
          "Slide forward in time or wait for sync ... then tap again!"
        );
      } else {
        const txs = data.transactions || [];
        const txTotal = Number(data.transaction_count) || txs.length;
        blockTxScroll.height = height;
        blockTxScroll.offset = txs.length;
        blockTxScroll.total = txTotal;
        html += '<div class="bs-tx-grid" id="bs-block-tx-grid">';
        if (!txs.length) {
          html += notIndexedCard("inbox", "Empty or parsing", "No transactions decoded for this block.", "");
        } else {
          html += txs.map(renderBlockTxCard).join("");
        }
        html += "</div>";
        if (txs.length && txTotal > txs.length) {
          html += '<div class="bs-scroll-sentinel" id="bs-block-tx-sentinel" aria-hidden="true"><span class="bs-scroll-spinner"></span></div>';
        }
      }
      if (body) body.innerHTML = html;
      bindBlockTxCards(body);
      setupBlockTxScroll(body, height, route);
      loadedRoute = route;
      if (push) setHashIfNeeded(route);
    } catch (e) {
      if (routeLoading === route && body) body.innerHTML = notIndexedCard("error_outline", "Oops", String(e.message), "");
    } finally {
      if (routeLoading === route) routeLoading = "";
    }
  }

  async function openTx(txid, push, force) {
    const route = "blockstep/tx/" + encodeURIComponent(txid);
    if (routeLoading === route) return;
    if (!push && !force && routeAlreadyShowing(route, "tx")) return;
    if (push) pushHistory({ view: view, height: currentHeight, time: currentTime, txid });
    routeLoading = route;
    setView("tx");
    const body = $("bs-tx-body");
    if (body) waitLoad(body, "Loading transaction…");
    try {
      const data = await api("/tx?txid=" + encodeURIComponent(txid));
      if (routeLoading !== route) return;
      if (!data.found) {
        body.innerHTML = notIndexedCard(
          availIcon(data.availability, "search_off"),
          "Not found locally",
          data.availability?.message || "Transaction not on this node.",
          data.dogego_tip
        );
        loadedRoute = route;
        return;
      }
      const av = data.availability || {};
      const pq = data.pq_commitment || null;
      let html =
        '<div class="bs-hero-card">' +
        '<span class="bs-hero-icon">' + matIcon(availIcon(av, "receipt_long")) + "</span>" +
        "<h2>Transaction</h2>" +
        copyRowHtml(txid, "bs-mono bs-txid-full", "Copy transaction id") +
        "<p>" + esc(av.message || "") + "</p>";
      if (pq && pq.tag) {
        html +=
          '<p class="bs-pq-banner">' +
          matIcon("verified_user", "bs-pq-banner-icon") +
          "<span><strong>Post-quantum</strong> · " + esc(pq.tag) +
          (pq.scheme ? " (" + esc(pq.scheme) + ")" : "") +
          "</span></p>";
      }
      html += pqCarrierBannerHtml(data.pq_carrier || null);
      if (data.fee_doge != null && isFinite(Number(data.fee_doge))) {
        html += '<p class="bs-fee-line"><strong>Fee:</strong> ' + Number(data.fee_doge).toFixed(8) + " DOGE</p>";
      }
      html += "</div>";
      html += '<div class="bs-flow"><h3>Inputs</h3><div class="bs-flow-list">';
      (data.inputs || []).forEach((inp) => {
        if (inp.type === "coinbase") {
          html += '<div class="bs-flow-item bs-flow-coinbase">' + matIcon(flowIcon(inp, "generating_tokens"), "bs-flow-icon") + "<span>" + esc(inp.label) + "</span></div>";
          return;
        }
        const jump = inp.can_jump ? ' data-jump-tx="' + esc(inp.prev_txid) + '"' : "";
        const amt = inp.doge != null ? '<strong class="bs-flow-amt">' + Number(inp.doge).toFixed(8) + " DOGE</strong>" : "";
        const pqIn = inp.pq_carrier_reveal || inp.pq_tag
          ? pqChipHtml(inp.pq_tag, "PQ carrier reveal (TX_R)")
          : "";
        const copyTx = inp.prev_txid ? '<div class="bs-flow-copy-slot">' + copyBtnHtml(inp.prev_txid, "Copy transaction id") + "</div>" : "";
        html +=
          '<div class="bs-flow-item-row">' +
          '<button type="button" class="bs-flow-item' + (inp.can_jump ? " bs-jump" : " bs-dim") + '"' + jump + ">" +
          matIcon(flowIcon(inp, inp.can_jump ? "arrow_back" : "help_outline"), "bs-flow-icon") +
          '<div class="bs-flow-main">' + amt + pqIn +
          '<span class="bs-mono">' + esc(shortHash(inp.prev_txid)) + " · vout " + inp.prev_vout + "</span>" +
          "<small>" + esc(inp.hint || "") + "</small></div></button>" +
          copyTx +
          "</div>";
      });
      html += '</div><h3>Outputs</h3><div class="bs-flow-list">';
      (data.outputs || []).forEach((out) => {
        const voutN = out.index != null ? out.index : 0;
        const spendTx = out.spend_txid || "";
        const jumpTx = spendTx ? ' data-jump-tx="' + esc(spendTx) + '"' : "";
        const jumpAddr = !spendTx && out.can_jump
          ? ' data-jump-addr="' + esc(out.address) + '" data-hint-txid="' + esc(txid) + '" data-hint-vout="' + voutN + '"'
          : "";
        const jump = jumpTx || jumpAddr;
        const amt = out.doge != null ? '<strong class="bs-flow-amt">' + Number(out.doge).toFixed(8) + " DOGE</strong>" : "";
        const addr = out.address
          ? '<span class="bs-mono bs-flow-addr" title="' + attrEsc(out.address) + '">' + esc(out.address) + "</span>"
          : (out.asm ? '<span class="bs-mono">' + esc(out.asm) + "</span>" : "");
        const pqBadge = out.pq_tag || out.output_kind === "pq_carrier"
          ? pqChipHtml(out.pq_tag || "carrier", out.output_kind === "pq_carrier" ? "PQ carrier output (TX_C)" : "Post-quantum OP_RETURN")
          : "";
        const carrierLinks = out.output_kind === "pq_carrier" && data.pq_carrier ? pqCarrierLinksHtml(data.pq_carrier) : "";
        const canJump = spendTx || out.can_jump;
        const copyAddr = out.address ? '<div class="bs-flow-copy-slot">' + copyBtnHtml(out.address, "Copy address") + "</div>" : "";
        const opInline = opReturnInlineHtml(out);
        html +=
          '<div class="bs-flow-item-row bs-flow-out-row">' +
          '<button type="button" class="bs-flow-item' + (canJump ? " bs-jump" : " bs-dim") + '"' + jump + ">" +
          matIcon(flowIcon(out, spendTx ? "link" : out.can_jump ? "account_balance_wallet" : "call_made"), "bs-flow-icon") +
          '<div class="bs-flow-main">' + amt + addr + pqBadge +
          (carrierLinks ? '<span class="bs-pq-carrier-inline">' + carrierLinks + "</span>" : "") +
          "<small>" + esc(out.hint || "") + "</small></div></button>" +
          copyAddr +
          (opInline ? '<div class="bs-flow-op-slot">' + opInline + "</div>" : "") +
          "</div>";
      });
      html += "</div></div>";
      if (body) body.innerHTML = html;
      body.querySelectorAll("[data-jump-tx]").forEach((el) => {
        el.addEventListener("click", () => openTx(el.getAttribute("data-jump-tx"), true));
      });
      body.querySelectorAll("[data-jump-addr]").forEach((el) => {
        el.addEventListener("click", () =>
          openAddress(
            el.getAttribute("data-jump-addr"),
            true,
            el.getAttribute("data-hint-txid") || "",
            el.getAttribute("data-hint-vout")
          )
        );
      });
      bindCopyButtons(body);
      bindOpReturnDecodeButtons(body);
      body.querySelectorAll(".bs-pq-carrier-jump").forEach((btn) => {
        btn.addEventListener("click", (e) => {
          e.preventDefault();
          const id = btn.getAttribute("data-jump-tx");
          if (id) openTx(id, true);
        });
      });
      loadedRoute = route;
      if (push) setHashIfNeeded(route);
    } catch (e) {
      if (routeLoading === route && body) body.innerHTML = notIndexedCard("error_outline", "Oops", String(e.message), "");
    } finally {
      if (routeLoading === route) routeLoading = "";
    }
  }

  function teardownAddrScroll() {
    if (addrScroll.observer) {
      addrScroll.observer.disconnect();
      addrScroll.observer = null;
    }
    addrScroll = { address: "", recvOffset: 0, recvTotal: 0, spendOffset: 0, spendTotal: 0, data: null, loading: false, observer: null, hintTxid: "", hintVout: "" };
  }

  async function loadMoreAddrPage(body, route) {
    if (addrScroll.loading) return;
    const needRecv = addrScroll.recvOffset < addrScroll.recvTotal;
    const needSpend = addrScroll.spendOffset < addrScroll.spendTotal;
    if (!needRecv && !needSpend) return;
    addrScroll.loading = true;
    const sentinel = body.querySelector("#bs-addr-scroll-sentinel");
    if (sentinel) {
      sentinel.classList.add("is-loading");
      sentinel.hidden = false;
    }
    updateAddrScrollFooter(body);
    try {
      let url = "/address?address=" + encodeURIComponent(addrScroll.address);
      url += "&recv_offset=" + addrScroll.recvOffset + "&recv_limit=" + BS_ADDR_PAGE;
      url += "&spend_offset=" + addrScroll.spendOffset + "&spend_limit=" + BS_ADDR_PAGE;
      if (addrScroll.hintTxid) url += "&hint_txid=" + encodeURIComponent(addrScroll.hintTxid);
      if (addrScroll.hintVout !== "" && Number(addrScroll.hintVout) >= 0) url += "&hint_vout=" + encodeURIComponent(addrScroll.hintVout);
      const data = await api(url);
      if (routeLoading !== route) return;
      const recvGrid = body.querySelector("#bs-addr-recv-grid");
      const spendGrid = body.querySelector("#bs-addr-spend-grid");
      const newRecv = data.matching_outputs || [];
      const newSpend = data.matching_spends || [];
      if (addrScroll.data) {
        addrScroll.data.matching_outputs = (addrScroll.data.matching_outputs || []).concat(newRecv);
        addrScroll.data.matching_spends = (addrScroll.data.matching_spends || []).concat(newSpend);
      }
      const filterQ = (body.querySelector(".bs-addr-filter") && body.querySelector(".bs-addr-filter").value.trim()) || "";
      if (recvGrid && newRecv.length) {
        recvGrid.insertAdjacentHTML("beforeend", renderAddressHitCards(newRecv, "receive", filterQ, true));
      }
      if (spendGrid && newSpend.length) {
        spendGrid.insertAdjacentHTML("beforeend", renderAddressHitCards(newSpend, "spend", filterQ, true));
      }
      addrScroll.recvOffset += newRecv.length;
      addrScroll.spendOffset += newSpend.length;
      if (data.matching_output_count != null) addrScroll.recvTotal = Number(data.matching_output_count);
      if (data.matching_spend_count != null) addrScroll.spendTotal = Number(data.matching_spend_count);
      bindAddressBody(body, addrScroll.address, addrScroll.data);
      updateAddrScrollFooter(body);
      if (sentinel) {
        sentinel.classList.remove("is-loading");
        if (addrScroll.recvOffset >= addrScroll.recvTotal && addrScroll.spendOffset >= addrScroll.spendTotal) {
          sentinel.remove();
          const foot = body.querySelector("#bs-addr-scroll-footer");
          if (foot) foot.hidden = true;
          if (addrScroll.observer) addrScroll.observer.disconnect();
        }
      }
    } catch (_) {
      if (sentinel) sentinel.classList.remove("is-loading");
    } finally {
      addrScroll.loading = false;
    }
  }

  function setupAddrScroll(body, route) {
    if (addrScroll.observer) addrScroll.observer.disconnect();
    const sentinel = body.querySelector("#bs-addr-scroll-sentinel");
    if (!sentinel) return;
    if (addrScroll.recvOffset >= addrScroll.recvTotal && addrScroll.spendOffset >= addrScroll.spendTotal) {
      sentinel.remove();
      return;
    }
    const scrollRoot = body;
    const hasMore = () =>
      addrScroll.recvOffset < addrScroll.recvTotal || addrScroll.spendOffset < addrScroll.spendTotal;
    addrScroll.observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) void loadMoreAddrPage(body, route);
      },
      { root: scrollRoot, rootMargin: "120px", threshold: 0.01 }
    );
    addrScroll.observer.observe(sentinel);
    if (!body.dataset.addrScrollBound) {
      body.dataset.addrScrollBound = "1";
      body.addEventListener(
        "scroll",
        () => {
          if (!addrScroll.loading && hasMore()) {
            const rect = sentinel.getBoundingClientRect();
            const rootRect = scrollRoot.getBoundingClientRect();
            if (rect.top < rootRect.bottom + 160) void loadMoreAddrPage(body, route);
          }
        },
        { passive: true }
      );
    }
  }

  async function openAddress(address, push, hintTxid, hintVout) {
    const route = "blockstep/address/" + encodeURIComponent(address);
    if (routeLoading === route) return;
    if (!push && routeAlreadyShowing(route, "address")) return;
    if (push) pushHistory({ view: view, height: currentHeight, time: currentTime, address });
    routeLoading = route;
    teardownAddrScroll();
    setView("address");
    const body = $("bs-addr-body");
    if (body) waitLoad(body, "Sniffing address…");
    try {
      let url = "/address?address=" + encodeURIComponent(address) + "&recv_offset=0&recv_limit=" + BS_ADDR_PAGE + "&spend_offset=0&spend_limit=" + BS_ADDR_PAGE;
      if (hintTxid) url += "&hint_txid=" + encodeURIComponent(hintTxid);
      if (hintVout != null && hintVout !== "" && Number(hintVout) >= 0) url += "&hint_vout=" + encodeURIComponent(hintVout);
      const data = await api(url);
      if (routeLoading !== route) return;
      if (!data.found) {
        body.innerHTML = notIndexedCard(
          availIcon(data.availability, "cloud_off"),
          "Cannot explore yet",
          data.availability?.message || "",
          data.dogego_tip
        );
        loadedRoute = route;
        return;
      }
      addrScroll.address = address;
      addrScroll.hintTxid = hintTxid || "";
      addrScroll.hintVout = hintVout != null ? String(hintVout) : "";
      addrScroll.data = data;
      addrScroll.recvOffset = (data.matching_outputs || []).length;
      addrScroll.recvTotal = Number(data.matching_output_count) || addrScroll.recvOffset;
      addrScroll.spendOffset = (data.matching_spends || []).length;
      addrScroll.spendTotal = Number(data.matching_spend_count) || addrScroll.spendOffset;
      renderAddressBody(body, address, data);
      setupAddrScroll(body, route);
      loadedRoute = route;
      if (push) setHashIfNeeded(route);
    } catch (e) {
      if (routeLoading === route && body) body.innerHTML = notIndexedCard("error_outline", "Oops", String(e.message), "");
    } finally {
      if (routeLoading === route) routeLoading = "";
    }
  }

  function addrHitMatchesFilter(hit, q) {
    if (!q) return true;
    q = q.toLowerCase();
    const blob = [hit.txid, hit.height, hit.vout, hit.vin, hit.prev_txid].filter((x) => x != null).join(" ").toLowerCase();
    return blob.includes(q);
  }

  function addrHitTypeChip(kind, h) {
    if (kind === "spend") {
      return '<span class="wallet-tx-type wallet-tx-type-out"><span class="material-icons-round" aria-hidden="true">north_east</span>Sent</span>';
    }
    if (h.spend_txid) {
      return '<span class="wallet-tx-type wallet-tx-type-out"><span class="material-icons-round" aria-hidden="true">north_east</span>Spent</span>';
    }
    return '<span class="wallet-tx-type wallet-tx-type-in"><span class="material-icons-round" aria-hidden="true">south_west</span>Received</span>';
  }

  function addrHitConfBadge(h, kind) {
    if (h.height === -1) {
      return '<span class="wallet-tx-conf pending" title="Mempool">·</span>';
    }
    if (kind === "receive" && h.spend_height >= 0) {
      return '<span class="wallet-tx-conf confirmed" title="Spent in block ' + h.spend_height + '">' + h.spend_height + "</span>";
    }
    if (h.height >= 0) {
      return '<span class="wallet-tx-conf confirmed" title="Block ' + h.height + '">' + h.height + "</span>";
    }
    return '<span class="wallet-tx-conf" title="Unknown">-</span>';
  }

  function addrHitRowHtml(h, kind, idx) {
    const doge = h.value_satoshi != null ? h.value_satoshi / 1e8 : 0;
    const isIn = kind === "receive" && !h.spend_txid;
    const openTxid = kind === "receive" && h.spend_txid ? h.spend_txid : h.txid;
    let meta;
    if (kind === "spend") {
      meta = "vin " + h.vin + (h.prev_txid ? " · spends " + shortHash(h.prev_txid) : "");
    } else if (h.spend_txid) {
      meta = "vout " + h.vout + " → spent in " + shortHash(h.spend_txid);
    } else {
      meta = "vout " + h.vout + " · unspent";
    }
    const blockLabel = h.height === -1 ? "mempool" : "block #" + h.height;
    const amtStr = (isIn ? "+" : "-") + Math.abs(doge).toFixed(4);
    const txidLine = openTxid
      ? '<span class="wallet-tx-row-txid mono" title="' + esc(openTxid) + '">' + esc(openTxid) + "</span>"
      : "";
    return (
      '<div class="wallet-tx-row bs-addr-tx-row" role="button" tabindex="0" data-addr-idx="' + idx + '" data-txid="' + esc(openTxid) + '"' +
      (h.height >= 0 ? ' data-block="' + h.height + '"' : "") +
      (kind === "receive" && h.txid ? ' data-recv-txid="' + esc(h.txid) + '"' : "") +
      ' title="View transaction">' +
      '<div class="wallet-tx-row-main">' +
      '<div class="wallet-tx-row-top">' + addrHitTypeChip(kind, h) + addrHitConfBadge(h, kind) + "</div>" +
      '<div class="wallet-tx-row-amt wallet-tx-amt ' + (isIn ? "in" : "out") + '">' + esc(amtStr) + " DOGE</div>" +
      '<div class="wallet-tx-row-sub">' +
      '<span class="wallet-tx-row-date">' + esc(blockLabel) + "</span>" +
      '<span class="wallet-tx-row-addr mono">' + esc(meta) + "</span>" +
      "</div>" +
      txidLine +
      "</div>" +
      '<div class="wallet-tx-row-side">' +
      (openTxid ? '<button type="button" class="wallet-tx-row-copy bs-addr-copy" data-copy-txid="' + esc(openTxid) + '" title="Copy txid" aria-label="Copy txid"><span class="material-icons-round">content_copy</span></button>' : "") +
      '<span class="material-icons-round wallet-tx-row-chevron" aria-hidden="true">chevron_right</span>' +
      "</div></div>"
    );
  }

  function renderAddressHitCards(hits, kind, filterQ, cardsOnly) {
    const filtered = (hits || []).filter((h) => addrHitMatchesFilter(h, filterQ));
    if (!filtered.length) {
      if (cardsOnly) return "";
      return '<p class="bs-missing-tip">No ' + (kind === "spend" ? "spends" : "receives") + (filterQ ? " match your filter" : " in the recent window") + ".</p>";
    }
    let html = cardsOnly ? "" : '<div class="wallet-tx-feed bs-addr-feed">';
    filtered.forEach((h, i) => {
      html += addrHitRowHtml(h, kind, i);
    });
    if (!cardsOnly) html += "</div>";
    return html;
  }

  function bindAddressBody(body, address, data) {
    body.querySelectorAll(".bs-addr-tx-row[data-txid]").forEach((row) => {
      if (row.dataset.bound) return;
      row.dataset.bound = "1";
      const open = () => openTx(row.getAttribute("data-txid"), true);
      row.addEventListener("click", (e) => {
        if (e.target.closest(".wallet-tx-row-copy, .bs-addr-copy")) return;
        open();
      });
      row.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          open();
        }
      });
    });
    body.querySelectorAll(".bs-addr-copy[data-copy-txid]").forEach((btn) => {
      if (btn.dataset.copyBound) return;
      btn.dataset.copyBound = "1";
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        const txid = btn.getAttribute("data-copy-txid");
        if (txid && navigator.clipboard) navigator.clipboard.writeText(txid).catch(() => {});
      });
    });
    const filter = body.querySelector(".bs-addr-filter");
    if (filter && !filter.dataset.bound) {
      filter.dataset.bound = "1";
      filter.addEventListener("input", () => {
        const q = filter.value.trim().toLowerCase();
        body.querySelectorAll(".bs-addr-feed .bs-addr-tx-row").forEach((row) => {
          if (!q) {
            row.hidden = false;
            return;
          }
          const blob = (row.textContent || "").toLowerCase();
          row.hidden = !blob.includes(q);
        });
      });
    }
    bindCopyButtons(body);
  }

  function renderAddressBody(body, address, data, filterQ) {
    if (!body) return;
    const av = data.availability || {};
    const bal = data.utxo_balance || {};
    let html =
      '<div class="bs-hero-card">' +
      '<span class="bs-hero-icon">' + matIcon(availIcon(av, "wallet")) + "</span>" +
      "<h2>Address</h2>" +
      copyRowHtml(address, "bs-mono", "Copy address") +
      "<p>" + esc(av.message || "") + "</p>" +
      '<div class="bs-addr-stat-row">';
    if (bal.available) {
      html +=
        '<div class="bs-addr-stat"><strong>' + Number(bal.total_doge || 0).toFixed(8) + " DOGE</strong><span>Confirmed balance · " + (bal.utxo_count || 0) + " UTXO</span></div>";
    } else if (bal.error) {
      html += '<div class="bs-addr-stat"><strong>...</strong><span>Balance: ' + esc(bal.error) + "</span></div>";
    }
    if (data.total_received_doge_window != null) {
      html += '<div class="bs-addr-stat"><strong>' + Number(data.total_received_doge_window).toFixed(4) + " DOGE</strong><span>Received in stored blocks</span></div>";
    }
    if (data.total_spent_doge_window != null && Number(data.total_spent_doge_window) > 0) {
      html += '<div class="bs-addr-stat"><strong>' + Number(data.total_spent_doge_window).toFixed(4) + " DOGE</strong><span>Spent in window</span></div>";
    }
    html += "</div></div>";
    html += '<div class="bs-addr-toolbar"><input type="search" class="bs-addr-filter" placeholder="Filter by txid, block, vout…" value="' + esc(filterQ || "") + '" /></div>';
    const hits = data.matching_outputs || [];
    const spends = data.matching_spends || [];
    html += '<div class="bs-addr-section"><h3>Received outputs</h3><div class="wallet-tx-feed bs-addr-feed" id="bs-addr-recv-grid">' + renderAddressHitCards(hits, "receive", filterQ, true) + "</div></div>";
    html += '<div class="bs-addr-section"><h3>Spent inputs</h3><div class="wallet-tx-feed bs-addr-feed" id="bs-addr-spend-grid">' + renderAddressHitCards(spends, "spend", filterQ, true) + "</div></div>";
    if (!hits.length && !spends.length && data.dogego_empty_reason) {
      html += '<p class="bs-missing-tip">' + esc(data.dogego_empty_reason) + "</p>";
    }
    if (addrScroll.recvOffset < addrScroll.recvTotal || addrScroll.spendOffset < addrScroll.spendTotal || data.has_more) {
      html +=
        '<div class="wallet-tx-scroll-footer" id="bs-addr-scroll-footer" hidden>' +
        '<div class="wallet-tx-progress-track"><div class="wallet-tx-progress-fill"></div></div>' +
        '<span class="wallet-tx-progress-count">0 / 0</span>' +
        '<span class="wallet-tx-progress-hint">Scroll for more</span>' +
        "</div>";
      html += '<div class="bs-scroll-sentinel" id="bs-addr-scroll-sentinel" aria-hidden="true"><span class="bs-scroll-spinner"></span></div>';
    }
    if (data.truncated && !data.indexed) {
      html += '<p class="bs-missing-tip">Showing first matches only ... full history needs deeper indexing.</p>';
    }
    if (data.dogego_note) {
      html += '<p class="bs-missing-tip">' + esc(data.dogego_note) + "</p>";
    }
    body.innerHTML = html;
    updateAddrScrollFooter(body);
    bindAddressBody(body, address, data);
  }

  function updateAddrScrollFooter(body) {
    const foot = body && body.querySelector("#bs-addr-scroll-footer");
    if (!foot) return;
    const recvShown = addrScroll.recvOffset;
    const recvTotal = addrScroll.recvTotal;
    const spendShown = addrScroll.spendOffset;
    const spendTotal = addrScroll.spendTotal;
    const total = recvTotal + spendTotal;
    const shown = recvShown + spendShown;
    if (recvShown >= recvTotal && spendShown >= spendTotal) {
      foot.hidden = true;
      return;
    }
    foot.hidden = false;
    const pct = total > 0 ? Math.min(100, Math.round((shown / total) * 100)) : 0;
    const fill = foot.querySelector(".wallet-tx-progress-fill");
    const count = foot.querySelector(".wallet-tx-progress-count");
    if (fill) fill.style.width = pct + "%";
    if (count) count.textContent = shown.toLocaleString() + " / " + total.toLocaleString();
  }

  function stepBlock(delta) {
    const maxH = meta ? navigableHeight() : currentHeight + delta;
    const h = Math.max(0, Math.min(maxH, currentHeight + delta));
    openBlock(h, true);
  }

  function onShow() {
    const root = $("panel-blockstep");
    if (!root) return;
    const raw = curHash();
    const deep = raw.startsWith("blockstep/");
    const finish = () => {
      if (deep) {
        if (loadedRoute === raw || routeLoading === raw) return;
        parseHashRoute();
        return;
      }
      loadedRoute = raw;
      setView("timeline");
      renderTimeline();
    };
    if (root.dataset.loaded !== "1") {
      root.dataset.loaded = "1";
      if (!deep) {
        setView("timeline");
        currentTime = currentTime || GENESIS_TS;
        currentHeight = currentHeight || 0;
        renderTimeline();
        updateIndexNote();
      }
    }
    ensureMeta(finish);
  }

  function refreshTx(txid) {
    if (!txid) return;
    const route = "blockstep/tx/" + encodeURIComponent(txid);
    if (view === "tx" && loadedRoute === route) {
      void openTx(txid, false, true);
    }
  }

  function parseHashRoute() {
    const raw = curHash();
    const parts = raw.split("/");
    if (parts[0] !== "blockstep") return;
    if (parts[1] === "block" && parts[2]) {
      openBlock(parseInt(parts[2], 10), false);
    } else if (parts[1] === "tx" && parts[2]) {
      openTx(decodeURIComponent(parts[2]), false);
    } else if (parts[1] === "address" && parts[2]) {
      openAddress(decodeURIComponent(parts[2]), false);
    } else {
      loadedRoute = raw;
      setView("timeline");
    }
  }

  function bindUI() {
    $("bs-back-btn")?.addEventListener("click", popHistory);
    $("bs-prev-block")?.addEventListener("click", () => stepBlock(-1));
    $("bs-next-block")?.addEventListener("click", () => stepBlock(1));
    $("bs-jump-genesis")?.addEventListener("click", async () => {
      ensureMeta(async () => {
        pushHistory({ view, height: currentHeight, time: currentTime });
        await syncHeightFromTime(timelineStart());
        setView("timeline");
      });
    });
    $("bs-jump-tip")?.addEventListener("click", () => {
      ensureMeta(() => {
        pushHistory({ view, height: currentHeight, time: currentTime });
        currentHeight = navigableHeight();
        currentTime = navigableTime();
        renderTimeline();
        setView("timeline");
      });
    });
    $("bs-open-block")?.addEventListener("click", () => openBlock(currentHeight, true));
    const slider = $("bs-time-slider");
    if (slider) {
      let timer = null;
      slider.addEventListener("input", () => {
        currentTime = parseInt(slider.value, 10);
        $("bs-time-label") && ($("bs-time-label").textContent = fmtDate(currentTime) + " · " + fmtTime(currentTime));
        drawTimelineTrack(parseInt(slider.min, 10), parseInt(slider.max, 10));
        clearTimeout(timer);
        timer = setTimeout(() => {
          syncHeightFromTime(currentTime).catch(() => {});
        }, 200);
      });
    }
  }

  bindUI();
  global.DogeGoBlockStep = { onShow, parseHashRoute, openTx, openBlock, openAddress, refreshTx };
})(window);
