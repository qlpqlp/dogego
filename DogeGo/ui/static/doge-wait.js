/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* DogeGo ... shared “much wait” loading animation for async UI */
(function (global) {
  const DEFAULT_MESSAGES = [
    "Much fetch. Very wait.",
    "Wow. So loading.",
    "Doge is digging for data…",
    "Sniffing the chain… such patience.",
    "One moment. Many treat soon.",
    "Very sync. Much retrieve.",
    "Hold the leash ... data inbound.",
    "Such query. Wow.",
  ];

  function esc(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function pickMessage(custom) {
    if (custom) return String(custom);
    return DEFAULT_MESSAGES[Math.floor(Math.random() * DEFAULT_MESSAGES.length)];
  }

  function html(message, opts) {
    const o = opts || {};
    const msg = pickMessage(message);
    const compact = !!o.compact;
    const inline = !!o.inline;
    const cls =
      "doge-wait" +
      (compact ? " doge-wait-compact" : "") +
      (inline ? " doge-wait-inline" : "") +
      (o.block === false ? "" : "");
    const size = compact || inline ? 28 : 48;
    const logo = (global.DogeGoLogo || "/dogecoin.svg");
    const stage =
      '<div class="doge-wait-stage" aria-hidden="true">' +
      '<span class="doge-wait-ring"></span>' +
      '<img class="doge-wait-coin" src="' + esc(logo) + '" width="' + size + '" height="' + size + '" alt="" />' +
      '<span class="doge-wait-tag">wow</span>' +
      '<span class="doge-wait-paw doge-wait-paw-1"></span>' +
      '<span class="doge-wait-paw doge-wait-paw-2"></span>' +
      '<span class="doge-wait-paw doge-wait-paw-3"></span>' +
      "</div>";
    if (inline) {
      const logoInline = (global.DogeGoLogo || "/dogecoin.svg");
      return (
        '<span class="' + cls + '" role="status" aria-live="polite">' +
        '<img class="doge-wait-coin-inline" src="' + esc(logoInline) + '" width="20" height="20" alt="" />' +
        '<span class="doge-wait-msg">' + esc(msg) + "</span>" +
        "</span>"
      );
    }
    return (
      '<div class="' + cls + '" role="status" aria-live="polite">' +
      stage +
      '<p class="doge-wait-msg">' + esc(msg) + "</p>" +
      "</div>"
    );
  }

  function set(el, message, opts) {
    if (!el) return;
    el.innerHTML = html(message, opts);
    el.classList.add("doge-wait-host");
  }

  function mount(el, message, opts) {
    set(el, message || el.getAttribute("data-doge-wait-msg"), opts);
  }

  function mountAll(root) {
    const base = root || document;
    base.querySelectorAll("[data-doge-wait]").forEach((el) => {
      const compact = el.hasAttribute("data-doge-wait-compact");
      const inline = el.hasAttribute("data-doge-wait-inline");
      mount(el, el.getAttribute("data-doge-wait-msg"), { compact, inline });
    });
  }

  global.DogeGoWait = { html, set, mount, mountAll, messages: DEFAULT_MESSAGES };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => mountAll());
  } else {
    mountAll();
  }
})(typeof window !== "undefined" ? window : globalThis);
