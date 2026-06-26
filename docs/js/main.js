(function () {
  "use strict";

  var toggle = document.querySelector(".nav-toggle");
  var nav = document.getElementById("site-nav");
  var headerOffset = 72;

  function closeMobileNav() {
    if (!nav || !nav.classList.contains("is-open")) return;
    nav.classList.remove("is-open");
    if (toggle) {
      toggle.setAttribute("aria-expanded", "false");
      toggle.setAttribute("aria-label", "Open menu");
      var icon = toggle.querySelector(".material-icons-round");
      if (icon) icon.textContent = "menu";
    }
  }

  if (toggle && nav) {
    toggle.addEventListener("click", function () {
      var open = nav.classList.toggle("is-open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
      toggle.setAttribute("aria-label", open ? "Close menu" : "Open menu");
      var icon = toggle.querySelector(".material-icons-round");
      if (icon) icon.textContent = open ? "close" : "menu";
    });
  }

  /* Smooth anchor navigation with sticky-header offset */
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
        } catch (err) {
          /* file:// and other origins may reject pushState */
        }
      }
    });
  });

  /* Coming soon modal */
  var modal = document.getElementById("soon-modal");
  var modalTitle = document.getElementById("soon-modal-title");
  var modalDesc = document.getElementById("soon-modal-desc");

  var soonCopy = {
    download: {
      title: "Downloads arriving soon",
      desc: "Pre-built binaries for Windows, macOS, Linux, and BSD are being packaged now. The v0.1.0-beta release lands on GitHub in a few days."
    },
    github: {
      title: "Source code going public soon",
      desc: "DogeGo will be open source at github.com/qlpqlp/dogego. The repository and release artifacts publish with the beta in a few days."
    },
    build: {
      title: "Build instructions coming with the repo",
      desc: "Once the repository is public you can clone it and run go build ./cmd/dogego. Hang tight, the beta drops in a few days."
    }
  };

  function openSoon(kind) {
    if (!modal) return;
    var copy = soonCopy[kind] || soonCopy.github;
    modalTitle.textContent = copy.title;
    modalDesc.textContent = copy.desc;
    modal.hidden = false;
    modal.classList.add("is-visible");
    document.body.classList.add("modal-open");
    var closeBtn = modal.querySelector(".soon-close");
    if (closeBtn) closeBtn.focus();
  }

  function closeSoon() {
    if (!modal) return;
    modal.classList.remove("is-visible");
    document.body.classList.remove("modal-open");
    window.setTimeout(function () {
      modal.hidden = true;
    }, 280);
  }

  document.querySelectorAll("[data-soon]").forEach(function (el) {
    el.addEventListener("click", function (e) {
      e.preventDefault();
      openSoon(el.getAttribute("data-soon") || "github");
    });
  });

  if (modal) {
    modal.querySelector(".soon-backdrop").addEventListener("click", closeSoon);
    modal.querySelector(".soon-close").addEventListener("click", closeSoon);
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && modal.classList.contains("is-visible")) closeSoon();
    });
  }

  /* Section nav highlight */
  var sections = document.querySelectorAll("section[id]");
  var navLinks = document.querySelectorAll('.site-nav a[href^="#"], .footer-links a[href^="#"]');

  if (sections.length && navLinks.length && "IntersectionObserver" in window) {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (!entry.isIntersecting) return;
          var id = entry.target.getAttribute("id");
          navLinks.forEach(function (link) {
            link.classList.toggle("is-active", link.getAttribute("href") === "#" + id);
          });
        });
      },
      { rootMargin: "-40% 0px -50% 0px", threshold: 0 }
    );
    sections.forEach(function (section) { observer.observe(section); });
  }
})();
