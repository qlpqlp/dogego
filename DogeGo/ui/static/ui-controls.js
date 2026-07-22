/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */
/* Modern choice cards + toggle helpers (dashboard + setup wizard). */
(function (global) {
  var delegateBound = false;

  function syncChoiceCardsFor(target) {
    if (!target || !target.id) return;
    document.querySelectorAll('.choice-cards[data-target="' + target.id + '"]').forEach(function (grid) {
      var v = String(target.value || "");
      grid.querySelectorAll(".choice-card").forEach(function (c) {
        var on = c.getAttribute("data-value") === v;
        c.classList.toggle("selected", on);
        c.setAttribute("aria-selected", on ? "true" : "false");
      });
    });
  }

  function activateChoiceCard(card) {
    if (!card) return;
    var grid = card.closest(".choice-cards[data-target]");
    if (!grid) return;
    var targetId = grid.getAttribute("data-target");
    var target = document.getElementById(targetId);
    if (!target) return;
    var v = card.getAttribute("data-value");
    if (v == null) return;
    target.value = v;
    syncChoiceCardsFor(target);
    target.dispatchEvent(new Event("change", { bubbles: true }));
  }

  function bindChoiceCardDelegation() {
    if (delegateBound) return;
    delegateBound = true;
    document.addEventListener("click", function (e) {
      var card = e.target.closest && e.target.closest(".choice-cards[data-target] .choice-card");
      if (!card) return;
      e.preventDefault();
      activateChoiceCard(card);
    });
    document.addEventListener("keydown", function (e) {
      if (e.key !== "Enter" && e.key !== " ") return;
      var card = e.target.closest && e.target.closest(".choice-cards[data-target] .choice-card");
      if (!card) return;
      e.preventDefault();
      activateChoiceCard(card);
    });
  }

  function bindChoiceCards() {
    bindChoiceCardDelegation();
    document.querySelectorAll(".choice-cards[data-target]").forEach(function (grid) {
      var targetId = grid.getAttribute("data-target");
      var target = document.getElementById(targetId);
      if (target) syncChoiceCardsFor(target);
    });
  }

  function enhanceToggles(root) {
    (root || document).querySelectorAll('input[type="checkbox"]').forEach(function (el) {
      if (el.getAttribute("role") !== "switch") el.setAttribute("role", "switch");
    });
    (root || document).querySelectorAll('.seg-control input[type="radio"]').forEach(function (el) {
      if (!el.id && el.name) {
        el.id = el.name + "-" + el.value;
      }
    });
  }

  function init(root) {
    bindChoiceCards();
    enhanceToggles(root);
  }

  global.DogeGoControls = {
    init: init,
    bindChoiceCards: bindChoiceCards,
    syncChoiceCardsFor: syncChoiceCardsFor,
    syncChoiceCard: function (id) {
      var t = document.getElementById(id);
      if (t) syncChoiceCardsFor(t);
    },
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { init(); });
  } else {
    init();
  }
})(window);
