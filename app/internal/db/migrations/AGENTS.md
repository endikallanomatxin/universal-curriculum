# Database migrations

- Give each migration one coherent schema or data transformation.
- Include a valid `Down` section and keep dependency order explicit.
- Do not use `IF NOT EXISTS` to hide unexpected schema divergence.
- Do not duplicate indexes created by `PRIMARY KEY` or `UNIQUE`.
- Do not modify migrations that have reached production.
- Test the resulting schema, data transformation and migration round trip.
  Inspect migration text only for a critical guard that cannot be verified more
  directly; do not duplicate the migration line by line in tests.
- Run the migration round-trip test after changing migrations.
