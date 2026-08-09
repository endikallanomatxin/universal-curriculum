(function () {
  "use strict";

  const svgNamespace = "http://www.w3.org/2000/svg";

  function initializeRebaseGraph(root) {
    if (!root || root.rebaseGraphInitialized) return;
    root.rebaseGraphInitialized = true;

    const layout = root.querySelector(".proposal-rebase-graph__layout");
    const svg = root.querySelector(".proposal-rebase-graph__svg");
    const pathLayer = root.querySelector("[data-rebase-graph-paths]");
    if (!layout || !svg || !pathLayer) return;

    const nodes = new Map();
    root.querySelectorAll("[data-rebase-node]").forEach(function (item) {
      nodes.set(item.dataset.rebaseNode, item);
    });
    const edges = Array.from(root.querySelectorAll("[data-rebase-edge]")).map(function (edge) {
      return {
        source: edge.dataset.source,
        target: edge.dataset.target
      };
    });
    let frame = 0;

    function scheduleDraw() {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(draw);
    }

    function anchorPoint(item, layoutBox) {
      const anchor = item && item.querySelector("[data-rebase-anchor]");
      if (!anchor) return null;
      const box = anchor.getBoundingClientRect();
      return {
        x: box.left - layoutBox.left + box.width / 2,
        y: box.top - layoutBox.top + box.height / 2,
        height: box.height
      };
    }

    function edgePath(source, target) {
      const direction = target.y >= source.y ? 1 : -1;
      const sourceY = source.y + direction * source.height / 2;
      const targetY = target.y - direction * (target.height / 2 + 5);
      const easing = 28;
      return "M " + source.x + " " + sourceY +
        " C " + source.x + " " + (sourceY + direction * easing) +
        " " + target.x + " " + (targetY - direction * easing) +
        " " + target.x + " " + targetY;
    }

    function draw() {
      const layoutBox = layout.getBoundingClientRect();
      svg.setAttribute("viewBox", "0 0 " + layoutBox.width + " " + layoutBox.height);
      svg.setAttribute("width", layoutBox.width);
      svg.setAttribute("height", layoutBox.height);
      pathLayer.replaceChildren();
      const points = new Map();
      nodes.forEach(function (item, id) {
        points.set(id, anchorPoint(item, layoutBox));
      });
      edges.forEach(function (edge) {
        const source = points.get(edge.source);
        const target = points.get(edge.target);
        if (!source || !target) return;
        const path = document.createElementNS(svgNamespace, "path");
        path.setAttribute("d", edgePath(source, target));
        path.setAttribute("marker-end", "url(#" + pathLayer.dataset.arrowMarker + ")");
        path.classList.add("proposal-rebase-graph__edge");
        pathLayer.appendChild(path);
      });
    }

    const observer = new ResizeObserver(scheduleDraw);
    observer.observe(layout);
    scheduleDraw();
  }

  function initialize(root) {
    if (root.matches && root.matches("[data-rebase-graph]")) initializeRebaseGraph(root);
    if (root.querySelectorAll) root.querySelectorAll("[data-rebase-graph]").forEach(initializeRebaseGraph);
  }

  initialize(document);
  document.addEventListener("DOMContentLoaded", function () { initialize(document); });
  document.addEventListener("htmx:load", function (event) { initialize(event.detail.elt || document); });
  document.addEventListener("htmx:afterSwap", function (event) { initialize(event.target || document); });
})();
