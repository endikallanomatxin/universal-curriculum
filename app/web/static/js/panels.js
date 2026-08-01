(function () {
  "use strict";

  const closeDuration = 280;
  const closeLayoutDelay = 120;
  const closeNavigationDelay = closeDuration + 50;
  const navigationByRequest = new WeakMap();

  function directChildContaining(group, element) {
    let current = element;
    while (current && current.parentElement !== group) current = current.parentElement;
    return current && current.parentElement === group ? current : null;
  }

  function beginPanelClose(panel, restoreFocus, preserveLayout, complete) {
    if (!panel || panel.hidden || panel.classList.contains("is-closing")) return false;
    const trigger = panel.activeTrigger ||
      (panel.id && document.querySelector('[data-open-panel="' + panel.id + '"]'));
    window.clearTimeout(panel.closeTimer);
    panel.panelPreserveLayoutWhileClosing = preserveLayout;
    if (preserveLayout && panel.parentElement) {
      panel.parentElement.classList.add("is-panel-motion-active");
    }
    panel.classList.remove("is-opening");
    panel.classList.add("is-closing");
    if (trigger) trigger.setAttribute("aria-expanded", "false");
    const recomposeDelay = preserveLayout ? 0 : closeDuration;
    panel.closeTimer = window.setTimeout(function () {
      recomposeAfterClientClose(panel, function () {
        if (restoreFocus && trigger) {
          trigger.scrollIntoView({ behavior: "auto", block: "nearest", inline: "end" });
          trigger.focus({ preventScroll: true });
        }
        if (complete) complete();
      });
    }, recomposeDelay);
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
        beginPanelClose(panel, true, true, function () {
          updateURLAfterClientClose(panel);
        });
      });
    });
  }

  function navigationFor(event) {
    const request = event.detail && event.detail.xhr;
    return request && navigationByRequest.get(request);
  }

  function paneTransitionKey(panel) {
    return panel && (
      panel.dataset.panelTransitionKey || panel.id || panel.getAttribute("aria-labelledby")
    );
  }

  function visibleWorkspacePaneKeys() {
    const workspace = document.querySelector("#workspace");
    if (!workspace) return [];
    return Array.from(workspace.children).filter(function (panel) {
      return panel.matches("[data-layout-panel]") &&
        !panel.hidden &&
        !panel.classList.contains("is-closing");
    }).map(paneTransitionKey).filter(Boolean);
  }

  function markMatchedTransitionContent(root) {
    Array.from(root.children).forEach(function (panel) {
      if (!panel.matches("[data-layout-panel]") || panel.classList.contains("is-closing")) return;
      const inner = Array.from(panel.children).find(function (child) {
        return child.matches(".ui-pane__inner");
      });
      if (inner && getComputedStyle(inner).viewTransitionName.endsWith("-content")) {
        panel.style.viewTransitionClass = "panel-surface matched-panel-surface";
        inner.style.viewTransitionClass = "matched-panel-content";
      }
    });
  }

  function keepTransitionContentOnlyForPersistentPanes(root, navigation) {
    if (!navigation || !navigation.viewTransition) return;
    const existing = new Set(navigation.existingPaneKeys || []);
    Array.from(root.children).forEach(function (panel) {
      if (!panel.matches("[data-layout-panel]")) return;
      const persistent = existing.has(paneTransitionKey(panel));
      const inner = Array.from(panel.children).find(function (child) {
        return child.matches(".ui-pane__inner");
      });
      if (persistent) {
        if (inner && getComputedStyle(inner).viewTransitionName.endsWith("-content")) {
          panel.style.viewTransitionClass = "panel-surface matched-panel-surface";
          inner.style.viewTransitionClass = "matched-panel-content";
        }
        return;
      }
      panel.panelContentTransitionSuppressed = true;
      if (inner) inner.style.removeProperty("view-transition-name");
    });
  }

  function clearTransitionClasses(root) {
    root.querySelectorAll('[data-layout-panel][style*="view-transition-class"]').forEach(function (panel) {
      if (panel.parentElement && panel.parentElement.matches(".pane-stack")) {
        panel.style.viewTransitionClass = "panel-surface";
      } else {
        panel.style.removeProperty("view-transition-class");
      }
    });
    root.querySelectorAll('.ui-pane__inner[style*="view-transition-class"]').forEach(function (inner) {
      inner.style.removeProperty("view-transition-class");
    });
  }

  function recomposeDuringPanelExit(panel) {
    const root = panel && panel.parentElement;
    if (!root || !window.panelLayout) return;
    if (typeof document.startViewTransition !== "function") {
      window.panelLayout.refresh();
      return;
    }
    const navigation = {
      viewTransition: true,
      existingPaneKeys: visibleWorkspacePaneKeys()
    };
    markMatchedTransitionContent(root);
    root.classList.add("is-panel-layout-snapshot");
    const transition = document.startViewTransition(function () {
      window.panelLayout.refresh();
      root.getBoundingClientRect();
      keepTransitionContentOnlyForPersistentPanes(root, navigation);
    });
    transition.ready.finally(function () {
      root.classList.remove("is-panel-layout-snapshot");
    });
    transition.finished.finally(function () {
      root.classList.remove("is-panel-layout-snapshot");
      clearTransitionClasses(root);
    });
  }

  function recomposeAfterClientClose(panel, complete) {
    const root = panel.parentElement;
    const shell = panel.closest("#app-shell");
    if (shell) shell.classList.add("is-shell-navigation");
    const finishMotion = function () {
      if (shell) shell.classList.remove("is-shell-navigation");
    };
    const finish = function () {
      if (root) root.classList.remove("is-panel-motion-active");
      panel.hidden = true;
      panel.classList.remove("is-closing");
      delete panel.panelPreserveLayoutWhileClosing;
      if (window.panelLayout) window.panelLayout.settle();
    };
    if (!root || typeof document.startViewTransition !== "function") {
      finish();
      finishMotion();
      complete();
      return;
    }

    const navigation = {
      viewTransition: true,
      existingPaneKeys: visibleWorkspacePaneKeys()
    };
    markMatchedTransitionContent(root);
    const transition = document.startViewTransition(function () {
      finish();
      keepTransitionContentOnlyForPersistentPanes(root, navigation);
    });
    transition.finished.finally(function () {
      clearTransitionClasses(root);
      finishMotion();
      complete();
    });
  }

  function updateURLAfterClientClose(panel) {
    const queryParameter = panel.dataset.panelCloseQuery;
    if (!queryParameter) return;
    const url = new URL(window.location.href);
    if (!url.searchParams.has(queryParameter)) return;
    url.searchParams.delete(queryParameter);
    window.history.replaceState(window.history.state, "", url.pathname + url.search + url.hash);
  }

  function resolvedNavigationMode(trigger) {
    const declaredMode = trigger && trigger.dataset.panelNavigation;
    if (declaredMode === "workspace") {
      const workspace = document.querySelector("#workspace");
      return !workspace || workspace.hidden || workspace.dataset.shellView === "home" ? "open" : "replace";
    }
    if (declaredMode !== "open-or-replace") return declaredMode;
    const group = trigger.closest("[data-panel-group]");
    const origin = directChildContaining(group, trigger);
    if (!group || !origin) return "open";
    const children = Array.from(group.children);
    const originIndex = children.indexOf(origin);
    const hasPanelToReplace = children.some(function (panel, index) {
      return index > originIndex &&
        panel.matches("[data-nested-panel]") &&
        !panel.hidden &&
        !panel.classList.contains("is-closing");
    });
    return hasPanelToReplace ? "replace" : "open";
  }

  function animateNavigatedPanels(root, navigation) {
    if (!navigation || navigation.mode !== "open" || navigation.viewTransition) return;
    const panels = navigation.scope === "workspace"
      ? [root]
      : Array.from(root.querySelectorAll('[data-panel-motion="horizontal"]:not([hidden])'));
    const panel = panels[panels.length - 1];
    if (!panel || panel.panelMotionEntered) return;
    panel.panelMotionEntered = true;
    root.classList.add("is-panel-motion-active");
    panel.classList.add("is-opening");
    let motionFinished = false;
    const finishMotion = function () {
      if (motionFinished) return;
      motionFinished = true;
      panel.classList.remove("is-opening");
      root.classList.remove("is-panel-motion-active");
      if (window.panelLayout) window.panelLayout.refresh();
    };
    panel.addEventListener("animationend", finishMotion, { once: true });
    panel.addEventListener("animationcancel", finishMotion, { once: true });
  }

  function initializePanelNavigation(root) {
    root.querySelectorAll('[data-panel-navigation="close"]').forEach(function (trigger) {
      if (trigger.panelNavigationInitialized) return;
      trigger.panelNavigationInitialized = true;
      trigger.addEventListener("click", function (event) {
        event.preventDefault();
        const panel = trigger.closest("[data-panel-motion]");
        if (!panel || panel.classList.contains("is-closing")) return;

        panel.classList.remove("is-opening");
        panel.classList.add("is-closing");
        const group = panel.parentElement;
        if (group) {
          group.classList.add("is-panel-motion-active");
          group.getBoundingClientRect();
        }
        panel.panelLayoutTimer = window.setTimeout(function () {
          recomposeDuringPanelExit(panel);
        }, closeLayoutDelay);
        let navigated = false;
        const navigate = function () {
          if (navigated) return;
          navigated = true;
          window.clearTimeout(panel.panelNavigationTimer);
          htmx.trigger(trigger, "panel-close");
        };
        panel.addEventListener("animationend", navigate, { once: true });
        panel.panelNavigationTimer = window.setTimeout(navigate, closeNavigationDelay);
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    initializePanels(document);
    initializePanelNavigation(document);
  });
  document.addEventListener("htmx:load", function (event) {
    const root = event.detail.elt || document;
    initializePanels(root);
    initializePanelNavigation(root);
  });
  document.addEventListener("htmx:beforeRequest", function (event) {
    const trigger = event.detail && event.detail.elt;
    const request = event.detail && event.detail.xhr;
    const declaredMode = trigger && trigger.dataset.panelNavigation;
    const mode = resolvedNavigationMode(trigger);
    if (request && mode) {
      navigationByRequest.set(request, {
        mode: mode,
        scope: declaredMode === "workspace" ? "workspace" : "panel",
        viewTransition: mode === "open" && typeof document.startViewTransition === "function",
        existingPaneKeys: visibleWorkspacePaneKeys()
      });
      if (mode === "open") {
        const workspace = document.querySelector("#workspace");
        if (workspace && typeof document.startViewTransition === "function") {
          markMatchedTransitionContent(workspace);
        }
        trigger.setAttribute(
          "hx-swap",
          typeof document.startViewTransition === "function"
            ? "outerHTML transition:true"
            : "outerHTML settle:0"
        );
        if (window.panelLayout) window.panelLayout.beginSettlement();
      }
      if (mode === "replace") trigger.setAttribute("hx-swap", "outerHTML transition:true");
      if (mode === "close") trigger.setAttribute("hx-swap", "outerHTML settle:0");
    }
  });
  document.addEventListener("htmx:afterRequest", function (event) {
    const navigation = navigationFor(event);
    if (navigation && navigation.mode === "open" && event.detail && event.detail.failed) {
      if (window.panelLayout) window.panelLayout.cancelSettlement();
    }
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.target && event.target.id === "workspace") {
      keepTransitionContentOnlyForPersistentPanes(event.target, navigationFor(event));
    }
  });
  document.addEventListener("htmx:afterSettle", function (event) {
    if (event.target && event.target.id === "workspace") {
      const root = event.target;
      const navigation = navigationFor(event);
      if (navigation && navigation.mode === "open" && window.panelLayout) {
        window.panelLayout.settle();
      }
      keepTransitionContentOnlyForPersistentPanes(root, navigation);
      if (navigation && navigation.viewTransition) {
        window.setTimeout(function () {
          clearTransitionClasses(root);
        }, 500);
      }
      window.requestAnimationFrame(function () {
        animateNavigatedPanels(root, navigation);
      });
    }
  });
  document.addEventListener("panel:navigate", function (event) {
    closePanelsAfter(event.detail && event.detail.panel);
  });
})();
