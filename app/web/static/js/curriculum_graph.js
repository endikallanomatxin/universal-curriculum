(function () {
  "use strict";

  const svgNamespace = "http://www.w3.org/2000/svg";

  function initializeCurriculumGraph(root) {
    if (!root || root.curriculumGraphInitialized) return;
    root.curriculumGraphInitialized = true;

    const layout = root.querySelector(".curriculum-graph__layout");
    const svg = root.querySelector(".curriculum-graph__svg");
    const pathLayer = root.querySelector("[data-curriculum-graph-paths]");
    if (!layout || !svg || !pathLayer) return;

    const nodes = new Map();
    root.querySelectorAll("[data-curriculum-node]").forEach(function (item) {
      nodes.set(item.dataset.unitId, item);
    });
    const edges = Array.from(root.querySelectorAll("[data-curriculum-edge]")).map(function (edge) {
      return {
        prerequisiteID: edge.dataset.prerequisiteId,
        dependentID: edge.dataset.dependentId
      };
    });
    const boundaries = Array.from(root.querySelectorAll("[data-curriculum-boundary]")).map(function (boundary) {
      return {
        unitID: boundary.dataset.unitId,
        direction: boundary.dataset.direction,
        count: Number(boundary.dataset.count) || 0
      };
    });
    const incoming = new Map();
    const outgoing = new Map();
    edges.forEach(function (edge) {
      if (!incoming.has(edge.dependentID)) incoming.set(edge.dependentID, []);
      if (!outgoing.has(edge.prerequisiteID)) outgoing.set(edge.prerequisiteID, []);
      incoming.get(edge.dependentID).push(edge);
      outgoing.get(edge.prerequisiteID).push(edge);
    });

    let renderedPaths = [];
    let highlightedUnitID = "";
    let frame = 0;

    function scheduleDraw() {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(draw);
    }

    function draw() {
      const sampleAnchor = root.querySelector("[data-curriculum-anchor]");
      const anchorWidth = sampleAnchor ? sampleAnchor.getBoundingClientRect().width : 12;
      const laneSpacing = Math.ceil(anchorWidth / 2 + 9);
      const contentGap = parseFloat(getComputedStyle(root).getPropertyValue("--curriculum-content-gap")) || 2;
      const boundaryGutter = boundaries.some(function (boundary) {
        return boundary.direction === "dependents";
      }) ? 34 : 0;
      const nodeLaneCount = Math.max(1, ...Array.from(nodes.values()).map(function (item) {
        return (Number(item.dataset.nodeLane) || 0) + 1;
      }));
      const graphWidth = 8 + contentGap + Math.ceil(anchorWidth) +
        (nodeLaneCount - 1) * laneSpacing + boundaryGutter;
      root.style.setProperty("--curriculum-graph-width", graphWidth + "px");
      root.style.setProperty("--curriculum-lane-spacing", laneSpacing + "px");
      root.style.setProperty("--curriculum-boundary-gutter", boundaryGutter + "px");

      window.requestAnimationFrame(function () {
        const layoutBox = layout.getBoundingClientRect();
        svg.setAttribute("viewBox", "0 0 " + layoutBox.width + " " + layoutBox.height);
        svg.setAttribute("width", layoutBox.width);
        svg.setAttribute("height", layoutBox.height);
        pathLayer.replaceChildren();
        renderedPaths = [];

        function anchorPoint(item) {
          const anchor = item.querySelector("[data-curriculum-anchor]");
          if (!anchor) return null;
          const box = anchor.getBoundingClientRect();
          return {
            x: box.left - layoutBox.left + box.width / 2,
            y: box.top - layoutBox.top + box.height / 2,
            width: box.width,
            height: box.height
          };
        }

        function edgePath(source, target) {
          const sourceY = source.y + source.height / 2;
          const targetY = target.y - target.height / 2 - 5;
          return "M " + source.x + " " + sourceY +
            " L " + target.x + " " + targetY;
        }

        edges.forEach(function (edge) {
          const prerequisite = nodes.get(edge.prerequisiteID);
          const dependent = nodes.get(edge.dependentID);
          if (!prerequisite || !dependent) return;
          const source = anchorPoint(prerequisite);
          const target = anchorPoint(dependent);
          if (!source || !target) return;
          const path = document.createElementNS(svgNamespace, "path");
          path.setAttribute("d", edgePath(source, target));
          path.setAttribute("marker-end", "url(#" + pathLayer.dataset.arrowMarker + ")");
          path.classList.add("curriculum-graph__edge");
          pathLayer.appendChild(path);
          renderedPaths.push({ edge: edge, path: path });
        });
        boundaries.forEach(function (boundary) {
          const item = nodes.get(boundary.unitID);
          const point = item && anchorPoint(item);
          if (!point) return;
          const isPrerequisite = boundary.direction === "prerequisites";
          const nodeEdgeX = point.x + (isPrerequisite ? -point.width / 2 : point.width / 2);
          const outerX = isPrerequisite
            ? 4
            : Math.max(nodeEdgeX + 16, graphWidth - 4);
          const y = point.y;
          const startX = isPrerequisite ? outerX : nodeEdgeX;
          const endX = isPrerequisite ? nodeEdgeX : outerX;
          const path = document.createElementNS(svgNamespace, "path");
          path.setAttribute("d", "M " + startX + " " + y + " H " + endX);
          path.setAttribute("marker-end", "url(#" + pathLayer.dataset.boundaryArrowMarker + ")");
          path.classList.add("curriculum-graph__edge", "curriculum-graph__edge--boundary");
          pathLayer.appendChild(path);
          const count = document.createElementNS(svgNamespace, "text");
          count.setAttribute("x", isPrerequisite ? outerX + 2 : outerX - 2);
          count.setAttribute("y", y - 9);
          count.setAttribute("text-anchor", isPrerequisite ? "start" : "end");
          count.classList.add("curriculum-graph__boundary-count");
          count.textContent = "+" + boundary.count;
          pathLayer.appendChild(count);
          renderedPaths.push({ boundary: boundary, path: path, count: count });
        });
        applyHighlight(highlightedUnitID);
      });
    }

    function collectRelations(unitID) {
      const nodeDistances = new Map([[unitID, 0]]);
      const edgeDistances = new Map();
      function visit(adjacency, nextKey) {
        const distances = new Map([[unitID, 0]]);
        const pending = [unitID];
        while (pending.length) {
          const currentID = pending.shift();
          const currentDistance = distances.get(currentID);
          (adjacency.get(currentID) || []).forEach(function (edge) {
            const key = edge.prerequisiteID + ":" + edge.dependentID;
            const knownEdgeDistance = edgeDistances.get(key);
            if (knownEdgeDistance === undefined || currentDistance < knownEdgeDistance) {
              edgeDistances.set(key, currentDistance);
            }
            const nextID = edge[nextKey];
            const nextDistance = currentDistance + 1;
            const knownDirectionDistance = distances.get(nextID);
            if (knownDirectionDistance === undefined || nextDistance < knownDirectionDistance) {
              distances.set(nextID, nextDistance);
              pending.push(nextID);
            }
            const knownNodeDistance = nodeDistances.get(nextID);
            if (knownNodeDistance === undefined || nextDistance < knownNodeDistance) {
              nodeDistances.set(nextID, nextDistance);
            }
          });
        }
      }
      visit(incoming, "prerequisiteID");
      visit(outgoing, "dependentID");
      return { nodes: nodeDistances, edges: edgeDistances };
    }

    function applyHighlight(unitID) {
      const selected = unitID && nodes.has(unitID);
      const relations = selected ? collectRelations(unitID) : { nodes: new Map(), edges: new Map() };
      root.classList.toggle("is-filtering", Boolean(selected));
      nodes.forEach(function (item, id) {
        const distance = relations.nodes.get(id);
        item.classList.toggle("is-related", distance !== undefined);
        if (distance === undefined) delete item.dataset.relationDistance;
        else item.dataset.relationDistance = String(Math.min(distance, 4));
      });
      renderedPaths.forEach(function (rendered) {
        if (rendered.boundary) {
          const distance = relations.nodes.get(rendered.boundary.unitID);
          const related = distance !== undefined;
          rendered.path.classList.toggle("is-related", related);
          if (related) rendered.path.dataset.relationDistance = String(Math.min(distance, 4));
          else delete rendered.path.dataset.relationDistance;
          if (rendered.count) {
            rendered.count.classList.toggle("is-related", related);
            if (related) rendered.count.dataset.relationDistance = String(Math.min(distance, 4));
            else delete rendered.count.dataset.relationDistance;
          }
          return;
        }
        const key = rendered.edge.prerequisiteID + ":" + rendered.edge.dependentID;
        const distance = relations.edges.get(key);
        const related = distance !== undefined;
        rendered.path.classList.toggle("is-related", related);
        if (related) rendered.path.dataset.relationDistance = String(Math.min(distance, 4));
        else delete rendered.path.dataset.relationDistance;
        rendered.path.setAttribute("marker-end", "url(#" +
          (related ? pathLayer.dataset.highlightedArrowMarker : pathLayer.dataset.arrowMarker) + ")");
      });
    }

    root.addEventListener("mouseover", function (event) {
      const item = event.target.closest("[data-curriculum-node]");
      if (!item) return;
      const previousItem = event.relatedTarget && event.relatedTarget.closest ?
        event.relatedTarget.closest("[data-curriculum-node]") : null;
      if (previousItem === item) return;
      highlightedUnitID = item.dataset.unitId;
      applyHighlight(highlightedUnitID);
    });
    root.addEventListener("mouseout", function (event) {
      const item = event.target.closest("[data-curriculum-node]");
      if (!item) return;
      const nextItem = event.relatedTarget && event.relatedTarget.closest ?
        event.relatedTarget.closest("[data-curriculum-node]") : null;
      if (nextItem === item) return;
      highlightedUnitID = "";
      applyHighlight("");
    });
    root.addEventListener("focusin", function (event) {
      const item = event.target.closest("[data-curriculum-node]");
      if (!item) return;
      highlightedUnitID = item.dataset.unitId;
      applyHighlight(highlightedUnitID);
    });
    root.addEventListener("focusout", function () {
      window.requestAnimationFrame(function () {
        if (!root.contains(document.activeElement)) {
          highlightedUnitID = "";
          applyHighlight("");
        }
      });
    });
    root.addEventListener("keydown", function (event) {
      if (event.key !== "Escape") return;
      highlightedUnitID = "";
      applyHighlight("");
      if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
    });

    if (window.ResizeObserver) new ResizeObserver(scheduleDraw).observe(layout);
    else window.addEventListener("resize", scheduleDraw);
    scheduleDraw();
  }

  function initializeGraphSearch(search) {
    if (!search || search.graphSearchInitialized) return;
    search.graphSearchInitialized = true;
    const input = search.querySelector("[data-graph-search-input]");
    const results = search.querySelector("[data-graph-search-results]");
    const options = Array.from(search.querySelectorAll("[data-graph-search-option]"));
    const empty = search.querySelector("[data-graph-search-empty]");
    if (!input || !results) return;

    function normalized(value) {
      return value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLocaleLowerCase();
    }

    function filter() {
      const query = normalized(input.value.trim());
      let matches = 0;
      options.forEach(function (option) {
        const visible = normalized(option.dataset.unitName).includes(query);
        option.hidden = !visible;
        if (visible) matches += 1;
      });
      if (empty) empty.hidden = matches > 0;
      results.hidden = false;
      input.setAttribute("aria-expanded", "true");
    }

    function close() {
      results.hidden = true;
      input.setAttribute("aria-expanded", "false");
    }

    input.addEventListener("focus", filter);
    input.addEventListener("input", filter);
    input.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        close();
        input.blur();
      } else if (event.key === "ArrowDown") {
        const first = options.find(function (option) { return !option.hidden; });
        if (first) {
          event.preventDefault();
          first.focus();
        }
      }
    });
    options.forEach(function (option) {
      option.addEventListener("click", close);
    });
    document.addEventListener("pointerdown", function (event) {
      if (!search.contains(event.target)) close();
    });
  }

  function initializeAll(root) {
    if (root.matches && root.matches("[data-curriculum-graph]")) initializeCurriculumGraph(root);
    root.querySelectorAll("[data-curriculum-graph]").forEach(initializeCurriculumGraph);
    if (root.matches && root.matches("[data-graph-search]")) initializeGraphSearch(root);
    root.querySelectorAll("[data-graph-search]").forEach(initializeGraphSearch);
  }

  document.addEventListener("DOMContentLoaded", function () { initializeAll(document); });
  document.addEventListener("htmx:load", function (event) { initializeAll(event.detail.elt || document); });
  document.addEventListener("htmx:configRequest", function (event) {
    const trigger = event.detail.elt;
    const explorer = trigger && trigger.closest && trigger.closest("#curriculum-explorer");
    if (!explorer) return;
    const nodes = Array.from(explorer.querySelectorAll("[data-curriculum-node]"));
    event.detail.parameters.layout_order = nodes.map(function (node) {
      return node.dataset.unitId;
    }).join(",");
    event.detail.parameters.layout_lanes = nodes.map(function (node) {
      return node.dataset.unitId + ":" + (Number(node.dataset.nodeLane) || 0);
    }).join(",");
  });
})();
