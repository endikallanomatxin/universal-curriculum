# Pane layout and transitions

Application pages use a horizontal sequence of panes that moves from broad
context on the left to the most specific content on the right. Pane sizing is
negotiated from declarations in the rendered HTML rather than from
page-specific viewport breakpoints.

## Pane modes

Every layout participant declares ordered width modes in rem through
`data-panel-modes`. For `.ui-pane` participants these values describe usable
content width; the allocator adds the active mode's horizontal padding when it
calculates the pane's outer width. Widths on structural group participants
continue to describe their total allocated width. The shared layout in
`web/static/js/panel_layout.js`:

1. assigns every visible pane its smallest mode;
2. satisfies required modes from right to left;
3. attempts larger desired modes in the same order, collapsing lower-priority
   panes on the left when necessary;
4. distributes remaining space from right to left; and
5. lets `data-panel-fill` absorb all otherwise unused space.

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

The outer app shell is always a panel group, including on the home view. Home
presentation CSS lets its sole visible navigation pane fill the welcome
composition, but the pane remains initialized by the allocator. A
workspace-only HTMX swap can therefore add the workspace to the same retained
group and immediately negotiate navigation plus content; home is a visual mode
of the shared shell, not a separate layout tree.

## Responsive content

The negotiated mode controls the pane's role in the workspace. Content inside
a pane should respond to the space actually assigned to it, using container
queries where appropriate rather than duplicating the global allocation
algorithm with viewport media queries.

Normal content panes in the same group share one horizontal padding value. The
allocator first reserves the shared minimum outside each pane's declared
content width, then derives any increase from the space left after mode
negotiation. Constrained groups keep the minimum, while genuinely spare width
increases every pane's padding together up to the shared maximum. The terminal
fill pane may then absorb the remaining width without making its own spacing
differ from its siblings. Vertical padding remains viewport-responsive, and
structural modes such as compact, breadcrumb and mobile retain their explicit
padding; their declared content widths likewise exclude that padding.

Panel capacity and content measure are independent. A terminal pane normally
declares `data-panel-fill` and may therefore grow to the remaining workspace
width without a hard maximum. Its `.ui-pane__inner` declares a shared readable
measure through `data-pane-content-width`: `narrow` for navigation and forms,
`reading` for long-form material, the default `standard` measure, or `wide` for
graphs and dense workspaces. Content stays aligned to the pane's leading edge;
unused width remains part of the pane rather than becoming an unrelated gap in
the shell.

The shared navigation supports full, sidebar, icon-only and mobile-launcher
presentations. Content panes that cannot remain useful at narrow widths may
declare a breadcrumb mode. A breadcrumb retains the current contextual title
vertically while removing the rest of the pane content. Its vertical offset
must leave the mobile navigation launcher unobstructed.

At `42rem` and below, the allocator switches to the mobile composition: only
the rightmost visible pane in each group receives width and every pane to its
left uses its zero-width mode. The workspace turns the visible panes'
`data-panel-breadcrumb` labels into one lightweight trail fixed across its top.
CSS owns this breakpoint through `--mobile-panel-composition`; JavaScript reads
the resolved signal rather than repeating the media query. The independent
`web/static/js/panel_breadcrumbs.js` controller renders direct workspace panes
after the layout completes.
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

Normal shell navigation replaces `#workspace`, preserving the shared primary
navigation. Returning to the home composition is the exception: its brand link
replaces `#app-shell` because the home response also owns personalized welcome
and recommendation content inside the primary navigation. Restricting that swap
to the workspace would retain stale non-home navigation markup after a reload.

An editor or detail that users can reasonably expect to survive a reload must
encode its identity in the URL and be rendered open by the server. Use
client-only `data-open-panel` state only for short-lived auxiliary panels whose
loss on reload is intentional. If such a server-rendered detail closes locally
to preserve a custom exit transition, the close must also remove its identifying
query parameter from the current history entry once the transition completes.

`web/static/js/panels.js` owns this interaction. New panel interactions should
reuse its declarative triggers and stable panel boundaries rather than adding
page-specific show/hide code. It dispatches `panel:configure` before revealing
a panel; domain controllers populate their editor through that event. The
shared controller must not contain proposal, curriculum or learning-path
configuration.

Server-rendered pane navigation uses the same controller through two
declarations:

- the destination pane declares its visual motion with
  `data-panel-motion="horizontal"`;
- the navigation trigger declares `data-panel-navigation="open"`, `"replace"`
  or `"close"`. A trigger that can either add a new right-hand pane or replace
  the one already there declares `"open-or-replace"`; the controller resolves
  its mode from the visible sibling panes before issuing the request. Primary
  navigation declares `"workspace"`: leaving the home composition opens the
  whole workspace from the right, while moving between populated workspaces is
  a replacement.

