(function (global) {
  "use strict";

  /** Same repos as DogeGo version/update_sources.go (website checks all three). */
  var UPDATE_SOURCES = [
    { owner: "qlpqlp", repo: "dogego" },
    { owner: "dogeorg", repo: "dogego" },
    { owner: "dogecoinfoundation", repo: "dogego" }
  ];

  var PLATFORM_ASSETS = [
    { id: "windows", os: ["win"], arch: ["x64", "amd64"], asset: "dogego-windows-amd64.exe", labelKey: "download.platforms.windows.name" },
    { id: "macos-intel", os: ["mac"], arch: ["x64", "amd64"], asset: "dogego-darwin-amd64", labelKey: "download.platforms.macos.name", sub: "Intel" },
    { id: "macos-arm", os: ["mac"], arch: ["arm64"], asset: "dogego-darwin-arm64", labelKey: "download.platforms.macos.name", sub: "Apple Silicon" },
    { id: "linux", os: ["linux"], arch: ["x64", "amd64", "arm64", "arm", "x86"], asset: "dogego-linux-amd64", labelKey: "download.platforms.linux.name" }
  ];

  var state = {
    loading: true,
    release: null,
    sourcesChecked: [],
    error: null
  };

  function t(key, params) {
    if (global.DogeGoSiteI18n && typeof global.DogeGoSiteI18n.t === "function") {
      return global.DogeGoSiteI18n.t(key, params);
    }
    return key;
  }

  function normalizeSemver(v) {
    v = String(v || "").trim().toLowerCase().replace(/^v/, "");
    var dash = v.indexOf("-");
    if (dash >= 0) v = v.slice(0, dash);
    var plus = v.indexOf("+");
    if (plus >= 0) v = v.slice(0, plus);
    return v;
  }

  function parseSemverParts(v) {
    var parts = normalizeSemver(v).split(".");
    return [0, 1, 2].map(function (i) {
      var n = parseInt(parts[i], 10);
      return isNaN(n) ? 0 : n;
    });
  }

  function semverCompare(a, b) {
    var av = parseSemverParts(a);
    var bv = parseSemverParts(b);
    for (var i = 0; i < 3; i++) {
      if (av[i] < bv[i]) return -1;
      if (av[i] > bv[i]) return 1;
    }
    return 0;
  }

  function detectPlatform() {
    var ua = global.navigator.userAgent || "";
    var platform = (global.navigator.userAgentData && global.navigator.userAgentData.platform) ||
      global.navigator.platform || "";
    var p = platform.toLowerCase();
    var isWin = p.indexOf("win") >= 0 || /Windows/i.test(ua);
    var isMac = p.indexOf("mac") >= 0 || /Mac OS X/i.test(ua);
    var isLinux = !isWin && !isMac && (p.indexOf("linux") >= 0 || /Linux/i.test(ua));
    var arch = "x64";
    if (global.navigator.userAgentData && global.navigator.userAgentData.architecture) {
      arch = global.navigator.userAgentData.architecture.toLowerCase();
    } else if (/aarch64|arm64/i.test(ua)) {
      arch = "arm64";
    } else if (/i686|i386/i.test(ua)) {
      arch = "x86";
    }
    if (isWin) return { os: "win", arch: arch };
    if (isMac) return { os: "mac", arch: arch };
    if (isLinux) return { os: "linux", arch: arch };
    return { os: "other", arch: arch };
  }

  function pickPlatformSpec(detected) {
    for (var i = 0; i < PLATFORM_ASSETS.length; i++) {
      var spec = PLATFORM_ASSETS[i];
      if (spec.os.indexOf(detected.os) < 0) continue;
      if (spec.arch.indexOf(detected.arch) >= 0) return spec;
    }
    if (detected.os === "mac") {
      return detected.arch === "arm64"
        ? PLATFORM_ASSETS.find(function (s) { return s.id === "macos-arm"; })
        : PLATFORM_ASSETS.find(function (s) { return s.id === "macos-intel"; });
    }
    if (detected.os === "win") return PLATFORM_ASSETS.find(function (s) { return s.id === "windows"; });
    if (detected.os === "linux") return PLATFORM_ASSETS.find(function (s) { return s.id === "linux"; });
    return null;
  }

  function findAsset(assets, name) {
    if (!assets || !name) return null;
    var want = name.toLowerCase();
    for (var i = 0; i < assets.length; i++) {
      var a = assets[i];
      if (a.name && a.name.toLowerCase() === want && a.browser_download_url) return a;
    }
    return null;
  }

  function findChecksumUrl(assets, assetName) {
    if (!assetName) return "";
    var want = (assetName + ".sha256").toLowerCase();
    for (var i = 0; i < (assets || []).length; i++) {
      var a = assets[i];
      if (a.name && a.name.toLowerCase() === want && a.browser_download_url) return a.browser_download_url;
    }
    return "";
  }

  function fetchLatestRelease(owner, repo) {
    var url = "https://api.github.com/repos/" + encodeURIComponent(owner) + "/" + encodeURIComponent(repo) + "/releases/latest";
    return fetch(url, {
      headers: {
        Accept: "application/vnd.github+json",
        "User-Agent": "DogeGo-Website-Release-Check"
      }
    }).then(function (resp) {
      if (resp.status === 404) return null;
      if (!resp.ok) throw new Error(owner + "/" + repo + ": HTTP " + resp.status);
      return resp.json();
    }).then(function (rel) {
      if (!rel || !rel.tag_name) return null;
      return {
        tag: rel.tag_name,
        version: normalizeSemver(rel.tag_name),
        name: rel.name || rel.tag_name,
        body: typeof rel.body === "string" ? rel.body : "",
        htmlUrl: rel.html_url,
        source: "https://github.com/" + owner + "/" + repo,
        sourceLabel: owner + "/" + repo,
        assets: rel.assets || [],
        publishedAt: rel.published_at || ""
      };
    });
  }

  function fetchBestRelease() {
    var sourcesChecked = [];
    return Promise.all(UPDATE_SOURCES.map(function (src) {
      sourcesChecked.push(src.owner + "/" + src.repo);
      return fetchLatestRelease(src.owner, src.repo).catch(function (err) {
        return { error: err };
      });
    })).then(function (results) {
      var best = null;
      results.forEach(function (r) {
        if (!r || r.error || !r.tag) return;
        if (!best || semverCompare(r.version, best.version) > 0) best = r;
      });
      return { release: best, sourcesChecked: sourcesChecked };
    });
  }

  function assetRows(release) {
    var rows = [];
    PLATFORM_ASSETS.forEach(function (spec) {
      var asset = findAsset(release.assets, spec.asset);
      if (!asset) return;
      var checksum = findChecksumUrl(release.assets, spec.asset);
      rows.push({
        id: spec.id,
        label: t(spec.labelKey) + (spec.sub ? " (" + spec.sub + ")" : ""),
        fileName: asset.name,
        url: asset.browser_download_url,
        checksumUrl: checksum
      });
    });
    return rows;
  }

  /** Matches DogeGo/version/version.go defaults when no GitHub release exists. */
  var SITE_CLIENT_VERSION = "0.1.0";
  var SITE_CORE_BASE = "1.14.9";
  var SITE_CHANNEL = "beta";

  function formatDisplayVersion(tag) {
    if (tag) {
      var v = String(tag).trim().replace(/^v/i, "");
      return v + " (" + SITE_CORE_BASE + ")";
    }
    return SITE_CLIENT_VERSION + "-" + SITE_CHANNEL + " (" + SITE_CORE_BASE + ")";
  }

  function updateVersionDisplay(rel) {
    var display = formatDisplayVersion(rel && rel.tag ? rel.tag : null);
    var topbar = document.getElementById("topbar-version");
    if (topbar) {
      topbar.textContent = display;
      topbar.title = rel && rel.tag ? "Latest release on GitHub" : "DogeGo version";
    }
    var demoVer = document.querySelector(".demo-topbar-ver");
    if (demoVer) demoVer.textContent = display;
  }

  function setHeroReleaseNote(text) {
    var el = document.getElementById("hero-release-note");
    if (!el) return;
    var textEl = el.querySelector(".hero-status-text");
    if (textEl) textEl.textContent = text;
    else setText(el, text);
  }

  function setText(el, text) {
    if (el) el.textContent = text;
  }

  function setHtml(el, html) {
    if (el) el.innerHTML = html;
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /** Small safe subset of GitHub release markdown for the download banner. */
  function renderReleaseMarkdown(md) {
    var lines = String(md || "").replace(/\r\n/g, "\n").split("\n");
    var html = [];
    var inList = false;

    function closeList() {
      if (inList) {
        html.push("</ul>");
        inList = false;
      }
    }

    function inlineFormat(s) {
      s = escapeHtml(s);
      s = s.replace(/`([^`]+)`/g, "<code>$1</code>");
      s = s.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
      s = s.replace(
        /\[([^\]]+)\]\((https?:[^)\s]+)\)/g,
        '<a href="$2" rel="noopener noreferrer" target="_blank">$1</a>'
      );
      return s;
    }

    lines.forEach(function (line) {
      if (/^###\s+/.test(line)) {
        closeList();
        html.push("<h4>" + inlineFormat(line.replace(/^###\s+/, "")) + "</h4>");
        return;
      }
      if (/^##\s+/.test(line)) {
        closeList();
        html.push("<h3>" + inlineFormat(line.replace(/^##\s+/, "")) + "</h3>");
        return;
      }
      if (/^#\s+/.test(line)) {
        closeList();
        html.push("<h3>" + inlineFormat(line.replace(/^#\s+/, "")) + "</h3>");
        return;
      }
      if (/^[-*]\s+/.test(line)) {
        if (!inList) {
          html.push("<ul>");
          inList = true;
        }
        html.push("<li>" + inlineFormat(line.replace(/^[-*]\s+/, "")) + "</li>");
        return;
      }
      if (!String(line).trim()) {
        closeList();
        return;
      }
      closeList();
      html.push("<p>" + inlineFormat(line) + "</p>");
    });
    closeList();
    return html.join("");
  }

  var notesExpanded = false;

  function setNotesToggle(visible) {
    var btn = document.getElementById("release-notes-toggle");
    if (!btn) return;
    btn.hidden = !visible;
    if (!visible) {
      notesExpanded = false;
      return;
    }
    btn.textContent = notesExpanded ? t("download.showLess") : t("download.showMore");
  }

  function applyNotesCollapse(bannerBody) {
    if (!bannerBody) return;
    bannerBody.classList.remove("is-collapsed", "is-expanded");
    setNotesToggle(false);
    requestAnimationFrame(function () {
      bannerBody.classList.add("is-collapsed");
      if (notesExpanded) bannerBody.classList.add("is-expanded");
      var overflow = bannerBody.scrollHeight > bannerBody.clientHeight + 4;
      if (!overflow) {
        bannerBody.classList.remove("is-collapsed", "is-expanded");
        setNotesToggle(false);
        return;
      }
      setNotesToggle(true);
    });
  }

  function bindNotesToggle() {
    var btn = document.getElementById("release-notes-toggle");
    if (!btn || btn.getAttribute("data-bound") === "1") return;
    btn.setAttribute("data-bound", "1");
    btn.addEventListener("click", function () {
      notesExpanded = !notesExpanded;
      var body = document.getElementById("release-banner-body");
      if (!body) return;
      body.classList.toggle("is-expanded", notesExpanded);
      body.classList.add("is-collapsed");
      btn.textContent = notesExpanded ? t("download.showLess") : t("download.showMore");
    });
  }

  function clearBannerNotesChrome() {
    var meta = document.getElementById("release-banner-meta");
    var body = document.getElementById("release-banner-body");
    if (meta) {
      meta.hidden = true;
      meta.innerHTML = "";
    }
    if (body) body.classList.remove("is-collapsed", "is-expanded");
    setNotesToggle(false);
  }

  function render() {
    var banner = document.getElementById("release-banner");
    var bannerIcon = document.getElementById("release-banner-icon");
    var bannerTitle = document.getElementById("release-banner-title");
    var bannerMeta = document.getElementById("release-banner-meta");
    var bannerBody = document.getElementById("release-banner-body");
    var bannerAction = document.getElementById("release-banner-action");
    var assetsWrap = document.getElementById("release-assets");
    var assetsList = document.getElementById("release-assets-list");
    var platformGrid = document.getElementById("platform-grid");
    var heroDownload = document.getElementById("hero-download-btn");

    if (state.loading) {
      if (banner) banner.setAttribute("data-state", "loading");
      if (bannerIcon) bannerIcon.textContent = "hourglass_top";
      setText(bannerTitle, t("download.checkingTitle"));
      clearBannerNotesChrome();
      setText(bannerBody, t("download.checkingBody"));
      if (bannerAction) bannerAction.hidden = true;
      if (assetsWrap) assetsWrap.hidden = true;
      updateVersionDisplay(null);
      return;
    }

    var rel = state.release;
    if (!rel) {
      if (banner) banner.setAttribute("data-state", "soon");
      if (bannerIcon) bannerIcon.textContent = "schedule";
      setText(bannerTitle, t("download.soonTitle"));
      clearBannerNotesChrome();
      setHtml(bannerBody, t("download.soonBody"));
      if (bannerAction) {
        bannerAction.hidden = false;
        bannerAction.textContent = t("download.viewSource");
        bannerAction.href = "https://github.com/qlpqlp/dogego";
        bannerAction.removeAttribute("download");
      }
      if (assetsWrap) assetsWrap.hidden = true;
      if (heroDownload) {
        heroDownload.href = "#download";
        heroDownload.removeAttribute("target");
        heroDownload.removeAttribute("rel");
        var heroLabel = heroDownload.querySelector("[data-i18n='hero.download']");
        if (heroLabel) heroLabel.textContent = t("download.soonCta");
      }
      setHeroReleaseNote(t("download.soonHeroNote"));
      updateVersionDisplay(null);
      if (platformGrid) {
        platformGrid.querySelectorAll(".platform-card").forEach(function (card) {
          card.classList.add("platform-soon");
          var link = card.querySelector(".platform-link");
          if (link) {
            link.textContent = t("download.soonLink");
            link.href = "#download";
            link.removeAttribute("target");
            link.removeAttribute("rel");
          }
        });
      }
      return;
    }

    var rows = assetRows(rel);
    var tagDisplay = rel.tag || ("v" + rel.version);

    if (banner) banner.setAttribute("data-state", "ready");
    if (bannerIcon) bannerIcon.textContent = "rocket_launch";
    setText(bannerTitle, t("download.readyTitle", { version: tagDisplay }));
    if (bannerMeta) {
      bannerMeta.hidden = false;
      setHtml(bannerMeta, t("download.readyBody", { source: rel.sourceLabel }));
    }
    if (rel.body && String(rel.body).trim()) {
      setHtml(bannerBody, renderReleaseMarkdown(rel.body));
      applyNotesCollapse(bannerBody);
    } else {
      clearBannerNotesChrome();
      if (bannerMeta) bannerMeta.hidden = true;
      setHtml(bannerBody, t("download.readyBody", { source: rel.sourceLabel }));
    }
    if (bannerAction) {
      bannerAction.hidden = false;
      bannerAction.textContent = t("download.getReleases");
      bannerAction.href = rel.htmlUrl;
      bannerAction.target = "_blank";
      bannerAction.rel = "noopener noreferrer";
    }

    // Multi-platform releases: hero CTA scrolls to #download so users pick an OS.
    // Platform cards / asset rows still deep-link each binary.
    if (heroDownload) {
      heroDownload.href = "#download";
      heroDownload.removeAttribute("target");
      heroDownload.removeAttribute("rel");
      var hl = heroDownload.querySelector("[data-i18n='hero.download']");
      if (hl) {
        hl.textContent = rows.length
          ? t("download.heroCta", { version: tagDisplay })
          : t("hero.download");
      }
    }
    setHeroReleaseNote(t("download.readyHeroNote", { source: rel.sourceLabel }));
    updateVersionDisplay(rel);

    if (assetsWrap && assetsList) {
      assetsList.innerHTML = "";
      if (rows.length) {
        assetsWrap.hidden = false;
        rows.forEach(function (row) {
          var li = document.createElement("li");
          li.className = "release-asset-row";
          var a = document.createElement("a");
          a.className = "release-asset-link";
          a.href = row.url;
          a.target = "_blank";
          a.rel = "noopener noreferrer";
          a.textContent = row.label + " · " + row.fileName;
          li.appendChild(a);
          if (row.checksumUrl) {
            var sha = document.createElement("a");
            sha.className = "release-asset-sha";
            sha.href = row.checksumUrl;
            sha.target = "_blank";
            sha.rel = "noopener noreferrer";
            sha.textContent = ".sha256";
            li.appendChild(sha);
          }
          assetsList.appendChild(li);
        });
      } else {
        assetsWrap.hidden = true;
      }
    }

    if (platformGrid) {
      platformGrid.querySelectorAll(".platform-card").forEach(function (card) {
        card.classList.remove("platform-soon");
        var pid = card.getAttribute("data-platform");
        var link = card.querySelector(".platform-link");
        if (!link) return;
        var match = rows.filter(function (r) {
          if (pid === "windows") return r.id === "windows";
          if (pid === "macos") return r.id === "macos-intel" || r.id === "macos-arm";
          if (pid === "linux") return r.id === "linux";
          return false;
        });
        if (match.length) {
          link.textContent = t("download.downloadLink");
          link.href = match[0].url;
          link.target = "_blank";
          link.rel = "noopener noreferrer";
        } else {
          link.textContent = t("download.getReleases");
          link.href = rel.htmlUrl;
          link.target = "_blank";
          link.rel = "noopener noreferrer";
        }
      });
    }

    var schema = document.querySelector('script[type="application/ld+json"]');
    if (schema) {
      try {
        var json = JSON.parse(schema.textContent);
        json.softwareVersion = rel.version || normalizeSemver(rel.tag);
        json.downloadUrl = rel.htmlUrl;
        schema.textContent = JSON.stringify(json);
      } catch (_) {}
    }
  }

  function refresh() {
    state.loading = true;
    state.error = null;
    render();
    return fetchBestRelease().then(function (result) {
      state.loading = false;
      state.release = result.release;
      state.sourcesChecked = result.sourcesChecked;
      render();
      document.dispatchEvent(new CustomEvent("dogego:releases", { detail: state }));
    }).catch(function (err) {
      state.loading = false;
      state.release = null;
      state.error = err && err.message ? err.message : String(err);
      render();
    });
  }

  function init() {
    bindNotesToggle();
    render();
    refresh();
    document.addEventListener("dogego:locale", function () { render(); });
    if (global.DogeGoSiteI18n && global.DogeGoSiteI18n.ready) {
      global.DogeGoSiteI18n.ready().then(function () { render(); });
    }
  }

  global.DogeGoReleases = {
    refresh: refresh,
    getState: function () { return state; },
    normalizeSemver: normalizeSemver,
    semverCompare: semverCompare
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window);
