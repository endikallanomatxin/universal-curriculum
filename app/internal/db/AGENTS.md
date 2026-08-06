# Database access

- Keep this package focused on PostgreSQL persistence and retrieval.
- Use parameterized SQL.
- Preserve transaction boundaries established by services.
- Let PostgreSQL enforce structural invariants with keys, constraints and
  foreign keys.
- Avoid indexes already provided by `PRIMARY KEY` or `UNIQUE`.
- Wrap errors with enough context to identify the failed operation.
- Store session and API token secrets only as cryptographic hashes.
- Use SQL mocks to protect meaningful arguments, hydration, error handling and
  critical filters or joins. Avoid copying a whole query into a test or testing
  a trivial one-statement wrapper unless that statement carries a real risk.
- Prefer a database-backed test when correctness depends on PostgreSQL
  semantics or constraints that a mock cannot exercise faithfully.
