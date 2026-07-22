# Web UI

- Use server-rendered templates and HTMX for progressive enhancement.
- Prefer simple structure, spacing and hierarchy over decorative wrappers.
- Reuse shared CSS, JavaScript and template fragments before adding page-specific
  alternatives.
- Keep the interface accessible without JavaScript where practical.
- Load templates through `services.LoadTemplates`.
- Build application pages from the shared shell and pane vocabulary. Context
  moves from general on the left to specific on the right.
- Keep shell navigation functional without HTMX; use enhanced swaps only for
  continuity and motion.
