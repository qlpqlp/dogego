(function (global) {
  "use strict";

  var LS_LOCALE = "dogego_locale";
  var FALLBACK = "en";
  var BASE = global.DogeGoSiteLocaleBase || "locales/";

  var LOCALES = [
    { code: "en", name: "English", native: "English" },
    { code: "fr", name: "French", native: "Français" },
    { code: "pt-PT", name: "Portuguese (Portugal)", native: "Português" },
    { code: "de", name: "German", native: "Deutsch" },
    { code: "zh", name: "Chinese", native: "中文" },
    { code: "ja", name: "Japanese", native: "日本語" }
  ];

  var OG_LOCALE = {
    en: "en_US",
    fr: "fr_FR",
    "pt-PT": "pt_PT",
    de: "de_DE",
    zh: "zh_CN",
    ja: "ja_JP"
  };

  var dict = {};
  var fallbackDict = {};
  var locale = FALLBACK;
  var readyResolve;
  var readyPromise = new Promise(function (resolve) { readyResolve = resolve; });
  var loadedScripts = {};

  function isFileProtocol() {
    return global.location && global.location.protocol === "file:";
  }

  function localeBundle(code) {
    var bundles = global.DogeGoSiteLocaleBundles;
    return bundles && bundles[code] ? bundles[code] : null;
  }

  function loadLocaleScript(code) {
    var cached = localeBundle(code);
    if (cached) return Promise.resolve(cached);
    if (loadedScripts[code]) {
      return loadedScripts[code];
    }
    loadedScripts[code] = new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = BASE + encodeURIComponent(code) + ".js";
      s.async = true;
      s.onload = function () {
        var data = localeBundle(code);
        if (data) resolve(data);
        else reject(new Error("locale " + code));
      };
      s.onerror = function () { reject(new Error("locale " + code)); };
      document.head.appendChild(s);
    });
    return loadedScripts[code];
  }

  function loadLocaleFetch(code) {
    return fetch(BASE + encodeURIComponent(code) + ".json", { cache: "no-cache" })
      .then(function (r) {
        if (!r.ok) throw new Error("locale " + code);
        return r.json();
      });
  }

  function loadLocale(code) {
    if (isFileProtocol()) return loadLocaleScript(code);
    return loadLocaleFetch(code);
  }

  function detectLocale() {
    try {
      var stored = localStorage.getItem(LS_LOCALE);
      if (stored && LOCALES.some(function (l) { return l.code === stored; })) return stored;
    } catch (_) {}
    var q = new URLSearchParams(global.location.search).get("lang");
    if (q && LOCALES.some(function (l) { return l.code === q; })) return q;
    var nav = (global.navigator.language || global.navigator.userLanguage || "").toLowerCase();
    if (nav.indexOf("pt") === 0) return nav.indexOf("pt-br") === 0 ? FALLBACK : "pt-PT";
    if (nav.indexOf("fr") === 0) return "fr";
    if (nav.indexOf("de") === 0) return "de";
    if (nav.indexOf("zh") === 0) return "zh";
    if (nav.indexOf("ja") === 0) return "ja";
    return FALLBACK;
  }

  function getRawFrom(obj, key) {
    if (!obj || !key) return undefined;
    var parts = key.split(".");
    var cur = obj;
    for (var i = 0; i < parts.length; i++) {
      if (cur == null || typeof cur !== "object") return undefined;
      cur = cur[parts[i]];
    }
    return cur;
  }

  function get(obj, key) {
    var val = getRawFrom(obj, key);
    return typeof val === "string" ? val : undefined;
  }

  function getRaw(key) {
    var val = getRawFrom(dict, key);
    if (val == null && locale !== FALLBACK) val = getRawFrom(fallbackDict, key);
    return val;
  }

  function t(key, params) {
    var s = get(dict, key);
    if (s == null && locale !== FALLBACK) s = get(fallbackDict, key);
    if (s == null) return key;
    if (params) {
      Object.keys(params).forEach(function (k) {
        s = s.replace(new RegExp("\\{" + k + "\\}", "g"), String(params[k]));
      });
    }
    return s;
  }

  function applyDOM(root) {
    var scope = root || document;
    scope.querySelectorAll("[data-i18n]").forEach(function (el) {
      var key = el.getAttribute("data-i18n");
      if (!key) return;
      var val = t(key);
      if (val === key) return;
      el.textContent = val;
    });
    scope.querySelectorAll("[data-i18n-html]").forEach(function (el) {
      var key = el.getAttribute("data-i18n-html");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.innerHTML = val;
    });
    scope.querySelectorAll("[data-i18n-placeholder]").forEach(function (el) {
      var key = el.getAttribute("data-i18n-placeholder");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.setAttribute("placeholder", val);
    });
    scope.querySelectorAll("[data-i18n-title]").forEach(function (el) {
      var key = el.getAttribute("data-i18n-title");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.setAttribute("title", val);
    });
    scope.querySelectorAll("[data-i18n-aria]").forEach(function (el) {
      var key = el.getAttribute("data-i18n-aria");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.setAttribute("aria-label", val);
    });
  }

  function applyMeta() {
    var title = t("meta.title");
    var docsPage = document.body && document.body.getAttribute("data-docs-page");
    if (!docsPage && title !== "meta.title") document.title = title;
    var desc = t("meta.description");
    if (desc !== "meta.description") {
      var md = document.querySelector('meta[name="description"]');
      if (md) md.setAttribute("content", desc);
      if (!docsPage) {
        var ogt = document.querySelector('meta[property="og:title"]');
        if (ogt) ogt.setAttribute("content", title);
        var ogd = document.querySelector('meta[property="og:description"]');
        if (ogd) ogd.setAttribute("content", desc);
        var twt = document.querySelector('meta[name="twitter:title"]');
        if (twt) twt.setAttribute("content", title);
        var twd = document.querySelector('meta[name="twitter:description"]');
        if (twd) twd.setAttribute("content", desc);
      }
      var ogl = document.querySelector('meta[property="og:locale"]');
      if (ogl) ogl.setAttribute("content", OG_LOCALE[locale] || "en_US");
    }
    document.documentElement.lang = locale === "pt-PT" ? "pt" : locale.split("-")[0];
  }

  function updateLangPickerLabel() {
    var cur = LOCALES.filter(function (l) { return l.code === locale; })[0];
    if (!cur) return;
    var label = document.getElementById("site-lang-label");
    if (label) label.textContent = cur.native;
    document.querySelectorAll(".demo-lang-label").forEach(function (el) {
      el.textContent = cur.native;
    });
  }

  function buildLangMenu() {
    var menu = document.getElementById("site-lang-menu");
    if (!menu) return;
    menu.innerHTML = LOCALES.map(function (l) {
      var sel = l.code === locale ? ' aria-selected="true"' : "";
      return '<button type="button" role="option" data-lang="' + l.code + '"' + sel + ">" + l.native + "</button>";
    }).join("");
    menu.querySelectorAll("[data-lang]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        setLocale(btn.getAttribute("data-lang"));
        closeLangMenu();
      });
    });
  }

  function closeLangMenu() {
    var menu = document.getElementById("site-lang-menu");
    var btn = document.getElementById("site-lang-btn");
    if (menu) menu.hidden = true;
    if (btn) btn.setAttribute("aria-expanded", "false");
  }

  function setLocale(code) {
    if (!LOCALES.some(function (l) { return l.code === code; })) return Promise.resolve();
    return loadLocale(code).then(function (data) {
      locale = code;
      dict = data;
      try { localStorage.setItem(LS_LOCALE, code); } catch (_) {}
      applyMeta();
      applyDOM();
      updateLangPickerLabel();
      buildLangMenu();
      document.dispatchEvent(new CustomEvent("dogego:locale", { detail: { locale: code } }));
    }).catch(function () {
      if (code !== FALLBACK) return setLocale(FALLBACK);
    });
  }

  function initLangPicker() {
    var btn = document.getElementById("site-lang-btn");
    var menu = document.getElementById("site-lang-menu");
    if (!btn || !menu) return;
    buildLangMenu();
    btn.addEventListener("click", function () {
      var open = menu.hidden;
      menu.hidden = !open;
      btn.setAttribute("aria-expanded", open ? "true" : "false");
    });
    document.addEventListener("click", function (e) {
      if (!e.target.closest(".lang-picker")) closeLangMenu();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") closeLangMenu();
    });
  }

  function boot() {
    locale = detectLocale();
    Promise.all([
      loadLocale(FALLBACK).then(function (d) { fallbackDict = d; }),
      loadLocale(locale)
    ]).then(function (results) {
      dict = results[1];
      if (locale !== FALLBACK && (!dict || !dict.meta)) {
        dict = fallbackDict;
        locale = FALLBACK;
      }
      applyMeta();
      applyDOM();
      updateLangPickerLabel();
      initLangPicker();
      readyResolve(locale);
    }).catch(function () {
      locale = FALLBACK;
      dict = fallbackDict;
      applyDOM();
      initLangPicker();
      readyResolve(locale);
    });
  }

  global.DogeGoSiteI18n = {
    t: t,
    get: getRaw,
    locale: function () { return locale; },
    locales: LOCALES,
    ready: function () { return readyPromise; },
    setLocale: setLocale,
    applyDOM: applyDOM
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})(window);
