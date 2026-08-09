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
        url: (panel.dataset.panelBreadcrumbUrl || "").trim(),
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
    const signature = items.map(function (item) {
      return item.label + "\u001e" + item.url;
    }).join("\u001f");
    if (trail.breadcrumbSignature === signature) {
      const shouldHide = items.length === 0;
      if (trail.hidden !== shouldHide) trail.hidden = shouldHide;
      return;
    }

    trail.breadcrumbSignature = signature;
    trail.replaceChildren();
    items.forEach(function (item, index) {
      const current = index === items.length - 1;
      const control = !current && item.url
        ? document.createElement("a")
        : document.createElement("button");
      control.textContent = item.label;
      if (control.tagName === "A") {
        control.href = item.url;
        control.setAttribute("hx-get", item.url);
        control.setAttribute("hx-target", "#workspace");
        control.setAttribute("hx-select", "#workspace");
        control.setAttribute("hx-swap", "outerHTML transition:true");
        control.setAttribute("hx-push-url", "true");
        control.dataset.panelNavigation = "replace";
      } else {
        control.type = "button";
        if (!current) {
          control.addEventListener("click", function () {
            document.dispatchEvent(new CustomEvent("panel:navigate", {
              detail: { panel: item.panel }
            }));
          });
        }
      }
      if (current) control.setAttribute("aria-current", "page");
      trail.appendChild(control);
      if (control.tagName === "A") htmx.process(control);
    });
    trail.hidden = items.length === 0;
  }

  document.addEventListener("panel-layout:complete", function (event) {
    renderBreadcrumbs(event.detail.shell, event.detail.mobile);
  });
})();
