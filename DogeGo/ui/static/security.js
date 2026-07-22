/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Dashboard PIN + optional WebAuthn biometric gate (server-verified session). */
(function (global) {
  const LS_BIO = "dogego_webauthn_pref";
  const LS_APPLY_PIN = "dogego_apply_pending_pin";
  const PIN6 = /^\d{6}$/;
  const NO_PIN_STATUS = { pin_enabled: false, unlocked: true, locked: false };

  let status = { pin_enabled: false, unlocked: true, locked: false };
  /** True after Lock until PIN unlock; prevents dashboard refresh from re-opening the session UI. */
  let userWantsLocked = false;

  function normalizeStatus(raw) {
    if (!raw || typeof raw !== "object") return { ...NO_PIN_STATUS };
    return {
      pin_enabled: raw.pin_enabled === true,
      unlocked: raw.pin_enabled === true ? raw.unlocked === true : true,
      locked: raw.pin_enabled === true && raw.locked === true,
      locked_seconds: Number(raw.locked_seconds) || 0,
      failed_attempts: Number(raw.failed_attempts) || 0,
      max_failures: Number(raw.max_failures) || 3,
      webauthn_enabled: raw.webauthn_enabled === true,
      remote_auth_active: raw.remote_auth_active === true,
    };
  }

  function clearStalePinStorage() {
    localStorage.removeItem("dogego_web_ui_pin_enabled");
    if (localStorage.getItem(LS_APPLY_PIN) !== "1") {
      localStorage.removeItem("dogego_pending_pin");
      localStorage.removeItem(LS_APPLY_PIN);
    }
  }
  let onUnlock = [];
  let lastDashboardSessionOpen = false;
  let pinBuffer = "";
  let pinSubmitting = false;
  /** Ignore transient locked status / wallet 401s right after a successful unlock (cookie race). */
  let unlockGraceUntil = 0;
  let statusFetchGen = 0;
  /** PIN overlay deferred until boot splash finishes (first summary paint). */
  let pendingBootOverlayMsg = null;

  function emitUnlock() {
    onUnlock.forEach((fn) => { try { fn(); } catch (_) { /* */ } });
  }

  function inUnlockGrace() {
    return Date.now() < unlockGraceUntil;
  }

  function needsUnlock() {
    if (inUnlockGrace()) return false;
    return status.pin_enabled && !status.unlocked;
  }

  function shouldShowLockScreen() {
    if (inUnlockGrace() && status.pin_enabled) return false;
    if (status.remote_auth_active && status.pin_enabled && !status.unlocked) return true;
    return status.pin_enabled && (userWantsLocked || needsUnlock());
  }

  const UNLOCK_MSG = "Enter your 6-digit PIN to view wallet balances, history, and send DOGE.";
  const REMOTE_UNLOCK_MSG = "Enter your 6-digit PIN to use this dashboard from another device on your network.";

  function lockOverlayMessage() {
    if (status.locked) {
      return "Too many wrong PINs. Try again in " + Math.ceil((status.locked_seconds || 0) / 60) + " minutes.";
    }
    if (status.remote_auth_active) return REMOTE_UNLOCK_MSG;
    return UNLOCK_MSG;
  }

  function showOverlay(msg) {
    const el = document.getElementById("sec-overlay");
    if (!el) return;
    // Wait until boot splash finishes so PIN appears after initial data is ready.
    if (document.body.classList.contains("boot-loading")) {
      pendingBootOverlayMsg = msg || lockOverlayMessage();
      return;
    }
    pendingBootOverlayMsg = null;
    el.hidden = false;
    el.removeAttribute("hidden");
    const m = document.getElementById("sec-overlay-msg");
    if (m && msg) m.textContent = msg;
    const err = document.getElementById("sec-overlay-err");
    if (err) {
      err.textContent = "";
      err.style.display = "none";
    }
    document.body.classList.add("wallet-locked");
    syncPinPadState();
  }

  function hideOverlay() {
    pendingBootOverlayMsg = null;
    const el = document.getElementById("sec-overlay");
    if (el) {
      el.hidden = true;
      el.setAttribute("hidden", "");
    }
    document.body.classList.remove("wallet-locked");
  }

  function flushBootDeferredOverlay() {
    if (document.body.classList.contains("boot-loading")) return;
    if (!shouldShowLockScreen()) {
      pendingBootOverlayMsg = null;
      return;
    }
    const msg = pendingBootOverlayMsg || lockOverlayMessage();
    pendingBootOverlayMsg = null;
    showOverlay(msg);
  }

  async function fetchStatus() {
    const hadPIN = status.pin_enabled;
    const gen = ++statusFetchGen;
    try {
      const r = await fetch("/api/security/status", { credentials: "same-origin", cache: "no-store" });
      if (gen !== statusFetchGen) return status;
      if (!r.ok) {
        if (!hadPIN) status = { ...NO_PIN_STATUS };
        return status;
      }
      const next = normalizeStatus(await r.json());
      if (gen !== statusFetchGen) return status;
      status = next;
      if (!status.pin_enabled) {
        clearStalePinStorage();
        userWantsLocked = false;
        unlockGraceUntil = 0;
      } else if (status.unlocked) {
        // Server confirmed session; clear grace and do not force-lock from polls.
        unlockGraceUntil = 0;
        if (!userWantsLocked) {
          /* session open */
        }
      } else if (needsUnlock() && !inUnlockGrace()) {
        // Session expired/missing; show lock. Do not set userWantsLocked here during grace.
        userWantsLocked = true;
      }
      return status;
    } catch (_) {
      if (gen !== statusFetchGen) return status;
      if (!hadPIN) status = { ...NO_PIN_STATUS };
      return status;
    }
  }

  function syncSettingsPanel() {
    const box = document.getElementById("st-sec-setup");
    const noneHint = document.getElementById("st-sec-none-hint");
    const statusLine = document.getElementById("st-sec-status");
    const disableBox = document.getElementById("st-sec-disable-pin")?.closest(".form-field");
    const lockSettings = document.getElementById("st-sec-lock");
    if (noneHint) noneHint.hidden = !!status.pin_enabled;
    if (box) box.hidden = false;
    if (disableBox) disableBox.hidden = !status.pin_enabled;
    const bioRow = document.getElementById("st-sec-biometric")?.closest(".switch");
    if (bioRow) bioRow.hidden = !status.pin_enabled;
    if (lockSettings) lockSettings.hidden = !status.pin_enabled;
    if (statusLine) {
      if (!status.pin_enabled) {
        statusLine.textContent = "Dashboard PIN: off (no unlock screen).";
      } else if (status.locked) {
        statusLine.textContent = "Dashboard PIN: on ... locked (" +
          Math.ceil((status.locked_seconds || 0) / 60) + " min remaining).";
      } else if (status.unlocked && !userWantsLocked) {
        statusLine.textContent = "Dashboard PIN: on ... wallet unlocked for this browser session.";
      } else {
        statusLine.textContent = "Dashboard PIN: on ... enter PIN on the unlock screen.";
      }
    }
  }

  function syncDashboardLockNav() {
    const btn = document.getElementById("nav-dashboard-lock");
    const label = document.getElementById("nav-dashboard-lock-label");
    const icon = document.getElementById("nav-dashboard-lock-icon");
    if (!btn) return;
    btn.hidden = !status.pin_enabled;
    if (btn.hidden) return;
    const locked = shouldShowLockScreen();
    const lockLbl = (global.DogeGoI18n && global.DogeGoI18n.t("nav.lockDashboard")) || "Lock dashboard";
    const unlockLbl = (global.DogeGoI18n && global.DogeGoI18n.t("nav.unlockDashboard")) || "Unlock dashboard";
    if (label) label.textContent = locked ? unlockLbl : lockLbl;
    if (icon) icon.textContent = locked ? "lock_open" : "lock";
    btn.title = locked ? unlockLbl : lockLbl;
    btn.setAttribute("aria-label", btn.title);
  }

  async function refreshUI() {
    await fetchStatus();
    syncSettingsPanel();
    syncDashboardLockNav();
    if (typeof global.DogeGoSyncTopbarLock === "function") global.DogeGoSyncTopbarLock();
    const bioBtn = document.getElementById("sec-biometric-btn");
    if (bioBtn) {
      bioBtn.hidden = !status.pin_enabled || !status.webauthn_enabled;
    }
    // While submitting a PIN, only skip overlay churn if we are still locked.
    // Successful unlock must hide the overlay even before pinSubmitting clears.
    if (pinSubmitting && !status.unlocked && !inUnlockGrace()) {
      syncPinPadState();
      return status;
    }
    if (!status.pin_enabled) {
      userWantsLocked = false;
      hideOverlay();
      return status;
    }
    if (shouldShowLockScreen()) {
      lastDashboardSessionOpen = false;
      showOverlay(lockOverlayMessage());
    } else {
      userWantsLocked = false;
      hideOverlay();
      const sessionOpen = status.pin_enabled && (status.unlocked || inUnlockGrace());
      if (sessionOpen && !lastDashboardSessionOpen) {
        emitUnlock();
      }
      lastDashboardSessionOpen = sessionOpen;
    }
    syncPinPadState();
    return status;
  }

  function pinFromPad() {
    return pinBuffer;
  }

  function renderPinPad() {
    const cells = document.querySelectorAll("#sec-pin-pad .pin-cell");
    cells.forEach((c, i) => {
      if (i < pinBuffer.length) {
        c.dataset.digit = pinBuffer[i];
        c.textContent = "•";
        c.classList.add("filled");
      } else {
        c.classList.remove("filled");
        c.textContent = "";
        delete c.dataset.digit;
      }
    });
    syncPinPadState();
  }

  function syncPinPadState() {
    const pad = document.getElementById("sec-pin-pad");
    const busy = document.getElementById("sec-pin-busy");
    const submitBtn = document.getElementById("sec-pin-submit");
    const inputsLocked = status.locked || pinSubmitting;
    if (pad) pad.classList.toggle("pin-pad-busy", pinSubmitting);
    if (pad) {
      pad.querySelectorAll("button[data-digit], #sec-pin-back").forEach((btn) => {
        btn.disabled = inputsLocked;
      });
    }
    if (busy) busy.hidden = !pinSubmitting;
    if (submitBtn) {
      const ready = pinBuffer.length === 6 && !pinSubmitting && !status.locked;
      submitBtn.hidden = !ready;
      submitBtn.disabled = !ready;
    }
  }

  function clearPad() {
    pinBuffer = "";
    pinSubmitting = false;
    renderPinPad();
  }

  function fillPad(d) {
    if (pinSubmitting || status.locked || pinBuffer.length >= 6) return;
    pinBuffer += String(d);
    renderPinPad();
    if (pinBuffer.length === 6) {
      setTimeout(() => { void submitPinEntry(); }, 0);
    }
  }

  async function submitPinEntry() {
    if (pinSubmitting) return;
    const pin = pinBuffer;
    if (!PIN6.test(pin)) return;
    pinSubmitting = true;
    syncPinPadState();
    try {
      await unlockWithPIN(pin);
      pinBuffer = "";
      renderPinPad();
    } catch (e) {
      const err = document.getElementById("sec-overlay-err");
      if (err) {
        err.textContent = e.message || String(e);
        err.style.display = "block";
      }
    } finally {
      pinSubmitting = false;
      syncPinPadState();
    }
  }

  async function unlockWithPIN(pin) {
    const r = await fetch("/api/security/unlock", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pin }),
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) {
      clearPad();
      userWantsLocked = true;
      unlockGraceUntil = 0;
      await refreshUI();
      throw new Error(data.error || "Unlock failed");
    }
    // Optimistic unlock before status poll; avoids pinSubmitting skipping hideOverlay
    // and background refresh re-locking before the session cookie is visible.
    userWantsLocked = false;
    unlockGraceUntil = Date.now() + 8000;
    status = { ...status, pin_enabled: true, unlocked: true, locked: false };
    hideOverlay();
    const wasOpen = lastDashboardSessionOpen;
    lastDashboardSessionOpen = true;
    if (!wasOpen) emitUnlock();
    await refreshUI();
  }

  function bufURL(buf) {
    const bytes = new Uint8Array(buf);
    let s = "";
    for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
    return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
  }

  function credToJSON(cred) {
    const res = cred.response;
    const out = { id: cred.id, rawId: bufURL(cred.rawId), type: cred.type, response: {} };
    if (res.clientDataJSON) out.response.clientDataJSON = bufURL(res.clientDataJSON);
    if (res.attestationObject) out.response.attestationObject = bufURL(res.attestationObject);
    if (res.authenticatorData) out.response.authenticatorData = bufURL(res.authenticatorData);
    if (res.signature) out.response.signature = bufURL(res.signature);
    if (res.userHandle) out.response.userHandle = bufURL(res.userHandle);
    return out;
  }

  function parseCreationOptions(pk) {
    const o = JSON.parse(JSON.stringify(pk));
    o.challenge = Uint8Array.from(atob(o.challenge.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
    if (o.user && o.user.id) {
      o.user.id = Uint8Array.from(atob(o.user.id.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
    }
    if (Array.isArray(o.excludeCredentials)) {
      o.excludeCredentials = o.excludeCredentials.map((cred) => ({
        ...cred,
        id: Uint8Array.from(atob(cred.id.replace(/-/g, "+").replace(/_/g, "/")), (ch) => ch.charCodeAt(0)),
      }));
    }
    return o;
  }

  function parseRequestOptions(pk) {
    const o = JSON.parse(JSON.stringify(pk));
    o.challenge = Uint8Array.from(atob(o.challenge.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
    if (Array.isArray(o.allowCredentials)) {
      o.allowCredentials = o.allowCredentials.map((cred) => ({
        ...cred,
        id: Uint8Array.from(atob(cred.id.replace(/-/g, "+").replace(/_/g, "/")), (ch) => ch.charCodeAt(0)),
      }));
    }
    return o;
  }

  async function registerWebAuthn() {
    const r1 = await fetch("/api/security/webauthn/register/begin", {
      method: "POST",
      credentials: "same-origin",
    });
    const j1 = await r1.json().catch(() => ({}));
    if (!r1.ok) throw new Error(j1.error || "Biometric setup failed");
    const cred = await navigator.credentials.create({ publicKey: parseCreationOptions(j1.publicKey) });
    const r2 = await fetch("/api/security/webauthn/register/finish", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id: j1.session_id, credential: credToJSON(cred) }),
    });
    const j2 = await r2.json().catch(() => ({}));
    if (!r2.ok) throw new Error(j2.error || "Biometric registration failed");
    localStorage.setItem(LS_BIO, "1");
    await refreshUI();
  }

  async function tryWebAuthnUnlock() {
    if (!window.PublicKeyCredential || !status.pin_enabled) return false;
    if (!status.webauthn_enabled) return false;
    try {
      const r1 = await fetch("/api/security/webauthn/login/begin", {
        method: "POST",
        credentials: "same-origin",
      });
      const j1 = await r1.json().catch(() => ({}));
      if (!r1.ok) throw new Error(j1.error || "Biometric login failed");
      const cred = await navigator.credentials.get({ publicKey: parseRequestOptions(j1.publicKey) });
      const r2 = await fetch("/api/security/webauthn/login/finish", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ session_id: j1.session_id, credential: credToJSON(cred) }),
      });
      const j2 = await r2.json().catch(() => ({}));
      if (!r2.ok) throw new Error(j2.error || "Biometric unlock failed");
      userWantsLocked = false;
      unlockGraceUntil = Date.now() + 8000;
      status = { ...status, pin_enabled: true, unlocked: true, locked: false };
      hideOverlay();
      const wasOpen = lastDashboardSessionOpen;
      lastDashboardSessionOpen = true;
      if (!wasOpen) emitUnlock();
      await refreshUI();
      return true;
    } catch (_) {
      return false;
    }
  }

  async function setupPIN(current, neu) {
    const r = await fetch("/api/security/setup", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ current_pin: current || "", new_pin: neu }),
    });
    const data = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(data.error || "Setup failed");
    userWantsLocked = false;
    await refreshUI();
  }

  async function lockNow() {
    if (!status.pin_enabled) return;
    userWantsLocked = true;
    unlockGraceUntil = 0;
    clearPad();
    status.unlocked = false;
    showOverlay(UNLOCK_MSG);
    try {
      const r = await fetch("/api/security/lock", { method: "POST", credentials: "same-origin", cache: "no-store" });
      if (!r.ok) {
        const data = await r.json().catch(() => ({}));
        throw new Error(data.error || "Lock failed (HTTP " + r.status + ")");
      }
      status.unlocked = false;
      await refreshUI();
    } catch (e) {
      userWantsLocked = true;
      status.unlocked = false;
      showOverlay(UNLOCK_MSG);
      await refreshUI();
      const err = document.getElementById("sec-overlay-err");
      if (err && shouldShowLockScreen()) {
        err.textContent = e.message || String(e);
        err.style.display = "block";
      }
      throw e;
    }
  }

  function guardFetch(url, options) {
    if (!shouldShowLockScreen()) return fetch(url, options);
    return Promise.resolve(new Response(JSON.stringify({ error: "wallet_locked", pin_required: true }), {
      status: 401,
      headers: { "Content-Type": "application/json" },
    }));
  }

  async function noteWalletAPIUnauthorized(res) {
    if (!res || res.status !== 401) return;
    if (inUnlockGrace()) return;
    let data = {};
    try { data = await res.clone().json(); } catch (_) { /* */ }
    if (data.pin_required || data.error === "wallet_locked") {
      userWantsLocked = true;
      status.unlocked = false;
      await refreshUI();
    }
  }

  function onPinOverlayKeydown(e) {
    const overlay = document.getElementById("sec-overlay");
    if (!overlay || overlay.hidden) return;
    if (status.locked) return;
    if (e.key >= "0" && e.key <= "9") {
      e.preventDefault();
      fillPad(e.key);
      return;
    }
    if (e.key === "Backspace") {
      e.preventDefault();
      if (pinSubmitting) return;
      pinBuffer = pinBuffer.slice(0, -1);
      renderPinPad();
      const err = document.getElementById("sec-overlay-err");
      if (err) err.textContent = "";
      return;
    }
    if (e.key === "Enter" && pinBuffer.length === 6 && !pinSubmitting) {
      e.preventDefault();
      void submitPinEntry();
    }
  }

  function bindUI() {
    document.addEventListener("keydown", onPinOverlayKeydown);
    const pad = document.getElementById("sec-pin-pad");
    if (pad) {
      pad.querySelectorAll("button[data-digit]").forEach((btn) => {
        btn.addEventListener("click", () => {
          if (status.locked) return;
          fillPad(btn.dataset.digit);
        });
      });
      const back = document.getElementById("sec-pin-back");
      if (back) {
        back.addEventListener("click", () => {
          if (pinSubmitting) return;
          pinBuffer = pinBuffer.slice(0, -1);
          renderPinPad();
          const err = document.getElementById("sec-overlay-err");
          if (err) err.textContent = "";
        });
      }
    }
    const pinSubmit = document.getElementById("sec-pin-submit");
    if (pinSubmit) {
      pinSubmit.addEventListener("click", () => { void submitPinEntry(); });
    }
    const bioBtn = document.getElementById("sec-biometric-btn");
    if (bioBtn) {
      bioBtn.addEventListener("click", () => tryWebAuthnUnlock());
    }
    const lockBtn = document.getElementById("topbar-lock");
    if (lockBtn) {
      lockBtn.addEventListener("click", async () => {
        const wal = typeof global.DogeGoLastWallet === "function" ? global.DogeGoLastWallet() : null;
        const walletEnc = !!(wal && wal.enabled !== false && wal.encrypted === true);
        if (!walletEnc) return;
        const walletLocked = wal.unlocked === false;
        if (walletLocked && window.DogeGoWalletPassphrase && window.DogeGoWalletPassphrase.promptUnlock) {
          const ok = await window.DogeGoWalletPassphrase.promptUnlock({
            wallet: wal,
            message: "Your wallet file is encrypted. Enter the passphrase to load spend keys (Core walletpassphrase).",
          });
          if (ok && typeof global.DogeGoRefresh === "function") await global.DogeGoRefresh();
          if (typeof global.DogeGoSyncTopbarLock === "function") global.DogeGoSyncTopbarLock();
          return;
        }
        if (walletEnc && wal.unlocked !== false && window.DogeGoWalletPassphrase && window.DogeGoWalletPassphrase.lock) {
          try {
            await window.DogeGoWalletPassphrase.lock();
            if (typeof global.DogeGoRefresh === "function") await global.DogeGoRefresh();
          } catch (e) {
            alert(e.message || String(e));
          }
          if (typeof global.DogeGoSyncTopbarLock === "function") global.DogeGoSyncTopbarLock();
        }
      });
    }
    const navDashLock = document.getElementById("nav-dashboard-lock");
    if (navDashLock) {
      navDashLock.addEventListener("click", async () => {
        if (!status.pin_enabled) return;
        if (shouldShowLockScreen()) {
          clearPad();
          showOverlay(lockOverlayMessage());
          return;
        }
        try {
          await lockNow();
        } catch (_) { /* overlay shows error */ }
      });
    }
    ["st-sec-new", "st-sec-confirm", "st-sec-current", "st-sec-disable-pin"].forEach((id) => {
      const el = document.getElementById(id);
      if (!el) return;
      el.addEventListener("input", () => {
        const d = el.value.replace(/\D/g, "").slice(0, 6);
        if (el.value !== d) el.value = d;
      });
    });
    document.getElementById("st-sec-save")?.addEventListener("click", async () => {
      const cur = (document.getElementById("st-sec-current")?.value || "").trim();
      const a = (document.getElementById("st-sec-new")?.value || "").trim();
      const b = (document.getElementById("st-sec-confirm")?.value || "").trim();
      const msg = document.getElementById("st-sec-msg");
      if (!PIN6.test(a)) {
        if (msg) { msg.textContent = "PIN must be exactly 6 numbers"; msg.className = "alert err show"; }
        return;
      }
      if (a !== b) {
        if (msg) { msg.textContent = "PINs do not match"; msg.className = "alert err show"; }
        return;
      }
      try {
        await setupPIN(cur, a);
        if (msg) { msg.textContent = "PIN saved ... wallet actions unlocked for 30 minutes."; msg.className = "alert ok show"; }
      } catch (e) {
        if (msg) { msg.textContent = e.message; msg.className = "alert err show"; }
      }
    });
    document.getElementById("st-sec-lock")?.addEventListener("click", () => {
      lockNow().catch(() => { /* err surfaced in overlay */ });
    });
    document.getElementById("st-sec-disable")?.addEventListener("click", async () => {
      const pin = document.getElementById("st-sec-disable-pin")?.value || "";
      try {
        const r = await fetch("/api/security/disable", {
          method: "POST",
          credentials: "same-origin",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ pin }),
        });
        const data = await r.json().catch(() => ({}));
        if (!r.ok) throw new Error(data.error || "Failed");
        localStorage.removeItem("dogego_web_ui_pin_enabled");
        localStorage.removeItem(LS_APPLY_PIN);
        localStorage.removeItem("dogego_pending_pin");
        userWantsLocked = false;
        await refreshUI();
      } catch (e) {
        alert(e.message);
      }
    });
    document.getElementById("st-sec-biometric")?.addEventListener("change", async (e) => {
      const on = e.target.checked;
      if (!on) {
        localStorage.removeItem(LS_BIO);
        try {
          await fetch("/api/security/webauthn/clear", { method: "POST", credentials: "same-origin" });
        } catch (_) { /* */ }
        await refreshUI();
        return;
      }
      try {
        await registerWebAuthn();
        const bioBtn = document.getElementById("sec-biometric-btn");
        if (bioBtn) bioBtn.hidden = false;
      } catch (err) {
        e.target.checked = false;
        localStorage.removeItem(LS_BIO);
        alert(err.message || String(err));
      }
    });
  }

  async function applyPendingPinFromSetup() {
    if (localStorage.getItem(LS_APPLY_PIN) !== "1") {
      localStorage.removeItem("dogego_pending_pin");
      return;
    }
    const pending = localStorage.getItem("dogego_pending_pin");
    localStorage.removeItem(LS_APPLY_PIN);
    localStorage.removeItem("dogego_pending_pin");
    if (!pending || !PIN6.test(pending)) return;
    try {
      await setupPIN("", pending);
    } catch (_) { /* datadir may not be ready yet; user can set PIN in Settings */ }
  }

  global.DogeGoSecurity = {
    init: async () => {
      clearStalePinStorage();
      bindUI();
      document.addEventListener("dogego:boot-ready", () => {
        try { flushBootDeferredOverlay(); } catch (_) { /* */ }
      });
      await refreshUI();
      await applyPendingPinFromSetup();
      await refreshUI();
      if (localStorage.getItem(LS_BIO) === "1" || status.webauthn_enabled) {
        const cb = document.getElementById("st-sec-biometric");
        if (cb) cb.checked = status.webauthn_enabled;
      }
    },
    refresh: refreshUI,
    lock: lockNow,
    needsUnlock: () => shouldShowLockScreen(),
    guardFetch,
    noteWalletAPIUnauthorized,
    onUnlock: (fn) => { onUnlock.push(fn); },
    setupPIN,
    status: () => status,
  };
})(window);
