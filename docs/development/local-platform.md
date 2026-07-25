# Local platform stack (coach-api + worker)

Operator guide for the **Baseline Scan Story 4 / Task 10** Docker Compose stack.
This is **not** Feature Zero's offline thinproof suite (`deploy/compose/thinproof`,
`mise run test-acceptance-thin-proof`).

## Profiles

| Profile | Services | Model path | GitHub credentials |
| --- | --- | --- | --- |
| **`core`** | `coach-api`, `coach-worker`, `postgres`, `redis` | In-process deterministic **stub** (`MODEL_GATEWAY_BASE_URL` unset) | **None** (credential-free) |
| **`llm`** | Same as `core` | OpenAI-compat gateway → host-native **llama.cpp** via `host.docker.internal` | Still none for smoke; live App/OAuth only if you add them |

Compose file: repo-root [`compose.yaml`](../../compose.yaml).

## mise tasks

```sh
mise run platform-up              # build + up core profile; wait until API healthy
mise run platform-down            # compose down (keeps named volume by default)
mise run platform-stop            # alias of platform-down
mise run platform-smoke           # ambient-credential guard + E2E smoke against running stack
mise run platform-llm-validate    # operator-only: docs + optional llm profile bring-up hints
```

Typical loop:

```sh
mise run platform-up
mise run platform-smoke
mise run platform-down
```

Smoke **must** fail non-zero when the stack is down (connection refused / timeout).

## Credential-free smoke — minimal env

Compose injects the env below. **Do not** set GitHub App or OAuth variables for
core/smoke. The ambient-credential guard (`go run ./cmd/acceptance-guard-preflight`)
runs before smoke so ambient developer credentials cannot make the run pass
falsely.

### API (`coach-api`)

| Variable | Value in core | Purpose |
| --- | --- | --- |
| `COACH_HTTP_ADDR` | `:8080` | Listen address |
| `COACH_JWT_SIGNING_KEY` | fixed local string (≥32 bytes) | Coach JWT HMAC key |
| `COACH_JWT_ISSUER` | `http://coach-api:8080` | JWT `iss` |
| `COACH_AUTH_TEST_MINT` | `1` | Enable `POST /v1/auth/test-mint` (non-production) |
| `COACH_REDIS_ADDR` | `redis:6379` | Redis Streams |
| `COACH_REDIS_STREAM` | `coach-jobs` | Shared with worker |
| `COACH_REDIS_CONSUMER_GROUP` | `coach-api` | Enqueue-side group name |
| `COACH_PG_DSN` | `postgres://coach:coach@postgres:5432/coach?sslmode=disable` | Job store |
| `COACH_AUTHZ_BYPASS_OWNER` | `coach-smoke` | Submit-time authz bypass owner |
| `COACH_AUTHZ_BYPASS_REPO` | `fixture-repo` | Submit-time authz bypass repo |

**Omitted on purpose (must stay unset for credential-free smoke):**

- `COACH_GITHUB_APP_ID`
- `COACH_GITHUB_APP_PRIVATE_KEY` / `COACH_GITHUB_APP_PRIVATE_KEY_PATH`
- `COACH_GITHUB_OAUTH_CLIENT_ID` / `COACH_GITHUB_OAUTH_CLIENT_SECRET`
- any other GitHub credential

When both bypass vars are set, coach-api starts without App credentials and
wraps a fail-closed deny-all authorizer in `BypassAuthorizer` so **only** the
fixture owner/repo pair can be submitted.

### Worker (`coach-worker`)

