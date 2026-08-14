# Universal Curriculum

## Architecture

The Go module lives in `app`. The usual dependency flow is:

`web/REST/MCP adapters → services → db → models`

These four layers are represented by the top-level packages under
`app/internal`: `server`, `services`, `db` and `models`. Adapter-specific and
presentation packages belong below `server`.

- Handlers own HTTP concerns: authorization, parsing and responses.
- A handler may call `db` directly for a simple one-to-one persistence
  operation.
- Use a service for business rules, coordinated reads or writes, transactions,
  side effects, external resources or reuse outside HTTP.
- Database code uses raw PostgreSQL queries.
- Schema changes use Goose migrations.
- Read `docs/specification.md` before changing product behaviour.
- Keep web, REST and MCP as sibling adapters. An adapter must not call another
  adapter; shared workflows belong in services. MCP details are documented in
  `docs/implementation/mcp.md`.

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
- Keep future release descriptions in `docs/plan`; when a release is completed,
  move its file to `docs/releases` in the release preparation commit.
- After changing `docs/plan` or `docs/releases`, run
  `go generate ./internal/server/releaseinfo` from `app` so the web catalog
  remains synchronized with those canonical files.
- After changing `docs/openapi.yaml`, run `go generate ./internal/server` from
  `app` so the embedded delivery copy remains synchronized.
- After user feedback corrects an implementation, consider whether reusable
  repository knowledge available beforehand would have prevented the mistake.
  If so, update the applicable `AGENTS.md` or canonical implementation document
  in the same change; keep one-off preferences and visual tuning in code and
  tests.

## Release workflow

- Prepare each release on `develop`, including its release document and
  generated catalogs or contracts.
- Merge `develop` into `main` with `--no-ff` and the commit message
  `Release X.Y.Z`, then place the annotated tag `vX.Y.Z` on that merge.
- Immediately fast-forward `develop` to `main` with `git merge --ff-only main`;
  do not create a second merge from `main` back into `develop`.
- After a release, `main` and `develop` must point to the release merge, and new
  work must continue on `develop` from that shared commit.
- Push `main`, `develop` and the release tag together after validation.

## Tests

- Prefer tests that protect relevant behaviour, business rules, regressions,
  data integrity, permissions, transactions or important external contracts.
- Assert observable outcomes at the most useful level. Avoid tests whose main
  effect is to freeze private call sequences, exact SQL text, template
  composition, CSS classes or purely visual layout.
- Before adding a test, check nearby coverage and extend the closest meaningful
  case when possible. A new test should fail for a plausible regression, not
  merely because an equivalent implementation was rearranged.
- Small helpers and simple persistence operations do not need isolated tests by
  default. Test them when they contain a boundary condition, mapping, error case
  or other behaviour with enough risk to justify maintenance.
- Do not optimize for test count. When simplifying tests, preserve coverage of
  the domain risks and contracts that matter.

## Validation

Run from `app`:

```bash
go test ./...
```

Run the complete PostgreSQL integration suite from the repository root. This
provides `TEST_DATABASE_URL`; running the same Go tests without it skips the
database-backed cases:

```bash
podman compose -f compose.dev.yaml --profile test run --rm integration-tests
```

After changing migrations, also run from the repository root:

```bash
podman compose -f compose.dev.yaml --profile test run --rm migration-tests
```
