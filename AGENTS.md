# Universal Curriculum

## Architecture

The Go module lives in `app`. The usual dependency flow is:

`server → services → db → models`

- Handlers own HTTP concerns: authorization, parsing and responses.
- A handler may call `db` directly for a simple one-to-one persistence
  operation.
- Use a service for business rules, coordinated reads or writes, transactions,
  side effects, external resources or reuse outside HTTP.
- Database code uses raw PostgreSQL queries.
- Schema changes use Goose migrations.
- Read `docs/specification.md` before changing product behaviour.

## Repository rules

- Follow existing patterns unless deliberately refactoring them.
- Do not introduce frameworks without an explicit architectural decision.
- Keep domain invariants close to the model and enforce structural invariants
  in PostgreSQL where possible.
- Write commit messages in clear, standard English.

## Documentation and agent guidance

- `docs/specification.md` defines product behaviour.
- `docs/implementation` documents mechanisms that may evolve without changing
  the product specification.
- `AGENTS.md` files contain the constraints and working procedures applicable
  to their directory tree. Keep them concise and point to canonical
  implementation documents instead of duplicating detailed mechanisms.
- Keep documentation and applicable `AGENTS.md` files aligned with changes to
  architecture or conventions.
- After user feedback corrects an implementation, consider whether reusable
  repository knowledge available beforehand would have prevented the mistake.
  If so, update the applicable `AGENTS.md` or canonical implementation document
  in the same change; keep one-off preferences and visual tuning in code and
  tests.

## Validation

Run from `app`:

```bash
go test ./...
```

After changing migrations, also run from the repository root:

```bash
podman compose -f compose.dev.yaml --profile test run --rm migration-tests
```
