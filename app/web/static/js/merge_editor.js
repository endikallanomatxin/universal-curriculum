(function () {
  "use strict";

  function lines(content) {
    return String(content || "").replace(/\r\n/g, "\n").match(/[^\n]*\n|[^\n]+$/g) || [];
  }

  function appendPart(parts, kind, values) {
    if (!values.length) return;
    const previous = parts[parts.length - 1];
    if (previous && previous.kind === kind) previous.values.push.apply(previous.values, values);
    else parts.push({ kind: kind, values: values.slice() });
  }

  function compare(previous, current) {
    const lengths = Array.from({ length: previous.length + 1 }, function () {
      return new Uint32Array(current.length + 1);
    });
    for (let oldIndex = previous.length - 1; oldIndex >= 0; oldIndex--) {
      for (let newIndex = current.length - 1; newIndex >= 0; newIndex--) {
        lengths[oldIndex][newIndex] = previous[oldIndex] === current[newIndex]
          ? lengths[oldIndex + 1][newIndex + 1] + 1
          : Math.max(lengths[oldIndex + 1][newIndex], lengths[oldIndex][newIndex + 1]);
      }
    }
    const parts = [];
    let oldIndex = 0;
    let newIndex = 0;
    while (oldIndex < previous.length && newIndex < current.length) {
      if (previous[oldIndex] === current[newIndex]) {
        appendPart(parts, "same", [previous[oldIndex++]])
        newIndex++;
      } else if (lengths[oldIndex + 1][newIndex] >= lengths[oldIndex][newIndex + 1]) {
        appendPart(parts, "deleted", [previous[oldIndex++]])
      } else {
        appendPart(parts, "inserted", [current[newIndex++]])
      }
    }
    appendPart(parts, "deleted", previous.slice(oldIndex));
    appendPart(parts, "inserted", current.slice(newIndex));
    return parts;
  }

  function diffParts(previous, current) {
    let prefix = 0;
    while (prefix < previous.length && prefix < current.length && previous[prefix] === current[prefix]) prefix++;
    let suffix = 0;
    while (suffix < previous.length - prefix && suffix < current.length - prefix &&
      previous[previous.length - 1 - suffix] === current[current.length - 1 - suffix]) suffix++;
    const parts = [];
    appendPart(parts, "same", previous.slice(0, prefix));
    const oldMiddle = previous.slice(prefix, previous.length - suffix);
    const newMiddle = current.slice(prefix, current.length - suffix);
    if (oldMiddle.length * newMiddle.length > 2000000) {
      appendPart(parts, "deleted", oldMiddle);
      appendPart(parts, "inserted", newMiddle);
    } else {
      compare(oldMiddle, newMiddle).forEach(function (part) {
        appendPart(parts, part.kind, part.values);
      });
    }
    if (suffix) appendPart(parts, "same", previous.slice(previous.length - suffix));
    return parts;
  }

  function changesFrom(base, variant) {
    const changes = [];
    let position = 0;
    let pending = null;
    diffParts(base, variant).forEach(function (part) {
      if (part.kind === "same") {
        if (pending) changes.push(pending);
        pending = null;
        position += part.values.length;
        return;
      }
      if (!pending) pending = { start: position, end: position, replacement: [] };
      if (part.kind === "deleted") {
        pending.end += part.values.length;
        position += part.values.length;
      } else {
        pending.replacement.push.apply(pending.replacement, part.values);
      }
    });
    if (pending) changes.push(pending);
    return changes;
  }

  function applyChanges(base, start, end, changes) {
    const output = [];
    let position = start;
    changes.forEach(function (change) {
      if (change.start < start || change.start > end || change.end > end) return;
      output.push.apply(output, base.slice(position, change.start));
      output.push.apply(output, change.replacement);
      position = change.end;
    });
    output.push.apply(output, base.slice(position, end));
    return output.join("");
  }

  function mergeParts(result, accepted, proposal) {
    const base = lines(result);
    const acceptedChanges = changesFrom(base, lines(accepted));
    const proposalChanges = changesFrom(base, lines(proposal));
    const all = acceptedChanges.concat(proposalChanges).sort(function (left, right) {
      return left.start - right.start || left.end - right.end;
    });
    if (!all.length) return [{ kind: "same", result: result, accepted: accepted, proposal: proposal }];
    const parts = [];
    let cursor = 0;
    for (let index = 0; index < all.length;) {
      const start = all[index].start;
      let end = all[index].end;
      index++;
      while (index < all.length && all[index].start <= end) {
        end = Math.max(end, all[index].end);
        index++;
      }
      if (start > cursor) {
        const shared = base.slice(cursor, start).join("");
        parts.push({ kind: "same", result: shared, accepted: shared, proposal: shared });
      }
      parts.push({
        kind: "change",
        result: base.slice(start, end).join(""),
        accepted: applyChanges(base, start, end, acceptedChanges),
        proposal: applyChanges(base, start, end, proposalChanges)
      });
      cursor = end;
    }
    if (cursor < base.length) {
      const shared = base.slice(cursor).join("");
      parts.push({ kind: "same", result: shared, accepted: shared, proposal: shared });
    }
    return parts;
  }

  function textarea(value, className, label) {
    const field = document.createElement("textarea");
    field.className = className || "";
    field.value = value;
    field.rows = 1;
    field.spellcheck = true;
    field.dataset.autoresize = "";
    field.dataset.autoresizeUnbounded = "";
    field.dataset.mergeComparisonPart = "";
    field.setAttribute("aria-label", label);
    return field;
  }

  function alternative(label, value, emptyText, modifier) {
    const container = document.createElement("div");
    container.className = "proposal-rebase-merge-part__alternative " + modifier;
    const heading = document.createElement("div");
    const title = document.createElement("span");
    title.className = "label";
    title.textContent = label;
    const button = document.createElement("button");
    button.className = "secondary-button secondary-button--compact";
    button.type = "button";
    button.dataset.useMergeAlternative = "";
    button.textContent = "Use this version";
    heading.append(title, button);
    const preview = value ? document.createElement("pre") : document.createElement("p");
    preview.textContent = value || emptyText;
    const source = document.createElement("textarea");
    source.hidden = true;
    source.dataset.alternativeSource = "";
    source.value = value;
    container.append(heading, preview, source);
    return container;
  }

  function syncResult(editor) {
    editor.querySelector("[data-merge-result]").value = Array.from(
      editor.querySelectorAll("[data-merge-comparison-part]"),
      function (part) { return part.value; }
    ).join("");
  }

  function renderComparison(editor) {
    const comparison = editor.querySelector("[data-merge-comparison]");
    const result = editor.querySelector("[data-merge-result]").value;
    const accepted = editor.querySelector("[data-merge-accepted-source]").value;
    const proposal = editor.querySelector("[data-merge-proposal-source]").value;
    comparison.replaceChildren();
    mergeParts(result, accepted, proposal).forEach(function (part) {
      if (part.kind === "same") {
        comparison.append(textarea(part.result,
          "proposal-rebase-merge-part proposal-rebase-merge-part--same", "Shared content"));
        return;
      }
      const section = document.createElement("section");
      section.className = "proposal-rebase-merge-part proposal-rebase-merge-part--change";
      section.setAttribute("aria-label", "Content discrepancy");
      const heading = document.createElement("div");
      heading.className = "proposal-rebase-merge-part__result-heading";
      const label = document.createElement("span");
      label.className = "label";
      label.textContent = "Resolved version";
      heading.append(label);
      section.append(
        heading,
        textarea(part.result, "", "Resolved content"),
        alternative("Accepted version", part.accepted,
          "This passage is not part of the accepted curriculum.",
          "proposal-rebase-merge-part__alternative--accepted"),
        alternative("Proposal version", part.proposal,
          "This passage is not part of the proposal.",
          "proposal-rebase-merge-part__alternative--proposal")
      );
      comparison.append(section);
    });
    if (window.autoResizeTextareas) window.autoResizeTextareas(comparison);
  }

  function initialize(root) {
    const editors = [];
    if (root.matches && root.matches("[data-merge-editor]")) editors.push(root);
    if (root.querySelectorAll) editors.push.apply(editors, root.querySelectorAll("[data-merge-editor]"));
    editors.forEach(function (editor) {
      if (editor.dataset.mergeEditorInitialized !== undefined) return;
      editor.dataset.mergeEditorInitialized = "";
      editor.querySelector("[data-merge-result]").value = editor.querySelector("[data-merge-proposal-source]").value;
      renderComparison(editor);
    });
  }

  initialize(document);
  document.addEventListener("DOMContentLoaded", function () { initialize(document); });
  document.addEventListener("htmx:load", function (event) { initialize(event.detail.elt || document); });
  document.addEventListener("htmx:afterSwap", function (event) { initialize(event.target || document); });
  document.addEventListener("view-switcher:change", function (event) {
    const editor = event.target.closest("[data-merge-editor]");
    if (editor && event.detail.value === "comparison") renderComparison(editor);
  });
  document.addEventListener("input", function (event) {
    const editor = event.target.closest && event.target.closest("[data-merge-editor]");
    if (editor && event.target.matches("[data-merge-comparison-part]")) syncResult(editor);
  });
  document.addEventListener("click", function (event) {
    const button = event.target.closest && event.target.closest("[data-use-merge-alternative]");
    if (!button) return;
    const change = button.closest(".proposal-rebase-merge-part--change");
    const result = change.querySelector("[data-merge-comparison-part]");
    result.value = button.closest(".proposal-rebase-merge-part__alternative")
      .querySelector("[data-alternative-source]").value;
    result.dispatchEvent(new Event("input", { bubbles: true }));
    result.focus();
  });
})();
