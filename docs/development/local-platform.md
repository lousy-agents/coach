# Local platform stack (coach-api + worker)

Operator guide for the **Baseline Scan Story 4 / Task 10** Docker Compose stack.
This is **not** Feature Zero's offline thinproof suite (`deploy/compose/thinproof`,
`mise run test-acceptance-thin-proof`).

**End-user quickstart:** [`docs/pilot-local-quickstart.md`](../pilot-local-quickstart.md).
This page is the operator env-var reference.

## Profiles

| Profile | Services | Model path | GitHub credentials |
| --- | --- | --- | --- |
| **`core`** | `coach-api`, `coach-worker`, `postgres`, `redis`, `model-stub`, `ai-gateway` | Worker → **Envoy AI Gateway** (`ai-gateway:1975`) → **model-stub** (canned OpenAI chat completions, no weights) | **None** (credential-free) |
| **`llm`** | Same as `core` | Worker → **ai-gateway** → host-native **llama.cpp** (`AIGW_OPENAI_BASE_URL=http://host.docker.internal:…`) | Still none for smoke; live App/OAuth only if you add them |

Compose file: repo-root [`compose.yaml`](../../compose.yaml).

**Local config:** copy [`.env.example`](../../.env.example) → `.env` (gitignored). Compose loads `.env` for `${VAR}` interpolation into service `environment`. Path C PEM convention: host `secrets/github-app.pem` → container `/secrets/github-app.pem` (`secrets/` is always bind-mounted). Recreate containers after `.env` edits.

### Inference data plane

```text
coach-worker (internal/modelgateway OpenAI client)
  → ai-gateway  (Envoy AI Gateway CLI / aigw, :1975)
  → model-stub  (core)  or  llama.cpp (llm)
```

- **Product policy** (schema validation, typed unavailable errors, degrade-to-deterministic) stays in Go `internal/modelgateway`.
- **ai-gateway** is a transparent OpenAI-compatible data-plane proxy (token/access-log shape for production parity). It must **not** rewrite upstream 5xx into 202 (see `docs/architecture/system-overview.md` pilot wake path).
- Reference Gateway API config: [`deploy/compose/platform/aigw/config.yaml`](../../deploy/compose/platform/aigw/config.yaml). Compose starts aigw with `OPENAI_BASE_URL` / `OPENAI_API_KEY` (env-synthesized route, same as upstream aigw examples).

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
| `COACH_GITHUB_*` | empty via `${…:-}` | Path C from `.env`; leave unset/empty for credential-free smoke |

**Must stay empty for credential-free smoke** (no Path C keys in `.env`):

- `COACH_GITHUB_APP_ID`
- `COACH_GITHUB_APP_PRIVATE_KEY` / `COACH_GITHUB_APP_PRIVATE_KEY_PATH`
- `COACH_GITHUB_OAUTH_CLIENT_ID` / `COACH_GITHUB_OAUTH_CLIENT_SECRET`
- any other GitHub credential

When both bypass vars are set and App credentials are empty, coach-api starts
without App credentials and wraps a fail-closed deny-all authorizer in
`BypassAuthorizer` so **only** the fixture owner/repo pair can be submitted.
Path C sets App + OAuth in `.env` while bypass remains for fixture smoke.

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
| `MODEL_GATEWAY_BASE_URL` | `http://ai-gateway:1975/v1` | OpenAI-compat client → Envoy AI Gateway |
| `MODEL_GATEWAY_MODEL` | `local` | Logical model id |
| `MODEL_GATEWAY_TIMEOUT` | `120s` | Outbound judgment timeout |
| `MODEL_GATEWAY_API_KEY` | `unused` | Bearer passed through aigw (stub ignores) |
| `MODEL_GATEWAY_DISABLE_THINKING` | unset (core) | When `1`/`true`, request body includes `think: false` (local llm Path B) |
| `COACH_JUDGMENT_MAX_WALL_TIME` | `10m` default | Judgment-phase wall (min `5m` if set) |
| `COACH_MAX_HIDDEN_MUTATION_JUDGMENTS` | `0`→16 | Cap judged hidden-mutation signals; negative = unlimited |
| `COACH_MAX_FINDINGS_PER_JUDGMENT_PACK` | `4` | Max findings per Judge call |
| `COACH_MAX_JUDGMENT_PROMPT_TOKENS` | `3500` | Pack split estimator |
| `COACH_JUDGMENT_FILE_AFFINITY_MIN_FINDINGS` | `5` | Dense-path dedicated packs |
| `COACH_JUDGMENT_EVIDENCE_WINDOW_LINES` | `15` | ±N evidence lines |
| `COACH_GITHUB_APP_ID` / `_PRIVATE_KEY_PATH` | empty | Path C from `.env` |
| volumes | `secrets/` → `/secrets` | PEM convention; directory always mounted |

