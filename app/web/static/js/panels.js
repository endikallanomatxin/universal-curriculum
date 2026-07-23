(function () {
  "use strict";

  function directChildContaining(group, element) {
    let current = element;
    while (current && current.parentElement !== group) current = current.parentElement;
    return current && current.parentElement === group ? current : null;
  }

  function closePanelsToTheRight(trigger, targetPanel) {
    const group = targetPanel.parentElement;
    const origin = directChildContaining(group, trigger);
    if (!group || !origin) return;
    const children = Array.from(group.children);
    const originIndex = children.indexOf(origin);
    children.forEach(function (panel, index) {
      if (index <= originIndex || panel === targetPanel ||
          !panel.matches("[data-nested-panel]") || panel.hidden) return;
      window.clearTimeout(panel.closeTimer);
      panel.classList.remove("is-opening");
      panel.classList.add("is-closing");
      const panelTrigger = panel.activeTrigger;
      if (panelTrigger) panelTrigger.setAttribute("aria-expanded", "false");
      panel.closeTimer = window.setTimeout(function () {
        panel.hidden = true;
        panel.classList.remove("is-closing");
      }, 280);
    });
  }

  function initializePanels(root) {
    root.querySelectorAll("[data-open-panel]").forEach(function (trigger) {
      if (trigger.dataset.panelInitialized === "true") return;
      trigger.dataset.panelInitialized = "true";
      trigger.addEventListener("click", function () {
        const panel = document.getElementById(trigger.dataset.openPanel);
        if (!panel) return;
        closePanelsToTheRight(trigger, panel);
        window.clearTimeout(panel.closeTimer);
        panel.classList.remove("is-closing");
        panel.activeTrigger = trigger;
        if (trigger.dataset.dependentId) {
          const input = panel.querySelector("[data-dependent-input]");
          const name = panel.querySelector("[data-dependent-name]");
          const existingPrerequisites = trigger.dataset.prerequisiteIds.split(",");
          const prerequisiteInput = panel.querySelector("[data-prerequisite-input]");
          const searchInput = panel.querySelector("[data-unit-search-input]");
          const current = panel.querySelector("[data-current-dependencies]");
          const removeForm = panel.querySelector("[data-remove-dependency-form]");
          const removeDependentInput = panel.querySelector("[data-remove-dependent-input]");
          const removePrerequisiteInput = panel.querySelector("[data-remove-prerequisite-input]");
          if (input) input.value = trigger.dataset.dependentId;
          if (name) name.textContent = trigger.dataset.dependentName;
          if (prerequisiteInput) prerequisiteInput.value = "";
          if (searchInput) searchInput.value = "";
          if (removeDependentInput) removeDependentInput.value = trigger.dataset.dependentId;
          if (current) current.replaceChildren();
          panel.querySelectorAll("[data-unit-search-option]").forEach(function (option) {
            const isCurrent = existingPrerequisites.includes(option.dataset.unitId);
            option.hidden = option.dataset.unitId === trigger.dataset.dependentId || isCurrent;
            if (isCurrent && current) {
              const row = document.createElement("div");
              row.className = "dependency-editor__row";
              const copy = document.createElement("div");
              const title = document.createElement("strong");
              const description = document.createElement("span");
              const remove = document.createElement("button");
              title.textContent = option.dataset.unitName;
              description.textContent = option.dataset.unitDescription;
              remove.type = "button";
              remove.className = "editor-action";
              remove.textContent = "Remove";
              remove.addEventListener("click", function () {
                if (removePrerequisiteInput) removePrerequisiteInput.value = option.dataset.unitId;
                if (removeForm) removeForm.requestSubmit();
              });
              copy.append(title, description);
              row.append(copy, remove);
              current.append(row);
            }
          });
          if (current && !current.children.length) {
            const empty = document.createElement("p");
            empty.className = "proposal-history__empty";
            empty.textContent = "No prerequisites selected.";
            current.append(empty);
          }
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

  function initializeUnitSearch(root) {
    root.querySelectorAll("[data-unit-search]").forEach(function (search) {
      if (search.dataset.searchInitialized === "true") return;
      search.dataset.searchInitialized = "true";
      const input = search.querySelector("[data-unit-search-input]");
      const results = search.querySelector("[data-unit-search-results]");
      const empty = search.querySelector("[data-unit-search-empty]");
      const hiddenInput = search.closest("form").querySelector("[data-prerequisite-input]");
      const options = Array.from(search.querySelectorAll("[data-unit-search-option]"));
      if (!input || !results || !hiddenInput) return;

      function filterOptions() {
        const query = input.value.trim().toLocaleLowerCase();
        let matches = 0;
        options.forEach(function (option) {
          const unavailable = option.hidden;
          const matchesQuery = option.dataset.unitName.toLocaleLowerCase().includes(query);
          option.classList.toggle("is-filtered", unavailable || !matchesQuery);
          if (!unavailable && matchesQuery) matches += 1;
        });
        if (empty) empty.hidden = matches > 0;
        results.hidden = false;
        input.setAttribute("aria-expanded", "true");
      }

      input.addEventListener("focus", filterOptions);
      input.addEventListener("input", filterOptions);
      options.forEach(function (option) {
        option.addEventListener("click", function () {
          hiddenInput.value = option.dataset.unitId;
          results.hidden = true;
          input.setAttribute("aria-expanded", "false");
          search.closest("form").requestSubmit();
        });
      });
      search.closest("form").addEventListener("submit", function (event) {
        if (hiddenInput.value) return;
        event.preventDefault();
        input.setCustomValidity("Select a unit from the search results.");
        input.reportValidity();
        input.setCustomValidity("");
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    initializePanels(document);
    initializeUnitSearch(document);
    const dependencyID = new URL(window.location.href).searchParams.get("edit_dependencies");
    if (dependencyID) {
      const trigger = document.querySelector('[data-open-panel="edit-dependencies-panel"][data-dependent-id="' +
        CSS.escape(dependencyID) + '"]');
      if (trigger) trigger.click();
    }
  });
  document.addEventListener("htmx:load", function (event) {
    const root = event.detail.elt || document;
    initializePanels(root);
    initializeUnitSearch(root);
  });
})();
