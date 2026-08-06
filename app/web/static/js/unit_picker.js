(function () {
  "use strict";

  function normalized(value) {
    return value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLocaleLowerCase();
  }

  function initializeUnitPicker(picker) {
    if (picker.unitPickerInitialized) return;
    picker.unitPickerInitialized = true;
    const input = picker.querySelector("[data-unit-picker-input]");
    const results = picker.querySelector("[data-unit-picker-results]");
    const empty = picker.querySelector("[data-unit-picker-empty]");
    const current = picker.querySelector("[data-unit-picker-current]");
    const count = picker.querySelector("[data-unit-picker-count]");
    const options = Array.from(picker.querySelectorAll("[data-unit-picker-option]"));
    let selectedIDs = new Set();
    let excludedIDs = new Set();
    if (!input || !results || !current) return;

    function closeResults() {
      results.hidden = true;
      input.setAttribute("aria-expanded", "false");
    }

    function filterOptions() {
      const query = normalized(input.value.trim());
      let matches = 0;
      options.forEach(function (option) {
        const unavailable = option.dataset.unitRetired === "true" ||
          selectedIDs.has(option.dataset.unitId) ||
          excludedIDs.has(option.dataset.unitId);
        const matchesQuery = normalized(option.dataset.unitName).includes(query);
        option.classList.toggle("is-filtered", unavailable || !matchesQuery);
        if (!unavailable && matchesQuery) matches += 1;
      });
      if (empty) empty.hidden = matches > 0;
      results.hidden = false;
      input.setAttribute("aria-expanded", "true");
    }

    function renderSelection() {
      current.replaceChildren();
      options.forEach(function (option) {
        if (!selectedIDs.has(option.dataset.unitId)) return;
        const row = document.createElement("div");
        const title = document.createElement("strong");
        const remove = document.createElement("button");
        row.className = "selection-list__row";
        title.textContent = option.dataset.unitName;
        remove.type = "button";
        remove.className = "secondary-button";
        remove.textContent = "Remove";
        remove.addEventListener("click", function () {
          if (selectedIDs.size === 1 && picker.dataset.unitPickerRequiredMessage) {
            input.setCustomValidity(picker.dataset.unitPickerRequiredMessage);
            input.reportValidity();
            input.setCustomValidity("");
            return;
          }
          const event = new CustomEvent("unit-picker:remove", {
            bubbles: true,
            cancelable: true,
            detail: { id: option.dataset.unitId, name: option.dataset.unitName }
          });
          if (!picker.dispatchEvent(event)) return;
          selectedIDs.delete(option.dataset.unitId);
          renderSelection();
          filterOptions();
          notifyChange();
        });
        row.append(title, remove);
        if (picker.dataset.unitPickerInputName) {
          const hiddenInput = document.createElement("input");
          hiddenInput.type = "hidden";
          hiddenInput.name = picker.dataset.unitPickerInputName;
          hiddenInput.value = option.dataset.unitId;
          row.append(hiddenInput);
        }
        current.append(row);
      });
      if (!current.children.length) {
        const message = document.createElement("p");
        message.className = "empty-message";
        message.textContent = picker.dataset.unitPickerEmptySelection || "No units selected.";
        current.append(message);
      }
      if (count) count.textContent = selectedIDs.size + " selected";
    }

    function notifyChange() {
      picker.dispatchEvent(new CustomEvent("unit-picker:change", { bubbles: true }));
    }

    function configure(configuration) {
      selectedIDs = new Set(configuration.selectedIDs || []);
      excludedIDs = new Set(configuration.excludedIDs || []);
      input.value = "";
      closeResults();
      options.forEach(function (option) { option.classList.remove("is-filtered"); });
      renderSelection();
    }

    options.forEach(function (option) {
      option.addEventListener("click", function () {
        const event = new CustomEvent("unit-picker:add", {
          bubbles: true,
          cancelable: true,
          detail: { id: option.dataset.unitId, name: option.dataset.unitName }
        });
        if (!picker.dispatchEvent(event)) return;
        selectedIDs.add(option.dataset.unitId);
        renderSelection();
        input.value = "";
        closeResults();
        input.focus();
        notifyChange();
      });
    });
    input.addEventListener("focus", filterOptions);
    input.addEventListener("input", filterOptions);
    input.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        closeResults();
        input.blur();
      } else if (event.key === "ArrowDown") {
        const first = options.find(function (option) {
          return !option.classList.contains("is-filtered");
        });
        if (first) {
          event.preventDefault();
          first.focus();
        }
      }
    });
    document.addEventListener("pointerdown", function (event) {
      if (!picker.contains(event.target)) closeResults();
    });
    const form = picker.closest("form");
    if (form && picker.dataset.unitPickerRequiredMessage) {
      form.addEventListener("submit", function (event) {
        if (selectedIDs.size) return;
        event.preventDefault();
        input.setCustomValidity(picker.dataset.unitPickerRequiredMessage);
        input.reportValidity();
        input.setCustomValidity("");
      });
    }
    picker.unitPicker = { configure: configure };
    configure({
      selectedIDs: (picker.dataset.unitPickerSelectedIds || "").split(",").filter(Boolean),
      excludedIDs: []
    });
  }

  function initializeAll(root) {
    if (root.matches && root.matches("[data-unit-picker]")) initializeUnitPicker(root);
    root.querySelectorAll("[data-unit-picker]").forEach(initializeUnitPicker);
  }

  document.addEventListener("DOMContentLoaded", function () { initializeAll(document); });
  document.addEventListener("htmx:load", function (event) { initializeAll(event.detail.elt || document); });
})();
