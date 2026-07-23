(function () {
  "use strict";

  function initializeLearningPaths(root) {
    const panel = document.getElementById("learning-path-editor-panel");
    if (!panel) return;

    const form = panel.querySelector("[data-learning-path-form]");
    const deleteForm = panel.querySelector("[data-learning-path-delete-form]");
    const heading = panel.querySelector("[data-learning-path-editor-title]");
    const nameInput = panel.querySelector("[data-learning-path-name]");
    const descriptionInput = panel.querySelector("[data-learning-path-description]");
    const searchInput = panel.querySelector("[data-learning-path-unit-search-input]");
    const results = panel.querySelector("[data-learning-path-unit-results]");
    const empty = panel.querySelector("[data-learning-path-unit-empty]");
    const current = panel.querySelector("[data-learning-path-current-targets]");
    const options = Array.from(panel.querySelectorAll("[data-learning-path-unit-option]"));
    const count = panel.querySelector("[data-selected-unit-count]");
    let selectedIDs = new Set();
    if (!form || !nameInput || !descriptionInput) return;

    function updateCount() {
      if (count) count.textContent = selectedIDs.size + " selected";
    }

    function filterUnits() {
      const query = searchInput ? searchInput.value.trim().toLocaleLowerCase() : "";
      let matches = 0;
      options.forEach(function (option) {
        const matchesQuery = option.dataset.unitName.toLocaleLowerCase().includes(query);
        const filtered = selectedIDs.has(option.dataset.unitId) || !matchesQuery;
        option.classList.toggle("is-filtered", filtered);
        if (!filtered) matches++;
      });
      if (empty) empty.hidden = matches > 0;
      if (results) results.hidden = false;
      if (searchInput) searchInput.setAttribute("aria-expanded", "true");
    }

    function renderSelection() {
      if (!current) return;
      current.replaceChildren();
      options.forEach(function (option) {
        if (!selectedIDs.has(option.dataset.unitId)) return;
        const row = document.createElement("div");
        const copy = document.createElement("div");
        const title = document.createElement("strong");
        const description = document.createElement("span");
        const remove = document.createElement("button");
        const hiddenInput = document.createElement("input");
        row.className = "dependency-editor__row";
        title.textContent = option.dataset.unitName;
        description.textContent = option.dataset.unitDescription;
        remove.type = "button";
        remove.className = "editor-action";
        remove.textContent = "Remove";
        remove.addEventListener("click", function () {
          selectedIDs.delete(option.dataset.unitId);
          renderSelection();
          filterUnits();
        });
        hiddenInput.type = "hidden";
        hiddenInput.name = "unit_ids";
        hiddenInput.value = option.dataset.unitId;
        copy.append(title, description);
        row.append(copy, remove, hiddenInput);
        current.append(row);
      });
      if (!current.children.length) {
        const message = document.createElement("p");
        message.className = "proposal-history__empty";
        message.textContent = "No target units selected.";
        current.append(message);
      }
      updateCount();
    }

    function configureEditor(trigger) {
      const pathID = trigger.dataset.learningPathId;
      selectedIDs = new Set(
        (trigger.dataset.learningPathUnits || "").split(",").filter(Boolean)
      );
      form.action = pathID ? "/learn/paths/" + encodeURIComponent(pathID) : "/learn/paths";
      heading.textContent = pathID ? "Edit path" : "New path";
      nameInput.value = pathID ? trigger.dataset.learningPathName || "" : "";
      descriptionInput.value = pathID ? trigger.dataset.learningPathDescription || "" : "";
      if (searchInput) searchInput.value = "";
      if (results) results.hidden = true;
      if (searchInput) searchInput.setAttribute("aria-expanded", "false");
      options.forEach(function (option) { option.classList.remove("is-filtered"); });
      if (deleteForm) {
        deleteForm.hidden = !pathID;
        deleteForm.action = pathID ? "/learn/paths/" + encodeURIComponent(pathID) + "/delete" : "";
      }
      renderSelection();
      if (window.autoResizeTextareas) window.autoResizeTextareas(panel);
    }

    root.querySelectorAll('[data-open-panel="learning-path-editor-panel"]').forEach(function (trigger) {
      if (trigger.dataset.learningPathInitialized === "true") return;
      trigger.dataset.learningPathInitialized = "true";
      trigger.addEventListener("click", function () { configureEditor(trigger); });
    });

    if (panel.dataset.learningPathInitialized !== "true") {
      panel.dataset.learningPathInitialized = "true";
      options.forEach(function (option) {
        option.addEventListener("click", function () {
          selectedIDs.add(option.dataset.unitId);
          renderSelection();
          if (searchInput) searchInput.value = "";
          if (results) results.hidden = true;
          if (searchInput) {
            searchInput.setAttribute("aria-expanded", "false");
            searchInput.focus();
          }
        });
      });
      if (searchInput) {
        searchInput.addEventListener("focus", filterUnits);
        searchInput.addEventListener("input", filterUnits);
      }
      form.addEventListener("submit", function (event) {
        if (selectedIDs.size > 0) return;
        event.preventDefault();
        searchInput.setCustomValidity("Select at least one target unit.");
        searchInput.reportValidity();
        searchInput.setCustomValidity("");
      });
      updateCount();
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    initializeLearningPaths(document);
  });
  document.addEventListener("htmx:load", function (event) {
    initializeLearningPaths(event.detail.elt || document);
  });
})();
