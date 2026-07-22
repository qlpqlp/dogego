(function () {
  "use strict";

  var reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  var tabs = ["overview", "blockstep", "mempool", "features"];
  var WIZARD_STEP_KEYS = ["profile", "data", "network", "sync", "finish"];
  var EXPLAINER_ICONS = {
    overview: "insights",
    blockstep: "pets",
    mempool: "pending_actions",
    features: "verified"
  };
  var CONSOLE_TYPES = ["info", "dim", "ok", "ok", "info", "ok", "info", "ok", "warn", "info", "dim"];
  var DEFAULT_MONTHS = ["Jan", "Mar", "Jun", "Sep", "Dec"];

  var navItems = document.querySelectorAll(".demo-nav-item[data-demo-tab]");
  var panels = document.querySelectorAll(".demo-panel");
  var syncFill = document.getElementById("demo-sync-fill");
  var syncPct = document.getElementById("demo-sync-pct");
  var tipEl = document.getElementById("demo-tip");
  var mempoolEl = document.getElementById("demo-mempool");
  var peersOutKpi = document.getElementById("demo-peers-out-kpi");
  var peersInKpi = document.getElementById("demo-peers-in-kpi");
  var peersShort = document.getElementById("demo-peers-short");
  var txProcessedEl = document.getElementById("demo-tx-processed");
  var consoleEl = document.getElementById("demo-console-log");
  var bsThumb = document.getElementById("demo-bs-thumb");
  var bsSynced = document.getElementById("demo-bs-synced");
  var bsTime = document.getElementById("demo-bs-time");
  var bsHeight = document.getElementById("demo-bs-height");
  var explainerEl = document.getElementById("demo-explainer");
  var wizardSteps = document.querySelectorAll(".wiz-setup-steps li");
  var wizardPanels = document.querySelectorAll(".wiz-demo-panel");
  var wizKicker = document.getElementById("wiz-demo-kicker");

  var syncVal = 68.4;
  var tipVal = 5412884;
  var connectedVal = 3701204;
  var storedVal = 3842880;
  var mempoolVal = 124;
  var peerOut = 8;
  var peerIn = 2;
  var bsPos = 68;
  var bsBlock = 3842016;
  var downloadRate = 1840;

  var tabIndex = 0;
  var explainerIndex = 0;
  var wizIndex = 0;
  var consoleIdx = 0;
  var charIdx = 0;
  var currentLine = null;
  var explainers = {};
  var consoleLines = [];
  var timers = [];

  function t(key, params) {
    if (window.DogeGoSiteI18n) return DogeGoSiteI18n.t(key, params);
    return key;
  }

  function localeTag() {
    if (!window.DogeGoSiteI18n) return "en-US";
    var loc = DogeGoSiteI18n.locale();
    if (loc === "pt-PT") return "pt-PT";
    if (loc === "zh") return "zh-CN";
    if (loc === "ja") return "ja-JP";
    if (loc === "de") return "de-DE";
    if (loc === "fr") return "fr-FR";
    return "en-US";
  }

  function fmtNum(n) {
    return n.toLocaleString(localeTag());
  }

  function loadExplainers() {
    var result = {};
    tabs.forEach(function (tab) {
      var base = window.DogeGoSiteI18n ? DogeGoSiteI18n.get("demo.explainer." + tab) : null;
      if (!base || typeof base !== "object") return;
      result[tab] = {
        icon: EXPLAINER_ICONS[tab],
        title: base.title || "",
        body: base.body || "",
        tags: Array.isArray(base.tags) ? base.tags.slice() : []
      };
    });
    return result;
  }

  function loadConsoleLines() {
    var lines = window.DogeGoSiteI18n ? DogeGoSiteI18n.get("console.lines") : null;
    if (!Array.isArray(lines)) return [];
    return lines.map(function (m, i) {
      return { t: CONSOLE_TYPES[i] || "info", m: String(m) };
    });
  }

  function blockstepMonths() {
    var months = window.DogeGoSiteI18n ? DogeGoSiteI18n.get("demo.blockstepMonths") : null;
    return Array.isArray(months) && months.length ? months : DEFAULT_MONTHS;
  }

  function updateSyncDock(pctStr) {
    var main = document.getElementById("demo-sync-dock-main");
    if (main) main.textContent = t("demo.syncDockMain", { pct: pctStr });
    var meta = document.getElementById("demo-sync-dock-meta");
    if (meta) {
      var behind = Math.max(0, tipVal - connectedVal);
      meta.innerHTML = t("demo.syncDockMeta", {
        connected: fmtNum(connectedVal),
        stored: fmtNum(storedVal),
        behind: fmtNum(behind),
        rate: fmtNum(Math.round(downloadRate))
      });
    }
  }

  function updateWizardKicker() {
    if (!wizKicker || !wizardSteps.length) return;
    var shown = (wizIndex + wizardSteps.length - 1) % wizardSteps.length;
    wizKicker.textContent = t("wizard.stepKicker", {
      n: shown + 1,
      label: t("wizard.steps." + WIZARD_STEP_KEYS[shown])
    });
  }

  function showExplainer(data) {
    if (!explainerEl || !data) return;
    explainerEl.innerHTML =
      '<div class="explainer-icon"><span class="material-icons-round">' + data.icon + "</span></div>" +
      '<div class="explainer-body">' +
      "<h3>" + data.title + "</h3>" +
      "<p>" + data.body + "</p>" +
      '<div class="explainer-tags">' +
      data.tags.map(function (tag) { return "<span>" + tag + "</span>"; }).join("") +
      "</div></div>";
    explainerEl.classList.remove("explainer-fade");
    void explainerEl.offsetWidth;
    explainerEl.classList.add("explainer-fade");
  }

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
      showExplainer(explainers[tab] || explainers.overview);
    }
  }

  function tickMetrics() {
    if (syncVal < 99.2) {
      syncVal += 0.08 + Math.random() * 0.22;
      if (syncVal > 99.2) syncVal = 99.2;
    }
    tipVal += Math.floor(Math.random() * 4);
    connectedVal += Math.floor(Math.random() * 120);
    if (connectedVal > storedVal - 50000) storedVal += Math.floor(Math.random() * 200);
    if (Math.random() > 0.55) mempoolVal += Math.floor(Math.random() * 6) - 2;
    if (mempoolVal < 60) mempoolVal = 60;
    if (mempoolVal > 280) mempoolVal = 280;
    if (peerOut < 12 && Math.random() > 0.82) peerOut += 1;
    if (peerIn < 4 && syncVal > 55 && Math.random() > 0.88) peerIn += 1;
    downloadRate += (Math.random() - 0.45) * 80;
    if (downloadRate < 900) downloadRate = 900;
    if (downloadRate > 2400) downloadRate = 2400;

    var pctStr = syncVal.toFixed(1);
    if (syncFill) syncFill.style.width = pctStr + "%";
    if (syncPct) syncPct.textContent = pctStr;
    if (tipEl) tipEl.textContent = fmtNum(tipVal);
    if (mempoolEl) mempoolEl.textContent = t("demo.mempoolTx", { n: mempoolVal });
    if (peersOutKpi) peersOutKpi.textContent = String(peerOut);
    if (peersInKpi) peersInKpi.textContent = String(peerIn);
    if (peersShort) {
      peersShort.innerHTML =
        t("demo.peersOutIn", { out: peerOut, in: peerIn });
    }
    if (txProcessedEl) {
      txProcessedEl.textContent = t("demo.txProcessedM", { n: (connectedVal / 1e6).toFixed(1) });
    }
    updateSyncDock(pctStr);

    if (tabs[tabIndex] === "blockstep" && !reduced) {
      bsPos += Math.random() > 0.45 ? 1.8 : -1.8;
      if (bsPos < 52) bsPos = 52;
      if (bsPos > 92) bsPos = 92;
      bsBlock += Math.floor(Math.random() * 1200);
      if (bsThumb) bsThumb.style.left = bsPos.toFixed(1) + "%";
      if (bsSynced) bsSynced.style.width = bsPos.toFixed(1) + "%";
      if (bsHeight) bsHeight.textContent = t("demo.blockHeight", { n: fmtNum(bsBlock) });
      if (bsTime) {
        var months = blockstepMonths();
        var year = 2013 + Math.floor((bsPos / 100) * 12);
        var month = months[Math.floor((bsPos / 20) % months.length)];
        bsTime.textContent = month + " " + (4 + Math.floor(bsPos / 12)) + ", " + year;
      }
    }
  }

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

  function advanceWizard() {
    if (reduced || !wizardSteps.length) return;
    if (wizKicker) {
      wizKicker.textContent = t("wizard.stepKicker", {
        n: wizIndex + 1,
        label: t("wizard.steps." + WIZARD_STEP_KEYS[wizIndex])
      });
    }
    wizardSteps.forEach(function (s, i) {
      s.classList.toggle("active", i === wizIndex);
      s.classList.toggle("done", i < wizIndex);
    });
    wizardPanels.forEach(function (p, i) {
      p.classList.toggle("wiz-panel-active", i === wizIndex);
    });
    wizIndex = (wizIndex + 1) % wizardSteps.length;
  }

  function resetConsole() {
    if (consoleEl) consoleEl.innerHTML = "";
    consoleIdx = 0;
    charIdx = 0;
    currentLine = null;
  }

  function stopTimers() {
    timers.forEach(function (id) { window.clearInterval(id); });
    timers = [];
  }

  function startSidebarScroll() {
    var sidebar = document.querySelector(".demo-sidebar");
    if (!sidebar || reduced) return;
    var dir = 1;
    var pauseUntil = 0;
    timers.push(window.setInterval(function () {
      if (window.matchMedia("(max-width: 960px)").matches) return;
      var max = sidebar.scrollHeight - sidebar.clientHeight;
      if (max <= 4) return;
      var now = Date.now();
      if (now < pauseUntil) return;
      sidebar.scrollTop += dir * 0.55;
      if (sidebar.scrollTop >= max - 0.5) {
        dir = -1;
        pauseUntil = now + 1600;
      } else if (sidebar.scrollTop <= 0.5) {
        dir = 1;
        pauseUntil = now + 1600;
      }
    }, 40));
  }

  function startTimers() {
    stopTimers();
    if (reduced) return;
    timers.push(window.setInterval(switchTab, 5200));
    timers.push(window.setInterval(tickMetrics, 900));
    timers.push(window.setInterval(appendConsoleChar, 22));
    timers.push(window.setInterval(advanceWizard, 3400));
    startSidebarScroll();
  }

  function refreshLocaleContent() {
    explainers = loadExplainers();
    consoleLines = loadConsoleLines();
    showExplainer(explainers[tabs[explainerIndex]] || explainers.overview);
    updateWizardKicker();
    tickMetrics();
  }

  function boot() {
    explainers = loadExplainers();
    consoleLines = loadConsoleLines();
    if (explainerEl) showExplainer(explainers.overview);
    tickMetrics();
    advanceWizard();
    startTimers();
  }

  function onLocaleChange() {
    stopTimers();
    resetConsole();
    refreshLocaleContent();
    startTimers();
  }

  if (window.DogeGoSiteI18n) {
    DogeGoSiteI18n.ready().then(boot);
    document.addEventListener("dogego:locale", onLocaleChange);
  } else {
    boot();
  }
})();
