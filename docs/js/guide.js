(function () {
  "use strict";

  var BASE = new URL(".", window.location.href);
  var MD_ROOT = new URL("../md/", BASE);
  var manifestCache = null;
  var historyStack = [];
  var currentPath = "";
  var fileBundleReady = null;

  function isFileProtocol() {
    return window.location.protocol === "file:";
  }

  function $(id) {
    return document.getElementById(id);
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function loadScript(src) {
    return new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = src;
      s.async = true;
      s.onload = function () { resolve(); };
      s.onerror = function () { reject(new Error("Failed to load " + src)); };
      document.head.appendChild(s);
    });
  }

  function ensureFileBundles() {
    if (!isFileProtocol()) return Promise.resolve();
    if (window.DogeGoGuideManifest && window.DogeGoGuideMarkdown) return Promise.resolve();
    if (fileBundleReady) return fileBundleReady;
    fileBundleReady = Promise.resolve()
      .then(function () {
        if (!window.DogeGoGuideManifest) return loadScript(new URL("manifest.js", BASE).href);
      })
      .then(function () {
        if (!window.DogeGoGuideMarkdown) return loadScript(new URL("content-bundle.js", BASE).href);
      });
    return fileBundleReady;
  }

  function normalizePath(p) {
    p = String(p || "").trim().replace(/\\/g, "/").replace(/^\/+/, "");
    while (p.indexOf("./") === 0) p = p.slice(2);
    var parts = [];
    p.split("/").forEach(function (seg) {
      if (!seg || seg === ".") return;
      if (seg === "..") {
        if (parts.length) parts.pop();
        return;
      }
      parts.push(seg);
    });
    return parts.join("/");
  }

  function resolveHref(basePath, href) {
    href = String(href || "").trim();
    if (!href) return null;
    if (/^https?:\/\//i.test(href) || /^mailto:/i.test(href)) {
      return { external: true, href: href };
    }
    if (href.charAt(0) === "#") {
      // In-doc anchors only (not our file:// path hash)
      if (href.indexOf("doc=") === 1) return null;
      return { path: normalizePath(basePath), anchor: href };
    }
    var hash = "";
    var hashIdx = href.indexOf("#");
    if (hashIdx >= 0) {
      hash = href.slice(hashIdx);
      href = href.slice(0, hashIdx);
    }
    if (!href) return { path: normalizePath(basePath), anchor: hash };
    if (!/\.md$/i.test(href)) return null;
    var baseDir = normalizePath(basePath).split("/").slice(0, -1).join("/");
    var joined = normalizePath((baseDir ? baseDir + "/" : "") + href);
    return { path: joined, anchor: hash };
  }

  function readPathFromLocation() {
    if (isFileProtocol()) {
      var h = (window.location.hash || "").replace(/^#/, "");
      if (h.indexOf("doc=") === 0) return decodeURIComponent(h.slice(4));
      if (h && /\.md$/i.test(h)) return decodeURIComponent(h);
      return "";
    }
    return new URL(window.location.href).searchParams.get("path") || "";
  }

  function setQueryPath(path, replace) {
    if (isFileProtocol()) {
      var next = path ? "#doc=" + encodeURIComponent(path) : "#";
      if (replace) {
        try { history.replaceState({ path: path }, "", next); } catch (_) { window.location.hash = next; }
      } else {
        try { history.pushState({ path: path }, "", next); } catch (_) { window.location.hash = next; }
      }
      return;
    }
    var url = new URL(window.location.href);
    if (path) url.searchParams.set("path", path);
    else url.searchParams.delete("path");
    if (replace) history.replaceState({ path: path }, "", url);
    else history.pushState({ path: path }, "", url);
  }

  function ensureMarked() {
    if (!window.marked) return;
    if (window.marked.__dogegoGuide) return;
    if (typeof window.marked.setOptions === "function") {
      window.marked.setOptions({ gfm: true, breaks: false });
    }
    window.marked.__dogegoGuide = true;
  }

  function renderMath(root) {
    if (!root || typeof window.renderMathInElement !== "function") return;
    try {
      window.renderMathInElement(root, {
        delimiters: [
          { left: "$$", right: "$$", display: true },
          { left: "\\[", right: "\\]", display: true },
          { left: "$", right: "$", display: false },
          { left: "\\(", right: "\\)", display: false }
        ],
        throwOnError: false,
        strict: "ignore"
      });
    } catch (_) { /* ignore */ }
  }

  // Keep $$ / $ math intact so marked does not turn _ into <em>.
  function extractMath(md) {
    var slots = [];
    var out = String(md || "").replace(/\$\$[\s\S]+?\$\$/g, function (m) {
      var i = slots.length;
      slots.push(m);
      return "@@DOGEGO_MATH_" + i + "@@";
    });
    out = out.replace(/(^|[^\\$])\$([^$\n]+?)\$/g, function (_, pre, body) {
      var i = slots.length;
      slots.push("$" + body + "$");
      return pre + "@@DOGEGO_MATH_" + i + "@@";
    });
    return { md: out, slots: slots };
  }

  function restoreMath(html, slots) {
    return String(html || "").replace(/@@DOGEGO_MATH_(\d+)@@/g, function (_, i) {
      return slots[+i] || "";
    });
  }

  function wrapGuideTables(root) {
    if (!root) return;
    root.querySelectorAll("table").forEach(function (table) {
      if (table.parentElement && table.parentElement.classList.contains("guide-table-scroll")) return;
      var wrap = document.createElement("div");
      wrap.className = "guide-table-scroll";
      wrap.setAttribute("role", "region");
      wrap.setAttribute("aria-label", "Table");
      table.parentNode.insertBefore(wrap, table);
      wrap.appendChild(table);
    });
  }

  function parseMarkdown(md) {
    ensureMarked();
    var extracted = extractMath(md);
    var html;
    if (window.marked && typeof window.marked.parse === "function") {
      html = window.marked.parse(extracted.md);
    } else {
      html = "<pre>" + escapeHtml(md) + "</pre>";
    }
    return restoreMath(html, extracted.slots);
  }

  function scrollToAnchor(anchor) {
    if (!anchor || anchor === "#") return;
    var id = anchor.replace(/^#/, "");
    var el = document.getElementById(id) || document.querySelector('[id="' + id.replace(/"/g, "") + '"]');
    if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  function bindDocLinks(basePath) {
    var body = $("guide-doc");
    if (!body) return;
    body.querySelectorAll("a[href]").forEach(function (a) {
      var href = (a.getAttribute("href") || "").trim();
      if (!href) return;
      if (/^https?:\/\//i.test(href) || /^mailto:/i.test(href)) {
        a.target = "_blank";
        a.rel = "noopener noreferrer";
        return;
      }
      a.addEventListener("click", function (ev) {
        ev.preventDefault();
        var resolved = resolveHref(basePath, href);
        if (!resolved) return;
        if (resolved.external) {
          window.open(resolved.href, "_blank", "noopener,noreferrer");
          return;
        }
        if (resolved.anchor && (!resolved.path || resolved.path === normalizePath(basePath))) {
          scrollToAnchor(resolved.anchor);
          return;
        }
        openDoc(resolved.path, { anchor: resolved.anchor, push: true });
      });
    });
  }

  function markActiveLink(path) {
    document.querySelectorAll(".guide-rail-links a").forEach(function (a) {
      var p = a.getAttribute("data-path") || "";
      a.classList.toggle("is-active", p === path);
    });
  }

  function filterRail(q) {
    q = String(q || "").trim().toLowerCase();
    document.querySelectorAll(".guide-rail-section").forEach(function (sec) {
      var title = ((sec.querySelector(".guide-rail-section-copy h2") || {}).textContent || "").toLowerCase();
      var body = ((sec.querySelector(".guide-rail-section-copy p") || {}).textContent || "").toLowerCase();
      var links = sec.querySelectorAll(".guide-rail-links li");
      var any = false;
      links.forEach(function (li) {
        var text = (li.textContent || "").toLowerCase();
        var show = !q || text.indexOf(q) >= 0 || title.indexOf(q) >= 0 || body.indexOf(q) >= 0;
        li.hidden = !show;
        if (show) any = true;
      });
      sec.hidden = !!(q && !any);
    });
  }

  function sectionIcon(sec) {
    return escapeHtml(sec.icon || "folder");
  }

  function renderRail(manifest) {
    var root = $("guide-rail-sections");
    if (!root) return;
    root.innerHTML = "";
    (manifest.sections || []).forEach(function (sec) {
      var wrap = document.createElement("section");
      wrap.className = "guide-rail-section";
      wrap.innerHTML =
        '<div class="guide-rail-section-head">' +
        '<span class="guide-rail-section-icon" aria-hidden="true"><span class="material-icons-round">' +
        sectionIcon(sec) +
        "</span></span>" +
        '<div class="guide-rail-section-copy">' +
        "<h2>" +
        escapeHtml(sec.title || "") +
        "</h2>" +
        (sec.body ? "<p>" + escapeHtml(sec.body) + "</p>" : "") +
        "</div></div>" +
        '<ul class="guide-rail-links"></ul>';
      var ul = wrap.querySelector("ul");
      (sec.links || []).forEach(function (link) {
        var li = document.createElement("li");
        var a = document.createElement("a");
        a.href = isFileProtocol()
          ? "#doc=" + encodeURIComponent(link.path || "")
          : "?path=" + encodeURIComponent(link.path || "");
        a.setAttribute("data-path", link.path || "");
        a.innerHTML =
          '<span class="material-icons-round guide-link-icon" aria-hidden="true">description</span>' +
          '<span class="guide-link-label">' +
          escapeHtml(link.label || link.path || "") +
          "</span>" +
          '<span class="material-icons-round guide-link-chevron" aria-hidden="true">chevron_right</span>';
        a.addEventListener("click", function (ev) {
          ev.preventDefault();
          openDoc(link.path || "", { push: true });
          setTocOpen(false);
        });
        li.appendChild(a);
        ul.appendChild(li);
      });
      root.appendChild(wrap);
    });
  }

  function setTocOpen(open) {
    var layout = $("guide-main");
    var toggle = $("guide-toc-toggle");
    var backdrop = $("guide-toc-backdrop");
    var closeBtn = $("guide-toc-close");
    if (!layout) return;
    layout.classList.toggle("guide-toc-open", !!open);
    document.body.classList.toggle("guide-toc-lock", !!open);
    if (toggle) {
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    }
    if (backdrop) {
      backdrop.hidden = !open;
      backdrop.setAttribute("aria-hidden", open ? "false" : "true");
    }
    if (closeBtn) closeBtn.tabIndex = open ? 0 : -1;
  }

  function bindTocDrawer() {
    var toggle = $("guide-toc-toggle");
    var backdrop = $("guide-toc-backdrop");
    var closeBtn = $("guide-toc-close");
    if (toggle) {
      toggle.addEventListener("click", function () {
        var layout = $("guide-main");
        setTocOpen(!(layout && layout.classList.contains("guide-toc-open")));
      });
    }
    if (backdrop) {
      backdrop.addEventListener("click", function () {
        setTocOpen(false);
      });
    }
    if (closeBtn) {
      closeBtn.addEventListener("click", function () {
        setTocOpen(false);
      });
    }
    window.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") setTocOpen(false);
    });
    window.addEventListener("resize", function () {
      if (window.matchMedia("(min-width: 961px)").matches) setTocOpen(false);
    });
  }

  async function loadManifest() {
    if (manifestCache) return manifestCache;
    await ensureFileBundles();
    if (window.DogeGoGuideManifest) {
      manifestCache = window.DogeGoGuideManifest;
      return manifestCache;
    }
    var r = await fetch(new URL("manifest.json", BASE).href + "?ts=" + Date.now(), { cache: "no-store" });
    if (!r.ok) throw new Error("Could not load docs index (HTTP " + r.status + ")");
    manifestCache = await r.json();
    return manifestCache;
  }

  async function loadMarkdown(path) {
    await ensureFileBundles();
    if (window.DogeGoGuideMarkdown && Object.prototype.hasOwnProperty.call(window.DogeGoGuideMarkdown, path)) {
      return window.DogeGoGuideMarkdown[path];
    }
    if (isFileProtocol()) {
      throw new Error("Document not in local bundle: " + path + ". Run: node docs/scripts/sync-guide-md.js");
    }
    var url = new URL(path, MD_ROOT).href;
    var r = await fetch(url + (url.indexOf("?") >= 0 ? "&" : "?") + "ts=" + Date.now(), { cache: "no-store" });
    if (!r.ok) throw new Error("Document not found: " + path);
    return r.text();
  }

  async function openDoc(rel, opts) {
    opts = opts || {};
    var path = normalizePath(rel);
    var body = $("guide-doc");
    var crumb = $("guide-crumb");
    var title = $("guide-doc-title");
    if (!body) return;
    if (!path) {
      body.className = "guide-empty";
      body.textContent = "Pick a document from the list.";
      return;
    }
    body.className = "guide-doc markdown-body";
    body.innerHTML = "<p class=\"label\">Loading…</p>";
    if (crumb) crumb.textContent = path;
    markActiveLink(path);

    try {
      var md = await loadMarkdown(path);
      if (window.marked && typeof window.marked.parse === "function") {
        body.innerHTML = parseMarkdown(md);
      } else {
        body.innerHTML = "<pre>" + escapeHtml(md) + "</pre>";
      }
      renderMath(body);
      wrapGuideTables(body);
      if (title) {
        var h1 = body.querySelector("h1");
        title.textContent = h1 ? h1.textContent : path;
      }
      document.title = (title && title.textContent ? title.textContent + " · " : "") + "DogeGo Docs";
      bindDocLinks(path);
      if (opts.push) {
        if (currentPath && historyStack[historyStack.length - 1] !== currentPath) {
          historyStack.push(currentPath);
        }
        setQueryPath(path, false);
      } else if (opts.replace) {
        setQueryPath(path, true);
      }
      currentPath = path;
      if (opts.anchor) scrollToAnchor(opts.anchor);
      else window.scrollTo({ top: 0, behavior: "smooth" });
    } catch (e) {
      body.className = "guide-error";
      var msg = e.message || String(e);
      if (isFileProtocol()) {
        msg += " Prefer HTTP preview: from repo root run npx serve docs then open http://localhost:3000/guide/";
      }
      body.textContent = msg;
    }
  }

  function goBack() {
    if (historyStack.length) {
      var prev = historyStack.pop();
      openDoc(prev, { replace: true });
      return;
    }
    window.location.href = "../";
  }

  async function boot() {
    bindTocDrawer();
    var search = $("guide-search");
    if (search) {
      search.addEventListener("input", function () {
        filterRail(search.value);
      });
    }
    var back = $("guide-back");
    if (back) back.addEventListener("click", goBack);
    var home = $("guide-home-doc");
    if (home) {
      home.addEventListener("click", function () {
        openDoc((manifestCache && manifestCache.defaultPath) || "docs/DOCUMENTATION.md", { push: true });
      });
    }

    window.addEventListener("popstate", function (ev) {
      var p = (ev.state && ev.state.path) || readPathFromLocation();
      if (p) openDoc(p, { replace: true });
    });
    window.addEventListener("hashchange", function () {
      if (!isFileProtocol()) return;
      var p = readPathFromLocation();
      if (p && p !== currentPath) openDoc(p, { replace: true });
    });

    try {
      var manifest = await loadManifest();
      var heroTitle = $("guide-hero-title");
      var heroLead = $("guide-hero-lead");
      if (heroTitle && manifest.title) heroTitle.textContent = manifest.title;
      if (heroLead && manifest.subtitle) heroLead.textContent = manifest.subtitle;
      renderRail(manifest);
      var initial =
        readPathFromLocation() ||
        manifest.defaultPath ||
        "docs/DOCUMENTATION.md";
      await openDoc(initial, { replace: true });
    } catch (e) {
      var body = $("guide-doc");
      if (body) {
        body.className = "guide-error";
        var msg = e.message || String(e);
        if (isFileProtocol()) {
          msg += " Prefer HTTP preview: from repo root run npx serve docs then open http://localhost:3000/guide/";
        }
        body.textContent = msg;
      }
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
