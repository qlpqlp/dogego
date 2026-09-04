(function () {
  "use strict";

  var shell = document.getElementById("site-shell");
  var sidebar = document.getElementById("sidebar-nav");
  var toggle = document.getElementById("nav-toggle");
  var backdrop = document.getElementById("nav-backdrop");
  var headerOffset = 52;
  var LS_SIDEBAR_COLLAPSED = "dogego_site_sidebar_collapsed";
  var mobileMq = window.matchMedia("(max-width: 900px)");

  function isMobile() {
    return mobileMq.matches;
  }

  function navMenuLabel(open, collapsed) {
    if (window.DogeGoSiteI18n) {
      if (isMobile()) {
        var key = open ? "nav.closeMenu" : "nav.openMenu";
        var label = DogeGoSiteI18n.t(key);
        if (label !== key) return label;
        return open ? "Close menu" : "Open menu";
      }
      if (collapsed) {
        var pin = DogeGoSiteI18n.t("nav.pinSidebar");
        if (pin !== "nav.pinSidebar") return pin;
        return "Pin sidebar open";
      }
      var collapse = DogeGoSiteI18n.t("nav.collapseSidebar");
      if (collapse !== "nav.collapseSidebar") return collapse;
      return "Collapse to icons";
    }
    if (isMobile()) return open ? "Close menu" : "Open menu";
    return collapsed ? "Pin sidebar open" : "Collapse to icons";
  }

  function syncToggleLabel() {
    if (!toggle || !shell) return;
    var collapsed = shell.classList.contains("sidebar-collapsed");
    var open = shell.classList.contains("nav-open");
    toggle.setAttribute("aria-label", navMenuLabel(open, collapsed));
    toggle.title = toggle.getAttribute("aria-label") || "";
  }

  function setNavOpen(open) {
    if (!shell) return;
    shell.classList.toggle("nav-open", open);
    if (backdrop) {
      backdrop.classList.toggle("show", open);
      backdrop.hidden = !open;
      backdrop.setAttribute("aria-hidden", open ? "false" : "true");
    }
    if (toggle) toggle.setAttribute("aria-expanded", open ? "true" : "false");
    document.body.classList.toggle("site-nav-open", open && isMobile());
    syncToggleLabel();
  }

  function setDesktopCollapsed(collapsed) {
    if (!shell) return;
    shell.classList.toggle("sidebar-collapsed", collapsed);
    document.body.classList.toggle("sidebar-collapsed", collapsed);
    try {
      localStorage.setItem(LS_SIDEBAR_COLLAPSED, collapsed ? "1" : "0");
    } catch (_) {}
    syncToggleLabel();
  }

  function closeMobileNav() {
    setNavOpen(false);
  }

  function placeTopbarChrome() {
    var topSlot = document.getElementById("topbar-chrome");
    var sideSlot = document.getElementById("sidebar-chrome");
    var lang = document.getElementById("site-lang-picker");
    var foundation = document.getElementById("site-foundation");
    if (!topSlot || !sideSlot || !lang || !foundation) return;

    if (isMobile()) {
      if (lang.parentElement !== sideSlot) sideSlot.appendChild(lang);
      if (foundation.parentElement !== sideSlot) sideSlot.appendChild(foundation);
      sideSlot.hidden = false;
      topSlot.hidden = true;
    } else {
      if (lang.parentElement !== topSlot) topSlot.appendChild(lang);
      if (foundation.parentElement !== topSlot) topSlot.appendChild(foundation);
      sideSlot.hidden = true;
      topSlot.hidden = false;
    }
  }

  function toggleSidebar() {
    if (!shell) return;
    if (isMobile()) {
      setNavOpen(!shell.classList.contains("nav-open"));
      return;
    }
    setDesktopCollapsed(!shell.classList.contains("sidebar-collapsed"));
  }

  function applyDesktopPref() {
    if (!shell) return;
    if (isMobile()) {
      shell.classList.remove("sidebar-collapsed");
      document.body.classList.remove("sidebar-collapsed");
      return;
    }
    var collapsed = true;
    try {
      if (localStorage.getItem(LS_SIDEBAR_COLLAPSED) === "0") collapsed = false;
    } catch (_) {}
    setDesktopCollapsed(collapsed);
  }

  if (toggle) {
    toggle.addEventListener("click", toggleSidebar);
  }

  if (backdrop) {
    backdrop.addEventListener("click", closeMobileNav);
  }

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && shell && shell.classList.contains("nav-open")) {
      closeMobileNav();
    }
  });

  mobileMq.addEventListener("change", function () {
    closeMobileNav();
    applyDesktopPref();
    placeTopbarChrome();
  });

  applyDesktopPref();
  placeTopbarChrome();

  if (window.DogeGoSiteI18n) {
    DogeGoSiteI18n.ready().then(syncToggleLabel);
    document.addEventListener("dogego:locale", syncToggleLabel);
  }

  document.querySelectorAll('a[href^="#"]').forEach(function (link) {
    link.addEventListener("click", function (e) {
      var hash = link.getAttribute("href");
      if (!hash || hash === "#") return;

      if (hash === "#top") {
        e.preventDefault();
        window.scrollTo({ top: 0, behavior: "smooth" });
        closeMobileNav();
        return;
      }

      var target = document.querySelector(hash);
      if (!target) return;

      e.preventDefault();
      var top = target.getBoundingClientRect().top + window.pageYOffset - headerOffset;
      window.scrollTo({ top: Math.max(0, top), behavior: "smooth" });
      closeMobileNav();

      if (history.pushState && (location.protocol === "http:" || location.protocol === "https:")) {
        try {
          history.pushState(null, "", hash);
        } catch (_) {}
      }
    });
  });

  var sections = document.querySelectorAll("section[id]");
  var navLinks = document.querySelectorAll('#sidebar-nav a[href^="#"], .footer-links a[href^="#"]');

  if (sections.length && navLinks.length && "IntersectionObserver" in window) {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (!entry.isIntersecting) return;
          var id = entry.target.getAttribute("id");
          navLinks.forEach(function (link) {
            var active = link.getAttribute("href") === "#" + id;
            link.classList.toggle("is-active", active);
            link.classList.toggle("active", active);
          });
        });
      },
      { rootMargin: "-40% 0px -50% 0px", threshold: 0 }
    );
    sections.forEach(function (section) { observer.observe(section); });
  }

  function syncBuildLineNumbers() {
    var code = document.getElementById("build-source-code");
    var lines = document.getElementById("build-source-lines");
    if (!code || !lines) return;
    var text = code.textContent || "";
    var n = text.replace(/\n$/, "").split("\n").length;
    if (n < 1) n = 1;
    var out = [];
    for (var i = 1; i <= n; i++) out.push(String(i));
    lines.textContent = out.join("\n");
  }

  function bindCodeCopyButtons() {
    document.querySelectorAll(".code-copy-btn[data-copy-target]").forEach(function (btn) {
      if (btn.getAttribute("data-copy-bound") === "1") return;
      btn.setAttribute("data-copy-bound", "1");
      btn.addEventListener("click", function () {
        var id = btn.getAttribute("data-copy-target");
        var el = id ? document.getElementById(id) : null;
        if (!el) return;
        var text = el.textContent || "";
        var label = btn.querySelector(".code-copy-label");
        var icon = btn.querySelector(".material-icons-round");
        var copiedLabel = window.DogeGoSiteI18n ? DogeGoSiteI18n.t("download.copiedCode") : "Copied";
        var copyLabel = window.DogeGoSiteI18n ? DogeGoSiteI18n.t("download.copyCode") : "Copy";
        if (copiedLabel === "download.copiedCode") copiedLabel = "Copied";
        if (copyLabel === "download.copyCode") copyLabel = "Copy";

        function markCopied() {
          btn.classList.add("is-copied");
          if (label) label.textContent = copiedLabel;
          if (icon) icon.textContent = "check";
          window.setTimeout(function () {
            btn.classList.remove("is-copied");
            if (label) label.textContent = copyLabel;
            if (icon) icon.textContent = "content_copy";
          }, 1800);
        }

        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(markCopied).catch(function () {
            fallbackCopy(text, markCopied);
          });
        } else {
          fallbackCopy(text, markCopied);
        }
      });
    });
  }

  function fallbackCopy(text, onDone) {
    try {
      var ta = document.createElement("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.style.position = "fixed";
      ta.style.left = "-9999px";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
      if (onDone) onDone();
    } catch (_) {}
  }

  syncBuildLineNumbers();
  bindCodeCopyButtons();
  document.addEventListener("dogego:locale", function () {
    window.setTimeout(function () {
      syncBuildLineNumbers();
      bindCodeCopyButtons();
    }, 0);
  });

  (function initDocCards() {
    function go(el) {
      var href = el.getAttribute("data-doc");
      if (href) window.location.href = href;
    }
    document.querySelectorAll("[data-doc]").forEach(function (el) {
      if (!el.hasAttribute("tabindex")) el.setAttribute("tabindex", "0");
      if (!el.hasAttribute("role")) el.setAttribute("role", "link");
      el.addEventListener("click", function (e) {
        if (e.target.closest("a[href]")) return;
        go(el);
      });
      el.addEventListener("keydown", function (e) {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          go(el);
        }
      });
    });
  })();

  (function initCodeReviewBanner() {
    var LS_KEY = "dogego_code_review_banner_v1";
    var banner = document.getElementById("code-review-banner");
    var dismissBtn = document.getElementById("code-review-banner-dismiss");
    if (!banner) return;
    function show() {
      banner.hidden = false;
      document.body.classList.add("has-code-review-banner");
    }
    function hide() {
      banner.hidden = true;
      document.body.classList.remove("has-code-review-banner");
    }
    try {
      if (localStorage.getItem(LS_KEY) === "1") {
        hide();
        return;
      }
    } catch (_) { /* */ }
    show();
    if (!dismissBtn) return;
    dismissBtn.addEventListener("click", function () {
      try {
        localStorage.setItem(LS_KEY, "1");
      } catch (_) { /* */ }
      hide();
    });
  })();
})();
