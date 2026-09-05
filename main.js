/* ==========================================================================
   wrkflw — landing page interactions
   ========================================================================== */

(function () {
  "use strict";

  /* Mobile menu ---------------------------------------------------------- */

  var body = document.body;
  var burger = document.getElementById("burger");
  var overlay = document.getElementById("menu-overlay");
  var menu = document.getElementById("mobile-menu");
  if (burger && overlay && menu) {
  var menuLinks = menu.querySelectorAll("a");

  function openMenu() {
    body.classList.add("menu-open");
    burger.setAttribute("aria-expanded", "true");
    overlay.hidden = false;
    menu.hidden = false;
  }

  function closeMenu() {
    body.classList.remove("menu-open");
    burger.setAttribute("aria-expanded", "false");
    overlay.hidden = true;
    menu.hidden = true;
  }

  burger.addEventListener("click", function () {
    if (body.classList.contains("menu-open")) {
      closeMenu();
    } else {
      openMenu();
    }
  });

  overlay.addEventListener("click", closeMenu);

  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      closeMenu();
    }
  });

  menuLinks.forEach(function (link) {
    link.addEventListener("click", closeMenu);
  });

  window.addEventListener("resize", function () {
    if (window.innerWidth > 720) {
      closeMenu();
    }
  });
  }

  /* Stats count-up -------------------------------------------------------- */

  var statValues = Array.prototype.slice.call(
    document.querySelectorAll(".stat-value")
  );

  var easeOutCubic = function (t) {
    return 1 - Math.pow(1 - t, 3);
  };

  var format = function (value, decimals, suffix) {
    return value.toFixed(decimals) + suffix;
  };

  function runCountUp(el, duration, delay) {
    setTimeout(function () {
      var target = parseFloat(el.getAttribute("data-target"));
      var suffix = el.getAttribute("data-suffix") || "";
      var decimals = parseInt(el.getAttribute("data-decimals") || "0", 10);
      var start = null;

      function step(now) {
        if (start === null) {
          start = now;
        }
        var progress = Math.min((now - start) / duration, 1);
        var value = target * easeOutCubic(progress);
        el.textContent = format(value, decimals, suffix);
        if (progress < 1) {
          requestAnimationFrame(step);
        } else {
          el.textContent = format(target, decimals, suffix);
        }
      }

      requestAnimationFrame(step);
    }, delay);
  }

  function setFinal(el) {
    var target = parseFloat(el.getAttribute("data-target"));
    var suffix = el.getAttribute("data-suffix") || "";
    var decimals = parseInt(el.getAttribute("data-decimals") || "0", 10);
    el.textContent = format(target, decimals, suffix);
  }

  var reduced =
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  if (reduced) {
    statValues.forEach(setFinal);
  } else {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            var index = statValues.indexOf(entry.target);
            var duration = 1500 + index * 80;
            var delay = 480 + index * 90;
            runCountUp(entry.target, duration, delay);
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.25 }
    );

    statValues.forEach(function (el) {
      observer.observe(el);
    });
  }
})();