/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Wallet file passphrase unlock (encryptwallet / walletpassphrase); separate from dashboard PIN. */
(function (global) {
  const DEFAULT_TIMEOUT = 600;
  let pendingResolve = null;

  function $(id) {
    return document.getElementById(id);
  }

  function hideUnlockErr(errEl) {
    if (!errEl) return;
    errEl.textContent = "";
    errEl.hidden = true;
    errEl.classList.remove("show");
  }

  function showUnlockErr(errEl, msg) {
    if (!errEl) return;
    errEl.textContent = msg;
    errEl.hidden = false;
    errEl.classList.add("show");
  }

  function closeModal(ok) {
    const modal = $("wallet-unlock-modal");
    if (modal) modal.hidden = true;
    const pass = $("wallet-unlock-pass");
    if (pass) pass.value = "";
    hideUnlockErr($("wallet-unlock-err"));
    const fn = pendingResolve;
    pendingResolve = null;
    if (fn) fn(!!ok);
  }

  async function unlockRequest(passphrase, timeout) {
    const r = await fetch("/api/wallet/unlock", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      cache: "no-store",
      body: JSON.stringify({ passphrase: passphrase, timeout: timeout }),
    });
    const body = await r.json().catch(() => ({}));
    if (!r.ok) {
      const err = new Error(body.error || ("HTTP " + r.status));
      err.walletLocked = body.wallet_locked === true;
      throw err;
    }
    return body;
  }

  async function lockWallet() {
    const r = await fetch("/api/wallet/lock", {
      method: "POST",
      credentials: "same-origin",
      cache: "no-store",
    });
    const body = await r.json().catch(() => ({}));
    if (!r.ok) {
      throw new Error(body.error || ("HTTP " + r.status));
    }
    return body;
  }

  function bindUI() {
    $("wallet-unlock-cancel") && $("wallet-unlock-cancel").addEventListener("click", () => closeModal(false));
    $("wallet-unlock-backdrop") && $("wallet-unlock-backdrop").addEventListener("click", () => closeModal(false));
    $("wallet-unlock-go") && $("wallet-unlock-go").addEventListener("click", async () => {
      const passEl = $("wallet-unlock-pass");
      const errEl = $("wallet-unlock-err");
      const btn = $("wallet-unlock-go");
      const pass = passEl ? passEl.value : "";
      if (!pass) {
        showUnlockErr(errEl, "Enter your wallet passphrase.");
        return;
      }
      const timeoutEl = $("wallet-unlock-timeout");
      const timeout = timeoutEl ? parseInt(timeoutEl.value, 10) : DEFAULT_TIMEOUT;
      if (btn) btn.disabled = true;
      try {
        await unlockRequest(pass, isFinite(timeout) && timeout >= 0 ? timeout : DEFAULT_TIMEOUT);
        closeModal(true);
      } catch (e) {
        showUnlockErr(errEl, e.message || String(e));
      } finally {
        if (btn) btn.disabled = false;
      }
    });
    $("wallet-unlock-pass") && $("wallet-unlock-pass").addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        $("wallet-unlock-go") && $("wallet-unlock-go").click();
      }
    });
  }

  function promptUnlock(opts) {
    opts = opts || {};
    if (pendingResolve) {
      return Promise.resolve(false);
    }
    const modal = $("wallet-unlock-modal");
    const msg = $("wallet-unlock-msg");
    const pass = $("wallet-unlock-pass");
    if (!modal) return Promise.resolve(false);
    if (msg) {
      msg.textContent = opts.message ||
        "Your wallet file is encrypted. Enter the passphrase to load spend keys (same as walletpassphrase in Core).";
    }
    if (pass) {
      pass.value = "";
      setTimeout(() => pass.focus(), 50);
    }
    hideUnlockErr($("wallet-unlock-err"));
    modal.hidden = false;
    return new Promise((resolve) => {
      pendingResolve = resolve;
    });
  }

  async function ensureUnlocked(opts) {
    const wal = opts && opts.wallet;
    if (!wal || !wal.encrypted || wal.unlocked !== false) return true;
    return promptUnlock(opts);
  }

  global.DogeGoWalletPassphrase = {
    bindUI,
    promptUnlock,
    ensureUnlocked,
    unlock: unlockRequest,
    lock: lockWallet,
    defaultTimeout: DEFAULT_TIMEOUT,
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bindUI);
  } else {
    bindUI();
  }
})(typeof window !== "undefined" ? window : globalThis);
