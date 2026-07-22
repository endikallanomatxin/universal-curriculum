# HTTP server

- Keep handlers focused on authorization, parsing, invoking operations and
  rendering responses.
- Direct database calls are acceptable only for simple one-to-one persistence.
- Use services for business rules, coordinated operations and transactions.
- Parse and validate input before changing domain state.
- Prefer server-rendered HTML and use HTMX for fragment updates.
- Return explicit HTTP errors without terminating the server.

