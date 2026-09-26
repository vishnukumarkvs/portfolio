/* portfolio — progressive enhancement only.
   The site is fully usable with JavaScript disabled: the project "Details"
   link is an ordinary href to a real page. htmx upgrades it to an in-place
   expand, and all that is left here is the theme toggle. */
(function () {
  "use strict";

  var root = document.documentElement;

  /* Theme. The choice is already applied before first paint by an inline
     script in the document head; this only handles the toggle itself. */
  var toggle = document.querySelector(".theme-toggle");
  if (toggle) {
    toggle.addEventListener("click", function () {
      var next = root.getAttribute("data-theme") === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("theme", next); } catch (e) {}
    });
  }

  /* Collapse an expanded project. htmx fills the container; emptying it puts
     the summary back, and because the summary was never replaced there is
     nothing to restore and no second request to make. */
  document.addEventListener("click", function (e) {
    var close = e.target.closest("[data-close-project]");
    if (!close) return;

    e.preventDefault();
    var box = document.getElementById("more-" + close.getAttribute("data-close-project"));
    if (box) box.innerHTML = "";
  });
})();
