# Graph navigation

The graph is rendered from a server-selected local neighbourhood rather than
loading every curriculum unit into the client. Selecting a unit requests a new
neighbourhood through HTMX and replaces the workspace while preserving normal
links as a non-JavaScript fallback.

Layout hints from the currently visible graph are sent with navigation
requests. The server uses the previous order and lanes as starting hints for
the next layout, and stable view-transition names let shared nodes move between
their old and new positions.

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
editing searches the complete published curriculum.

The current focused neighbourhood includes one prerequisite level and two
dependent levels, with at most four neighbors selected at each traversal step.
Immediate co-prerequisites of direct dependents are included when there are no
more than three. Boundary arrows expose omitted relationships without loading
the missing nodes.

An unfocused full-curriculum view starts from graph entry points instead of
rendering every unit at once.
