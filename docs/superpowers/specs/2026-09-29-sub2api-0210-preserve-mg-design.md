# Sub2API 0.2.10 upgrade with Model-Gate behavior preservation

## Goal and baseline

Upgrade the Model-Gate fork from the deployed `0.2.8-mg.1` source
`ebcfdc7ffe9ebd27aea8e1d5cc67f107536ae256` to official Sub2API
`v0.2.10` (`2f3fed2fdb0787141294cec81487a5df30426f7f`). Preserve every
intentional frontend and backend behavior added by Model-Gate, while adding
the upstream frontend and backend features and fixes from 0.2.9 and 0.2.10.
The candidate starts from the current `origin/staging` tree, which matches
`origin/main` and the deployed application's source tree.

## Integration approach

Merge the complete, verified upstream release tag into a clean upgrade branch.
Keep official frontend and backend code together so API contracts, stores,
routes, and views advance as a unit. Resolve conflicts by retaining the
existing Model-Gate behavior and incorporating the new upstream behavior in
the same files. Record any case where the two behaviors cannot coexist as a
specific decision before changing that behavior. Do not replace the fork with
the official Docker image or copy only the backend directory.

The fork imported official 0.2.8 code without recording the official release
commit as an ancestor. Comparing that release commit with `origin/staging`
identified the 86 fork-specific paths below. Record the already-imported 0.2.8
commit as an ancestry-only merge with an unchanged tree before merging 0.2.10.
This makes the 0.2.8 release the comparison base for Git's three-way merge;
the ancestry commit changes no application files.

The 0.2.8 official tag to deployed fork comparison changes 86 paths. The
official 0.2.8 to 0.2.10 comparison changes 214 paths: 167 backend, 43
frontend, and three Compose files. The following paths are changed on both
sides and require explicit review even if Git merges them automatically:

- `backend/cmd/server/VERSION`
- `backend/internal/service/antigravity_gateway_compat.go`
- `backend/internal/service/antigravity_gateway_compat_test.go`
- `deploy/docker-compose.local.yml`
- `deploy/docker-compose.yml`
- `frontend/src/views/user/KeysView.vue`

Also review cross-file interactions between the Model-Gate Gemini reasoning
compatibility changes, the branded home/auth/user console and documentation
routes, the immutable-image CI/CD workflow, and the upstream 0.2.9-0.2.10
gateway, billing, account, and UI changes. The tag comparison shows no changed
`backend/migrations/**`, schema, `go.mod`, or `go.sum` paths; startup and data
compatibility still require validation rather than inference from that diff.

## Behavior and data boundaries

Keep Model-Gate's existing API compatibility behavior, branding, user console,
documentation, deployment layout, and image promotion rules. Add the upstream
Claude Sonnet 5.5 support, native Claude reset quota view, risk-control user
allowlist, dashboard display choice, and documented gateway, streaming, and
billing fixes. Existing production configuration and customer data remain in
place; validation uses isolated Staging data and credentials.

The candidate version is `0.2.10-mg.1`. The image is built once from the PR
head SHA, deployed to Staging by digest, and promoted to Production only after
the existing manual acceptance and authorization gates. Production deployment
updates the application container without changing PostgreSQL or Redis engine
versions. Database and host OS maintenance are separate activities.

## Verification and acceptance

1. Confirm the release tag and merged Git ancestry, then review every conflict
   and each of the six overlapping paths. Confirm the 86 fork-specific paths
   remain intentionally represented in the result.
2. Run backend tests, frontend tests, type checks, builds, deployment
   configuration checks, and security checks required by the repository.
3. Add or retain focused regression tests for Model-Gate Gemini reasoning and
   compatibility behavior, branded pages and documentation routes, and any
   old behavior touched by a conflict. Exercise upstream 0.2.9-0.2.10 fixes
   for composite routing, streaming usage, account allowlists, and billing.
4. Validate the candidate on isolated Staging: health, login and permissions,
   API keys, channels, representative streaming requests, usage accounting,
   frontend navigation, and responsive views. Compare behavior with the
   deployed baseline where preservation is material.
5. Record the Staging source SHA, immutable digest, test results, backup and
   rollback constraints. Stop promotion if an existing behavior is lost, a
   required test fails, or data compatibility is uncertain.

## Rollout decision

Only a candidate that preserves the Model-Gate behavior and passes Staging
acceptance can move through `staging -> main` and the manual production
workflow. The production `source_sha` is the successfully deployed Staging PR
head SHA. A new digest requires renewed acceptance. Rollback restores only the
application image; any database change needs its own recovery assessment.