| Variable | Value in core | Purpose |
| --- | --- | --- |
| `COACH_WORKER_ID` | `platform-worker-1` | Lease / consumer identity |
| `COACH_REDIS_ADDR` | `redis:6379` | Redis Streams |
| `COACH_REDIS_STREAM` | `coach-jobs` | **Same stream as API** |
| `COACH_REDIS_CONSUMER_GROUP` | `coach-workers` | Consume-side group — **must differ** from API (`coach-api`); both open a Redis Streams subscriber today, so a shared group lets the API drain work before the worker |
| `COACH_PG_DSN` | same as API | Shared job store |
| `COACH_SMOKE_FIXTURE_PATH` | `/fixtures/smoke-repo` | Mounted fixture tree |
| `COACH_SMOKE_REPO_OWNER` | `coach-smoke` | Owner pair → fixture path |
| `COACH_SMOKE_REPO_NAME` | `fixture-repo` | Repo pair → fixture path |
| `MODEL_GATEWAY_BASE_URL` | **unset** (core) | Unset → stub; set → OpenAI-compat |

GitHub App vars on the worker are optional and **unset** in core/smoke.

### Smoke client (`cmd/platform-smoke`)

| Variable | Default | Purpose |
| --- | --- | --- |
| `COACH_PLATFORM_SMOKE_BASE_URL` | `http://127.0.0.1:8080` | API base URL |
| `COACH_SMOKE_REPO_OWNER` | `coach-smoke` | Must match bypass + worker pair |
| `COACH_SMOKE_REPO_NAME` | `fixture-repo` | Must match bypass + worker pair |
| `COACH_PLATFORM_SMOKE_TIMEOUT` | `2m` | Overall smoke deadline |

Flow: mint token → `POST /v1/jobs` `repo_baseline_scan` for the fixture
owner/name (**no client-supplied clone URL**) → poll → `GET …/report` → require
provenance-tagged findings with **both** `source=deterministic` (fixture
signals) and `source=agent` (stub/gateway judgment path). Core smoke fails if
either is missing.

Fixture tree: `deploy/compose/platform/fixtures/smoke-repo/` (tiny Go sources that
trigger deterministic analysis).

## Postgres migrations

SQL under `internal/coachapi/migrations/*.sql` is mounted into Postgres
`docker-entrypoint-initdb.d` and applied **once** on first volume init (ordered
by filename). `PostgresStore` does not auto-migrate. To re-apply from scratch:

```sh
docker compose --profile core down -v
mise run platform-up
```

## `llm` profile (operator-only, not CI)

CI runs **only** the `core` profile (stub). Real-model validation is operator-run:

1. Start a host-native llama.cpp server with an OpenAI-compatible
   `/v1/chat/completions` endpoint (prefer Metal on macOS), e.g. listening on
   `127.0.0.1:8081`.
2. Export gateway env and bring up the `llm` profile:

   ```sh
   export MODEL_GATEWAY_BASE_URL=http://host.docker.internal:8081/v1
   export MODEL_GATEWAY_MODEL=local   # or your served model id
   docker compose --profile llm up -d --build
   # wait healthy, then:
   mise run platform-smoke
   ```

3. Expect the report to include ≥1 schema-valid `source=agent` judgment produced
   by llama.cpp (not only the stub).

**Linux fallback:** if host networking to llama.cpp is awkward, run a
containerized llama.cpp on the compose network and point
`MODEL_GATEWAY_BASE_URL` at that service instead of `host.docker.internal`.
GPU/weights are **not** required in CI and are not pulled by this repo's
compose file.

`mise run platform-llm-validate` prints these steps and exits 0 when used as a
docs/reminder task (it does not download weights).

## Distinction from thinproof (Feature Zero)

| | Platform stack (this doc) | Thinproof |
| --- | --- | --- |
| Path | `compose.yaml` | `deploy/compose/thinproof/` |
| mise | `platform-up` / `platform-smoke` | `thinproof-build` / `test-acceptance-thin-proof` |
| Network | Normal (pull postgres/redis) | Internal no-egress, `pull_policy: never` |
| Surface | Full `coach-api` + `coach-worker` HTTP/job path | `pkg/githubingest` → CodeSignal runner |
| Auth | Test mint + authz bypass | N/A (no coach-api) |
| CI job | `platform-smoke` | (via thinproof acceptance when run) |
