(function () {
  "use strict";

  function initializeInlineEditors(root) {
    root.querySelectorAll("[data-inline-editor-trigger]").forEach(function (trigger) {
      if (trigger.inlineEditorInitialized) return;
      trigger.inlineEditorInitialized = true;
      const editor = document.getElementById(trigger.dataset.inlineEditorTrigger);
      if (!editor) return;

      function setOpen(open) {
        const container = editor.closest("[data-inline-editing]") || editor.parentElement;
        if (open) {
          container.querySelectorAll("[data-inline-editor]").forEach(function (other) {
            if (other === editor) return;
            other.hidden = true;
            const otherDisplay = container.querySelector('[data-inline-editor-display="' + other.id + '"]');
            if (otherDisplay) otherDisplay.hidden = false;
            const otherTrigger = container.querySelector('[data-inline-editor-trigger="' + other.id + '"]');
            if (otherTrigger) otherTrigger.setAttribute("aria-expanded", "false");
          });
        }
        editor.hidden = !open;
        const display = container.querySelector('[data-inline-editor-display="' + editor.id + '"]');
        if (display) display.hidden = open;
        trigger.setAttribute("aria-expanded", String(open));
        if (open) {
          if (window.autoResizeTextareas) window.autoResizeTextareas(editor);
          const field = editor.querySelector("input:not([type=hidden]), textarea");
          if (field) field.focus({ preventScroll: true });
        }
      }

      trigger.addEventListener("click", function () { setOpen(editor.hidden); });
      const close = editor.querySelector("[data-inline-editor-close]");
      if (close) close.addEventListener("click", function () {
        setOpen(false);
        trigger.focus({ preventScroll: true });
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () { initializeInlineEditors(document); });
  document.addEventListener("htmx:load", function (event) {
    initializeInlineEditors(event.detail.elt || document);
  });
})();
