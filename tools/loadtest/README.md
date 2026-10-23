# Local gateway recovery load exercise

This exercises the real gateway with synthetic local PostgreSQL/Redis fixtures
and an HTTP mock. It never accepts a remote target or real upstream credentials.
It is intentionally separate from routine CI: the default matrix includes real
120-second waits. `aiohttp` and `psutil` are required. Use a fresh dedicated local
database; `seed` is not idempotent.

Test workspace: `/workspace/.onboarding/recovery-load`. Build the backend there
as `server` with the normal `embed` build tag. Start dedicated local containers
named `mg-recovery-load-pg` (PostgreSQL 18, user/database `loadtest`, port 15452)
and `mg-recovery-load-redis` (Redis 8.4, port 16392). Bind both only to 127.0.0.1.
Use at least 1.5GiB for PostgreSQL in this fixture, Redis 128MiB. No existing
project databases, real user records or deployment containers should be reused.

First initialize an empty gateway with its normal `AUTO_SETUP=true` mechanism,
using `DATA_DIR=/workspace/.onboarding/recovery-load/data`, the two local service
ports above, an entirely synthetic admin and JWT secret, and port 18086. Wait for
health, then stop that bootstrap process. This creates the schema/config without
accepting compliance statements or making administrative changes to any real site.

```sh
python tools/loadtest/gateway_recovery.py seed
python tools/loadtest/launch_gateway.py
python tools/loadtest/gateway_recovery.py serve
```

Run the mock in a separate terminal/process, then execute the matrix:

```sh
python tools/loadtest/gateway_recovery.py run
python tools/loadtest/gateway_recovery.py probes
python tools/loadtest/gateway_recovery.py cancel-probe
```

`launch_gateway.py` uses a 20/10 database pool, a local synthetic price catalogue,
2 Go processors and a 512MiB Go memory soft limit. The main matrix uses recovery
300/120/12 and waiting admission 256/64/2. `probes` restarts **only the isolated
gateway**, clears **only its dedicated Redis DB**, and changes **only its
synthetic accounts** to compare recovery off/on. Its final probe uses a scaled
3-second recovery budget. The scripts do not deploy changes anywhere.

Results are written to `results.json`, `resources.json`, and
`cancellation-slots.json` in the test workspace. The checked-in report records
the observed run, limitations and the excluded initial database-memory failure.
HTTP 200 alone is not counted as success: a semantic answer and, for streaming,
the terminal marker are required.

After collecting evidence, stop the PIDs recorded in `server.pid` and `mock.pid`
and remove only the two named temporary containers and their anonymous volumes.
Do not run these commands against a shared Docker context or real database.
