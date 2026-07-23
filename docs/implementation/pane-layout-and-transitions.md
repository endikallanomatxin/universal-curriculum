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

A zero-width mode is reserved for the narrow mobile composition and does not
participate in normal desktop and tablet negotiation. `data-panel-required-mode`
identifies the smallest mode that should normally remain usable, but a pane to
its right can block lower-priority expansions when its own required mode does
not fit.

Content and form panes declare that emergency state explicitly as
`collapsed:0`. They also expose a concise contextual label through
`data-panel-breadcrumb`; the label describes the pane's current subject rather
than its generic type whenever that subject is known.

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

At `42rem` and below, the allocator switches to the mobile composition: only
the rightmost visible pane in each group receives width and every pane to its
left uses its zero-width mode. The workspace turns the visible panes'
`data-panel-breadcrumb` labels into one lightweight trail fixed across its top.
This replaces vertical breadcrumb panes on phones without duplicating page
markup or domain-specific layout logic. Opening, closing and replacing panes
rebuilds the trail automatically. Each trail segment is actionable: selecting
an earlier segment closes the panels to its right through the shared panel
controller and promotes that context back to the full mobile viewport.
The empty trail container is rendered with every workspace so its insertion
does not alter the new View Transition snapshot. Named transition descendants
inside zero-width panes are suppressed: a graph that becomes contextual should
fade in place rather than interpolate toward its reflowed, hidden geometry.

## Pane operations

Opening an editor or detail from a pane normally replaces the visible panes to
the right of the pane containing the trigger. It does not accumulate unrelated
editors. Closing a right-hand pane restores the space to the remaining panes.

`web/static/js/panels.js` owns this interaction. New panel interactions should
reuse its declarative triggers and stable panel boundaries rather than adding
page-specific show/hide code. It dispatches `panel:configure` before revealing
a panel; domain controllers populate their editor through that event. The
shared controller must not contain proposal, curriculum or learning-path
configuration.

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

## History restoration

HTMX history snapshots serialize DOM attributes but not JavaScript listeners or
element properties. Controller initialization guards therefore live as
properties on the actual DOM node; they must not use `data-*-initialized`
attributes. A restored node receives fresh listeners, while repeated
initialization of the same live node remains idempotent. Generated controls
such as mobile breadcrumbs follow the same rule for their render signatures.

## Recalculation

The shared layout observes group and shell size with `ResizeObserver`, pane
visibility with `MutationObserver`, and scroll-driven changes on the next
animation frame. HTMX swaps reinitialize the observers and trigger a complete
inner-to-outer negotiation. Observer callbacks are coalesced into one animation
frame, and the allocator only writes geometry or breadcrumb markup when its
resolved value changed. The mobile media query also requests a layout directly
when the breakpoint changes. These constraints prevent layout writes from
feeding an unbounded resize loop.
