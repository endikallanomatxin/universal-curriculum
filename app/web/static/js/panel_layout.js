(function () {
  "use strict";

  function modesFor(panel) {
    const modes = (panel.dataset.panelModes || "").trim().split(/\s+/).map(function (entry) {
      const parts = entry.split(":");
      return { name: parts[0], width: Number(parts[1]) };
    }).filter(function (mode) {
      return mode.name && Number.isFinite(mode.width) && mode.width >= 0;
    });
    const childrenRequiredWidth = Number(panel.dataset.panelChildrenRequiredWidth);
    if (Number.isFinite(childrenRequiredWidth) && !modes.some(function (mode) {
      return mode.width === childrenRequiredWidth;
    })) {
      modes.push({ name: "children-required", width: childrenRequiredWidth });
    }
    const childrenDesiredWidth = Number(panel.dataset.panelChildrenDesiredWidth);
    if (Number.isFinite(childrenDesiredWidth) && !modes.some(function (mode) {
      return mode.width === childrenDesiredWidth;
    })) {
      modes.push({ name: "children-desired", width: childrenDesiredWidth });
    }
    return modes.sort(function (a, b) {
      return a.width - b.width;
    });
  }

  function ownRequiredWidth(definition) {
    const requiredMode = definition.panel.dataset.panelRequiredMode;
    const mode = definition.modes.find(function (candidate) {
      return candidate.name === requiredMode;
    });
    const childrenRequiredWidth = Number(definition.panel.dataset.panelChildrenRequiredWidth);
    return Math.max(
      mode ? mode.width : definition.modes[0].width,
      Number.isFinite(childrenRequiredWidth) ? childrenRequiredWidth : 0
    );
  }

  function ownDesiredWidth(definition) {
    const ownMaximum = definition.modes[definition.modes.length - 1].width;
    const childrenDesiredWidth = Number(definition.panel.dataset.panelChildrenDesiredWidth);
    return Math.max(
      ownMaximum,
      Number.isFinite(childrenDesiredWidth) ? childrenDesiredWidth : 0
    );
  }

  function visiblePanels(group) {
    return Array.from(group.children).filter(function (child) {
      return child.matches("[data-layout-panel]") && !child.hidden && !child.classList.contains("is-closing");
    });
  }

  function allocationOrder(definitions) {
    return definitions.map(function (_, index) {
      return index;
    }).reverse();
  }

  function selectModeAtWidth(definition, width) {
    let selection = 0;
    definition.modes.forEach(function (mode, index) {
      if (mode.width <= width) selection = index;
    });
    return selection;
  }

  function roundedWidth(width) {
    return Math.round(width * 1000) / 1000;
  }

  function setPanelGeometry(panel, width, mode) {
    const widthValue = roundedWidth(width) + "rem";
    let changed = false;
    if (panel.style.getPropertyValue("--panel-width") !== widthValue) {
      panel.style.setProperty("--panel-width", widthValue);
      changed = true;
    }
    if (panel.dataset.panelMode !== mode) {
      panel.dataset.panelMode = mode;
      changed = true;
    }
    return changed;
  }

  function setGroupValue(group, name, value) {
    const stringValue = String(roundedWidth(value));
    if (group.dataset[name] === stringValue) return false;
    group.dataset[name] = stringValue;
    return true;
  }

  function layoutMobileGroup(group, definitions, available) {
    const activeIndex = definitions.length - 1;
    let changed = false;
    definitions.forEach(function (definition, index) {
      const width = index === activeIndex ? available : 0;
      const selection = selectModeAtWidth(definition, width);
      changed = setPanelGeometry(
        definition.panel,
        width,
        definition.modes[selection].name
      ) || changed;
    });
    const activeDefinition = definitions[activeIndex];
    changed = setGroupValue(
      group,
      "panelChildrenRequiredWidth",
      ownRequiredWidth(activeDefinition)
    ) || changed;
    changed = setGroupValue(
      group,
      "panelChildrenDesiredWidth",
      ownDesiredWidth(activeDefinition)
    ) || changed;
    if (changed) group.dispatchEvent(new CustomEvent("panel-layout", { bubbles: true }));
  }

  function layoutGroup(group, mobile) {
    const panels = visiblePanels(group);
    if (!panels.length) return;

    const rootFontSize = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    const available = group.clientWidth / rootFontSize;
    const definitions = panels.map(function (panel) {
      return { panel: panel, modes: modesFor(panel) };
    }).filter(function (definition) {
      return definition.modes.length > 0;
    });
    if (!definitions.length) return;
    if (mobile) {
      layoutMobileGroup(group, definitions, available);
      return;
    }
    definitions.forEach(function (definition) {
      const visibleModes = definition.modes.filter(function (mode) { return mode.width > 0; });
      if (visibleModes.length) definition.modes = visibleModes;
    });

    const selections = definitions.map(function () { return 0; });
    const allocation = allocationOrder(definitions);
    let used = definitions.reduce(function (total, definition) {
      return total + definition.modes[0].width;
    }, 0);

    let requiredSpaceExhausted = false;
    for (let allocationIndex = 0; allocationIndex < allocation.length; allocationIndex += 1) {
      if (requiredSpaceExhausted) break;
      const index = allocation[allocationIndex];
      const definition = definitions[index];
      const requiredWidth = ownRequiredWidth(definition);
      for (let modeIndex = 1; modeIndex < definition.modes.length; modeIndex += 1) {
        const mode = definition.modes[modeIndex];
        if (mode.width > requiredWidth) break;
        const increase = mode.width - definition.modes[selections[index]].width;
        if (used + increase > available) {
          requiredSpaceExhausted = true;
          break;
        }
        selections[index] = modeIndex;
        used += increase;
      }
    }

    let higherPriorityPanelWantsSpace = false;
    for (let allocationIndex = 0; allocationIndex < allocation.length; allocationIndex += 1) {
      if (requiredSpaceExhausted || higherPriorityPanelWantsSpace) break;
      const index = allocation[allocationIndex];
      const definition = definitions[index];
      const desiredWidth = ownDesiredWidth(definition);
      for (let modeIndex = selections[index] + 1; modeIndex < definition.modes.length; modeIndex += 1) {
        const increase = definition.modes[modeIndex].width - definition.modes[selections[index]].width;
        if (used + increase > available) {
          for (let donorIndex = 0; donorIndex < index && used + increase > available; donorIndex += 1) {
            const donor = definitions[donorIndex];
            while (selections[donorIndex] > 0 && used + increase > available) {
              const lowerMode = donor.modes[selections[donorIndex] - 1];
              if (lowerMode.width === 0) break;
              const reduction = donor.modes[selections[donorIndex]].width - lowerMode.width;
              selections[donorIndex] -= 1;
              used -= reduction;
            }
          }
          if (used + increase > available) {
            higherPriorityPanelWantsSpace = definition.modes[modeIndex].width <= desiredWidth;
            break;
          }
        }
        selections[index] = modeIndex;
        used += increase;
      }
    }

    let spare = Math.max(0, available - used);
    const widths = definitions.map(function (definition, index) {
      return definition.modes[selections[index]].width;
    });
    for (let allocationIndex = 0; allocationIndex < allocation.length && spare > 0; allocationIndex += 1) {
      const index = allocation[allocationIndex];
      const definition = definitions[index];
      const panel = definition.panel;
      let maximum = definition.modes[definition.modes.length - 1].width;
      if (panel.hasAttribute("data-panel-fill")) maximum = Infinity;
      const capacity = Number.isFinite(maximum) ? Math.max(0, maximum - widths[index]) : spare;
      const addition = Math.min(spare, capacity);
      widths[index] += addition;
      spare -= addition;
      definition.modes.forEach(function (mode, modeIndex) {
        if (mode.width <= widths[index]) selections[index] = modeIndex;
      });
    }

    let changed = false;
    definitions.forEach(function (definition, index) {
      changed = setPanelGeometry(
        definition.panel,
        widths[index],
        definition.modes[selections[index]].name
      ) || changed;
    });
    changed = setGroupValue(group, "panelChildrenRequiredWidth", definitions.reduce(function (total, definition) {
      return total + ownRequiredWidth(definition);
    }, 0)) || changed;
    changed = setGroupValue(group, "panelChildrenDesiredWidth", definitions.reduce(function (total, definition) {
      return total + ownDesiredWidth(definition);
    }, 0)) || changed;
    if (changed) group.dispatchEvent(new CustomEvent("panel-layout", { bubbles: true }));
  }

  function mobileCompositionEnabled() {
    return getComputedStyle(document.documentElement)
      .getPropertyValue("--mobile-panel-composition").trim() === "1";
  }

  function layoutFrom(root) {
    const mobile = mobileCompositionEnabled();
    const groups = [];
    if (root.matches && root.matches("[data-panel-group]")) groups.push(root);
    root.querySelectorAll("[data-panel-group]").forEach(function (group) { groups.push(group); });
    groups.reverse().forEach(function (group) { layoutGroup(group, mobile); });
    document.dispatchEvent(new CustomEvent("panel-layout:complete", {
      detail: { shell: root, mobile: mobile }
    }));
  }

  function scheduleLayout(shell) {
    if (shell.panelLayoutFrame) return;
    shell.panelLayoutFrame = window.requestAnimationFrame(function () {
      shell.panelLayoutFrame = null;
      layoutFrom(shell);
    });
  }

  function initializePanelLayout() {
    const shell = document.querySelector("#app-shell");
    if (!shell) return;
    if (shell.classList.contains("app-shell--home")) {
      shell.querySelectorAll("[data-layout-panel]").forEach(function (panel) {
        panel.style.removeProperty("--panel-width");
        delete panel.dataset.panelMode;
      });
      shell.querySelectorAll("[data-panel-group]").forEach(function (group) {
        group.style.removeProperty("--panel-group-width");
        delete group.dataset.panelChildrenDesiredWidth;
      });
      return;
    }
    if (shell.panelLayoutObserver) shell.panelLayoutObserver.disconnect();

    const resizeObserver = new ResizeObserver(function () { scheduleLayout(shell); });
    shell.querySelectorAll("[data-panel-group]").forEach(function (group) {
      resizeObserver.observe(group);
      if (!group.panelLayoutScrollInitialized) {
        group.panelLayoutScrollInitialized = true;
        group.addEventListener("scroll", function () {
          if (group.panelLayoutScrollFrame) return;
          group.panelLayoutScrollFrame = window.requestAnimationFrame(function () {
            group.panelLayoutScrollFrame = null;
            scheduleLayout(shell);
          });
        }, { passive: true });
      }
    });
    resizeObserver.observe(shell);
    shell.panelLayoutObserver = resizeObserver;

    if (!shell.panelVisibilityObserver) {
      shell.panelVisibilityObserver = new MutationObserver(function () { scheduleLayout(shell); });
      shell.panelVisibilityObserver.observe(shell, {
        attributes: true,
        attributeFilter: ["hidden", "data-panel-breadcrumb"],
        subtree: true
      });
    }
    layoutFrom(shell);
  }

  document.addEventListener("DOMContentLoaded", initializePanelLayout);
  document.addEventListener("htmx:afterSwap", initializePanelLayout);
  window.panelLayout = { refresh: initializePanelLayout };
})();
