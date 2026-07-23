# Graph navigation

The graph is rendered from a server-selected local neighbourhood rather than
loading every curriculum unit into the client. Selecting a unit requests a new
neighbourhood through HTMX and replaces the workspace while preserving normal
links as a non-JavaScript fallback.

Layout hints from the currently visible graph are sent with navigation
requests. The server uses the previous order and lanes as starting hints for
the next layout, and stable view-transition names let shared nodes move between
their old and new positions.

Edges that continue beyond the visible neighbourhood are represented by
boundary arrows. Incoming boundaries connect to the left of a node and outgoing
boundaries connect to its right; their labels report how many adjacent units
are omitted.
