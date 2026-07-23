(function () {
  "use strict";

  function initializePanels(root) {
    root.querySelectorAll("[data-open-panel]").forEach(function (trigger) {
      if (trigger.dataset.panelInitialized === "true") return;
      trigger.dataset.panelInitialized = "true";
      trigger.addEventListener("click", function () {
        const panel = document.getElementById(trigger.dataset.openPanel);
        if (!panel) return;
        window.clearTimeout(panel.closeTimer);
        panel.classList.remove("is-closing");
        panel.activeTrigger = trigger;
        if (trigger.dataset.dependentId) {
          const input = panel.querySelector("[data-dependent-input]");
          const name = panel.querySelector("[data-dependent-name]");
          const existingPrerequisites = trigger.dataset.prerequisiteIds.split(",");
          if (input) input.value = trigger.dataset.dependentId;
          if (name) name.textContent = trigger.dataset.dependentName;
          panel.querySelectorAll('select[name="prerequisite_id"] option').forEach(function (option) {
            option.disabled = option.value === trigger.dataset.dependentId || existingPrerequisites.includes(option.value);
          });
        }
        if (trigger.dataset.unitId) {
          const form = panel.querySelector("[data-unit-edit-form]");
          const heading = panel.querySelector("[data-unit-heading]");
          const name = panel.querySelector("[data-unit-name-input]");
          const description = panel.querySelector("[data-unit-description-input]");
          if (form) form.action = "/admin/curriculum/units/" + encodeURIComponent(trigger.dataset.unitId);
          if (heading) heading.textContent = trigger.dataset.unitName;
          if (name) name.value = trigger.dataset.unitName;
          if (description) description.value = trigger.dataset.unitDescription;
        }
        panel.classList.add("is-opening");
        panel.hidden = false;
        trigger.setAttribute("aria-expanded", "true");
        if (window.panelLayout) window.panelLayout.refresh();
        panel.getBoundingClientRect();
        window.requestAnimationFrame(function () {
          panel.classList.remove("is-opening");
          if (window.autoResizeTextareas) window.autoResizeTextareas(panel);
          const firstField = panel.querySelector("form input:not([type=hidden]), form select, form textarea, form button");
          if (firstField) firstField.focus({ preventScroll: true });
          window.setTimeout(function () {
            panel.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "end" });
          }, 260);
        });
      });
    });

    root.querySelectorAll("[data-nested-panel]").forEach(function (panel) {
      const close = panel.querySelector("[data-close-panel]");
      if (!close || close.dataset.panelInitialized === "true") return;
      close.dataset.panelInitialized = "true";
      close.addEventListener("click", function () {
        const trigger = panel.activeTrigger || document.querySelector('[data-open-panel="' + panel.id + '"]');
        panel.classList.add("is-closing");
        if (window.panelLayout) window.panelLayout.refresh();
        if (trigger) {
          trigger.setAttribute("aria-expanded", "false");
        }
        panel.closeTimer = window.setTimeout(function () {
          panel.hidden = true;
          panel.classList.remove("is-closing");
          if (trigger) {
            trigger.scrollIntoView({ behavior: "auto", block: "nearest", inline: "end" });
            trigger.focus({ preventScroll: true });
          }
        }, 280);
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () { initializePanels(document); });
  document.addEventListener("htmx:load", function (event) { initializePanels(event.detail.elt || document); });
})();
