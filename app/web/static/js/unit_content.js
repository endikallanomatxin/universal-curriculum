(function () {
  "use strict";

  function renderMath(root) {
    if (typeof window.renderMathInElement !== "function") return;
    root.querySelectorAll(".unit-content__body").forEach(function (content) {
      if (content.dataset.mathRendered === "true") return;
      window.renderMathInElement(content, {
        delimiters: [
          { left: "$$", right: "$$", display: true },
          { left: "\\[", right: "\\]", display: true },
          { left: "\\(", right: "\\)", display: false },
          { left: "$", right: "$", display: false }
        ],
        throwOnError: false,
        strict: "warn"
      });
      content.dataset.mathRendered = "true";
    });
  }

  function highlightCode(root) {
    if (!window.hljs) return;
    root.querySelectorAll(".unit-content__body pre code:not([data-highlighted])").forEach(function (code) {
      const languageClass = Array.from(code.classList).find(function (className) {
        return className.indexOf("language-") === 0;
      });
      if (languageClass) {
        const language = languageClass.slice("language-".length);
        const label = document.createElement("span");
        label.className = "unit-content__code-language";
        label.textContent = language;
        code.parentElement.prepend(label);
        code.parentElement.classList.add("unit-content__code-block--labelled");
      }
      window.hljs.highlightElement(code);
    });
  }

  function enhanceContent(root) {
    renderMath(root);
    highlightCode(root);
  }

  document.addEventListener("click", function (event) {
    const trigger = event.target.closest("[data-content-view-trigger]");
    if (!trigger) return;
    const container = trigger.closest("[data-inline-editor-display]");
    if (!container) return;
    const view = trigger.dataset.contentViewTrigger;
    container.querySelectorAll("[data-content-view-trigger]").forEach(function (button) {
      button.setAttribute("aria-selected", String(button === trigger));
    });
    container.querySelectorAll("[data-content-view-panel]").forEach(function (panel) {
      panel.hidden = panel.dataset.contentViewPanel !== view;
    });
    enhanceContent(container);
  });

  window.addEventListener("message", function (event) {
    if (!event.data || event.data.type !== "unit-visualization-height") return;
    document.querySelectorAll(".unit-visualization").forEach(function (frame) {
      if (frame.contentWindow !== event.source) return;
      const height = Math.max(180, Math.min(1200, Number(event.data.height) || 0));
      frame.style.height = height + "px";
    });
  });

  document.addEventListener("DOMContentLoaded", function () { enhanceContent(document); });
  document.addEventListener("htmx:load", function (event) {
    enhanceContent(event.detail.elt || document);
  });
})();
