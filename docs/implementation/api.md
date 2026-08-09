# Experimental API

## Compatibility

The API is served under `/api` and is experimental throughout the 0.x release
line. Its OpenAPI document describes the exact contract of a release, but does
not promise compatibility with later minor releases. A stable, versioned path
will be chosen before 1.0 when the resource model has enough operational use.

`docs/openapi.yaml` is the canonical API contract. An implementation change is
not complete until that document describes the resulting HTTP behaviour. The
server embeds a delivery copy at `app/internal/server/openapi.yaml` so the
contract remains available in the minimal production image; release validation
checks that the two files are identical.

## HTTP conventions

Requests and responses use JSON, except responses with status `204`. Unknown
JSON fields, trailing JSON values, malformed IDs and unsupported query
parameters are rejected. Growing collections use `limit` and `offset` and
return a `page` object alongside their resource array.

Errors have one stable envelope:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "The request is invalid.",
    "fields": {"name": "is required"}
  }
}
```

`fields` is omitted when the error is not associated with individual input
fields. Internal errors are logged server-side and never expose SQL, tokens or
other implementation details.

## Authentication and authorization

Private endpoints accept `Authorization: Bearer <token>`. API tokens represent
one user and use that user's current permissions; 0.2.6 does not define scopes.
The raw token is returned only by its creation request. Persistence stores its
SHA-256 hash, a short non-secret prefix for identification, and management
metadata.

Cookie sessions and CSRF remain the web interface's authentication mechanism.
They do not authenticate API requests. Likewise bearer tokens do not bypass
CSRF on HTML form routes. CORS headers are not emitted in this release.

Revoking a token deletes its credential row immediately; historical token
metadata is not retained in this table. `last_used_at` is updated at most once
every fifteen minutes to avoid turning every API read into a database write.

## Adapter boundary

API handlers own JSON parsing, authorization, resource representation and HTTP
status selection. They call the existing services for coordinated curriculum
and learning workflows. Public one-to-one reads and token persistence may call
the database package directly.

The API's PostgreSQL workflow is exercised by the Compose `integration-tests`
service documented in the repository README. A plain `go test ./...` without
`TEST_DATABASE_URL` intentionally skips database-backed integration cases.
