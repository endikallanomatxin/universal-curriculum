(function () {
  "use strict";

  function normalized(value) {
    return value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLocaleLowerCase();
  }

  function initializeGraphSearch(search) {
    if (!search || search.graphSearchInitialized) return;
    search.graphSearchInitialized = true;
    const input = search.querySelector("[data-graph-search-input]");
    const results = search.querySelector("[data-graph-search-results]");
    const options = Array.from(search.querySelectorAll("[data-graph-search-option]"));
    const empty = search.querySelector("[data-graph-search-empty]");
    if (!input || !results) return;

    function filter() {
      const query = normalized(input.value.trim());
      let matches = 0;
      options.forEach(function (option) {
        const visible = normalized(option.dataset.unitName).includes(query);
        option.hidden = !visible;
        if (visible) matches += 1;
      });
      if (empty) empty.hidden = matches > 0;
      results.hidden = false;
      input.setAttribute("aria-expanded", "true");
    }

    function close() {
      results.hidden = true;
      input.setAttribute("aria-expanded", "false");
    }

    input.addEventListener("focus", filter);
    input.addEventListener("input", filter);
    input.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        close();
        input.blur();
      } else if (event.key === "ArrowDown") {
        const first = options.find(function (option) { return !option.hidden; });
        if (first) {
          event.preventDefault();
          first.focus();
        }
      }
    });
    options.forEach(function (option) {
      option.addEventListener("click", close);
    });
    document.addEventListener("pointerdown", function (event) {
      if (!search.contains(event.target)) close();
    });
  }

  function initializeGraphSearches(root) {
    if (root.matches && root.matches("[data-graph-search]")) initializeGraphSearch(root);
    root.querySelectorAll("[data-graph-search]").forEach(initializeGraphSearch);
  }

  document.addEventListener("DOMContentLoaded", function () { initializeGraphSearches(document); });
  document.addEventListener("htmx:load", function (event) {
    initializeGraphSearches(event.detail.elt || document);
  });
})();
