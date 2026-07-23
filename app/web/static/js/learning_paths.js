(function () {
  "use strict";

  document.addEventListener("panel:configure", function (event) {
    const panel = event.detail.panel;
    const trigger = event.detail.trigger;
    if (panel.id !== "learning-path-editor-panel") return;
    const form = panel.querySelector("[data-learning-path-form]");
    const deleteForm = panel.querySelector("[data-learning-path-delete-form]");
    const heading = panel.querySelector("[data-learning-path-editor-title]");
    const nameInput = panel.querySelector("[data-learning-path-name]");
    const descriptionInput = panel.querySelector("[data-learning-path-description]");
    const picker = panel.querySelector("[data-unit-picker]");
    const pathID = trigger.dataset.learningPathId;
    if (!form || !nameInput || !descriptionInput) return;

    form.action = pathID ? "/learn/paths/" + encodeURIComponent(pathID) : "/learn/paths";
    if (heading) heading.textContent = pathID ? "Edit path" : "New path";
    panel.dataset.panelBreadcrumb = pathID
      ? trigger.dataset.learningPathName || "Edit path"
      : "New path";
    nameInput.value = pathID ? trigger.dataset.learningPathName || "" : "";
    descriptionInput.value = pathID ? trigger.dataset.learningPathDescription || "" : "";
    if (deleteForm) {
      deleteForm.hidden = !pathID;
      deleteForm.action = pathID ? "/learn/paths/" + encodeURIComponent(pathID) + "/delete" : "";
    }
    if (picker && picker.unitPicker) {
      picker.unitPicker.configure({
        selectedIDs: (trigger.dataset.learningPathUnits || "").split(",").filter(Boolean)
      });
    }
    if (window.autoResizeTextareas) window.autoResizeTextareas(panel);
  });
})();
