(function () {
  "use strict";

  function setMobileMenu(open) {
    const navigation = document.querySelector(".primary-navigation");
    const toggle = navigation && navigation.querySelector("[data-mobile-menu-toggle]");
    if (!navigation || !toggle) return;
    navigation.classList.toggle("is-mobile-menu-open", open);
    toggle.setAttribute("aria-expanded", String(open));
    toggle.setAttribute("aria-label", open ? "Close menu" : "Open menu");
  }

  function initializeMobileMenu() {
    const navigation = document.querySelector(".primary-navigation");
    const toggle = navigation && navigation.querySelector("[data-mobile-menu-toggle]");
    if (!navigation || !toggle || toggle.menuInitialized) return;
    toggle.menuInitialized = true;
    toggle.addEventListener("click", function () {
      setMobileMenu(!navigation.classList.contains("is-mobile-menu-open"));
    });
    navigation.addEventListener("click", function (event) {
      if (event.target.closest("a[href]")) setMobileMenu(false);
    });
    if (!document.mobileMenuKeyboardInitialized) {
      document.mobileMenuKeyboardInitialized = true;
      document.addEventListener("keydown", function (event) {
        if (event.key === "Escape") setMobileMenu(false);
      });
    }
  }

  function targetsWorkspace(event) {
    const detail = event.detail || {};
    return [detail.target, detail.elt, event.target].some(function (element) {
      return element && element.id === "workspace";
    });
  }

  function synchroniseShell() {
    const shell = document.querySelector("#app-shell");
    const workspace = document.querySelector("#workspace");
    if (!shell || !workspace) return;

    shell.classList.toggle("app-shell--home", workspace.dataset.shellView === "home");
    setMobileMenu(false);
    if (workspace.dataset.documentTitle) document.title = workspace.dataset.documentTitle;

    shell.querySelectorAll(".primary-menu__link[href]").forEach(function (link) {
      const sectionPaths = (link.dataset.sectionPaths || new URL(link.href, window.location.href).pathname)
        .split(/\s+/)
        .filter(Boolean);
      const current = sectionPaths.some(function (sectionPath) {
        return window.location.pathname === sectionPath ||
          window.location.pathname.startsWith(sectionPath + "/");
      });
      if (current) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    if (window.panelLayout) window.panelLayout.refresh();
  }

  document.addEventListener("DOMContentLoaded", function () {
    initializeMobileMenu();
    synchroniseShell();
  });
  document.addEventListener("htmx:load", function () {
    initializeMobileMenu();
  });
  document.addEventListener("htmx:beforeSwap", function (event) {
    const shell = document.querySelector("#app-shell");
    if (shell && targetsWorkspace(event)) shell.classList.add("is-shell-navigation");
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.target && event.target.id === "workspace") synchroniseShell();
  });
  document.addEventListener("htmx:afterSettle", function (event) {
    const shell = document.querySelector("#app-shell");
    if (!shell || !targetsWorkspace(event)) return;
    if (window.panelLayout) window.panelLayout.refresh();
    window.requestAnimationFrame(function () {
      if (window.panelLayout) window.panelLayout.refresh();
      window.requestAnimationFrame(function () {
        if (window.panelLayout) window.panelLayout.refresh();
        shell.classList.remove("is-shell-navigation");
      });
    });
  });
  window.addEventListener("popstate", function () {
    window.setTimeout(function () {
      initializeMobileMenu();
      synchroniseShell();
    }, 0);
  });
  document.addEventListener("panel-layout", function () {
    const navigation = document.querySelector(".primary-navigation");
    if (navigation && navigation.dataset.panelMode !== "mobile") setMobileMenu(false);
  });
})();
