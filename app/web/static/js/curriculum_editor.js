(function () {
  "use strict";

  document.addEventListener("panel:configure", function (event) {
    const panel = event.detail.panel;
    const trigger = event.detail.trigger;
    if (!trigger.dataset.dependentId) return;
    const picker = panel.querySelector("[data-unit-picker]");
    const dependentInput = panel.querySelector("[data-dependent-input]");
    const dependentName = panel.querySelector("[data-dependent-name]");
    const removeDependentInput = panel.querySelector("[data-remove-dependent-input]");
    if (dependentInput) dependentInput.value = trigger.dataset.dependentId;
    if (removeDependentInput) removeDependentInput.value = trigger.dataset.dependentId;
    if (dependentName) dependentName.textContent = trigger.dataset.dependentName;
    panel.dataset.panelBreadcrumb = trigger.dataset.dependentName || "Dependencies";
    if (picker && picker.unitPicker) {
      picker.unitPicker.configure({
        selectedIDs: (trigger.dataset.prerequisiteIds || "").split(",").filter(Boolean),
        excludedIDs: [trigger.dataset.dependentId]
      });
    }
  });

  document.addEventListener("unit-picker:add", function (event) {
    const picker = event.target.closest("[data-dependency-unit-picker]");
    if (!picker) return;
    event.preventDefault();
    const form = picker.querySelector("form");
    const input = form && form.querySelector("[data-prerequisite-input]");
    if (input) input.value = event.detail.id;
    if (form) form.requestSubmit();
  });

  document.addEventListener("unit-picker:remove", function (event) {
    const picker = event.target.closest("[data-dependency-unit-picker]");
    if (!picker) return;
    event.preventDefault();
    const panel = picker.closest("[data-nested-panel]");
    const form = panel && panel.querySelector("[data-remove-dependency-form]");
    const input = form && form.querySelector("[data-remove-prerequisite-input]");
    if (input) input.value = event.detail.id;
    if (form) form.requestSubmit();
  });

  document.addEventListener("input", function (event) {
    const field = event.target;
    const form = field.closest && field.closest("[data-proposal-metadata]");
    if (!form) return;
    const proposalID = form.dataset.proposalMetadata;
    if (field.matches("[data-proposal-title-input]")) {
      const title = field.value.trim();
      if (!title) return;
      document.querySelectorAll('[data-proposal-title-preview="' + CSS.escape(proposalID) + '"]').forEach(function (preview) {
        preview.textContent = title;
      });
      const panel = form.closest("[data-panel-breadcrumb]");
      const breadcrumbTitle = form.querySelector(".proposal-workspace__breadcrumb-title");
      const breadcrumb = "Working on " + title;
      if (panel) panel.dataset.panelBreadcrumb = breadcrumb;
      if (breadcrumbTitle) breadcrumbTitle.textContent = breadcrumb;
      const workspace = panel && panel.parentElement;
      const trail = workspace && workspace.querySelector(":scope > [data-mobile-panel-breadcrumbs]");
      if (trail) {
        const panels = Array.from(workspace.children).filter(function (candidate) {
          return candidate.matches("[data-layout-panel][data-panel-breadcrumb]") && !candidate.hidden;
        });
        const index = panels.indexOf(panel);
        if (index >= 0 && trail.children[index]) trail.children[index].textContent = breadcrumb;
      }
    }
    if (field.matches("[data-proposal-rationale-input]")) {
      document.querySelectorAll('[data-proposal-rationale-preview="' + CSS.escape(proposalID) + '"]').forEach(function (preview) {
        preview.textContent = field.value.trim();
      });
    }
  });

  document.addEventListener("DOMContentLoaded", function () {
    const dependencyID = new URL(window.location.href).searchParams.get("edit_dependencies");
    if (!dependencyID) return;
    const trigger = document.querySelector('[data-open-panel="edit-dependencies-panel"][data-dependent-id="' +
      CSS.escape(dependencyID) + '"]');
    if (trigger) trigger.click();
  });
})();
