(function () {
  "use strict";

  function synchroniseShell() {
    const shell = document.querySelector("#app-shell");
    const workspace = document.querySelector("#workspace");
    if (!shell || !workspace) return;

    shell.classList.toggle("app-shell--home", workspace.dataset.shellView === "home");
    if (workspace.dataset.documentTitle) document.title = workspace.dataset.documentTitle;

    shell.querySelectorAll(".primary-menu__link[href]").forEach(function (link) {
      const current = new URL(link.href, window.location.href).pathname === window.location.pathname;
      if (current) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
  }

  document.addEventListener("DOMContentLoaded", synchroniseShell);
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.target && event.target.id === "workspace") synchroniseShell();
  });
  window.addEventListener("popstate", function () {
    window.setTimeout(synchroniseShell, 0);
  });
})();
