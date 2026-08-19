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
    { id: "macos-intel", os: ["mac"], arch: ["x64", "amd64"], asset: "dogego-darwin-amd64", labelKey: "download.platforms.macosIntel.name" },
    { id: "macos-arm", os: ["mac"], arch: ["arm64"], asset: "dogego-darwin-arm64", labelKey: "download.platforms.macosArm.name" },
    { id: "linux", os: ["linux"], arch: ["x64", "amd64", "arm64", "arm", "x86"], asset: "dogego-linux-amd64", labelKey: "download.platforms.linux.name" }
  ];

  var state = {
    loading: true,
    release: null,
    sourcesChecked: [],
    error: null,
    checksums: {}
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

  function parseChecksumFile(text) {
    var line = String(text || "").trim().split(/\s+/)[0] || "";
    line = line.toLowerCase();
    if (/^[a-f0-9]{64}$/.test(line)) return line;
    return "";
  }

  function fetchChecksumText(url) {
    if (!url) return Promise.resolve("");
    return fetch(url, {
      headers: { "User-Agent": "DogeGo-Website-Release-Check" }
    }).then(function (resp) {
      if (!resp.ok) return "";
      return resp.text();
    }).then(parseChecksumFile).catch(function () { return ""; });
  }

  function mapRelease(rel, owner, repo) {
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
      publishedAt: rel.published_at || "",
      prerelease: !!rel.prerelease
    };
  }

  function pickBestFromList(list, owner, repo) {
    var best = null;
    (list || []).forEach(function (rel) {
      if (!rel || rel.draft || !rel.tag_name) return;
      var mapped = mapRelease(rel, owner, repo);
      if (!mapped || !mapped.version) return;
      if (!best) {
        best = mapped;
        return;
      }
      var cmp = semverCompare(mapped.version, best.version);
      if (cmp > 0) {
        best = mapped;
        return;
      }
      if (cmp === 0) {
        if (mapped.publishedAt && best.publishedAt && mapped.publishedAt > best.publishedAt) {
          best = mapped;
        } else if (mapped.publishedAt === best.publishedAt && best.prerelease && !mapped.prerelease) {
          best = mapped;
        }
      }
    });
    return best;
  }

  function fetchLatestRelease(owner, repo) {
    var listUrl = "https://api.github.com/repos/" + encodeURIComponent(owner) + "/" + encodeURIComponent(repo) + "/releases?per_page=30";
    var latestUrl = "https://api.github.com/repos/" + encodeURIComponent(owner) + "/" + encodeURIComponent(repo) + "/releases/latest";
    var headers = {
      Accept: "application/vnd.github+json",
      "User-Agent": "DogeGo-Website-Release-Check"
    };
    return fetch(listUrl, { headers: headers }).then(function (resp) {
      if (resp.status === 404) return null;
      if (resp.ok) {
        return resp.json().then(function (list) {
          var best = pickBestFromList(list, owner, repo);
          if (best) return best;
          return fetch(latestUrl, { headers: headers }).then(function (latestResp) {
            if (latestResp.status === 404) return null;
            if (!latestResp.ok) return null;
            return latestResp.json().then(function (rel) {
              return mapRelease(rel, owner, repo);
            });
          });
        });
      }
      return fetch(latestUrl, { headers: headers }).then(function (latestResp) {
        if (latestResp.status === 404) return null;
        if (!latestResp.ok) throw new Error(owner + "/" + repo + ": HTTP " + latestResp.status);
        return latestResp.json().then(function (rel) {
          return mapRelease(rel, owner, repo);
        });
      });
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
        if (!best) {
          best = r;
          return;
        }
        var cmp = semverCompare(r.version, best.version);
        if (cmp > 0) {
          best = r;
          return;
        }
        if (cmp === 0) {
          if (r.publishedAt && best.publishedAt && r.publishedAt > best.publishedAt) {
            best = r;
          } else if (r.publishedAt === best.publishedAt && best.prerelease && !r.prerelease) {
            best = r;
          }
        }
      });
      return { release: best, sourcesChecked: sourcesChecked };
    });
  }

  function assetRows(release) {
    var rows = [];
    PLATFORM_ASSETS.forEach(function (spec) {
      var asset = findAsset(release.assets, spec.asset);
      if (!asset) return;
      rows.push({
        id: spec.id,
        label: t(spec.labelKey),
        fileName: asset.name,
        url: asset.browser_download_url,
        checksumUrl: findChecksumUrl(release.assets, spec.asset)
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
      topbar.title = rel && rel.tag
        ? (rel.prerelease ? "Latest pre-release on GitHub" : "Latest release on GitHub")
        : "DogeGo version";
    }
    var demoVer = document.querySelector(".demo-topbar-ver");
    if (demoVer) demoVer.textContent = display;
  }

  function setHeroReleaseNote(text) {
    var el = document.getElementById("hero-release-note");
    if (!el) return;
    var textEl = el.querySelector(".hero-status-text");
    if (textEl) textEl.textContent = text;
    else if (el) el.textContent = text;
  }

  function setText(el, text) {
    if (el) el.textContent = text;
  }

  function setHtml(el, html) {
    if (el) el.innerHTML = html;
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

  function copyText(text, btn) {
    if (!text) return;
    var label = btn && btn.querySelector(".platform-checksum-copy-label");
    var icon = btn && btn.querySelector(".material-icons-round");
    var copiedLabel = t("download.copiedSha");
    var copyLabel = t("download.copySha");
    if (copiedLabel === "download.copiedSha") copiedLabel = "Copied";
    if (copyLabel === "download.copySha") copyLabel = "Copy";

    function markCopied() {
      if (!btn) return;
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
  }

  function bindChecksumCopyButtons() {
    document.querySelectorAll(".platform-checksum-copy").forEach(function (btn) {
      if (btn.getAttribute("data-copy-bound") === "1") return;
      btn.setAttribute("data-copy-bound", "1");
      btn.addEventListener("click", function () {
        var card = btn.closest(".platform-card");
        if (!card) return;
        var hashEl = card.querySelector(".platform-checksum-hash");
        var hash = hashEl ? (hashEl.textContent || "").trim() : "";
        if (!hash || hash.indexOf("…") >= 0) return;
        copyText(hash, btn);
      });
    });
  }

  function setPlatformBadges(card, tagDisplay, prerelease) {
    var wrap = card.querySelector(".platform-badges");
    if (!wrap) return;
    var verBadge = wrap.querySelector(".platform-badge-version");
    var channelBadge = wrap.querySelector(".platform-badge-channel");
    if (verBadge) verBadge.textContent = tagDisplay;
    if (channelBadge) {
      channelBadge.textContent = prerelease ? t("download.badgePrerelease") : t("download.badgeStable");
      channelBadge.classList.toggle("platform-badge--prerelease", !!prerelease);
      channelBadge.classList.toggle("platform-badge--stable", !prerelease);
    }
    wrap.hidden = false;
  }

  function hidePlatformBadges(card) {
    var wrap = card.querySelector(".platform-badges");
    if (wrap) wrap.hidden = true;
  }

  function setPlatformChecksum(card, hash, pending) {
    var wrap = card.querySelector(".platform-checksum");
    var hashEl = card && card.querySelector(".platform-checksum-hash");
    if (!wrap || !hashEl) return;
    if (pending) {
      wrap.hidden = false;
      hashEl.textContent = t("download.checksumPending");
      hashEl.classList.add("platform-checksum-hash--pending");
      return;
    }
    hashEl.classList.remove("platform-checksum-hash--pending");
    if (hash) {
      wrap.hidden = false;
      hashEl.textContent = hash;
      hashEl.title = hash;
    } else {
      wrap.hidden = false;
      hashEl.textContent = t("download.checksumUnavailable");
      hashEl.title = "";
    }
  }

  function hidePlatformChecksum(card) {
    var wrap = card.querySelector(".platform-checksum");
    if (wrap) wrap.hidden = true;
  }

  function setGridStatus(stateName, message) {
    var el = document.getElementById("download-grid-status");
    if (!el) return;
    el.setAttribute("data-state", stateName || "loading");
    if (message) setText(el, message);
  }

  function renderPlatformCardsSoon() {
    var platformGrid = document.getElementById("platform-grid");
    if (!platformGrid) return;
    platformGrid.classList.add("platform-grid--loading");
    platformGrid.classList.remove("platform-grid--ready");
    platformGrid.querySelectorAll(".platform-card").forEach(function (card) {
      card.classList.add("platform-soon");
      hidePlatformBadges(card);
      hidePlatformChecksum(card);
      var link = card.querySelector(".platform-download-btn");
      if (link) {
        link.textContent = t("download.soonLink");
        link.href = "#download";
        link.removeAttribute("target");
        link.removeAttribute("rel");
      }
    });
  }

  function renderPlatformCardsLoading() {
    var platformGrid = document.getElementById("platform-grid");
    if (!platformGrid) return;
    platformGrid.classList.add("platform-grid--loading");
    platformGrid.classList.remove("platform-grid--ready");
    platformGrid.querySelectorAll(".platform-card").forEach(function (card) {
      card.classList.add("platform-soon");
      hidePlatformBadges(card);
      hidePlatformChecksum(card);
      var link = card.querySelector(".platform-download-btn");
      if (link) {
        link.textContent = t("download.checkingTitle");
        link.href = "#download";
        link.removeAttribute("target");
        link.removeAttribute("rel");
      }
    });
  }

  function renderPlatformCardsReady(rel, rows) {
    var platformGrid = document.getElementById("platform-grid");
    if (!platformGrid) return;
    platformGrid.classList.remove("platform-grid--loading");
    platformGrid.classList.add("platform-grid--ready");
    var tagDisplay = rel.tag || ("v" + rel.version);
    var rowById = {};
    rows.forEach(function (row) { rowById[row.id] = row; });

    platformGrid.querySelectorAll(".platform-card").forEach(function (card) {
      var pid = card.getAttribute("data-platform");
      var link = card.querySelector(".platform-download-btn");
      var match = rowById[pid];
      setPlatformBadges(card, tagDisplay, rel.prerelease);

      if (match) {
        card.classList.remove("platform-soon");
        if (link) {
          link.textContent = t("download.downloadLink");
          link.href = match.url;
          link.target = "_blank";
          link.rel = "noopener noreferrer";
        }
        var hash = state.checksums[match.id] || "";
        if (match.checksumUrl && !hash) {
          setPlatformChecksum(card, "", true);
        } else if (hash) {
          setPlatformChecksum(card, hash, false);
        } else {
          setPlatformChecksum(card, "", false);
        }
      } else {
        card.classList.add("platform-soon");
        if (link) {
          link.textContent = t("download.getReleases");
          link.href = rel.htmlUrl;
          link.target = "_blank";
          link.rel = "noopener noreferrer";
        }
        hidePlatformChecksum(card);
      }
    });
    bindChecksumCopyButtons();
  }

  function loadChecksums(rel, rows) {
    var jobs = rows.map(function (row) {
      if (!row.checksumUrl) {
        state.checksums[row.id] = "";
        return Promise.resolve();
      }
      return fetchChecksumText(row.checksumUrl).then(function (hash) {
        state.checksums[row.id] = hash;
      });
    });
    return Promise.all(jobs).then(function () {
      renderPlatformCardsReady(rel, rows);
    });
  }

  function render() {
    var platformGrid = document.getElementById("platform-grid");
    var heroDownload = document.getElementById("hero-download-btn");

    if (state.loading) {
      setGridStatus("loading", t("download.checkingBody"));
      renderPlatformCardsLoading();
      updateVersionDisplay(null);
      return;
    }

    var rel = state.release;
    if (!rel) {
      setGridStatus("soon", t("download.soonBodyPlain") || t("download.soonHeroNote"));
      renderPlatformCardsSoon();
      if (heroDownload) {
        heroDownload.href = "#download";
        heroDownload.removeAttribute("target");
        heroDownload.removeAttribute("rel");
        var heroLabel = heroDownload.querySelector("[data-i18n='hero.download']");
        if (heroLabel) heroLabel.textContent = t("download.soonCta");
      }
      setHeroReleaseNote(t("download.soonHeroNote"));
      updateVersionDisplay(null);
      return;
    }

    var rows = assetRows(rel);
    var tagDisplay = rel.tag || ("v" + rel.version);

    setGridStatus(
      rel.prerelease ? "prerelease" : "ready",
      rel.prerelease
        ? t("download.readyBodyPrereleasePlain", { source: rel.sourceLabel, version: tagDisplay })
        : t("download.readyBodyPlain", { source: rel.sourceLabel, version: tagDisplay })
    );

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
    setHeroReleaseNote(
      rel.prerelease
        ? t("download.readyHeroNotePrerelease", { source: rel.sourceLabel })
        : t("download.readyHeroNote", { source: rel.sourceLabel })
    );
    updateVersionDisplay(rel);

    state.checksums = {};
    renderPlatformCardsReady(rel, rows);
    if (rows.length) {
      loadChecksums(rel, rows);
    } else if (platformGrid) {
      platformGrid.classList.remove("platform-grid--loading");
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
    state.checksums = {};
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
    bindChecksumCopyButtons();
    render();
    refresh();
    document.addEventListener("dogego:locale", function () {
      render();
      bindChecksumCopyButtons();
    });
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
