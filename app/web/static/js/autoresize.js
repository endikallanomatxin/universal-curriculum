(function () {
  "use strict";

  function autoResize(textarea) {
    textarea.style.height = "auto";

    if (textarea.scrollHeight < 60) {
      textarea.style.height = "60px";
      textarea.style.overflowY = "hidden";
    } else if (textarea.scrollHeight > 360) {
      textarea.style.height = "360px";
      textarea.style.overflowY = "auto";
    } else {
      textarea.style.height = textarea.scrollHeight + "px";
      textarea.style.overflowY = "hidden";
    }
  }

  function autoResizeTextareas(root) {
    const selector = "textarea[data-autoresize]";
    if (root.matches && root.matches(selector)) autoResize(root);
    if (root.querySelectorAll) root.querySelectorAll(selector).forEach(autoResize);
  }

  window.autoResizeTextareas = autoResizeTextareas;
  document.addEventListener("DOMContentLoaded", function () { autoResizeTextareas(document); });
  document.addEventListener("input", function (event) {
    if (event.target.matches("textarea[data-autoresize]")) autoResize(event.target);
  });
  document.addEventListener("htmx:load", function (event) {
    autoResizeTextareas(event.detail.elt || document);
  });
})();
