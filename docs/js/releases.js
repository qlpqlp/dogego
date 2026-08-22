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
    checksums: {},
    fromFallback: false
  };

  var GITHUB_API_HEADERS = {
    Accept: "application/vnd.github+json"
  };

  var RELEASES_CACHE_KEY = "dogego:releases:v1";
  var RELEASES_CACHE_TTL_MS = 30 * 60 * 1000;

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

  function isGitHubRateLimited(resp) {
    return resp && (resp.status === 403 || resp.status === 429);
  }

  function readReleaseCache() {
    try {
      var raw = sessionStorage.getItem(RELEASES_CACHE_KEY);
      if (!raw) return null;
      var parsed = JSON.parse(raw);
      if (!parsed || !parsed.at || Date.now() - parsed.at > RELEASES_CACHE_TTL_MS) return null;
      return parsed.result || null;
    } catch (_) {
      return null;
    }
  }

  function writeReleaseCache(result) {
    if (!result || !result.release) return;
    try {
      sessionStorage.setItem(RELEASES_CACHE_KEY, JSON.stringify({ at: Date.now(), result: result }));
    } catch (_) {}
  }

  function fallbackManifestUrl() {
    try {
      return new URL("data/latest-release.json", window.location.href).href;
    } catch (_) {
      return "data/latest-release.json";
    }
  }

  function normalizeFallbackRelease(raw) {
    if (!raw || !raw.tag) return null;
    return {
      tag: raw.tag,
      version: raw.version || normalizeSemver(raw.tag),
      name: raw.name || raw.tag,
      body: typeof raw.body === "string" ? raw.body : "",
      htmlUrl: raw.htmlUrl,
      source: "https://github.com/" + raw.owner + "/" + raw.repo,
      sourceLabel: raw.sourceLabel || (raw.owner + "/" + raw.repo),
      owner: raw.owner,
      repo: raw.repo,
      assets: raw.assets || [],
      publishedAt: raw.publishedAt || "",
      prerelease: !!raw.prerelease,
      checksums: raw.checksums || null,
      fromFallback: true
    };
  }

  function readEmbeddedFallbackRelease() {
    var el = document.getElementById("release-fallback-data");
    if (!el) return null;
    try {
      var data = JSON.parse(el.textContent || "");
      if (!data || !data.release) return null;
      return normalizeFallbackRelease(data.release);
    } catch (_) {
      return null;
    }
  }

  function fetchFallbackRelease() {
    return fetch(fallbackManifestUrl(), { cache: "no-cache" }).then(function (resp) {
      if (!resp.ok) return readEmbeddedFallbackRelease();
      return resp.json().then(function (data) {
        if (!data || !data.release) return readEmbeddedFallbackRelease();
        return normalizeFallbackRelease(data.release);
      });
    }).catch(function () {
      return readEmbeddedFallbackRelease();
    });
  }

  // Prefer GitHub release asset digests from the JSON API (CORS-safe).
  // Do not fetch .sha256 sidecar URLs: release-assets.githubusercontent.com has no CORS.
  function digestFromAsset(asset) {
    if (!asset) return "";
    var d = String(asset.digest || "").trim();
    if (!d) return "";
    var m = /^sha256:([a-fA-F0-9]{64})$/.exec(d);
    if (m) return m[1].toLowerCase();
    if (/^[a-fA-F0-9]{64}$/.test(d)) return d.toLowerCase();
    return "";
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
      owner: owner,
      repo: repo,
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

  function fetchLatestReleaseFromList(owner, repo) {
    var listUrl = "https://api.github.com/repos/" + encodeURIComponent(owner) + "/" +
      encodeURIComponent(repo) + "/releases?per_page=30";
    return fetch(listUrl, { headers: GITHUB_API_HEADERS }).then(function (resp) {
      if (resp.status === 404) return null;
      if (isGitHubRateLimited(resp)) return { rateLimited: true };
      if (!resp.ok) return null;
      return resp.json().then(function (list) {
        return pickBestFromList(list, owner, repo);
      });
    });
  }

  function fetchLatestRelease(owner, repo) {
    return fetchLatestReleaseFromList(owner, repo);
  }

  function pickBestRelease(candidates) {
    var best = null;
    (candidates || []).forEach(function (r) {
      if (!r || r.error || r.rateLimited || !r.tag) return;
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
    return best;
  }

  function fetchRemainingSources(startIndex) {
    var tail = UPDATE_SOURCES.slice(startIndex);
    if (!tail.length) return Promise.resolve([]);
    var checked = [];
    return tail.reduce(function (chain, src) {
      return chain.then(function (results) {
        checked.push(src.owner + "/" + src.repo);
        return fetchLatestRelease(src.owner, src.repo).then(function (release) {
          results.push(release);
          return results;
        }).catch(function (err) {
          results.push({ error: err });
          return results;
        });
      });
    }, Promise.resolve([])).then(function (results) {
      return { results: results, sourcesChecked: checked };
    });
  }

  function fetchBestRelease() {
    var cached = readReleaseCache();
    if (cached) return Promise.resolve(cached);

    var sourcesChecked = [];
    var primary = UPDATE_SOURCES[0];
    sourcesChecked.push(primary.owner + "/" + primary.repo);

    return fetchLatestRelease(primary.owner, primary.repo).then(function (primaryRelease) {
      if (primaryRelease && primaryRelease.rateLimited) {
        return fetchFallbackRelease().then(function (fallback) {
          if (fallback) {
            return {
              release: fallback,
              sourcesChecked: sourcesChecked.concat(["docs/data/latest-release.json"])
            };
          }
          return { release: null, sourcesChecked: sourcesChecked };
        });
      }

      if (primaryRelease && primaryRelease.tag) {
        var hit = { release: primaryRelease, sourcesChecked: sourcesChecked };
        writeReleaseCache(hit);
        return hit;
      }

      var candidates = [];
      if (primaryRelease) candidates.push(primaryRelease);

      return fetchRemainingSources(1).then(function (tail) {
        sourcesChecked = sourcesChecked.concat(tail.sourcesChecked);
        candidates = candidates.concat(tail.results || []);
        var best = pickBestRelease(candidates);
        if (best && !best.fromFallback) {
          var result = { release: best, sourcesChecked: sourcesChecked };
          writeReleaseCache(result);
          return result;
        }
        if (best) return { release: best, sourcesChecked: sourcesChecked };
        return fetchFallbackRelease().then(function (fallback) {
          if (fallback) {
            return {
              release: fallback,
              sourcesChecked: sourcesChecked.concat(["docs/data/latest-release.json"])
            };
          }
          return { release: null, sourcesChecked: sourcesChecked };
        });
      });
    }).catch(function () {
      return fetchFallbackRelease().then(function (fallback) {
        if (fallback) {
          return {
            release: fallback,
            sourcesChecked: sourcesChecked.concat(["docs/data/latest-release.json"])
          };
        }
        return { release: null, sourcesChecked: sourcesChecked };
      });
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
        digest: digestFromAsset(asset)
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

  var GRID_STATUS_ICONS = {
    loading: "sync",
    ready: "verified",
    prerelease: "new_releases",
    soon: "schedule"
  };

  function setPlatformDownloadBtn(link, iconName, labelText) {
    if (!link) return;
    var icon = link.querySelector(".material-icons-round");
    var label = link.querySelector(".platform-download-label");
    if (icon && iconName) icon.textContent = iconName;
    if (label && labelText !== undefined) label.textContent = labelText;
    else if (labelText !== undefined) link.textContent = labelText;
    if (icon && iconName === "sync") {
      icon.classList.add("platform-download-icon--spin");
    } else if (icon) {
      icon.classList.remove("platform-download-icon--spin");
    }
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
    var state = stateName || "loading";
    el.setAttribute("data-state", state);
    var iconEl = el.querySelector(".download-grid-status-icon");
    if (iconEl) {
      iconEl.textContent = GRID_STATUS_ICONS[state] || GRID_STATUS_ICONS.loading;
      iconEl.classList.toggle("download-grid-status-icon--spin", state === "loading");
    }
    var textEl = el.querySelector(".download-grid-status-text");
    if (message && textEl) textEl.textContent = message;
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
        setPlatformDownloadBtn(link, "schedule", t("download.soonLink"));
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
        setPlatformDownloadBtn(link, "sync", t("download.checkingTitle"));
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
          setPlatformDownloadBtn(link, "download", t("download.downloadLink"));
          link.href = match.url;
          link.target = "_blank";
          link.rel = "noopener noreferrer";
        }
        var hash = state.checksums[match.id] || "";
        if (hash) {
          setPlatformChecksum(card, hash, false);
        } else {
          setPlatformChecksum(card, "", false);
        }
      } else {
        card.classList.add("platform-soon");
        if (link) {
          setPlatformDownloadBtn(link, "open_in_new", t("download.getReleases"));
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
    rows.forEach(function (row) {
      var fromManifest = rel.checksums && rel.checksums[row.id];
      state.checksums[row.id] = fromManifest || row.digest || "";
    });
    renderPlatformCardsReady(rel, rows);
    return Promise.resolve();
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