Core/smoke needs **none** of the judgment knobs or `MODEL_GATEWAY_DISABLE_THINKING` (defaults + stub gateway). GitHub App vars on the worker are optional and **empty** in core/smoke.

**Recreate containers** after changing `.env` (`MODEL_GATEWAY_*`, `AIGW_OPENAI_BASE_URL`, judgment, or GitHub keys) — compose does not hot-reload them.

### Envoy AI Gateway (`ai-gateway`)

| Variable | Value in core | Purpose |
| --- | --- | --- |
| `OPENAI_BASE_URL` | `http://model-stub:8090/v1` (via `AIGW_OPENAI_BASE_URL`) | Upstream OpenAI-compatible backend |
| `OPENAI_API_KEY` | `unused` | Required by aigw env-synthesized config |
| `AIGW_RUN_ID` | `0` | Stable run id for containers |
| Admin | `:1064` (`/health`, `/metrics`) | Healthcheck target |

Host ports: `1975` (proxy), `1064` (admin).

### Model stub (`model-stub`)

| Variable | Value in core | Purpose |
| --- | --- | --- |
| `COACH_MODEL_STUB_ADDR` | `:8090` | Listen address |
| Endpoints | `GET /healthz`, `POST /v1/chat/completions` | Canned schema-valid rubric JSON in assistant content |

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
signals) and `source=agent` (worker → aigw → model-stub judgment path). Core
smoke fails if either is missing.

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

CI runs **only** the `core` profile (aigw → model-stub). Real-model validation
is operator-run. The worker always calls **aigw**; aigw's upstream is any
OpenAI-compatible `POST /v1/chat/completions` server on the host or compose
network.

**Pilot-oriented steps** (Ollama **Qwen 3.5** `qwen3.5:4b` / `qwen3.5:4b-mlx`,
**Gemma 4** e.g. `gemma4:12b`, plus llama.cpp) live in
[`docs/pilot-local-quickstart.md`](../pilot-local-quickstart.md) Path B.
Condensed operator form:

1. Start a host OpenAI-compatible server (architecture reference: native
   llama.cpp with Metal on macOS; pilots often use Ollama). Example listen
   addresses: Ollama `127.0.0.1:11434`, llama.cpp `127.0.0.1:8081`.
2. Copy [`.env.example`](../../.env.example) → `.env` and set Path B keys
   (worker still calls aigw, not the host directly). **Recreate** worker/aigw
   after changing `.env`:

   ```sh
   cp .env.example .env
   # Edit .env — examples:
   #   AIGW_OPENAI_BASE_URL=http://host.docker.internal:11434/v1
   #   MODEL_GATEWAY_MODEL=qwen3.5:4b          # or gemma4:12b / local
   #   MODEL_GATEWAY_TIMEOUT=120s
   #   MODEL_GATEWAY_DISABLE_THINKING=1        # Ollama-style; leave empty for llama.cpp

   mkdir -p secrets
   docker compose --profile llm up -d --build
   # wait healthy, then:
   mise run platform-smoke
   ```

3. Expect the report to include ≥1 schema-valid `source=agent` judgment produced
   by the host model (not only model-stub).

**Linux fallback:** if host networking is awkward, run the model server on the
compose network and set `AIGW_OPENAI_BASE_URL=http://<service>:<port>/v1` in
`.env`. GPU/weights are **not** required in CI and are not pulled by this
repo's compose file.

`mise run platform-llm-validate` prints a short reminder and exits 0 (it does
not download weights).

## Distinction from thinproof (Feature Zero)

| | Platform stack (this doc) | Thinproof |
| --- | --- | --- |
| Path | `compose.yaml` | `deploy/compose/thinproof/` |
| mise | `platform-up` / `platform-smoke` | `thinproof-build` / `test-acceptance-thin-proof` |
| Network | Normal (pull postgres/redis/aigw) | Internal no-egress, `pull_policy: never` |
| Surface | Full `coach-api` + `coach-worker` + aigw data plane | `pkg/githubingest` → CodeSignal runner |
| Auth | Test mint + authz bypass | N/A (no coach-api) |
| CI job | `platform-smoke` | (via thinproof acceptance when run) |
