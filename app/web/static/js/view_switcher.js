(function () {
  "use strict";

  document.addEventListener("click", function (event) {
    const trigger = event.target.closest("[data-view-switcher-trigger]");
    if (!trigger) return;
    const switcher = trigger.closest("[data-view-switcher]");
    if (!switcher) return;
    const value = trigger.dataset.viewSwitcherTrigger;

    Array.from(switcher.querySelectorAll("[data-view-switcher-trigger]")).filter(function (button) {
      return button.closest("[data-view-switcher]") === switcher;
    }).forEach(function (button) {
      const selected = button === trigger;
      button.setAttribute("aria-selected", String(selected));
      button.tabIndex = selected ? 0 : -1;
    });
    Array.from(switcher.querySelectorAll("[data-view-switcher-panel]")).filter(function (panel) {
      return panel.closest("[data-view-switcher]") === switcher;
    }).forEach(function (panel) {
      panel.hidden = panel.dataset.viewSwitcherPanel !== value;
    });
    switcher.dispatchEvent(new CustomEvent("view-switcher:change", {
      bubbles: true,
      detail: { value: value }
    }));
  });

  document.addEventListener("keydown", function (event) {
    const trigger = event.target.closest("[data-view-switcher-trigger]");
    if (!trigger || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    const switcher = trigger.closest("[data-view-switcher]");
    if (!switcher) return;
    const triggers = Array.from(switcher.querySelectorAll("[data-view-switcher-trigger]"))
      .filter(function (candidate) { return candidate.closest("[data-view-switcher]") === switcher; });
    const current = triggers.indexOf(trigger);
    let next = event.key === "Home" ? 0 : event.key === "End" ? triggers.length - 1 :
      (current + (event.key === "ArrowRight" ? 1 : -1) + triggers.length) % triggers.length;
    event.preventDefault();
    triggers[next].focus();
    triggers[next].click();
  });
})();
