/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Client i18n: /locales/{code}.json + data-i18n* attributes. */
(function (global) {
  var LS_LOCALE = "dogego_locale";
  var FALLBACK = "en";

  var LOCALES = [
    { code: "en", name: "English", native: "English" },
    { code: "fr", name: "French", native: "Français" },
    { code: "pt-PT", name: "Portuguese (Portugal)", native: "Português (Portugal)" },
    { code: "de", name: "German", native: "Deutsch" },
    { code: "zh", name: "Chinese", native: "中文" },
    { code: "ja", name: "Japanese", native: "日本語" },
  ];

  var dict = {};
  var locale = FALLBACK;
  var readyResolve;
  var readyPromise = new Promise(function (resolve) { readyResolve = resolve; });

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

  function get(obj, key) {
    if (!obj || !key) return undefined;
    var parts = key.split(".");
    var cur = obj;
    for (var i = 0; i < parts.length; i++) {
      if (cur == null || typeof cur !== "object") return undefined;
      cur = cur[parts[i]];
    }
    return typeof cur === "string" ? cur : undefined;
  }

  function deepMergeLocale(base, overlay) {
    if (!base) return overlay || {};
    if (!overlay) return JSON.parse(JSON.stringify(base));
    var out = JSON.parse(JSON.stringify(base));
    function merge(into, from) {
      Object.keys(from).forEach(function (k) {
        var fv = from[k];
        var iv = into[k];
        if (fv && typeof fv === "object" && !Array.isArray(fv) && iv && typeof iv === "object" && !Array.isArray(iv)) {
          merge(iv, fv);
        } else {
          into[k] = fv;
        }
      });
    }
    merge(out, overlay);
    return out;
  }

  function t(key, params) {
    var s = get(dict, key) || get(dict, key.replace(/-/g, "_"));
    if (s == null && locale !== FALLBACK) {
      s = get(global.__dogego_i18n_fallback || {}, key);
    }
    if (s == null) return key;
    if (params) {
      Object.keys(params).forEach(function (k) {
        s = s.replace(new RegExp("\\{" + k + "\\}", "g"), String(params[k]));
      });
    }
    return s;
  }

  function loadLocale(code) {
    return fetch("/locales/" + encodeURIComponent(code) + ".json", { cache: "no-cache" })
      .then(function (r) {
        if (!r.ok) throw new Error("locale " + code);
        return r.json();
      });
  }

  function applyChoiceCards(scope) {
    (scope || document).querySelectorAll(".choice-card[data-i18n-card], .setup-option-card[data-i18n-card]").forEach(function (card) {
      var prefix = card.getAttribute("data-i18n-card");
      if (!prefix) return;
      var strong = card.querySelector("strong");
      var sub = card.querySelector("span");
      var title = t(prefix + ".title");
      var subtitle = t(prefix + ".sub");
      if (strong && title !== prefix + ".title") strong.textContent = title;
      if (sub && subtitle !== prefix + ".sub") sub.textContent = subtitle;
    });
  }

  function applyFieldLabels(scope) {
    (scope || document).querySelectorAll("[data-i18n-field]").forEach(function (wrap) {
      var fk = wrap.getAttribute("data-i18n-field");
      if (!fk) return;
      var label = wrap.tagName === "LABEL" ? wrap : wrap.querySelector("label");
      if (!label) return;
      var labelKey = "field." + fk + ".label";
      var helpKey = "field." + fk + ".help";
      var labelText = t(labelKey);
      var helpText = t(helpKey);
      var helpBtn = label.querySelector(".help-btn");
      Array.from(label.childNodes).forEach(function (node) {
        if (node !== helpBtn) label.removeChild(node);
      });
      if (labelText !== labelKey) {
        var textSpan = document.createElement("span");
        textSpan.className = "i18n-lbl";
        textSpan.textContent = labelText;
        label.appendChild(textSpan);
      }
      if (helpBtn) {
        if (helpText !== helpKey) {
          helpBtn.setAttribute("data-i18n-help", helpKey);
          helpBtn.setAttribute("data-help", helpText);
        }
        label.appendChild(helpBtn);
      }
      var input = wrap.querySelector("input, textarea, select");
      if (input) {
        var phKey = "field." + fk + ".placeholder";
        var ph = t(phKey);
        if (ph !== phKey && !input.getAttribute("data-i18n-placeholder")) {
          input.setAttribute("placeholder", ph);
        }
      }
    });
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
    scope.querySelectorAll(".help-btn[data-i18n-help], [data-i18n-help].help-btn").forEach(function (el) {
      var key = el.getAttribute("data-i18n-help");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.setAttribute("data-help", val);
    });
    scope.querySelectorAll("[data-i18n-help]:not(.help-btn)").forEach(function (el) {
      var key = el.getAttribute("data-i18n-help");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.setAttribute("data-help", val);
    });
    scope.querySelectorAll("[data-i18n-wait]").forEach(function (el) {
      var key = el.getAttribute("data-i18n-wait");
      if (!key) return;
      var val = t(key);
      if (val !== key) el.setAttribute("data-doge-wait-msg", val);
    });
    applyFieldLabels(scope);
    applyChoiceCards(scope);
    applyProfileCards(scope);
  }

  function applyProfileCards(scope) {
    var profileKey = {
      "first-run": "firstRun",
      "mainnet": "mainnet",
      "mainnet-wallet": "mainnetWallet",
      "spv-wallet": "spvWallet",
    };
    (scope || document).querySelectorAll(".wizard-profile-card").forEach(function (card) {
      var p = card.getAttribute("data-profile");
      var key = profileKey[p] || p;
      var title = card.querySelector(".wizard-profile-title");
      var sub = card.querySelector(".wizard-profile-sub");
      var titleText = t("wizard.profiles." + key + ".title");
      var subText = t("wizard.profiles." + key + ".sub");
      if (title && titleText !== "wizard.profiles." + key + ".title") title.textContent = titleText;
      if (sub && subText !== "wizard.profiles." + key + ".sub") sub.textContent = subText;
    });
  }

  function localeInfo(code) {
    return LOCALES.find(function (l) { return l.code === code; }) || LOCALES[0];
  }

  function updateLangPicker() {
    var btn = document.getElementById("lang-picker-label");
    if (btn) btn.textContent = localeInfo(locale).native;
    document.querySelectorAll(".lang-picker-item").forEach(function (el) {
      var c = el.getAttribute("data-lang");
      el.classList.toggle("selected", c === locale);
      el.setAttribute("aria-selected", c === locale ? "true" : "false");
    });
    document.documentElement.lang = locale.split("-")[0];
  }

  function bindLangPicker() {
    var wrap = document.getElementById("lang-picker");
    if (!wrap || wrap.dataset.bound === "1") return;
    wrap.dataset.bound = "1";
    var menuBtn = document.getElementById("lang-picker-btn");
    var menu = document.getElementById("lang-picker-menu");
    if (!menuBtn || !menu) return;

    LOCALES.forEach(function (loc) {
      var li = document.createElement("button");
      li.type = "button";
      li.className = "lang-picker-item";
      li.setAttribute("role", "option");
      li.setAttribute("data-lang", loc.code);
      li.innerHTML = "<span class=\"lang-picker-native\">" + loc.native + "</span><span class=\"lang-picker-en\">" + loc.name + "</span>";
      li.addEventListener("click", function () {
        setLocale(loc.code);
        menu.hidden = true;
        menuBtn.setAttribute("aria-expanded", "false");
      });
      menu.appendChild(li);
    });

    menuBtn.addEventListener("click", function (e) {
      e.stopPropagation();
      var open = menu.hidden;
      menu.hidden = !open;
      menuBtn.setAttribute("aria-expanded", open ? "true" : "false");
    });
    document.addEventListener("click", function () {
      menu.hidden = true;
      menuBtn.setAttribute("aria-expanded", "false");
    });
    menu.addEventListener("click", function (e) { e.stopPropagation(); });
  }

  function setLocale(code, opts) {
    var next = LOCALES.some(function (l) { return l.code === code; }) ? code : FALLBACK;
    var reload = !opts || opts.reload !== false;
    return loadLocale(next).then(function (data) {
      if (next !== FALLBACK && !global.__dogego_i18n_fallback) {
        return loadLocale(FALLBACK).then(function (fb) {
          global.__dogego_i18n_fallback = fb;
          return data;
        }).catch(function () { return data; });
      }
      return data;
    }).then(function (data) {
      locale = next;
      var fb = global.__dogego_i18n_fallback || {};
      dict = next === FALLBACK ? data : deepMergeLocale(fb, data);
      try { localStorage.setItem(LS_LOCALE, next); } catch (_) {}
      applyDOM(document);
      updateLangPicker();
      document.dispatchEvent(new CustomEvent("dogego:locale", { detail: { locale: next } }));
      if (reload && opts && opts.hard) global.location.reload();
      return next;
    }).catch(function () {
      if (next !== FALLBACK) return setLocale(FALLBACK, { reload: false });
      dict = {};
      return FALLBACK;
    });
  }

  function init() {
    locale = detectLocale();
    bindLangPicker();
    return loadLocale(FALLBACK).then(function (fb) {
      global.__dogego_i18n_fallback = fb;
      if (locale === FALLBACK) {
        dict = fb;
        applyDOM(document);
        updateLangPicker();
        readyResolve(locale);
        return locale;
      }
      return setLocale(locale, { reload: false }).then(function (l) {
        readyResolve(l);
        return l;
      });
    }).catch(function () {
      readyResolve(FALLBACK);
      return FALLBACK;
    });
  }

  global.DogeGoI18n = {
    LOCALES: LOCALES,
    t: t,
    getLocale: function () { return locale; },
    setLocale: setLocale,
    applyDOM: applyDOM,
    ready: function () { return readyPromise; },
    localeInfo: localeInfo,
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window);
