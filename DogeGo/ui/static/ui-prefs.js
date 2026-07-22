/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Sidebar visibility, simple mode, and accessible help popovers. */
(function (global) {
  const LS_NAV = "dogego_nav_hidden";
  const LS_HELP = "dogego_help_always";
  const LS_SIMPLE = "dogego_ui_simple";

  const NAV_IDS = [
    "send", "receive", "transactions", "overview", "blockstep", "explorer", "mempool",
    "analytics", "docs", "features", "console", "extensions", "settings",
  ];

  const SIMPLE_HIDDEN = ["docs", "features", "console"];

  const NAV_LABELS = {
    send: "Send",
    receive: "Receive",
    transactions: "History",
    blockstep: "BlockStep",
    overview: "Overview",
    explorer: "Explorer",
    mempool: "Mempool",
    analytics: "Analytics",
    docs: "Docs",
    features: "Features",
    console: "Console",
    extensions: "Extensions",
    settings: "Settings",
  };

  function loadHidden() {
    try {
      const raw = localStorage.getItem(LS_NAV);
      if (!raw) return [];
      const arr = JSON.parse(raw);
      return Array.isArray(arr) ? arr : [];
    } catch (_) {
      return [];
    }
  }

  function saveHidden(list) {
    localStorage.setItem(LS_NAV, JSON.stringify(list));
  }

  function isSimpleMode() {
    return localStorage.getItem(LS_SIMPLE) === "1";
  }

  function applySimpleMode() {
    document.body.classList.toggle("ui-simple", isSimpleMode());
  }

  function initSimpleDefaults() {
    if (localStorage.getItem(LS_SIMPLE) === null) {
      localStorage.setItem(LS_SIMPLE, "0");
    }
    applySimpleMode();
  }

  function setSimpleMode(on) {
    localStorage.setItem(LS_SIMPLE, on ? "1" : "0");
    if (on) {
      const hidden = new Set(loadHidden());
      SIMPLE_HIDDEN.forEach((id) => hidden.add(id));
      saveHidden([...hidden]);
    }
    applySimpleMode();
    applyNavVisibility();
    buildNavToggles(document.getElementById("st-nav-toggles"));
  }

  function applyNavVisibility() {
    const hidden = new Set(loadHidden());
    document.querySelectorAll(".nav-item[data-nav-id], .bottom-nav-item[data-nav-id], .nav-ext-block[data-nav-id]").forEach((el) => {
      const id = el.getAttribute("data-nav-id");
      const hide = hidden.has(id);
      el.hidden = hide;
      el.classList.toggle("nav-hidden", hide);
    });
    const extLabel = document.querySelector(".nav-ext-group-label");
    if (extLabel) extLabel.hidden = hidden.has("extensions");
    document.querySelectorAll(".nav-group-label").forEach((label) => {
      let sib = label.nextElementSibling;
      let any = false;
      while (sib && !sib.classList.contains("nav-group-label") && !sib.classList.contains("nav-divider")) {
        if (sib.matches?.(".nav-item") && !sib.hidden) any = true;
        sib = sib.nextElementSibling;
      }
      label.hidden = !any;
    });
  }

  function navLabel(id) {
    if (global.DogeGoI18n) {
      var tr = global.DogeGoI18n.t("nav." + id);
      if (tr && tr !== "nav." + id) return tr;
    }
    return NAV_LABELS[id] || id;
  }

  function buildNavToggles(container) {
    if (!container) return;
    container.innerHTML = "";
    const hidden = new Set(loadHidden());
    NAV_IDS.forEach((id) => {
      const row = document.createElement("div");
      row.className = "nav-toggle-row";
      const span = document.createElement("span");
      span.textContent = (global.DogeGoI18n ? global.DogeGoI18n.t("common.show", { name: navLabel(id) }) : "Show " + navLabel(id));
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.setAttribute("role", "switch");
      cb.checked = !hidden.has(id);
      cb.dataset.navId = id;
      cb.addEventListener("change", () => {
        const h = loadHidden().filter((x) => x !== id);
        if (!cb.checked) h.push(id);
        saveHidden(h);
        applyNavVisibility();
      });
      row.appendChild(span);
      row.appendChild(cb);
      container.appendChild(row);
    });
  }

  let helpPopover = null;

  function ensureHelpPopover() {
    if (helpPopover) return helpPopover;
    helpPopover = document.createElement("div");
    helpPopover.id = "help-popover";
    helpPopover.className = "help-popover";
    helpPopover.setAttribute("role", "tooltip");
    helpPopover.hidden = true;
    document.body.appendChild(helpPopover);
    return helpPopover;
  }

  function showHelp(btn) {
    const text = btn.getAttribute("data-help") || btn.getAttribute("data-help-title");
    if (!text) return;
    const pop = ensureHelpPopover();
    pop.textContent = text;
    pop.hidden = false;
    const r = btn.getBoundingClientRect();
    pop.style.left = Math.min(r.left, window.innerWidth - 320) + "px";
    pop.style.top = (r.bottom + 8) + "px";
  }

  function hideHelp() {
    if (helpPopover) helpPopover.hidden = true;
  }

  function bindHelp() {
    const always = localStorage.getItem(LS_HELP) === "1";
    document.body.classList.toggle("help-always", always);
    document.querySelectorAll(".help-btn[data-help]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (helpPopover && !helpPopover.hidden && helpPopover._source === btn) {
          hideHelp();
          return;
        }
        showHelp(btn);
        if (helpPopover) helpPopover._source = btn;
      });
      btn.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          btn.click();
        }
      });
    });
    document.addEventListener("click", (e) => {
      if (!e.target.closest?.(".help-btn") && !e.target.closest?.("#help-popover")) hideHelp();
    });
    document.getElementById("st-help-always")?.addEventListener("change", (e) => {
      localStorage.setItem(LS_HELP, e.target.checked ? "1" : "0");
      document.body.classList.toggle("help-always", e.target.checked);
    });
    const simpleCb = document.getElementById("st-simple-mode");
    if (simpleCb) {
      simpleCb.checked = isSimpleMode();
      simpleCb.addEventListener("change", (e) => setSimpleMode(e.target.checked));
    }
  }

  global.DogeGoPrefs = {
    init: () => {
      initSimpleDefaults();
      applyNavVisibility();
      buildNavToggles(document.getElementById("st-nav-toggles"));
      bindHelp();
      const helpCb = document.getElementById("st-help-always");
      if (helpCb) helpCb.checked = localStorage.getItem(LS_HELP) === "1";
      document.querySelectorAll(".nav-item, .bottom-nav-item").forEach((el) => {
        if (!el.getAttribute("data-nav-id") && el.getAttribute("data-tab")) {
          el.setAttribute("data-nav-id", el.getAttribute("data-tab"));
        }
      });
      applyNavVisibility();
    },
    applyNavVisibility,
    applySimpleMode,
    setSimpleMode,
    isSimpleMode,
    resetNav: () => {
      saveHidden([]);
      localStorage.setItem(LS_SIMPLE, "0");
      applySimpleMode();
      applyNavVisibility();
      buildNavToggles(document.getElementById("st-nav-toggles"));
      const simpleCb = document.getElementById("st-simple-mode");
      if (simpleCb) simpleCb.checked = false;
    },
  };
})(window);
