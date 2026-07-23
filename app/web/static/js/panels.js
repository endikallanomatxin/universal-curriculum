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
        document.dispatchEvent(new CustomEvent("panel:configure", {
          detail: { panel: panel, trigger: trigger }
        }));
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

  document.addEventListener("DOMContentLoaded", function () {
    initializePanels(document);
  });
  document.addEventListener("htmx:load", function (event) {
    initializePanels(event.detail.elt || document);
  });
})();
