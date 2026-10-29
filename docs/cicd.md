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

Create two GitHub Environments named `model-gate-staging` and
`model-gate-production`. The names are intentionally project-specific so they
cannot accidentally reuse deployment credentials from another project in the
same repository.

Use environment-scoped secrets for both environments so a staging job cannot
read production-only credentials. When the repository remains on a private
plan without enforced Environment reviewers, production is protected by a
manual workflow dispatch and an explicit `DEPLOY` confirmation instead of an
automatic merge-triggered deployment.

Required environment secrets:

```text
DEPLOY_HOST=154.222.26.115
DEPLOY_USER=root
DEPLOY_KEY=<private SSH key>
DEPLOY_KNOWN_HOSTS=<known_hosts entry for model-gate.cc>
```

Recommended variables:

```text
STAGING_DEPLOY_PATH=/opt/sub2api-staging
PRODUCTION_DEPLOY_PATH=/opt/sub2api
```

If the repository is later moved to a plan that enforces repository rules, the
repository should configure branch rules for `main` and `staging`:

- Pull requests are required.
- The CI, Frontend CI, and Security Scan checks are required.
- At least one approval is required.
- Force pushes, branch deletion, and bypassing required checks are disabled.

## Manual production release

In the private-plan setup, merge the reviewed `staging` changes to `main`, then
open **Actions -> Deploy Production -> Run workflow**. Enter the SHA of the
commit that produced the successful staging image and type `DEPLOY` in the
confirmation field. The workflow resolves the immutable GHCR digest, verifies
that the same image is running in staging, and only then updates production.

Do not enter the `main` merge commit unless staging was separately built from
that exact commit. The normal value is the feature or upgrade commit SHA shown
by the successful `Deploy Staging / build` run.

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
the previous image reference. Successful releases atomically persist only
`SUB2API_IMAGE` in the existing `.env`, preserving other settings and file
permissions. Rollback restores the running and persistent image together.
Persistence or verification failure fails the deployment rather than recording
a successful release. Verification unsets the temporary shell override before
resolving Compose, so a stale `.env` cannot pass through that override.

Production uses `deploy/docker-compose.production.override.yml` so the current
production Compose file is not overwritten by CI. The override injects the
target image digest during deployment; the successful digest is also saved in
`.env` for later manual recreation. Existing production base Compose files
should use `image: ${SUB2API_IMAGE:?SUB2API_IMAGE is required}` rather than a
fixed legacy tag. This is a one-time host configuration migration, not a reason
to replace production-specific settings with a generic Compose template.

To align an existing environment with its currently running, healthy pinned
image without pulling an image or recreating any container:

```sh
/opt/sub2api-staging/sub2api-deploy.sh sync-image staging
/opt/sub2api/sub2api-deploy.sh sync-image production
```

Back up the configuration securely before this migration. `verify-staging`
also checks that the persistent image agrees with the running image before
production promotion. These commands do not remove historical images.

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
python -m unittest discover -s deploy/tests -p 'test_deploy_image.py'
docker compose --env-file deploy/staging.env.example \
  -f deploy/docker-compose.local.yml \
  -f deploy/docker-compose.staging.yml config --quiet
```

The example environment intentionally contains placeholder values and must not
be used to start containers.
