# Database access

- Keep this package focused on PostgreSQL persistence and retrieval.
- Use parameterized SQL.
- Preserve transaction boundaries established by services.
- Let PostgreSQL enforce structural invariants with keys, constraints and
  foreign keys.
- Avoid indexes already provided by `PRIMARY KEY` or `UNIQUE`.
- Wrap errors with enough context to identify the failed operation.

