# SQL migrations

`000001_initialize.sql` defines the complete initial production database.

Do not split pre-production design history into migrations. Once the first
deployment exists, add forward-only schema changes as new Goose migrations and
leave deployed migrations immutable.
