# HTTP server

- Keep handlers focused on authorization, parsing, invoking operations and
  rendering responses.
- Direct database calls are acceptable only for simple one-to-one persistence.
- Use services for business rules, coordinated operations and transactions.
- Parse and validate input before changing domain state.
- Redirect successful browser form submissions to a GET. When the following
  page must reveal one-time data, keep it in short-lived server-side state
  rather than a URL or client-readable cookie.
- Prefer server-rendered HTML and use HTMX for fragment updates.
- Return explicit HTTP errors without terminating the server.
- Web handlers authenticate through the session middleware and require CSRF
  tokens for state-changing requests. REST handlers follow
  `docs/implementation/api.md`, authenticate only with personal bearer tokens
  and do not accept cookie sessions. MCP is a sibling adapter documented in
  `docs/implementation/mcp.md`; this package owns its OAuth discovery,
  authorization and consent HTTP endpoints.
- Handler tests should emphasize authorization, parsing and validation, status
  and feedback, and important HTTP or HTMX response contracts. Avoid tests that
  only prove a handler forwards values to an already-covered dependency.
