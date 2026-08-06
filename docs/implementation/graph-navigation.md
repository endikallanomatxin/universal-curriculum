# Graph navigation

The graph is rendered from a server-selected local neighbourhood rather than
loading every curriculum unit into the client. Selecting a unit requests a new
neighbourhood through HTMX and replaces the workspace while preserving normal
links as a non-JavaScript fallback.

Layout hints from the currently visible graph are sent with navigation
requests. The server uses the previous order and lanes as starting hints for
the next layout, and stable view-transition names let shared nodes move between
their old and new positions.

## Layout optimization

The server produces two deterministic topological seeds: one follows the
previous visible order and one ignores it in favour of graph structure. A
single shared budget explores at most 512 states in total, reachable through
adjacent independent units, so multi-start continuity does not multiply the
request cost.

Candidates first establish the minimum crossing count and edge span. Orders
with that crossing count and an edge span within a small deterministic quality
band remain eligible; continuity chooses among them. This means:

1. avoidable crossings are never preserved for continuity;
2. a clearly shorter graph wins over historical positioning; and
3. nearly equivalent arrangements minimize movement from the previous view.

Structural clarity therefore wins over preserving a poor historical layout,
while equivalent arrangements retain continuity. The fixed budget is
intentionally sized for the small neighbourhoods rendered by the application
rather than the complete curriculum.

When navigation has a focused unit, continuity is anchored around it. Movement
of that unit has weight four, movement of its immediate prerequisites and
dependents has weight two, and every other persistent unit has weight one.
Newly visible units have no previous position to preserve. These weights only
break ties inside the existing structural quality bands, so an anchor cannot
justify extra crossings, width or excessive bends.

The optimized order is then handed to the lane allocator to produce three
cheap candidates: one from a fresh state, one seeded by previous lanes and a
hybrid that keeps the fresh routing while moving persistent nodes towards a
previous lane only when that lane is clear at their row. Minimum width is
mandatory, bend distance has a small quality band, and movement breaks the
remaining tie. Ordering and routing deliberately remain separate, preserving
breathing room without allowing stale lane hints to dictate the result.

The browser groups sufficiently long dependencies with a shared source or
target into short common trunks and joins their branch points with continuous
Bézier curves.
Each edge remains an independent SVG path so relation highlighting can still
isolate it. Candidate curves are sampled against the measured node circles. On
collision, the renderer finds the nearest curve point, measures its local
tangent and proposes waypoints displaced just beyond the obstacle along either
normal. It iteratively retains only a waypoint that reduces the number of
collisions or their total penetration, with a fixed limit of six corrections.
Each accepted obstacle waypoint retains a tangent perpendicular to the radius
from the node centre to that waypoint, oriented to agree with the curve's prior
direction of travel. Regenerating the spline therefore cannot turn it back
towards the obstacle merely because neighbouring waypoints moved.
Ordinary edges use quadratic Bézier curves with their sole control point above
the target. They therefore have no independent source handle, while still
arriving vertically so arrowheads receive a clean, predictable tangent.
Every dependency begins with a quadratic segment at the centre of its source:
that segment is either the complete direct route, a shared outgoing trunk or
the first leg towards a collision detour. Cubics may only appear after it when
two subsequent tangencies must be controlled independently.
Shared trunks are considered per edge rather than per node. Only edges with
more than two visible units between their endpoints are eligible, and a trunk
is created only when at least two eligible edges share its source or target.
Nearby relations therefore remain independent even when the same node also has
distant relations. A branch leaving an outgoing shared trunk uses a vertical
departure tangent so it joins that trunk smoothly. The shared trunk is itself
a quadratic ending at the horizontal centre of its grouped destinations with
a vertical tangent; aligned groups naturally reduce to a straight trunk. When
both departure and arrival tangencies are constrained, one cubic segment
supplies their two independent controls. A collision detour composes a
quadratic that arrives at its first inferred waypoint with cubic segments
through any later waypoints.
Unconstrained intermediate tangents follow the normalized direction between
their neighbours; obstacle waypoints retain their clearance tangent, and the
destination retains its required vertical tangent. Control lengths are derived
from each segment chord and capped by the standard arrival handle.
An edge starts geometrically at the centre of its source node. The SVG is
stacked above later nodes, while a per-edge SVG mask cuts the path out inside
its own source circle. The curve therefore appears beneath its origin but may
remain visible over future units, without requiring a synthetic source handle.

Learn and Curriculum Modification render the same `curriculum-graph` and
`unit-navigation-search` templates. Server view models prepare consumer-specific
navigation and content URLs, current state and optional path targets; the
shared templates own the SVG markers, node and edge contract consumed by
`curriculum_graph.js`. The node body navigates to a new neighbourhood. Its
document action opens the unit content as a separate, explicit intent; one
interaction must not imply the other.

Navigation preserves the current workspace tools. When the content viewer is
closed, selecting another node only changes the focused neighbourhood. When
the viewer is already open, the same navigation keeps it open and synchronizes
its content with the newly focused unit. The shared server-side URL builder
applies this rule to graph nodes and search results without mobile-specific
client logic.

Edges that continue beyond the visible neighbourhood are represented by
boundary arrows. Incoming boundaries connect to the left of a node and outgoing
boundaries connect to its right; their labels report how many adjacent units
are omitted.

## Search and neighbourhood scope

Learn and Curriculum Modification expose a client-filtered unit search above
the graph. Results are ordinary HTMX-enhanced links: selecting one updates the
URL, focuses that unit and loads its local neighbourhood without opening its
content.
Search within a personal path is limited to units in that path; curriculum
editing searches the proposal's resulting curriculum.

The current focused neighbourhood includes one prerequisite level and two
dependent levels, with at most four neighbors selected at each traversal step.
Immediate co-prerequisites of direct dependents are included when there are no
more than three. Boundary arrows expose omitted relationships without loading
the missing nodes.

An unfocused full-curriculum view starts from graph entry points instead of
rendering every unit at once.

An open proposal instead starts in a proposal overview. Every unit touched by
one of its changes is included, including both dependency endpoints and every
recognition member, together with each affected unit's immediate prerequisites
and dependents. After navigation, the selected unit contributes the normal
neighbourhood while all affected units remain visible without bringing along
their own neighbours. Returning to `Proposal overview` removes the focus from
the URL and restores the initial context. A proposal without changes retains
the entry-point view so its author can still navigate the curriculum.
