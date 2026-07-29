(function () {
  "use strict";

  const closeDuration = 280;

  function directChildContaining(group, element) {
    let current = element;
    while (current && current.parentElement !== group) current = current.parentElement;
    return current && current.parentElement === group ? current : null;
  }

  function beginPanelClose(panel, restoreFocus) {
    if (!panel || panel.hidden || panel.classList.contains("is-closing")) return false;
    const trigger = panel.activeTrigger ||
      (panel.id && document.querySelector('[data-open-panel="' + panel.id + '"]'));
    window.clearTimeout(panel.closeTimer);
    panel.classList.remove("is-opening");
    panel.classList.add("is-closing");
    if (trigger) trigger.setAttribute("aria-expanded", "false");
    panel.closeTimer = window.setTimeout(function () {
      panel.hidden = true;
      panel.classList.remove("is-closing");
      if (restoreFocus && trigger) {
        trigger.scrollIntoView({ behavior: "auto", block: "nearest", inline: "end" });
        trigger.focus({ preventScroll: true });
      }
    }, closeDuration);
    return true;
  }

  function closeMatchingPanels(group, origin, predicate) {
    if (!group || !origin) return false;
    const children = Array.from(group.children);
    const originIndex = children.indexOf(origin);
    if (originIndex < 0) return false;
    let changed = false;
    children.forEach(function (panel, index) {
      if (index <= originIndex || !predicate(panel)) return;
      changed = beginPanelClose(panel, false) || changed;
    });
    return changed;
  }

  function closePanelsToTheRight(trigger, targetPanel) {
    const group = targetPanel.parentElement;
    const origin = directChildContaining(group, trigger);
    closeMatchingPanels(group, origin, function (panel) {
      return panel !== targetPanel && panel.matches("[data-nested-panel]");
    });
  }

  function closePanelsAfter(panel) {
    const group = panel && panel.parentElement;
    const changed = closeMatchingPanels(group, panel, function (candidate) {
      return candidate.matches("[data-layout-panel]");
    });
    if (changed && window.panelLayout) window.panelLayout.refresh();
  }

  function initializePanels(root) {
    root.querySelectorAll("[data-open-panel]").forEach(function (trigger) {
      if (trigger.panelInitialized) return;
      trigger.panelInitialized = true;
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
      if (!close || close.panelInitialized) return;
      close.panelInitialized = true;
      close.addEventListener("click", function () {
        const changed = beginPanelClose(panel, true);
        if (changed && window.panelLayout) window.panelLayout.refresh();
      });
    });
  }

  function animateServerPanels(root) {
    if (document.serverPanelContinuity) {
      document.serverPanelContinuity = false;
      return;
    }
    root.querySelectorAll("[data-panel-enter]:not([hidden])").forEach(function (panel) {
      if (panel.serverPanelEntered) return;
      panel.serverPanelEntered = true;
      panel.classList.add("is-opening");
      panel.addEventListener("animationend", function () {
        panel.classList.remove("is-opening");
      }, { once: true });
    });
  }

  function initializeServerPanelClose(root) {
    root.querySelectorAll("[data-server-panel-close]").forEach(function (trigger) {
      if (trigger.serverPanelCloseInitialized) return;
      trigger.serverPanelCloseInitialized = true;
      trigger.addEventListener("click", function (event) {
        event.preventDefault();
        const panel = trigger.closest("[data-panel-enter]");
        if (!panel || panel.classList.contains("is-closing")) return;

        panel.classList.remove("is-opening");
        panel.classList.add("is-closing");
        let navigated = false;
        const navigate = function () {
          if (navigated) return;
          navigated = true;
          window.clearTimeout(panel.serverPanelCloseTimer);
          htmx.trigger(trigger, "panel-close");
        };
        panel.addEventListener("animationend", navigate, { once: true });
        panel.serverPanelCloseTimer = window.setTimeout(navigate, closeDuration + 50);
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    initializePanels(document);
    initializeServerPanelClose(document);
  });
  document.addEventListener("htmx:load", function (event) {
    const root = event.detail.elt || document;
    initializePanels(root);
    initializeServerPanelClose(root);
  });
  document.addEventListener("htmx:beforeRequest", function (event) {
    const trigger = event.detail && event.detail.elt;
    document.serverPanelContinuity = !!(trigger && trigger.matches("[data-panel-continuity]"));
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.target && event.target.id === "workspace") animateServerPanels(event.target);
  });
  document.addEventListener("panel:navigate", function (event) {
    closePanelsAfter(event.detail && event.detail.panel);
  });
})();
