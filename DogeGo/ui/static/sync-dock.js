/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Fixed footer sync bar (Core-style): always visible with live node status. */
(function (global) {
  let expanded = false;
  let logsOpen = false;
  let lastLogAt = 0;

  function $(id) { return document.getElementById(id); }

  function chainActiveHeight(s) {
    const h = Number(s && (s.chain_active_height != null ? s.chain_active_height : s.contiguous_raw_height));
    return isFinite(h) && h >= 0 ? h : -1;
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

  function formatBodyIBDEtaMinutes(minutes) {
    const m = Number(minutes);
    if (!isFinite(m) || m <= 0) return "";
    if (m < 60) return m <= 1 ? "about 1 minute" : "about " + Math.ceil(m) + " minutes";
    if (m < 1440) {
      const h = Math.ceil(m / 60);
      return h <= 1 ? "about 1 hour" : "about " + h + " hours";
    }
    const d = Math.ceil(m / 1440);
    return d <= 1 ? "about 1 day" : "about " + d + " days";
  }

  function resolveSyncEta(s, labels) {
    if (s && s.sync_eta) return s.sync_eta;
    const bodyEta = formatBodyIBDEtaMinutes(s && s.dogego_body_ibd_eta_minutes);
    if (bodyEta) return bodyEta;
    const phase = labels && labels.phase ? labels.phase : "";
    if (phase.indexOf("header") >= 0) return "estimating…";
    return "...";
  }

  function connectLagDominant(s) {
    const lag = Number(s && (s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect));
    const rate = Number(s && s.dogego_connect_blocks_per_minute);
    return isFinite(lag) && lag > 64 && isFinite(rate) && rate > 0.05;
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
      if (bodyPct != null && isFinite(bodyPct)) pct = Math.max(pct, bodyPct);
    } else if (s && s.dogego_tx_verification_progress != null) {
      pct = Number(s.dogego_tx_verification_progress);
    }
    if (!isFinite(pct)) pct = 0;
    return capSyncPctByNetworkLag(s, Math.min(100, Math.max(0, Math.round(pct * 100))));
  }

  function formatLaneInFlight(lanes) {
    if (!lanes || typeof lanes !== "object") return "";
    const parts = Object.entries(lanes)
      .filter((e) => e[1] > 0)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 3)
      .map((e) => e[0] + ": " + e[1]);
    return parts.length ? " In-flight: " + parts.join("; ") + "." : "";
  }

  function syncPhaseLabels(s) {
    const tip = Number(s && s.tip_height);
    const pct = syncProgressPct(s);
    const mode = ((s && s.node_mode) || "full").toLowerCase();
    if (mode === "spv") {
      const text = isFinite(tip) && tip > 0
        ? "SPV headers · " + tip.toLocaleString()
        : "SPV · syncing headers";
      return { phase: text, sub: "", pct };
    }
    if (s && (s.dogego_ui_loading === true || s.warming_up === true) && !s.from_disk_snapshot) {
      const detail = (s.dogego_ui_loading_detail && String(s.dogego_ui_loading_detail)) ||
        "Loading local data…";
      const phaseMap = {
        warming: "Loading local data",
        utxo_cache: "Connecting blocks",
        snapshot_replay: "Connecting stored bodies",
        wallet_scan: "Scanning wallet",
        analytics: "Preparing analytics"
      };
      const phaseKey = s.dogego_ui_loading_phase;
      const phase = (phaseKey && phaseMap[phaseKey]) || "Loading local data";
      return { phase: phase, sub: detail, pct: pct, loading: true };
    }
    if (s && (s.from_disk_snapshot === true || s.summary_stale === true)) {
      const tipLbl = isFinite(tip) && tip >= 0 ? tip.toLocaleString() + " headers" : "last known tip";
      return {
        phase: "Updating…",
        sub: "Showing last known data · refreshing (" + tipLbl + ")",
        pct: pct,
        stale: true
      };
    }
    const behind = Number(s.blocks_behind_headers);
    const bodyPct = s.dogego_body_verification_progress != null
      ? Number(s.dogego_body_verification_progress) : null;
    const bodiesLag = bodyPct != null && isFinite(bodyPct) && bodyPct < 0.999 && isFinite(tip) && tip > 0;
    const ibd = s.initialblockdownload === true || s.ibd_active === true
      || (isFinite(behind) && behind > 32 && pct < 100)
      || bodiesLag
      || s.dogego_genesis_missing === true;
    const peerH = Number(s.peer_start_height);
    const headerCatchUp = isHeaderCatchUpPhase(s);
    let phase = "Connecting";
    let sub = "";
    if (pct >= 100 && !ibd && !bodiesLag) {
      const outH = Number(s.chain_active_height != null ? s.chain_active_height : s.contiguous_raw_height);
      const mp = Number(s.mempool_txs);
      const outN = Number(s.connections_out) || 0;
      const inN = Number(s.connections_in) || 0;
      let sub = "";
      if (isFinite(outH) && outH >= 0) sub += outH.toLocaleString() + " connected";
      if (isFinite(mp)) sub += (sub ? " · " : "") + mp.toLocaleString() + " mempool";
      if (outN || inN) sub += (sub ? " · " : "") + outN + "/" + inN + " peers";
      return { phase: "Up to date", sub: sub, pct: 100 };
    }
    if (headerCatchUp && isFinite(tip) && isFinite(peerH) && peerH > tip) {
      phase = "Downloading headers";
      const hdrPct = s.dogego_header_ibd_progress != null
        ? Math.round(Number(s.dogego_header_ibd_progress) * 100) : pct;
      sub = tip.toLocaleString() + " / ~" + peerH.toLocaleString() + " (" + hdrPct + "% headers)";
    } else if (connectLagDominant(s)) {
      phase = "Connecting blocks";
      const lag = Number(s.dogego_connect_lag || s.dogego_stored_bodies_ahead_connect);
      sub = isFinite(lag) && lag > 0 ? lag.toLocaleString() + " blocks behind chainActive" : "Replaying stored bodies";
      const boost = formatConnectCatchUpBoost(s);
      if (boost) sub += " · boost " + boost;
    } else if (ibd) {
      phase = "Syncing blocks";
      const parts = [];
      if (s.dogego_body_ibd_header_paused && s.dogego_body_verification_progress != null) {
        const bp = Math.round(Number(s.dogego_body_verification_progress) * 100);
        if (bp > 0 && bp < 100) parts.push(bp + "% bodies");
      }
      if (isFinite(behind) && behind > 0) parts.push(behind.toLocaleString() + " behind");
      const eta = resolveSyncEta(s, { phase });
      if (eta) parts.push("~" + eta);
      const rate = Number(s.blocks_per_minute);
      if (isFinite(rate) && rate > 0) parts.push(rate.toFixed(1) + " blk/min");
      if (s.dogego_body_ibd_header_paused) parts.push("headers paused");
      sub = parts.join(" · ") || (s.sync_status_line || "");
    } else if (s.headers_syncing) {
      phase = "Syncing headers";
      sub = isFinite(tip) ? "Tip " + tip.toLocaleString() : "Finding peers…";
    } else {
      phase = "Syncing";
      sub = s.sync_status_line || "";
    }
    const act = s.dogego_sync_activity;
    if (act && act.headline && s.dogego_sync_health !== "forward_ibd_stalled") {
      if (act.headline) phase = act.headline;
      if (act.detail) sub = act.detail;
    }
    return { phase, sub, pct };
  }

  function needsDock(s) {
    return !!s;
  }

  function setBodyPad(show, tall) {
    document.body.classList.toggle("sync-dock-visible", show);
    document.body.classList.toggle("sync-dock-expanded", show && tall);
  }

  function setExpanded(on) {
    expanded = !!on;
    const panel = $("sync-dock-panel");
    const btn = $("sync-dock-bar-btn");
    if (panel) panel.hidden = !expanded;
    if (btn) btn.setAttribute("aria-expanded", expanded ? "true" : "false");
    const chev = $("sync-dock-chevron");
    if (chev) chev.textContent = expanded ? "expand_more" : "expand_less";
    setBodyPad($("sync-dock") && !$("sync-dock").hidden, expanded && logsOpen);
  }

  function setLogsOpen(on) {
    logsOpen = !!on;
    const pre = $("sync-dock-logs");
    const btn = $("sync-dock-logs-btn");
    if (pre) pre.hidden = !logsOpen;
    if (btn) {
      btn.innerHTML = '<span class="material-icons-round" aria-hidden="true">terminal</span> ' +
        (logsOpen ? "Hide logs" : "Show logs");
    }
    setBodyPad($("sync-dock") && !$("sync-dock").hidden, expanded && logsOpen);
    if (logsOpen) {
      if (pre && window.DogeGoWait) window.DogeGoWait.set(pre, "Tailing sync logs…", { compact: true });
      loadLogs();
    }
  }

  function collapseAll() {
    setLogsOpen(false);
    setExpanded(false);
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
    const pre = $("sync-dock-logs");
    if (!pre || !logsOpen) return;
    const now = Date.now();
    if (now - lastLogAt < 2000) return;
    lastLogAt = now;
    try {
      const r = await fetch("/api/logs?limit=40", { cache: "no-store", credentials: "same-origin" });
      if (!r.ok) return;
      const j = await r.json();
      pre.textContent = formatLogLines(j.lines) || "(waiting for log lines…)";
      pre.scrollTop = pre.scrollHeight;
    } catch (_) {}
  }

  function setMetric(id, text) {
    const el = $(id);
    if (!el) return;
    el.classList.remove("ui-pending");
    el.removeAttribute("aria-busy");
    el.querySelectorAll(":scope > .ui-skel-bar").forEach((n) => n.remove());
    el.textContent = text;
    if (global.DogeGoFormat && global.DogeGoFormat.clearCompactStat) {
      // Strip tip state after text set only when not a compact target.
      // clearCompactStat also clears text? No, but it removes tip classes.
      // Defer: leave tip attrs; callers that need plain text pass through clear.
      el.classList.remove("stat-num", "has-num-tip", "is-tip-open");
      el.removeAttribute("data-full");
      el.removeAttribute("aria-label");
      if (el.dataset && el.dataset.numTipTab === "1") {
        el.removeAttribute("tabindex");
        delete el.dataset.numTipTab;
      }
    }
  }

  function setCompactMetric(id, n, opts) {
    const el = $(id);
    if (!el) return;
    if (global.DogeGoFormat && global.DogeGoFormat.setCompactStat) {
      global.DogeGoFormat.setCompactStat(el, n, opts);
      return;
    }
    const x = Number(n);
    const suffix = (opts && opts.suffix) || "";
    if (!isFinite(x) || ((opts && opts.requireNonNeg) && x < 0)) {
      setMetric(id, (opts && opts.fallback) || "...");
      return;
    }
    setMetric(id, x.toLocaleString() + suffix);
  }

  function resumeWarnText(warnings) {
    if (!Array.isArray(warnings) || !warnings.length) return "";
    const i18nFn = (global.DogeGoI18n && global.DogeGoI18n.t) ? global.DogeGoI18n.t.bind(global.DogeGoI18n) : null;
    const prefix = i18nFn ? i18nFn("pages.overview.ibdResumeWarn") : "Restart resume";
    const labels = warnings.map((w) => {
      const key = "syncDock.warn." + w;
      return i18nFn ? (i18nFn(key) || w) : w.replace(/_/g, " ");
    });
    return prefix + ": " + labels.join(", ");
  }

  let lastOperatorCert = null;

  function setOperatorCert(c) {
    lastOperatorCert = c || null;
    updateOperatorCertMetric();
  }

  function updateOperatorCertMetric() {
    const el = $("sync-dock-operator-cert");
    if (!el) return;
    const c = lastOperatorCert;
    if (!c || !c.total) {
      // Keep skeleton until first cert payload arrives.
      if (!el.classList.contains("ui-pending")) {
        setMetric("sync-dock-operator-cert", "...");
      }
      return;
    }
    const i18nFn = (global.DogeGoI18n && global.DogeGoI18n.t) ? global.DogeGoI18n.t.bind(global.DogeGoI18n) : null;
    const okLbl = i18nFn ? i18nFn("syncDock.operatorCertOk") : "ok";
    const failLbl = i18nFn ? i18nFn("syncDock.operatorCertFail") : "check";
    let txt = c.pass + "/" + c.total + " " + (c.live_ok ? okLbl : failLbl);
    if (c.solo_pass != null && (c.solo_ok !== c.live_ok || c.solo_pass !== c.pass)) {
      const soloLbl = i18nFn ? i18nFn("pages.overview.operatorCertSolo") : "solo";
      txt += " · " + soloLbl + " " + c.solo_pass + "/" + c.total;
    }
    if (c.corpus_total) {
      const corpLbl = i18nFn ? i18nFn("syncDock.mempoolCorpusShort") : "corpus";
      txt += " · " + corpLbl + " " + (c.corpus_passed || 0) + "/" + c.corpus_total;
    }
    setMetric("sync-dock-operator-cert", txt);
  }

  function update(s) {
    const dock = $("sync-dock");
    if (!dock) return;
    const show = needsDock(s);
    dock.hidden = !show;
    setBodyPad(show, expanded && logsOpen);
    if (!show) {
      return;
    }
    const labels = syncPhaseLabels(s);
    const pct = labels.pct;
    const fill = $("sync-dock-fill");
    if (fill) {
      fill.style.width = (labels.loading ? 35 : pct) + "%";
      fill.classList.toggle("sync-dock-fill--pulse", !!labels.loading);
      fill.classList.toggle("sync-dock-fill--stale", !!labels.stale);
    }
    dock.classList.toggle("sync-dock--loading", !!labels.loading);
    dock.classList.toggle("sync-dock--stale", !!labels.stale);
    const prog = $("sync-dock-progress");
    if (prog) prog.setAttribute("aria-valuenow", String(labels.loading ? 0 : pct));
    setMetric("sync-dock-phase", labels.phase);
    setMetric("sync-dock-pct", labels.loading ? "…" : (pct + "%"));
    const sub = $("sync-dock-sub");
    if (sub) {
      sub.textContent = labels.sub || "";
      sub.hidden = !labels.sub;
    }
    const resumeWarn = $("sync-dock-resume-warn");
    if (resumeWarn) {
      const txt = resumeWarnText(s && s.dogego_restart_resume_warnings);
      resumeWarn.textContent = txt;
      resumeWarn.hidden = !txt;
    }
    const tip = Number(s.tip_height);
    const active = chainActiveHeight(s);
    const cont = Number(s.contiguous_raw_height);
    const behind = Number(s.blocks_behind_headers);
    const rate = Number(s.blocks_per_minute);
    if (active >= 0 && isFinite(tip)) {
      const connEl = $("sync-dock-connected");
      if (connEl && global.DogeGoFormat) {
        connEl.classList.remove("ui-pending");
        connEl.removeAttribute("aria-busy");
        connEl.querySelectorAll(":scope > .ui-skel-bar").forEach((n) => n.remove());
        const aTxt = global.DogeGoFormat.formatCompactNumber(active, { integer: true });
        const tTxt = global.DogeGoFormat.formatCompactNumber(tip, { integer: true });
        const aFull = global.DogeGoFormat.formatFullNumber(active, { integer: true });
        const tFull = global.DogeGoFormat.formatFullNumber(tip, { integer: true });
        connEl.textContent = aTxt + " / " + tTxt;
        if (Math.abs(active) >= global.DogeGoFormat.COMPACT_THRESHOLD ||
            Math.abs(tip) >= global.DogeGoFormat.COMPACT_THRESHOLD) {
          global.DogeGoFormat.markNumTip(connEl, aFull + " / " + tFull);
        } else {
          global.DogeGoFormat.clearCompactStat(connEl);
          connEl.textContent = aTxt + " / " + tTxt;
        }
      } else {
        setMetric("sync-dock-connected", active.toLocaleString() + " / " + tip.toLocaleString());
      }
    } else {
      setMetric("sync-dock-connected", "...");
    }
    if (isFinite(cont)) setCompactMetric("sync-dock-stored", cont, { integer: true });
    else setMetric("sync-dock-stored", "...");
    if (isFinite(behind) && behind > 0) setCompactMetric("sync-dock-behind", behind, { integer: true });
    else setMetric("sync-dock-behind", "0");
    const bodyLag = Number(s.dogego_body_lag_headers);
    if (isFinite(bodyLag) && bodyLag > 0) setCompactMetric("sync-dock-body-lag", bodyLag, { integer: true });
    else setMetric("sync-dock-body-lag", "0");
    const cpProbe = Number(s.dogego_checkpoint_probe);
    if (isFinite(cpProbe) && cpProbe >= 0) setCompactMetric("sync-dock-checkpoint", cpProbe, { integer: true });
    else setMetric("sync-dock-checkpoint", "...");
    const pool = Number(s.assist_peer_pool);
    setMetric("sync-dock-assist-pool", isFinite(pool) && pool > 0 ? String(pool) : "0");
    setMetric("sync-dock-rate", isFinite(rate) && rate > 0 ? rate.toFixed(1) + " blk/min" : "...");
    const boost = formatConnectCatchUpBoost(s);
    const boostWrap = $("sync-dock-connect-boost-wrap");
    if (boostWrap) boostWrap.hidden = !boost;
    setMetric("sync-dock-connect-boost", boost || "...");
    setMetric("sync-dock-eta", resolveSyncEta(s, labels));
    const out = Number(s.connections_out) || 0;
    const inn = Number(s.connections_in) || 0;
    setMetric("sync-dock-peers", out + "/" + inn);
    const mp = Number(s.mempool_txs);
    if (isFinite(mp)) setCompactMetric("sync-dock-mempool", mp, { integer: true });
    else setMetric("sync-dock-mempool", "0");
    const txp = Number(s.transactions_processed);
    if (isFinite(txp) && txp >= 0) setCompactMetric("sync-dock-tx-processed", txp, { integer: true, requireNonNeg: true });
    else setMetric("sync-dock-tx-processed", "...");
    updateOperatorCertMetric();
    if (global.DogeGoApplyUtxoReplayUI) global.DogeGoApplyUtxoReplayUI(s);
    if (logsOpen) loadLogs();
  }

  function init() {
    const barBtn = $("sync-dock-bar-btn");
    if (barBtn) {
      barBtn.addEventListener("click", (e) => {
        if (e.target.closest("#sync-dock-collapse") || e.target.closest("#sync-dock-logs-btn")) return;
        setExpanded(!expanded);
      });
    }
    $("sync-dock-logs-btn") && $("sync-dock-logs-btn").addEventListener("click", (e) => {
      e.stopPropagation();
      setLogsOpen(!logsOpen);
    });
    $("sync-dock-collapse") && $("sync-dock-collapse").addEventListener("click", (e) => {
      e.stopPropagation();
      collapseAll();
    });
  }

  global.DogeGoSyncDock = { init, update, collapseAll, setOperatorCert };
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window);
