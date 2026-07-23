# Web UI

- Use server-rendered templates and HTMX for progressive enhancement.
- Prefer simple structure, spacing and hierarchy over decorative wrappers.
- Reuse shared CSS, JavaScript and template fragments before adding page-specific
  alternatives.
- Keep the interface accessible without JavaScript where practical.
- Load templates through `services.LoadTemplates`.
- Build application pages from the shared shell and pane vocabulary. Context
  moves from general on the left to specific on the right.
- Declare responsive pane sizes with `data-panel-modes` in rem. Let the shared
  panel layout allocate modes from right to left. If a declared
  `data-panel-required-mode` does not fit, it blocks expansions to its left;
  larger modes remain optional. Remaining space is distributed from right to
  left up to each `data-panel-max`. `data-panel-fill` allows a pane without an
  explicit maximum to absorb all remaining space.
- Nested panel groups propagate the sum of their visible required modes to
  their parent, so outer context panels collapse before inner content does.
- Keep shell navigation functional without HTMX; use enhanced swaps only for
  continuity and motion.
- Reference CSS and JavaScript with the shared `assetVersion`; it is generated
  from the contents of `web/static`, so asset changes invalidate browser caches
  after an application restart.
