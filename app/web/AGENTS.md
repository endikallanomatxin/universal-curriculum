# Web UI

## Architecture

- Use server-rendered templates and HTMX for progressive enhancement.
- Keep the interface accessible without JavaScript where practical.
- Load templates through `services.LoadTemplates`.
- Reference CSS and JavaScript with the shared `assetVersion`; it is generated
  from the contents of `web/static`, so asset changes invalidate browser caches
  after an application restart.

## Shared vocabulary

- Prefer simple structure, spacing and hierarchy over decorative wrappers.
- Keep stylesheet boundaries conceptual: `base.css` owns foundations,
  `shell.css` owns application panes and navigation, `components.css` owns
  reusable primitives, and concept-level files such as
  `curriculum-graph.css` and `unit-content.css` serve every page that renders
  that concept. Page styles should contain only page composition and genuine
  feature-specific variants.
- Keep top-level page templates compositional. Put substantial feature
  fragments in the matching template subdirectory and load that directory
  explicitly through `services.LoadTemplates`.
- Build pages from system-wide interaction and visual primitives. Reuse and
  extend shared CSS, JavaScript and template fragments instead of creating
  page-specific versions of an existing concept.
- Put reusable behavior in shared templates, delegated JavaScript controllers
  and concept-level styles. Page-specific code may compose those primitives
  but must not reimplement their behavior.
- Before implementing an interaction, search the whole web application by
  interaction and visual concept, not only by page or entity name. Inspect the
  complete precedent across templates, styles, JavaScript, handlers, services
  and tests.
- Decide whether to reuse a shared primitive, extend its declarative contract,
  extract a pattern demonstrated by multiple callers, or keep the behavior
  local because its semantics are genuinely unique.
- When extracting or extending a shared primitive, migrate the callers that
  motivated it and test the shared contract. Do not leave parallel local
  implementations behind.
- Build application pages from the shared shell and pane vocabulary. Context
  moves from general on the left to specific on the right. Declare pane
  capabilities through `data-panel-*` attributes and let the shared allocator
  negotiate their widths. The canonical behavior is documented in
  `docs/implementation/pane-layout-and-transitions.md`.

## Interaction invariants

- Keep shell navigation functional without HTMX; use enhanced swaps only for
  continuity and motion.
- Define HTMX interactions as one explicit request, response, `hx-select`,
  `hx-target` and `hx-swap` contract. Replace the narrowest stable fragment
  that owns the changed state.
- Preserve relevant focus, scroll, selection and editor state across swaps.
- Verify loading, empty, validation and error states, keyboard operation, and
  narrow-container behavior for changed interactions.

## Tests

- Template tests should favor compilation, representative critical renders,
  permissions and read-only states, and important URLs or HTMX parameters.
- Do not lock presentation to exact class names, DOM ordering, partial
  composition, CSS values, spacing or breakpoints. Cover a visual detail only
  when it carries an accessibility or interaction contract that is not tested
  more directly elsewhere.
