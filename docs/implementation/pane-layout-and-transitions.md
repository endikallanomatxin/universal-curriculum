# Pane layout and transitions

Application pages use a horizontal sequence of panes that moves from broad
context on the left to the most specific content on the right. Pane sizing is
negotiated from declarations in the rendered HTML rather than from
page-specific viewport breakpoints.

## Pane modes

Every layout participant declares ordered width modes in rem through
`data-panel-modes`. The shared layout in `web/static/js/panel_layout.js`:

1. assigns every visible pane its smallest mode;
2. satisfies required modes from right to left;
3. attempts larger desired modes in the same order, collapsing lower-priority
   panes on the left when necessary;
4. distributes remaining space from right to left, respecting
   `data-panel-max`; and
5. lets `data-panel-fill` absorb otherwise unused space.

A zero-width mode is an emergency mode. It is not used as a normal donor mode
while a non-zero mode can still be retained. `data-panel-required-mode`
identifies the smallest mode that should normally remain usable, but a pane to
its right can block lower-priority expansions when its own required mode does
not fit.

Groups marked with `data-panel-group` expose the sum of their visible
children's required and desired widths to their parent. Layout therefore runs
from inner groups outward, allowing an outer context pane to collapse before
an inner editor loses the space it needs.

The home view is exceptional: navigation participates in the welcome
composition and fills the available width instead of using negotiated sidebar
modes.

## Responsive content

The negotiated mode controls the pane's role in the workspace. Content inside
a pane should respond to the space actually assigned to it, using container
queries where appropriate rather than duplicating the global allocation
algorithm with viewport media queries.

The shared navigation supports full, sidebar, icon-only and mobile-launcher
presentations. Content panes that cannot remain useful at narrow widths may
declare a breadcrumb mode. A breadcrumb retains the current contextual title
vertically while removing the rest of the pane content. Its vertical offset
must leave the mobile navigation launcher unobstructed.

## Pane operations

Opening an editor or detail from a pane normally replaces the visible panes to
the right of the pane containing the trigger. It does not accumulate unrelated
editors. Closing a right-hand pane restores the space to the remaining panes.

`web/static/js/panels.js` owns this interaction. New panel interactions should
reuse its declarative triggers and stable panel boundaries rather than adding
page-specific show/hide code.

## Motion and continuity

Width and position changes are expressed through shared CSS transitions. Keep
the element that owns a visual identity stable while its pane changes mode:
replacing icons, scaling their containing box or changing layout structure
mid-transition produces distortion or a final visual jump.

HTMX workspace navigation uses `transition:true`. Stable concepts receive
stable `view-transition-name` values so the browser can interpolate them
between old and new documents. Names must be unique in each rendered state.
Graph units use their persistent unit IDs; shell navigation uses fixed names.

When adding or removing a pane, preserve its resolved width during the
entering or closing phase. Let the remaining panes renegotiate around that
geometry, then remove the temporary state. This prevents the pane's internal
content from repeatedly reflowing while it moves. Prefer automatic layout and
View Transitions over imperative animation sequences.

Motion must degrade to an immediate, correct layout when View Transitions are
unavailable and respect `prefers-reduced-motion`.

## Recalculation

The shared layout observes group and shell size with `ResizeObserver`, pane
visibility with `MutationObserver`, and scroll-driven changes on the next
animation frame. HTMX swaps reinitialize the observers and trigger a complete
inner-to-outer negotiation.
