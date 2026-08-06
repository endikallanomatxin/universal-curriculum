/* Account interactions. */
(function () {
  "use strict";

  function copyText(value) {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(value);
    }
    return new Promise(function (resolve, reject) {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.setAttribute("readonly", "");
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      const copied = document.execCommand("copy");
      textarea.remove();
      if (copied) resolve();
      else reject(new Error("clipboard unavailable"));
    });
  }

  document.addEventListener("panel:configure", function (event) {
    const panel = event.detail && event.detail.panel;
    if (!panel || !panel.matches("[data-api-token-panel]")) return;
    const result = panel.querySelector("[data-api-token-result]");
    if (!result) return;

    result.remove();
    const form = panel.querySelector("[data-api-token-form]");
    if (form) {
      form.hidden = false;
      form.reset();
    }
    const title = panel.querySelector("[data-api-token-panel-title]");
    if (title) title.textContent = "New API token";
    panel.dataset.panelBreadcrumb = "New API token";
  });

  document.addEventListener("click", function (event) {
    const button = event.target.closest("[data-copy-api-token]");
    if (!button) return;
    const result = button.closest("[data-api-token-result]");
    const value = result && result.querySelector("[data-api-token-value]");
    const label = button.querySelector("[data-copy-api-token-label]");
    const status = result && result.querySelector("[data-copy-api-token-status]");
    if (!value) return;

    button.disabled = true;
    copyText(value.textContent.trim()).then(function () {
      if (label) label.textContent = "Copied";
      if (status) status.textContent = "Token copied to clipboard.";
      window.clearTimeout(button.copyResetTimer);
      button.copyResetTimer = window.setTimeout(function () {
        if (label) label.textContent = "Copy";
      }, 2000);
    }).catch(function () {
      if (status) status.textContent = "Unable to copy the token. Select it and copy it manually.";
    }).finally(function () {
      button.disabled = false;
    });
  });
})();
