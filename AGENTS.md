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
- Keep documentation and applicable `AGENTS.md` files aligned with changes to
  architecture or conventions.

## Validation

Run from `app`:

```bash
go test ./...
```

After changing migrations, also run from the repository root:

```bash
podman compose -f compose.dev.yaml --profile test run --rm migration-tests
```
