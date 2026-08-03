(function () {
  "use strict";

  const svgNamespace = "http://www.w3.org/2000/svg";

  function initializeCurriculumGraph(root) {
    if (!root || root.curriculumGraphInitialized) return;
    root.curriculumGraphInitialized = true;

    const layout = root.querySelector(".curriculum-graph__layout");
    const svg = root.querySelector(".curriculum-graph__svg");
    const definitions = svg && svg.querySelector("defs");
    const pathLayer = root.querySelector("[data-curriculum-graph-paths]");
    if (!layout || !svg || !definitions || !pathLayer) return;

    const nodes = new Map();
    root.querySelectorAll("[data-curriculum-node]").forEach(function (item) {
      nodes.set(item.dataset.unitId, item);
    });
    const edges = Array.from(root.querySelectorAll("[data-curriculum-edge]")).map(function (edge) {
      return {
        prerequisiteID: edge.dataset.prerequisiteId,
        dependentID: edge.dataset.dependentId,
        proposalState: edge.dataset.proposalState || ""
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
        definitions.querySelectorAll("[data-curriculum-edge-mask]").forEach(function (mask) {
          mask.remove();
        });
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

        const nodePoints = new Map();
        nodes.forEach(function (item, unitID) {
          nodePoints.set(unitID, anchorPoint(item));
        });
        const orderedPoints = Array.from(nodePoints.values()).filter(Boolean);
        const rowSpacing = orderedPoints.length > 1
          ? orderedPoints[1].y - orderedPoints[0].y
          : 48;
        const branchInset = rowSpacing * 0.35;
        const edgeKey = function (edge) {
          return edge.prerequisiteID + ":" + edge.dependentID;
        };
        function hasUsefulSharedSpan(edge) {
          const source = nodePoints.get(edge.prerequisiteID);
          const target = nodePoints.get(edge.dependentID);
          if (!source || !target) return false;
          let unitsBetween = 0;
          for (const point of orderedPoints) {
            if (point.y > source.y && point.y < target.y) unitsBetween += 1;
          }
          return unitsBetween > 2;
        }
        const sourceYs = new Map();
        const targetYs = new Map();
        nodePoints.forEach(function (point, unitID) {
          if (!point) return;
          sourceYs.set(unitID, point.y);
          targetYs.set(unitID, point.y - point.height / 2 - 5);
        });
        const outgoingHubs = new Map();
        const groupedOutgoingEdges = new Set();
        outgoing.forEach(function (sourceEdges, unitID) {
          const groupedEdges = sourceEdges.filter(hasUsefulSharedSpan);
          if (groupedEdges.length < 2) return;
          const sourceY = sourceYs.get(unitID);
          const firstTargetY = Math.min(...groupedEdges.map(function (edge) {
            return targetYs.get(edge.dependentID);
          }));
          const targetXTotal = groupedEdges.reduce(function (total, edge) {
            return total + nodePoints.get(edge.dependentID).x;
          }, 0);
          outgoingHubs.set(unitID, {
            x: targetXTotal / groupedEdges.length,
            y: Math.max(sourceY, firstTargetY - branchInset)
          });
          groupedEdges.forEach(function (edge) {
            groupedOutgoingEdges.add(edgeKey(edge));
          });
        });
        const incomingHubs = new Map();
        const groupedIncomingEdges = new Set();
        incoming.forEach(function (targetEdges, unitID) {
          const groupedEdges = targetEdges.filter(hasUsefulSharedSpan);
          if (groupedEdges.length < 2) return;
          const targetY = targetYs.get(unitID);
          const lastSourceY = Math.max(...groupedEdges.map(function (edge) {
            return sourceYs.get(edge.prerequisiteID);
          }));
          incomingHubs.set(unitID, Math.min(targetY, lastSourceY + branchInset));
          groupedEdges.forEach(function (edge) {
            groupedIncomingEdges.add(edgeKey(edge));
          });
        });

        const verticalHandle = 28;

        function directQuadraticPath(source, target) {
          const sourceY = source.y;
          const targetY = target.y - target.height / 2 - 5;
          return "M " + source.x + " " + sourceY +
            " Q " + target.x + " " + (targetY - verticalHandle) +
            " " + target.x + " " + targetY;
        }

        function verticalEndpointCubic(sourceX, sourceY, targetX, targetY, handle) {
          return " C " + sourceX + " " + (sourceY + handle) +
            " " + targetX + " " + (targetY - handle) +
            " " + targetX + " " + targetY;
        }

        function edgePath(edge, source, target) {
          const sourceY = source.y;
          const targetY = target.y - target.height / 2 - 5;
          const key = edgeKey(edge);
          const groupedAtSource = groupedOutgoingEdges.has(key);
          const groupedAtTarget = groupedIncomingEdges.has(key);
          const sourceHub = groupedAtSource
            ? outgoingHubs.get(edge.prerequisiteID)
            : { x: source.x, y: sourceY };
          const targetHubY = groupedAtTarget ? incomingHubs.get(edge.dependentID) : targetY;
          const hubSpan = targetHubY - sourceHub.y;
          if (hubSpan < branchInset) {
            return directQuadraticPath(source, target);
          }
          let path = "M " + source.x + " " + sourceY;
          if (groupedAtSource) {
            path += " Q " + sourceHub.x + " " + (sourceHub.y - verticalHandle) +
              " " + sourceHub.x + " " + sourceHub.y;
          }
          if (groupedAtSource) {
            path += verticalEndpointCubic(
              sourceHub.x, sourceHub.y, target.x, targetHubY, verticalHandle
            );
          } else {
            path += " Q " + target.x + " " + (targetHubY - verticalHandle) +
              " " + target.x + " " + targetHubY;
          }
          if (targetY > targetHubY) {
            path += " V " + targetY;
          }
          return path;
        }

        function normalizedVector(x, y) {
          const length = Math.hypot(x, y);
          if (length === 0) return { x: 0, y: 1 };
          return { x: x / length, y: y / length };
        }

        function obstacleSplinePath(source, target, waypoints) {
          const targetPoint = {
            x: target.x,
            y: target.y - target.height / 2 - 5
          };
          const points = [{ x: source.x, y: source.y }]
            .concat(waypoints.slice().sort(function (left, right) { return left.y - right.y; }))
            .concat([targetPoint]);
          if (points.length === 2) return directQuadraticPath(source, target);

          const tangents = new Array(points.length);
          for (let index = 1; index < points.length - 1; index += 1) {
            tangents[index] = points[index].tangent || normalizedVector(
              points[index + 1].x - points[index - 1].x,
              Math.max(1, points[index + 1].y - points[index - 1].y)
            );
          }
          tangents[points.length - 1] = { x: 0, y: 1 };

          const firstTarget = points[1];
          const firstTangent = tangents[1];
          const firstSpan = Math.hypot(
            firstTarget.x - points[0].x,
            firstTarget.y - points[0].y
          );
          const firstHandle = Math.min(verticalHandle, firstSpan / 3);
          let path = "M " + points[0].x + " " + points[0].y +
            " Q " + (firstTarget.x - firstTangent.x * firstHandle) + " " +
            (firstTarget.y - firstTangent.y * firstHandle) + " " +
            firstTarget.x + " " + firstTarget.y;

          for (let index = 1; index < points.length - 1; index += 1) {
            const start = points[index];
            const end = points[index + 1];
            const span = Math.hypot(end.x - start.x, end.y - start.y);
            const handle = Math.min(verticalHandle, span / 3);
            const startTangent = tangents[index];
            const endTangent = tangents[index + 1];
            path += " C " + (start.x + startTangent.x * handle) + " " +
              (start.y + startTangent.y * handle) + " " +
              (end.x - endTangent.x * handle) + " " +
              (end.y - endTangent.y * handle) + " " + end.x + " " + end.y;
          }
          return path;
        }

        function pathNodeCollisions(path, edge) {
          const length = path.getTotalLength();
          const collisions = [];
          for (const [unitID, point] of nodePoints) {
            if (!point || unitID === edge.prerequisiteID || unitID === edge.dependentID) continue;
            const radius = point.width / 2 + 2;
            let closest = null;
            for (let distance = 0; distance <= length; distance += 2) {
              const sample = path.getPointAtLength(distance);
              const separation = Math.hypot(sample.x - point.x, sample.y - point.y);
              if (!closest || separation < closest.separation) {
                closest = { distance: distance, point: sample, separation: separation };
              }
            }
            if (closest && closest.separation <= radius) {
              collisions.push({
                unitID: unitID,
                obstacle: point,
                radius: radius,
                closest: closest,
                penetration: radius - closest.separation
              });
            }
          }
          return collisions;
        }

        function collisionScore(collisions) {
          return {
            count: collisions.length,
            penetration: collisions.reduce(function (total, collision) {
              return total + collision.penetration;
            }, 0)
          };
        }

        function improvesCollisionScore(candidate, current) {
          return candidate.count < current.count ||
            (candidate.count === current.count && candidate.penetration < current.penetration - 0.25);
        }

        function collisionWaypointCandidates(path, collision, source, target) {
          const length = path.getTotalLength();
          const distance = collision.closest.distance;
          const before = path.getPointAtLength(Math.max(0, distance - 3));
          const after = path.getPointAtLength(Math.min(length, distance + 3));
          const tangent = normalizedVector(after.x - before.x, after.y - before.y);
          const normal = { x: -tangent.y, y: tangent.x };
          const offset = {
            x: collision.closest.point.x - collision.obstacle.x,
            y: collision.closest.point.y - collision.obstacle.y
          };
          const projection = offset.x * normal.x + offset.y * normal.y;
          const perpendicularSquared = Math.max(
            0,
            offset.x * offset.x + offset.y * offset.y - projection * projection
          );
          const clearance = 4;
          const radiusAlongNormal = Math.sqrt(Math.max(
            0,
            Math.pow(collision.radius + clearance, 2) - perpendicularSquared
          ));
          const targetY = target.y - target.height / 2 - 5;
          return [-1, 1].map(function (direction) {
            const displacement = -projection + direction * radiusAlongNormal;
            const waypoint = {
              x: Math.max(4, Math.min(graphWidth - 4,
                collision.closest.point.x + normal.x * displacement)),
              y: Math.max(source.y + 2, Math.min(targetY - 2,
                collision.closest.point.y + normal.y * displacement))
            };
            const radius = normalizedVector(
              waypoint.x - collision.obstacle.x,
              waypoint.y - collision.obstacle.y
            );
            let waypointTangent = { x: -radius.y, y: radius.x };
            if (waypointTangent.x * tangent.x + waypointTangent.y * tangent.y < 0) {
              waypointTangent = { x: -waypointTangent.x, y: -waypointTangent.y };
            }
            waypoint.tangent = waypointTangent;
            return waypoint;
          });
        }

        function avoidNodeCollisions(path, edge, source, target) {
          let collisions = pathNodeCollisions(path, edge);
          if (collisions.length === 0) return;
          let score = collisionScore(collisions);
          let waypoints = [];

          for (let iteration = 0; iteration < 6 && collisions.length > 0; iteration += 1) {
            const currentPath = path.getAttribute("d");
            let best = null;
            for (const collision of collisions) {
              path.setAttribute("d", currentPath);
              const candidates = collisionWaypointCandidates(path, collision, source, target);
              for (const waypoint of candidates) {
                const candidateWaypoints = waypoints.concat([waypoint]);
                const candidatePath = obstacleSplinePath(source, target, candidateWaypoints);
                path.setAttribute("d", candidatePath);
                const candidateCollisions = pathNodeCollisions(path, edge);
                const candidateScore = collisionScore(candidateCollisions);
                const movement = Math.hypot(
                  waypoint.x - collision.closest.point.x,
                  waypoint.y - collision.closest.point.y
                );
                if (!improvesCollisionScore(candidateScore, score)) continue;
                if (!best || improvesCollisionScore(candidateScore, best.score) ||
                    candidateScore.count === best.score.count &&
                    Math.abs(candidateScore.penetration - best.score.penetration) <= 0.25 &&
                    movement < best.movement) {
                  best = {
                    path: candidatePath,
                    waypoints: candidateWaypoints,
                    collisions: candidateCollisions,
                    score: candidateScore,
                    movement: movement
                  };
                }
              }
            }
            if (!best) {
              path.setAttribute("d", currentPath);
              break;
            }
            path.setAttribute("d", best.path);
            waypoints = best.waypoints;
            collisions = best.collisions;
            score = best.score;
          }
        }

        function maskEdgeSource(path, source, index) {
          const maskID = pathLayer.dataset.arrowMarker + "-source-mask-" + index;
          const mask = document.createElementNS(svgNamespace, "mask");
          mask.id = maskID;
          mask.dataset.curriculumEdgeMask = "";
          mask.setAttribute("maskUnits", "userSpaceOnUse");
          mask.setAttribute("x", "0");
          mask.setAttribute("y", "0");
          mask.setAttribute("width", layoutBox.width);
          mask.setAttribute("height", layoutBox.height);

          const visibleArea = document.createElementNS(svgNamespace, "rect");
          visibleArea.setAttribute("width", layoutBox.width);
          visibleArea.setAttribute("height", layoutBox.height);
          visibleArea.setAttribute("fill", "white");
          mask.appendChild(visibleArea);

          const sourceCutout = document.createElementNS(svgNamespace, "circle");
          sourceCutout.setAttribute("cx", source.x);
          sourceCutout.setAttribute("cy", source.y);
          sourceCutout.setAttribute("r", source.width / 2 + 1);
          sourceCutout.setAttribute("fill", "black");
          mask.appendChild(sourceCutout);

          definitions.appendChild(mask);
          path.setAttribute("mask", "url(#" + maskID + ")");
        }

        edges.forEach(function (edge, edgeIndex) {
          const source = nodePoints.get(edge.prerequisiteID);
          const target = nodePoints.get(edge.dependentID);
          if (!source || !target) return;
          const path = document.createElementNS(svgNamespace, "path");
          path.setAttribute("d", edgePath(edge, source, target));
          const arrowMarker = edge.proposalState
            ? pathLayer.dataset.proposedArrowMarker
            : pathLayer.dataset.arrowMarker;
          path.setAttribute("marker-end", "url(#" + arrowMarker + ")");
          path.classList.add("curriculum-graph__edge");
          if (edge.proposalState) {
            path.classList.add("curriculum-graph__edge--proposed", "is-proposal-" + edge.proposalState);
          }
          pathLayer.appendChild(path);
          avoidNodeCollisions(path, edge, source, target);
          maskEdgeSource(path, source, edgeIndex);
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
        const arrowMarker = related
          ? pathLayer.dataset.highlightedArrowMarker
          : rendered.edge.proposalState
            ? pathLayer.dataset.proposedArrowMarker
            : pathLayer.dataset.arrowMarker;
        rendered.path.setAttribute("marker-end", "url(#" + arrowMarker + ")");
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

  function initializeCurriculumGraphs(root) {
    if (root.matches && root.matches("[data-curriculum-graph]")) initializeCurriculumGraph(root);
    root.querySelectorAll("[data-curriculum-graph]").forEach(initializeCurriculumGraph);
  }

  document.addEventListener("DOMContentLoaded", function () { initializeCurriculumGraphs(document); });
  document.addEventListener("htmx:load", function (event) {
    initializeCurriculumGraphs(event.detail.elt || document);
  });
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
