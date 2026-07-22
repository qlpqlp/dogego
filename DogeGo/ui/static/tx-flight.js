/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Top transaction flight bar: background broadcast retry + live status polling. */
(function (global) {
  const STORAGE_KEY = "dogego_tx_flights";
  const POLL_MS = 2500;
  const RETRY_MS = 12000;
  const MAX_AGE_MS = 48 * 60 * 60 * 1000;
  let timer = null;

  function $(id) { return document.getElementById(id); }

  function loadFlights() {
    try {
      const raw = sessionStorage.getItem(STORAGE_KEY);
      const arr = raw ? JSON.parse(raw) : [];
      return Array.isArray(arr) ? arr : [];
    } catch (_) {
      return [];
    }
  }

  function saveFlights(list) {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(list));
    } catch (_) {}
  }

  function esc(s) {
    return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/"/g, "&quot;");
  }

  function shortTxid(txid) {
    if (!txid || txid.length < 16) return txid || "";
    return txid.slice(0, 8) + "…" + txid.slice(-8);
  }

  function statusLabel(st, f) {
    if (f && f.last_error && st !== "confirmed" && st !== "mempool") {
      return "Send failed";
    }
    switch (st) {
      case "queued": return "Queued";
      case "signing": return "Signing…";
      case "broadcasting": return "Broadcasting…";
      case "mempool": return "In mempool";
      case "confirmed": return "Confirmed";
      case "failed": return "Send failed";
      default: return "Tracking…";
    }
  }

  function statusClass(st, f) {
    if (st === "confirmed") return "ok";
    if (st === "failed" || (f && f.last_error && st !== "mempool")) return "err";
    if (st === "mempool") return "mempool";
    if (st === "broadcasting") return "broadcast";
    if (st === "signing") return "signing";
    if (st === "queued") return "queued";
    return "idle";
  }

  function hasBusyFlight() {
    const now = Date.now();
    return loadFlights().some((f) => {
      if (!f) return false;
      if (f.status === "signing" || f.status === "queued" || String(f.txid).startsWith("pending-") || String(f.txid).startsWith("q-")) return true;
      if (f.status === "broadcasting" && now - (f.created_at || 0) < 3 * 60 * 1000) return true;
      return false;
    });
  }

  function flightPhase(st) {
    if (st === "confirmed") return "success";
    if (st === "failed") return "failed";
    if (st === "mempool") return "mempool";
    if (st === "broadcasting") return "broadcast";
    if (st === "queued") return "queued";
    if (st === "signing") return "signing";
    return "idle";
  }

  function pruneFlights(list) {
    const now = Date.now();
    return list.filter((f) => {
      if (!f || !f.txid) return false;
      if (String(f.txid).startsWith("pending-")) {
        const age = now - (f.created_at || 0);
        if (age > 120000) return false;
      }
      if (f.status === "confirmed" && now - (f.updated_at || f.created_at || 0) > 10 * 60 * 1000) return false;
      if (now - (f.created_at || 0) > MAX_AGE_MS) return false;
      return true;
    });
  }

  function statusIcon(st) {
    switch (st) {
      case "queued": return "schedule";
      case "signing": return "draw";
      case "broadcasting": return "cell_tower";
      case "mempool": return "hourglass_top";
      case "confirmed": return "check_circle";
      case "failed": return "error_outline";
      default: return "sync";
    }
  }

  function statusDetail(f) {
    if (f.last_error) return f.last_error;
    if (f.status === "confirmed" && f.confirmations > 0) {
      return f.confirmations + " confirmation" + (f.confirmations === 1 ? "" : "s");
    }
    if (f.status === "mempool" || f.in_mempool) return "Waiting for block";
    if (f.status === "broadcasting") return "Propagating to network";
    if (f.status === "signing") {
      if (f.signing_detail) return f.signing_detail;
      return "Funding and signing";
    }
    if (f.status === "queued") return "Waiting for node to be ready";
    if (f.status === "failed") return "Send failed";
    return "Tracking";
  }

  function setBodyPad(on) {
    document.body.classList.toggle("tx-flight-visible", on);
  }

  function shuttleHtml(f) {
    const phase = flightPhase(f.status);
    const endIcon =
      f.status === "confirmed" ? "check_circle"
        : f.status === "failed" ? "error_outline"
          : f.status === "mempool" ? "hourglass_top"
            : "cell_tower";
    const logo = (typeof window !== "undefined" && window.DogeGoLogo) || "/dogecoin.svg";
    return (
      '<div class="tx-flight-shuttle tx-flight-phase-' + phase + '" aria-hidden="true">' +
      '<span class="tx-flight-shuttle-track"></span>' +
      '<img src="' + esc(logo) + '" alt="" class="tx-flight-shuttle-doge" />' +
      '<span class="material-icons-round tx-flight-shuttle-end">' + endIcon + "</span>" +
      "</div>"
    );
  }

  function render() {
    const bar = $("tx-flight-bar");
    if (!bar) return;
    let list = pruneFlights(loadFlights());
    saveFlights(list);
    if (!list.length) {
      bar.hidden = true;
      setBodyPad(false);
      return;
    }
    bar.hidden = false;
    setBodyPad(true);
    const items = list.map((f) => {
      const txid = esc(f.txid);
      const isPending = String(f.txid).startsWith("pending-");
      const cls = statusClass(f.status, f);
      const label = esc(statusLabel(f.status, f));
      const detail = esc(statusDetail(f));
      const errHtml = f.last_error
        ? '<span class="tx-flight-error" title="' + esc(f.last_error) + '">' + esc(f.last_error) + "</span>"
        : "";
      const amt = f.amount != null ? '<span class="tx-flight-amt">' + Number(f.amount).toFixed(4) + " DOGE</span>" : "";
      const dest = f.address ? "→ " + esc(shortTxid(f.address)) : "";
      const link = isPending
        ? '<span class="tx-flight-link tx-flight-link-pending">…</span>'
        : '<a class="tx-flight-link" href="#blockstep/tx/' + txid + '" title="Open in BlockStep">' + esc(shortTxid(f.txid)) + "</a>";
      const detailExtra = [detail, dest].filter(Boolean).join(" · ");
      return (
        '<div class="tx-flight-row ' + cls + '" data-txid="' + txid + '">' +
        shuttleHtml(f) +
        '<div class="tx-flight-compact">' +
        '<span class="tx-flight-status">' + label + "</span>" +
        amt +
        (errHtml || (detailExtra ? '<span class="tx-flight-sep">·</span><span class="tx-flight-detail-inline">' + detailExtra + "</span>" : "")) +
        '<span class="tx-flight-sep">·</span>' +
        link +
        "</div>" +
        '<div class="tx-flight-details">' + (errHtml || detail) + (dest ? " · " + dest : "") + "</div>" +
        '<div class="tx-flight-actions">' +
        '<button type="button" class="tx-flight-expand-btn" data-txid="' + txid + '" aria-label="Show details"><span class="material-icons-round">expand_more</span></button>' +
        '<button type="button" class="tx-flight-dismiss" data-txid="' + txid + '" aria-label="Dismiss"><span class="material-icons-round">close</span></button>' +
        "</div></div>"
      );
    }).join("");
    bar.innerHTML = '<div class="tx-flight-inner">' + items + "</div>";
    requestAnimationFrame(() => {
      const h = bar.offsetHeight || 44;
      document.documentElement.style.setProperty("--tx-flight-offset", h + "px");
    });
  }

  function updateFlight(txid, patch) {
    const list = loadFlights();
    let found = false;
    for (let i = 0; i < list.length; i++) {
      if (list[i].txid === txid) {
        list[i] = Object.assign({}, list[i], patch, { updated_at: Date.now() });
        found = true;
        break;
      }
    }
    if (!found && patch) {
      list.unshift(Object.assign({ txid: txid, created_at: Date.now(), updated_at: Date.now() }, patch));
    }
    saveFlights(pruneFlights(list));
    render();
  }

  function trackSend(opts) {
    const txid = (opts && opts.txid) || "";
    if (!txid) return;
    const err = String((opts && (opts.broadcast_error || opts.last_error)) || "").trim();
    const st = err ? "failed" : ((opts && opts.status) || "broadcasting");
    updateFlight(txid, {
      hex: opts.hex || "",
      amount: opts.amount,
      address: opts.address,
      status: st,
      confirmations: 0,
      last_error: err,
    });
    ensureTimer();
  }

  function trackQueued(pendingId, opts) {
    const id = pendingId || ("pending-" + Date.now());
    updateFlight(id, {
      status: "queued",
      amount: opts && opts.amount,
      address: opts && opts.address,
      last_error: "",
    });
    ensureTimer();
    return id;
  }

  function trackFailure(id, opts) {
    const err = String((opts && (opts.error || opts.last_error || opts.broadcast_error)) || "Send failed").trim();
    const flightId = id || ("pending-" + Date.now());
    updateFlight(flightId, {
      status: "failed",
      amount: opts && opts.amount,
      address: opts && opts.address,
      hex: (opts && opts.hex) || "",
      last_error: err,
    });
    ensureTimer();
    return flightId;
  }

  function trackSigning(opts) {
    const id = "pending-" + Date.now();
    updateFlight(id, {
      status: "signing",
      signing_detail: "Funding and signing",
      amount: opts && opts.amount,
      address: opts && opts.address,
      hex: "",
    });
    return id;
  }

  function updateSigningFlight(pendingId, detail) {
    if (!pendingId || !String(pendingId).startsWith("pending-")) return;
    updateFlight(pendingId, {
      status: "signing",
      signing_detail: detail || "Funding and signing",
    });
  }

  async function pollOne(f) {
    if (!f || !f.txid || String(f.txid).startsWith("pending-")) return;
    const prevStatus = f.status;
    const prevMempool = !!f.in_mempool;
    const prevConf = Number(f.confirmations) || 0;
    try {
      const r = await fetch("/api/wallet/tx-flight?txid=" + encodeURIComponent(f.txid), { cache: "no-store", credentials: "same-origin" });
      if (!r.ok) return;
      const j = await r.json();
      const st = j.status || f.status;
      const conf = Number(j.confirmations) || 0;
      const inMempool = !!j.in_mempool;
      updateFlight(f.txid, {
        status: st,
        confirmations: conf,
        in_mempool: inMempool,
      });
      const confirmed = st === "confirmed" || conf >= 1;
      const leftMempool = prevMempool && !inMempool;
      const newlyConfirmed = confirmed && prevStatus !== "confirmed" && prevConf < 1;
      if (newlyConfirmed || (leftMempool && confirmed)) {
        notifyTxSettled(f.txid, { status: st, confirmations: conf, in_mempool: inMempool });
      }
    } catch (_) {}
  }

  function notifyTxSettled(txid, detail) {
    if (!txid) return;
    try {
      global.dispatchEvent(new CustomEvent("dogego-tx-settled", {
        detail: Object.assign({ txid: txid }, detail || {}),
      }));
    } catch (_) {}
  }

  async function retryOne(f) {
    if (!f || !f.hex || f.status === "confirmed" || f.status === "signing") return;
    const age = Date.now() - (f.last_retry_at || 0);
    if (age < RETRY_MS) return;
    updateFlight(f.txid, { last_retry_at: Date.now() });
    try {
      const r = await fetch("/api/wallet/broadcast", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({ hex: f.hex }),
      });
      const j = await r.json().catch(() => ({}));
      if (r.ok) {
        updateFlight(f.txid, {
          status: j.status || "broadcasting",
          last_error: "",
        });
      } else if (j.error) {
        updateFlight(f.txid, { last_error: j.error, status: "failed" });
      }
    } catch (e) {
      updateFlight(f.txid, { last_error: String(e), status: "failed" });
    }
  }

  async function tick() {
    let list = pruneFlights(loadFlights());
    saveFlights(list);
    if (!list.length) {
      if (timer) {
        clearInterval(timer);
        timer = null;
      }
      render();
      return;
    }
    if (!hasBusyFlight()) {
      for (const f of list) {
        await pollOne(f);
        await retryOne(f);
      }
    }
    render();
  }

  function ensureTimer() {
    if (timer) return;
    timer = setInterval(() => { void tick(); }, POLL_MS);
    void tick();
  }

  function dismiss(txid) {
    const list = loadFlights().filter((f) => f.txid !== txid);
    saveFlights(list);
    render();
    if (!list.length && timer) {
      clearInterval(timer);
      timer = null;
    }
  }

  function init() {
    render();
    const list = loadFlights();
    if (list.length) ensureTimer();
    const bar = $("tx-flight-bar");
    if (bar) {
      bar.addEventListener("click", (e) => {
        const dismissBtn = e.target.closest(".tx-flight-dismiss");
        if (dismissBtn) {
          e.preventDefault();
          dismiss(dismissBtn.getAttribute("data-txid"));
          return;
        }
        const expandBtn = e.target.closest(".tx-flight-expand-btn");
        if (expandBtn) {
          e.preventDefault();
          const row = expandBtn.closest(".tx-flight-row");
          if (row) row.classList.toggle("is-expanded");
        }
      });
    }
  }

  global.DogeGoTxFlight = {
    init,
    trackSend,
    trackSigning,
    updateSigningFlight,
    trackQueued,
    trackFailure,
    dismiss,
    render,
    hasBusyFlight,
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window);
