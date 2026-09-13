# CI/CD

This repository deploys one immutable GHCR image to staging and production.
The image is built once by the root multi-stage `Dockerfile`; the frontend is
compiled inside the Docker build and embedded into the Go binary.

## Flow

```text
feature branch -> pull request -> staging
                         |
                         +-> CI checks
                         +-> GHCR staging-<commit> image
                         +-> staging deployment on 8081

staging -> pull request -> main
                         |
                         +-> production Environment approval
                         +-> verify the same staging image digest
                         +-> production deployment on 8080
```

The deployment script only updates the `sub2api` application service. It does
not recreate PostgreSQL or Redis.

## GitHub configuration

Create two GitHub Environments named `staging` and `production`.

The `production` environment must require an approver. Use environment-scoped
secrets for both environments so a staging job cannot read production-only
credentials.

Required environment secrets:

```text
DEPLOY_HOST=model-gate.cc
DEPLOY_USER=root
DEPLOY_KEY=<private SSH key>
DEPLOY_KNOWN_HOSTS=<known_hosts entry for model-gate.cc>
```

Recommended variables:

```text
STAGING_DEPLOY_PATH=/opt/sub2api-staging
PRODUCTION_DEPLOY_PATH=/opt/sub2api
```

The repository must also configure branch rules for `main` and `staging`:

- Pull requests are required.
- The CI, Frontend CI, and Security Scan checks are required.
- At least one approval is required.
- Force pushes, branch deletion, and bypassing required checks are disabled.

## Staging runtime

Staging is intentionally small but uses the real application path:

- one `sub2api` container;
- one isolated PostgreSQL container;
- one isolated Redis container;
- persistent data under `/opt/sub2api-staging`;
- application port `127.0.0.1:8081`;
- no production database, Redis, credentials, or model channels.

The first staging deployment creates a new database and generates credentials
in `/opt/sub2api-staging/credentials.txt` with mode `0600`. The deployment
workflow does not print those credentials.

## Deployment safety

`deploy/scripts/sub2api-deploy.sh` rejects non-expected paths and non-digest
images. Before deployment it validates the merged Compose configuration, saves
a PostgreSQL dump when the database container exists, pulls the target image,
updates only the application container, and checks both `/health` and `/`.
If the application does not become healthy, it recreates the application with
the previous image reference.

Production uses `deploy/docker-compose.production.override.yml` so the current
production Compose file is not overwritten by CI. The override injects the
target image digest only for the deployment command.

## Upstream updates

`upstream-check.yml` runs weekly and can also be started manually. It checks the
latest release of `https://github.com/Wei-Shaw/sub2api`, using its `main`
repository as the source and release tags such as `v0.2.4` as the upgrade
snapshot. When the release is not already contained in `staging`, the workflow
creates `upgrade/upstream-<version>` from `staging`, merges the release tag, and
opens a pull request back to `staging`.

An upstream merge conflict fails the workflow and requires manual resolution;
the workflow never updates `main` or deploys production directly. The workflow
requires repository Actions permissions for contents write and pull requests
write so it can push the upgrade branch and open the pull request.

## Local validation

```bash
sh -n deploy/scripts/sub2api-deploy.sh
sh -n deploy/scripts/sub2api-staging-init.sh
docker compose --env-file deploy/staging.env.example \
  -f deploy/docker-compose.local.yml \
  -f deploy/docker-compose.staging.yml config --quiet
```

The example environment intentionally contains placeholder values and must not
be used to start containers.
