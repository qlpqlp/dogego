/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Locale-aware compact KPI numbers + floating full-value tooltip */
(function (global) {
  "use strict";

  var COMPACT_THRESHOLD = 10000;
  var tipEl = null;
  var tipBound = false;
  var openEl = null;

  function uiNumberLocale() {
    if (global.DogeGoI18n && typeof global.DogeGoI18n.getLocale === "function") {
      var loc = global.DogeGoI18n.getLocale();
      if (loc) return loc;
    }
    var lang = document.documentElement && document.documentElement.lang;
    return lang || undefined;
  }

  function formatFullNumber(n, opts) {
    opts = opts || {};
    var x = Number(n);
    if (!isFinite(x)) return opts.fallback != null ? opts.fallback : "…";
    var locale = opts.locale != null ? opts.locale : uiNumberLocale();
    var fmtOpts = {};
    if (opts.maximumFractionDigits != null) fmtOpts.maximumFractionDigits = opts.maximumFractionDigits;
    else if (opts.integer !== false && opts.maximumFractionDigits == null && opts.minimumFractionDigits == null) {
      if (Math.abs(x - Math.round(x)) < 1e-9) fmtOpts.maximumFractionDigits = 0;
    }
    if (opts.minimumFractionDigits != null) fmtOpts.minimumFractionDigits = opts.minimumFractionDigits;
    if (opts.maximumFractionDigits != null) fmtOpts.maximumFractionDigits = opts.maximumFractionDigits;
    try {
      return x.toLocaleString(locale, fmtOpts);
    } catch (_) {
      return String(x);
    }
  }

  function formatCompactNumber(n, opts) {
    opts = opts || {};
    var x = Number(n);
    if (!isFinite(x)) return opts.fallback != null ? opts.fallback : "…";
    var threshold = opts.threshold != null ? opts.threshold : COMPACT_THRESHOLD;
    if (Math.abs(x) < threshold) return formatFullNumber(x, opts);
    var locale = opts.locale != null ? opts.locale : uiNumberLocale();
    try {
      return new Intl.NumberFormat(locale, {
        notation: "compact",
        compactDisplay: "short",
        maximumFractionDigits: opts.compactFractionDigits != null ? opts.compactFractionDigits : 1,
      }).format(x);
    } catch (_) {
      return formatFullNumber(x, opts);
    }
  }

  function clearCompactStat(el) {
    if (!el) return;
    el.classList.remove("stat-num", "has-num-tip", "is-tip-open");
    el.removeAttribute("data-full");
    el.removeAttribute("aria-label");
    if (el.dataset && el.dataset.numTipTab === "1") {
      el.removeAttribute("tabindex");
      delete el.dataset.numTipTab;
    }
    if (openEl === el) hideNumTip();
  }

  function ensureTipHost() {
    if (tipEl && tipEl.isConnected) return tipEl;
    tipEl = document.getElementById("doge-num-tip");
    if (!tipEl) {
      tipEl = document.createElement("div");
      tipEl.id = "doge-num-tip";
      tipEl.className = "doge-num-tip";
      tipEl.setAttribute("role", "tooltip");
      tipEl.hidden = true;
      document.body.appendChild(tipEl);
    }
    return tipEl;
  }

  function positionTip(anchor) {
    var tip = ensureTipHost();
    var r = anchor.getBoundingClientRect();
    tip.hidden = false;
    tip.classList.add("is-visible");
    var tw = tip.offsetWidth || 0;
    var th = tip.offsetHeight || 0;
    var left = r.left + r.width / 2 - tw / 2;
    var top = r.top - th - 10;
    if (left < 8) left = 8;
    if (left + tw > window.innerWidth - 8) left = Math.max(8, window.innerWidth - tw - 8);
    if (top < 8) top = r.bottom + 10;
    tip.style.left = Math.round(left) + "px";
    tip.style.top = Math.round(top) + "px";
  }

  function showNumTip(el) {
    if (!el || !el.classList.contains("has-num-tip")) return;
    var full = el.getAttribute("data-full");
    if (!full) return;
    if (openEl && openEl !== el) openEl.classList.remove("is-tip-open");
    openEl = el;
    el.classList.add("is-tip-open");
    var tip = ensureTipHost();
    tip.textContent = full;
    positionTip(el);
  }

  function hideNumTip() {
    if (openEl) {
      openEl.classList.remove("is-tip-open");
      openEl = null;
    }
    if (tipEl) {
      tipEl.hidden = true;
      tipEl.classList.remove("is-visible");
      tipEl.textContent = "";
    }
  }

  function bindTipEvents() {
    if (tipBound) return;
    tipBound = true;
    document.addEventListener("pointerover", function (ev) {
      var t = ev.target && ev.target.closest ? ev.target.closest(".stat-num.has-num-tip") : null;
      if (!t) return;
      if (ev.pointerType === "touch") return;
      showNumTip(t);
    }, true);
    document.addEventListener("pointerout", function (ev) {
      var t = ev.target && ev.target.closest ? ev.target.closest(".stat-num.has-num-tip") : null;
      if (!t) return;
      if (ev.pointerType === "touch") return;
      var related = ev.relatedTarget;
      if (related && t.contains(related)) return;
      if (t.classList.contains("is-tip-open") && ev.pointerType !== "mouse") return;
      hideNumTip();
    }, true);
    document.addEventListener("focusin", function (ev) {
      var t = ev.target && ev.target.closest ? ev.target.closest(".stat-num.has-num-tip") : null;
      if (t) showNumTip(t);
    });
    document.addEventListener("focusout", function (ev) {
      var t = ev.target && ev.target.closest ? ev.target.closest(".stat-num.has-num-tip") : null;
      if (!t) return;
      var related = ev.relatedTarget;
      if (related && t.contains(related)) return;
      hideNumTip();
    });
    document.addEventListener("click", function (ev) {
      var t = ev.target && ev.target.closest ? ev.target.closest(".stat-num.has-num-tip") : null;
      var coarse = global.matchMedia && global.matchMedia("(hover: none)").matches;
      if (t) {
        if (coarse) {
          if (openEl === t && t.classList.contains("is-tip-open")) hideNumTip();
          else showNumTip(t);
        }
        return;
      }
      if (openEl) hideNumTip();
    });
    window.addEventListener("scroll", function () {
      if (openEl) hideNumTip();
    }, true);
    window.addEventListener("resize", function () {
      if (openEl) hideNumTip();
    });
  }

  function markNumTip(el, full) {
    if (!el || full == null || full === "") {
      if (el) clearCompactStat(el);
      return;
    }
    el.classList.add("stat-num", "has-num-tip");
    el.setAttribute("data-full", String(full));
    el.setAttribute("aria-label", String(full));
    if (!el.hasAttribute("tabindex")) {
      el.tabIndex = 0;
      el.dataset.numTipTab = "1";
    }
    bindTipEvents();
    if (openEl === el && tipEl && !tipEl.hidden) {
      tipEl.textContent = String(full);
      positionTip(el);
    }
  }

  function setCompactStat(el, n, opts) {
    opts = opts || {};
    if (!el) return;
    el.classList.remove("ui-pending");
    el.removeAttribute("aria-busy");
    el.querySelectorAll(":scope > .ui-skel-bar").forEach(function (node) {
      node.remove();
    });

    var x = Number(n);
    if (!isFinite(x) || (opts.requireNonNeg && x < 0)) {
      clearCompactStat(el);
      el.textContent = opts.fallback != null ? opts.fallback : "…";
      return;
    }

    var suffix = opts.suffix != null ? String(opts.suffix) : "";
    var threshold = opts.threshold != null ? opts.threshold : COMPACT_THRESHOLD;
    var fullCore = formatFullNumber(x, opts);
    var compactCore = formatCompactNumber(x, opts);
    var full = fullCore + suffix;
    var compact = compactCore + suffix;
    var useCompact = Math.abs(x) >= threshold;

    el.classList.add("stat-num");
    el.textContent = useCompact ? compact : full;

    if (useCompact) {
      markNumTip(el, full);
    } else {
      el.classList.remove("has-num-tip", "is-tip-open");
      el.removeAttribute("data-full");
      el.removeAttribute("aria-label");
      if (el.dataset && el.dataset.numTipTab === "1") {
        el.removeAttribute("tabindex");
        delete el.dataset.numTipTab;
      }
      if (openEl === el) hideNumTip();
    }

    if (opts.fit !== false && typeof global.DogeGoFitStat === "function") {
      global.DogeGoFitStat(el);
    }
  }

  global.DogeGoFormat = {
    COMPACT_THRESHOLD: COMPACT_THRESHOLD,
    uiNumberLocale: uiNumberLocale,
    formatFullNumber: formatFullNumber,
    formatCompactNumber: formatCompactNumber,
    setCompactStat: setCompactStat,
    clearCompactStat: clearCompactStat,
    markNumTip: markNumTip,
    showNumTip: showNumTip,
    hideNumTip: hideNumTip,
  };
})(window);
