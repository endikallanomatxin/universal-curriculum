# Database migrations

- Give each migration one coherent schema or data transformation.
- Include a valid `Down` section and keep dependency order explicit.
- Do not use `IF NOT EXISTS` to hide unexpected schema divergence.
- Do not duplicate indexes created by `PRIMARY KEY` or `UNIQUE`.
- Do not modify migrations that have reached production.
- Run the migration round-trip test after changing migrations.

