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

The server first produces a deterministic topological order and then searches
a bounded set of other valid orders reachable through adjacent independent
units. Candidates are compared lexicographically:

1. fewer interleaving dependency intervals;
2. shorter total dependency span; and
3. less movement from the previous visible order.

Structural clarity therefore wins over preserving a poor historical layout,
while equivalent arrangements retain continuity. The search explores and
retains at most 512 candidates, which is intentionally sized for the small
neighbourhoods rendered by the application rather than the complete
curriculum.

The optimized order is then handed to the established lane allocator. Ordering
and routing deliberately remain separate: the bounded search improves the
topology without subsequently packing or shifting lanes according to indirect
geometric metrics. This preserves the allocator's existing breathing room and
keeps dense converging branches from becoming a compact braid.

The browser groups dependencies with a shared source or target into short
common trunks and joins their branch points with monotone cubic Bézier curves.
Each edge remains an independent SVG path so relation highlighting can still
isolate it. A candidate curve is sampled against the measured node circles and
falls back to its straight segment if bundling would introduce a node collision.

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