`open` uses a document View Transition when available. Persistent panes have
stable transition identities, so their old snapshots interpolate towards the
already-settled geometry while a pane present only in the new state enters from
the right. Pane surfaces and mode-specific inner content use separate
identities: the surface interpolates its bounds, while old and new content fade
at their respective geometries instead of scaling towards the new top-left
corner. The allocator derives mode-specific content identities from stable pane
keys. Modes that preserve the full content share one identity and interpolate
without crossfading their old and new snapshots; `breadcrumb` and `collapsed`
use distinct identities, leaving their old and new snapshots fixed at their
respective geometries while they crossfade. The no-crossfade transition class is
applied only to persistent panes during an opening or closing recomposition and
cleared after the motion, so same-level replacements retain their content fade.
While a pane has an independently captured inner snapshot, its old outer
snapshot is hidden; this prevents the same content appearing a second time
inside the surface that interpolates its bounds. The navigation controller
removes the content identity from genuinely new panes so their surface and
content travel together; retained panes keep it whenever an opening recomposes
the workspace. The home and detail menu contents likewise use separate
identities. Browsers without View Transitions retain the transform-based entry
fallback. `replace` preserves the pane boundary and enables a View Transition
for continuity between its old and new content. `close` moves the pane out to
the right before triggering its HTMX request. The controller associates the
operation with the individual request rather than storing navigation state on
`document`, so overlapping or unrelated requests cannot consume each other's
motion intent. Links retain normal `href` navigation as their non-JavaScript
fallback.

Client-only pane closure uses one View Transition for both sides of the
recomposition. The old snapshot of the departing pane moves right while the
persistent pane surfaces expand from the same starting frame. Their compatible
or breadcrumb content follows the same matching rules as an opening
recomposition. Sharing one transition avoids separate exit and resize clocks.
Server-rendered closure keeps its pane exit phase, then enables the HTMX View
Transition for the resulting workspace swap so persistent elements interpolate
back into their expanded layout instead of jumping after the pane disappears.

Learning-path selection uses `open-or-replace`: the first selected path adds the
curriculum-map pane from the right, while choosing another path with the map
already present replaces its content in place.

## Pane close control

Every closable pane uses `ui-pane__close`. Its position belongs to the pane
surface rather than to a title or form: `.ui-pane` is the positioning context
and keeps the control at a responsive inset from its upper inline-end corner.
That inset follows half the group's shared horizontal padding, within shared
minimum and maximum bounds, so wide surfaces retain proportionate breathing
room.
Headers containing the control reserve the shared close-control clearance, so
long titles and adjacent actions cannot occupy that area. Consumer styles must
not reposition the close control. In mobile composition the system-level inset
also clears the shared breadcrumb bar.

## Motion and continuity

Calculated pane widths and padding are applied without CSS interpolation. The
allocator owns geometry, while document View Transitions and pane transforms
own visible motion. Mixing those responsibilities lets an outer flex box and
its contents temporarily compose against different widths, producing clipping
that disappears after a reload. Keep the element that owns a visual identity
stable while its pane changes mode: replacing icons, scaling their containing
box or changing layout structure mid-transition produces distortion or a final
visual jump. The sidebar and icon-only menu modes keep each link's leading edge
and icon coordinate aligned; internal CSS transitions stay disabled while a
document View Transition recomposes those modes.

HTMX workspace navigation uses `transition:true`. Stable concepts receive
stable `view-transition-name` values so the browser can interpolate them
between old and new documents. Names must be unique in each rendered state.
Graph units use their persistent unit IDs; shell navigation uses fixed names.
Persistent shell icons suppress the browser's default snapshot crossfade so
they remain opaque while their shared transition group interpolates position.
That shared class also keeps them above the separate home and detail content
snapshots, whose crossfade must not temporarily cover persistent controls.

Elements whose identity survives a responsive reflow use stable, entity-based
`view-transition-name` values and the shared `layout-position` transition
class. The group interpolates its old and new boxes while only the new snapshot
is painted, so grid and container-query changes read as movement rather than a
scale or crossfade. Name the smallest meaningful persistent pieces as well as
their surface when internal placement changes; names must remain unique in the
rendered document.

When adding or removing a pane, preserve its resolved width during the
entering or closing phase. Let the remaining panes renegotiate around that
geometry, then remove the temporary state. This prevents the pane's internal
content from repeatedly reflowing while it moves. Prefer automatic layout and
View Transitions over imperative animation sequences.

An `open` navigation settles the complete replacement workspace before its
entrance begins. Geometry transitions are suppressed for that synchronous
double allocation, matching direct-load initialization, and motion starts on
the following frame after shell synchronization. Only the new rightmost pane
receives horizontal motion. This
keeps `ResizeObserver` feedback from interpolating the navigation, workspace
and fill-pane widths while the entering pane is moving. The pane stack clips
overflow for the duration of that motion so a transformed pane cannot introduce
scrollbars and change the space being allocated. Replacement navigation
continues to use document View Transitions instead. Completion and cancellation
both clear the visual state and refresh the settled layout. Navigation never
adjusts scroll position as part of motion cleanup because that could move shell
ancestors outside the pane group.

Motion must degrade to an immediate, correct layout when View Transitions are
unavailable and respect `prefers-reduced-motion`.

The layout allocator is the one shell script loaded without `defer`. It runs at
the end of the document, after the pane markup exists but before the browser's
first complete paint. Because browsers may paint progressively while parsing,
the shell remains invisible until this calculation marks its layout as ready.
During that synchronous calculation only, the shell suppresses pane geometry
interpolation. It flushes the first allocation, measures once more after
overflow and scrollbar geometry settle, then exposes the shell with negotiated
widths. A `noscript` fallback keeps the CSS layout visible without JavaScript.
Pane transitions and document View Transitions remain enabled for every visible
interaction.

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
negotiation. Each cycle first runs inner-to-outer so nested groups can publish
their requirements, then outer-to-inner so every nested group is sized against
its parent's final allocation before the browser paints. Observer callbacks are
coalesced into one animation frame, and the allocator only writes geometry when
its resolved value changed. Viewport changes resize the observed shell; the next
pass reads the mobile composition signal resolved by CSS. These constraints
prevent layout writes from feeding an unbounded resize loop.
