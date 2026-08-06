# Universal Curriculum

Open platform for collaboratively building and learning from a shared
curriculum.

The application is written in Go, uses PostgreSQL and is deployed to Render as
a Docker web service. File access is prepared behind an object-store
abstraction, but the 0.1.0 release does not store assets. Local storage is
ephemeral in production until asset support is introduced in 0.3.0.

## Repository structure

- `app/`: application source
  - `cmd/server/`: web server entry point
  - `cmd/migrate/`: database migration command
  - `internal/models/`: domain models
  - `internal/db/`: PostgreSQL access and Goose migrations
  - `internal/services/`: business workflows
  - `internal/server/`: HTTP server, configuration and handlers
  - `web/`: templates and static assets
- `docs/`: product and deployment documentation
- `compose.dev.yaml`: local development environment
- `render.yaml`: Render deployment blueprint

## Local development

Copy `.env.example` to `.env` and add a Resend API key. The ignored `.env`
file contains secrets; non-secret local email settings are defined in
`compose.dev.yaml`.

From the repository root:

```bash
podman compose -f compose.dev.yaml up --build
```

The application is available at `http://localhost:8080`.
Files written through the local object-store backend persist in the
Compose-managed `uploads` volume.

The development administrator is `admin@example.com` with password
`administrator`. Production credentials are configured as secret values in
Render and only create the first administrator; subsequent starts do not reset
the password.

Email is sent from `no-reply@auth.universalcurriculum.org` with the display name
“Universal Curriculum”. The `auth.universalcurriculum.org` subdomain must remain
verified in Resend.

Apply migrations manually with:

```bash
podman compose -f compose.dev.yaml run --rm app go run ./cmd/migrate up
```

## Validation

```bash
cd app
go test ./...
go build ./cmd/server
go build ./cmd/migrate
```

Run PostgreSQL migration tests from the repository root:

```bash
podman compose -f compose.dev.yaml --profile test run --rm migration-tests
```

See [`docs/deployment.md`](docs/deployment.md) for production deployment.

## License

The Universal Curriculum software is licensed under the GNU Affero General
Public License version 3 only (`AGPL-3.0-only`). See [LICENSE](LICENSE).

Original curriculum content published through the platform is licensed
separately under CC BY-SA 4.0 unless otherwise stated.

See [LICENSING.md](LICENSING.md) for details about third-party components and
trademarks.
