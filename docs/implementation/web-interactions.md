# Web interactions

Universal Curriculum renders complete pages on the server and uses HTMX as
progressive enhancement. An interaction must remain understandable as one
explicit response contract:

```text
request
→ handler and rendered template
→ response node selected by hx-select
→ node replaced through hx-target and hx-swap
→ browser history and transient state that must survive
```

Navigation links retain ordinary `href` behavior. Workspace navigation
replaces `#workspace` with the corresponding node from a complete response,
pushes the URL and enables View Transitions. Smaller interactions should
replace the narrowest stable fragment that owns their state.

Avoid replacing a large ancestor merely to report that a mutation succeeded.
Broad replacements can reset scroll, focus, an open editor or client-side
selection. A response used by both normal and HTMX requests must either contain
the selected node or deliberately render the matching fragment for HTMX.

## System-wide interaction vocabulary

Pages compose a shared interaction vocabulary; they do not own private
versions of common controls. Before adding markup, CSS or JavaScript, search
the whole web application by the interaction or visual concept rather than by
the current page name. Inspect the template, styles, delegated controller,
server contract and tests together.

Prefer, in order:

1. using the existing shared behavior without changes;
2. extending its declarative HTML contract with one generally useful option;
3. extracting a shared component once two real callers demonstrate the same
   contract; and
4. implementing page-specific behavior only when its semantics are genuinely
   unique.

A shared component owns its behavior and state vocabulary. Callers supply
data, identifiers, URLs and configuration; they must not reproduce its event
handlers, control markup or visual states. Shared JavaScript should be
delegated and initialize correctly after HTMX swaps. Shared styles should be
scoped to a named concept rather than to a page or bare semantic element.

When extracting or extending a shared pattern, migrate the immediate callers
that established it and cover the common contract in tests. Do not create a
generic component for a speculative future use, and do not leave a shared
component beside page-local copies of the behavior it is meant to own.

Current system-wide concepts include:

- shell and negotiated panes;
- right-hand panel opening, replacement and closing;
- graph rendering, neighbourhood navigation and unit navigation search;
- searchable unit picking and persistent selection;
- rendered unit documents and inline source editing;
- buttons, form controls and action hierarchy; and
- HTMX workspace navigation with View Transitions.

## Right-hand details and editors

The pane sequence represents increasing specificity. An action within one pane
normally replaces everything to its right before opening its result. Creation,
proposal details, history, dependency editing and learning-path editing follow
this model.

Inline editing is different: when the edited value is already the main subject
of a pane, the editor replaces that value in place. The unit title and rendered
document become their corresponding source controls rather than opening a
detached form. Cancelling restores the rendered value; saving preserves the
same surrounding pane and context.

Proposal-backed edits are idempotent. Submitting the published value must not
create a change and must remove an equivalent pending change from the working
proposal.

## Search and selection

Search results use a stable results boundary and a debounced input. Clearing a
native search field must refresh the results. Empty matches have an explicit
empty state.

Navigation search and picker search are distinct:

- navigation loads the selected unit's neighbourhood and content and updates
  browser history;
- selection changes a durable selected set without navigating away.

Picker selection is stored outside the replaceable result list. Changing a
query cannot silently deselect an existing choice. The prerequisite picker is
the shared interaction precedent for dependency editing and learning-path
targets. Both use `unit_picker.js`; domain adapters decide whether selection is
kept as form state or submitted immediately.

## Interaction quality

For every new or changed interaction, verify:

- normal-link or normal-form behavior where practical;
- target, select and swap alignment;
- keyboard operation, focus and an accessible current/selected state;
- loading, empty, validation and server-error states;
- preservation of relevant scroll, selection and editor state;
- narrow container behavior, not only narrow viewport behavior; and
- continuity with and without View Transitions.

Treat a page-local implementation of an existing system-wide concept as a
regression unless the different semantics are explicit and documented.
