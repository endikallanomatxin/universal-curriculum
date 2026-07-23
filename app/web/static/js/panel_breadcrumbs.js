(function () {
  "use strict";

  function directBreadcrumbPanels(workspace) {
    return Array.from(workspace.children).filter(function (panel) {
      return panel.matches("[data-layout-panel][data-panel-breadcrumb]") &&
        !panel.hidden &&
        !panel.classList.contains("is-closing");
    });
  }

  function breadcrumbItems(workspace) {
    return directBreadcrumbPanels(workspace).map(function (panel) {
      return {
        label: (panel.dataset.panelBreadcrumb || "").trim(),
        panel: panel
      };
    }).filter(function (item) {
      return item.label;
    });
  }

  function renderBreadcrumbs(shell, mobile) {
    const workspace = shell.querySelector("#workspace");
    if (!workspace) return;
    const trail = workspace.querySelector(":scope > [data-mobile-panel-breadcrumbs]");
    if (!trail) return;
    if (!mobile) {
      if (!trail.hidden) trail.hidden = true;
      return;
    }

    const items = breadcrumbItems(workspace);
    const signature = items.map(function (item) { return item.label; }).join("\u001f");
    if (trail.breadcrumbSignature === signature) {
      const shouldHide = items.length === 0;
      if (trail.hidden !== shouldHide) trail.hidden = shouldHide;
      return;
    }

    trail.breadcrumbSignature = signature;
    trail.replaceChildren();
    items.forEach(function (item, index) {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = item.label;
      if (index === items.length - 1) button.setAttribute("aria-current", "page");
      button.addEventListener("click", function () {
        document.dispatchEvent(new CustomEvent("panel:navigate", {
          detail: { panel: item.panel }
        }));
      });
      trail.appendChild(button);
    });
    trail.hidden = items.length === 0;
  }

  document.addEventListener("panel-layout:complete", function (event) {
    renderBreadcrumbs(event.detail.shell, event.detail.mobile);
  });
})();
