# Model Context Protocol

## Role and architecture

Universal Curriculum has two machine-facing adapters with different contracts:

- `/api` is the experimental, general-purpose REST API. Its canonical contract
  remains `docs/openapi.yaml`.
- `/mcp` is the agent-oriented Model Context Protocol adapter. Its tools group
  useful domain actions and are not required to mirror REST resources or HTTP
  methods.

MCP, REST and the web interface call the same application services and database
operations. The MCP adapter never calls `/api`, and the domain has no dependency
on the MCP SDK. Coordinated learning, proposal, rebase and publication rules
remain in `internal/services`.

The implementation uses the official MCP Go SDK v1.7.0 and Streamable HTTP in
stateless mode. That mode follows protocol version 2026-07-28's sessionless
model while the SDK retains initialization compatibility with older clients.
Only `POST /mcp` carries protocol messages; unsupported methods receive `405`.
The endpoint intentionally does not provide server-initiated subscriptions or
an autonomous Universal Curriculum agent.

The transport reuses three immutable MCP server catalogs for members,
contributors and administrators. Authentication selects the catalog
for each request, while tool handlers read the current user from that request's
SDK `TokenInfo`; no server instance contains request-specific identity.

## Instructions and resources

Server discovery advertises concise instructions that establish the important
workflow:

- before designing or modifying curriculum, call `get_authoring_guidance` and
  apply the canonical documents it returns;
- search existing units and factor shared knowledge before creating or
  substantially explaining a concept;
- produce focused, reusable units whose learner-facing content is nevertheless
  a finished microlesson given its genuine prerequisites;
- review every changed unit from that learner perspective before submission;
- change curriculum only through proposals;
- use `get_recommendations` rather than independently inferring the next unit;
- treat recorded progress as authoritative;
- inspect rebase state before changing a stale proposal; and
- submit only after an explicit user request, never as an implied part of
  editing or preparing a proposal.

The canonical documentation lives as embedded Markdown in
`internal/server/guidance`. The `curriculum-units`, `dependencies` and
`writing-content` pages, especially the latter, form the editorial contract.
They are exposed individually as MCP documentation resources for clients that
support resource reads. The read-only, argument-free
`get_authoring_guidance` tool returns those same canonical bodies together so
agents can load them through a model-callable operation when their host does
not expose resources.

Discovery instructions tell agents when to load the contract. The
`create_proposal_unit` and `update_proposal_unit` descriptions retain short
local reminders that `content` is final learning material rather than an
outline and direct agents to the canonical tool; they are not another copy of
the guidance.

These instructions guide a model; permissions and invariants are still enforced
in code.

The read-only resources are:

- `curriculum://about`, a compact explanation of the platform and workflows;
- `curriculum://documentation/{slug}`, canonical guidance shared with the web;
- `curriculum://published`, the current published graph as JSON; and
- `curriculum://units/{unit_id}`, one unit and its immediate relationships.

Parameterized reads and actions are tools. The current conceptual groups are:

- curriculum discovery and authoring guidance: `get_authoring_guidance`,
  `get_curriculum`, `search_units`, `get_unit`;
- learning: `get_learning_paths`, `create_learning_path`,
  `update_learning_path`, `delete_learning_path`, `get_progress`,
  `set_progress`, `get_recommendations`;
- contributor proposals: list, inspect, create, update and delete proposals; create, update
  and delete proposal units; converge a dependency; ensure a recognition;
  delete a proposal change; inspect and resolve rebase state; and submit;
- administrator proposal decisions: accept or reject a submitted proposal.

Proposal tools are registered for contributors and administrators. Their handlers still
enforce contributor permission and draft authorship so discovery filtering is
an ergonomic optimization rather than an authorization boundary.

Every tool has an explicit input and output schema. Results use an `ok` envelope
with either structured `data` or a structured error containing `code`,
`message`, optional `fields` and `retryable`. Validation failures, missing
resources, permission failures and domain conflicts are distinct. Internal
errors are logged and not exposed.

Read-only, mutating, idempotent and destructive hints use MCP tool annotations.
Creation tools explicitly state that they are not safe to retry after an
ambiguous transport failure. Dependency and progress setters converge to a
requested state, and an identical recognition is a successful no-op.

## Authentication

