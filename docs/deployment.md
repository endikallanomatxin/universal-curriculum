# Deployment

Production is deployed to Render using `render.yaml`:

- a Docker web service in Frankfurt;
- a managed Render PostgreSQL database;
- a persistent disk mounted at `/app/uploads` behind the object-store
  abstraction;
- `/usr/local/bin/migrate up` before each deployment;
- `/health` as the service health check;
- database credentials supplied by Render.

Commits to the main branch trigger deployment after the Render Blueprint has
been connected to the repository.

## Setup

1. Create or sync a Render Blueprint from `render.yaml`.
2. Connect the web service to the `main` branch.
3. Keep database credentials managed by Render.
4. Configure the public domain in Render when required.

TLS termination and proxying are handled by Render, not by the application.
The local storage backend is an implementation detail and can later be replaced
with remote object storage without changing file-consuming services.
