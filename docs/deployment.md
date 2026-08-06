# Deployment

Production is deployed to Render using `render.yaml`:

- a Docker web service in Frankfurt;
- a managed Render PostgreSQL database;
- local ephemeral storage at `/app/uploads` behind the object-store
  abstraction, with a persistent disk configuration prepared for 0.3.0;
- `/usr/local/bin/migrate up` before each deployment;
- `/health` as the service health check;
- database credentials supplied by Render;
- initial administrator credentials supplied as secret Render values.
- password recovery email delivered through Resend.

Commits to the main branch trigger deployment after the Render Blueprint has
been connected to the repository.

## Setup

1. Create or sync a Render Blueprint from `render.yaml`.
2. Connect the web service to the `main` branch.
3. Set `BOOTSTRAP_ADMIN_FULL_NAME`, `BOOTSTRAP_ADMIN_EMAIL` and
   `BOOTSTRAP_ADMIN_PASSWORD` to create the initial administrator. Optionally
   set `BOOTSTRAP_ADMIN_ALIAS`. These values are idempotent and do not modify an
   existing user.
4. Keep database credentials managed by Render.
5. Verify `auth.universalcurriculum.org` in Resend and configure
   `RESEND_API_KEY` as a secret. The Blueprint sets `EMAIL_FROM` to
   `Universal Curriculum <no-reply@auth.universalcurriculum.org>`.
6. Configure the public domain in Render when required.

TLS termination and proxying are handled by Render, not by the application.
`TRUST_RENDER_PROXY_HEADERS` is enabled in the Blueprint so password-recovery
rate limits use the original client address supplied by Render.
The local storage backend is currently ephemeral in production because no
feature stores assets yet. The persistent disk configuration can be enabled for
0.3.0, or the backend can be replaced with remote object storage, without
changing file-consuming services.