All MCP protocol requests require a bearer credential. Requiring authentication
at the transport boundary gives remote MCP clients the standard OAuth discovery
challenge and also protects private tool and resource enumeration. Anonymous
published-curriculum access remains available through REST. Tools omit
`securitySchemes` and inherit this server-wide policy; they do not advertise the
legacy `_meta.securitySchemes` compatibility mirror.

Two bearer credential forms are accepted:

1. A personal token created on the Account page. This is convenient for CLI and
   desktop clients that can configure an `Authorization` header. It has no
   independent scope or expiry and remains valid until deleted.
2. An OAuth access token for remote clients. Universal Curriculum implements
   Authorization Code with PKCE S256, the `mcp` resource indicator and one
   `mcp` scope. Access tokens last one hour. Refresh tokens last 30 days and are
   rotated on every use. Authorization codes are short-lived and one-time.

OAuth discovery is available at:

- `/.well-known/oauth-protected-resource/mcp` (also available at the origin
  form without `/mcp`);
- `/.well-known/oauth-authorization-server`;
- `/oauth/authorize`, `/oauth/token` and `/oauth/revoke`.

The authorization server supports current Client ID Metadata Documents (CIMD)
for public clients. The `client_id` must be a public HTTPS metadata URL whose
document identifies itself and lists the exact redirect URI. Metadata fetching
blocks loopback, private and link-local destinations and revalidates redirects.
The document must list `none` in
`token_endpoint_auth_methods_supported`; the legacy singular
`token_endpoint_auth_method: "none"` remains accepted for compatibility.
Dynamic Client Registration and client secrets are not implemented.

Both credential forms resolve the user and current `is_admin` permission on
every request. Secrets are stored only as SHA-256 hashes. OAuth access tokens
are audience-bound to the exact MCP resource. Their last-use metadata update is
best effort. The revocation endpoint accepts public-client access or refresh
tokens without revealing whether a token existed.

Each completed OAuth grant creates or refreshes one durable connection keyed by
user, client and resource. Reauthorization replaces the connection's previous
tokens. Users can inspect and revoke connected apps from Account; revoking a
connection deletes all of its access and refresh tokens atomically.

## Client setup

The production endpoint is:

```text
https://universalcurriculum.org/mcp
```

For a client that accepts static headers, create a personal token in Account
and configure the MCP URL plus:

```text
Authorization: Bearer uc_api_…
```

For ChatGPT, enable developer mode in ChatGPT settings, add a custom
plugin/connector using the HTTPS MCP URL, and complete the Universal Curriculum
OAuth login and consent screen. ChatGPT's current setup instructions are
maintained in the official [OpenAI connection guide](https://developers.openai.com/plugins/deploy/connect-chatgpt).
The ChatGPT client publishes a CIMD document, so no manual client registration
or secret is required.

Local development uses `http://localhost:8080/mcp`. The MCP endpoint itself
still requires a personal bearer token. OAuth CIMD deliberately requires a
public HTTPS client metadata URL; use a trusted HTTPS tunnel only when testing
the complete remote OAuth flow.

## Security boundaries and limitations

- Personal and OAuth tokens inherit all current account permissions. There are
  no finer MCP scopes yet; only authorize clients that should receive the same
  access as the account.
- Draft visibility and ownership, contributor and administrator checks,
  proposal invariants and decision authorization are enforced server-side.
- `submit_proposal` is destructive metadata-wise, requires a contributor who
  authors the draft, requires `confirmed: true`, and requires the current
  title as a stale/wrong-proposal guard. Clients should still show their own
  user confirmation UI.
- The OAuth consent page grants the single `mcp` scope. Account lists connected
  apps and can revoke a whole connection. Clients may also revoke an individual
  access or refresh token through `/oauth/revoke`; access otherwise ends after
  one hour and refresh eligibility after 30 days.
- Browser cross-origin mutation protection wraps `/mcp`. The public OAuth
  discovery documents alone allow cross-origin reads.

## Validation

Ordinary tests cover discovery, instructions, capabilities, resources, tool
schemas and annotations, structured permission errors, OAuth metadata,
redirect validation and consent. The Compose `integration-tests` profile also
executes an agent-style MCP workflow against PostgreSQL, including actual
Streamable HTTP authentication, user isolation, progress and recommendations,
proposal changes, validation conflicts, retry-safe actions and publication.
