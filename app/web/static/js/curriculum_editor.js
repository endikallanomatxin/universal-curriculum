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

  document.addEventListener("DOMContentLoaded", function () {
    const dependencyID = new URL(window.location.href).searchParams.get("edit_dependencies");
    if (!dependencyID) return;
    const trigger = document.querySelector('[data-open-panel="edit-dependencies-panel"][data-dependent-id="' +
      CSS.escape(dependencyID) + '"]');
    if (trigger) trigger.click();
  });
})();
