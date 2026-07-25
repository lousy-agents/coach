# Pilot Local Quickstart

A local Compose stack is how we prove the Coach platform before we pay for cloud inference. Async `repo_baseline_scan` in; a report that keeps deterministic findings separate from agent judgments out. This guide is for pilot engineers who want that loop on a laptop.

In about 15 minutes you can bring up API, worker, queue, and local inference, submit a scan, and pull a report. No GitHub App, OAuth, or cloud account is required for the first run. This is a self-serve lab for folks validating whether Coach feedback is worth repeating. It is not a hosted product, a team rollout, or a production security boundary.

Want deterministic signals only, with zero Docker? Use the [`coach codesignal` CLI](../README.md#coach-codesignal-cli-preview) instead. Operator and env-var detail lives in [`docs/development/local-platform.md`](./development/local-platform.md).

---

## What this path covers

Here is the surface area of this guide:

| You run | You get |
| --- | --- |
| Docker Compose and a few `mise` tasks | `coach-api`, `coach-worker`, Postgres, Redis, Envoy AI Gateway, and a model backend |
| Path A (default) | Full pipeline with a canned model stub (proves plumbing and provenance shape) |
| Path B (optional) | Same pipeline with a host-local Qwen 3.5 4B model for real rubric text |
| Path C (optional) | Same pipeline scanning a **remote** GitHub.com repository (no local clone of the target) |
| Success (A/B) | `platform-smoke: ok` and a report with both `source=deterministic` and `source=agent` findings |
| Success (C) | Job `completed` for `owner/repo` on GitHub, report pulled over localhost |

Languages analyzed today: Go, TypeScript, TSX. Paths A and B use an in-compose fixture (`coach-smoke/fixture-repo`). Path C uses GitHub's APIs for a repo you name (for example `lousy-agents/coach`). Coach still performs no GitHub writes. The API and worker run on your machine; only GitHub API traffic leaves the laptop.

---

## Prerequisites

### Required

- Docker with Compose.
- [mise](https://mise.jdx.dev/) (pins Go; the smoke client is `go run ./cmd/platform-smoke`).
- Free host ports: 8080 (API), 1975 and 1064 (AI gateway), 5432 (Postgres), 6379 (Redis).
- A clone of this repository.

```sh
git clone https://github.com/lousy-agents/coach.git
cd coach
mise install   # installs the Go version pinned by the repo
```

Do not export GitHub App, GitHub OAuth, or other ambient cloud credentials while running the smoke. The smoke preflight fails closed if they are present, so a green run cannot be caused by tokens already on your laptop.

### Optional (Path B: real local model)

- Host RAM enough for a 4B-class chat model (Apple Silicon with unified memory is a good fit).
- One of the following OpenAI-compatible servers:
  - [Ollama](https://ollama.com/) serving `qwen3.5:4b` (what most pilots use).
  - Ollama MLX (or another OpenAI-compatible server) serving `qwen3.5:4b-mlx` on Apple Silicon.
  - llama.cpp with a quantized Qwen-family GGUF (our architecture reference path).

The worker never talks to the model process directly. It always calls the in-compose AI gateway, which proxies OpenAI-compatible `POST /v1/chat/completions` to your host server.

### Optional (Path C: remote GitHub.com repository)

You need two separate GitHub integrations. Do not conflate them (ADR-002):

| Integration | Role | Used for |
| --- | --- | --- |
| **GitHub OAuth App** | Who you are | Browser login → Coach JWT on `/v1` |
| **GitHub App** (installation) | What Coach can read | Submit-time authz + worker Contents/tree fetch |

You also need:

- A GitHub user that has a **role** in the target repository (owner, collaborator, or org/team-derived access). Public readability alone is not enough. We deny `403` `repo_not_authorized` for public repos where you have no role on purpose (no-surveillance).
- The Coach GitHub App **installed** on the account or org that owns the target repo, with access to that repo.
- The target does **not** need to be cloned locally. Path C is remote-only: `repo_owner` + `repo_name` (+ optional `ref`). Client-supplied clone URLs are rejected.

---

## Path A: First taste (credential-free, about 10 minutes)

Path A proves the full loop: mint, submit, queue, worker, agent tools, gateway, model, report. It uses the model stub (no weights). Stub agent judgments are schema-valid and canned. That is useful for pipeline confidence, not for reading as insight.

### Start the stack

```sh
mise run platform-up
```

This builds images, starts the `core` profile, and waits until `http://127.0.0.1:8080` answers.

### Run the smoke scan

```sh
mise run platform-smoke
```

Expected ending line:

```text
platform-smoke: ok
```

If the stack is down, smoke exits non-zero (connection refused or timeout). That is intentional. We want a tight feedback loop, not a false green.

### What "ok" means

Smoke already fetched and checked the report. You should see logs that include a submitted `job_id`, then `platform-smoke: ok`.

"ok" means all of the following:

1. A Coach JWT was minted via the test-only `POST /v1/auth/test-mint` path (lab only, not production auth).
2. A `repo_baseline_scan` was accepted for the fixture pair `coach-smoke` / `fixture-repo`.
3. The job reached `completed`.
4. The report contains at least one finding with `source=deterministic` and at least one with `source=agent`.

Containers healthy without that dual-provenance report is not success.

To inspect a report yourself after a successful smoke (optional):

```sh
# Mint a short-lived lab token
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/v1/auth/test-mint \
  -H 'Content-Type: application/json' \
  -d '{"subject":"1","login":"platform-smoke"}' | jq -r .token)

# Use the job_id printed by platform-smoke, or submit again:
JOB=$(curl -s -X POST http://127.0.0.1:8080/v1/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"kind":"repo_baseline_scan","params":{"repo_owner":"coach-smoke","repo_name":"fixture-repo"}}' \
  | jq -r .id)

# Poll until completed, then:
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8080/v1/jobs/$JOB/report" | jq .
```

Look for findings like:

```json
{ "source": "deterministic", "…": "…" }
{ "source": "agent", "rubric_id": "…", "model_identity": "…", "…": "…" }
```

Agent rows never overwrite or suppress deterministic ones. If the model is down or returns invalid JSON for a rubric, we still deliver the deterministic portion and record a diagnostic. The smoke client requires `source=agent` rows, so a broken model path fails the smoke. That is how we keep the proof honest.

---

## Path B: Same stack, your local Qwen

Path B upgrades from canned stub judgments to a host-local Qwen 3.5 4B (or MLX variant). The Compose services stay the same. Only the AI gateway's upstream changes.

### 1. Start an OpenAI-compatible server on the host

Pick one option. The server must expose `POST /v1/chat/completions` and accept the `model` id you will set below.

#### Option 1: Ollama and `qwen3.5:4b` (recommended for most pilots)

```sh
ollama pull qwen3.5:4b
ollama serve   # if not already running; default http://127.0.0.1:11434
```

Quick check from the host:

```sh
curl -s http://127.0.0.1:11434/v1/models | jq .
```

#### Option 2: `qwen3.5:4b-mlx` on Apple Silicon

Use whatever OpenAI-compatible front-end you already run for MLX (for example Ollama's MLX backend, or another local server that speaks `/v1/chat/completions`). Pull or load `qwen3.5:4b-mlx`, then note:

- Host base URL (often still `http://127.0.0.1:11434/v1` for Ollama).
- Exact model id string the server expects in the chat-completions `model` field.

We do not ship an MLX runtime. We only need that HTTP contract.

#### Option 3: llama.cpp (architecture reference)

```sh
# Example shape only. Flags and GGUF path are yours to choose and record.
llama-server --port 8081   # must serve OpenAI-compat /v1/chat/completions
```

Use a quantized 4B-class Qwen-family GGUF. Record source, license, and digest for anything you evaluate seriously. Our production cloud target remains SGLang and Qwen3.5-4B after this local gate. Local serving is a stand-in.

### 2. Point the gateway and bring up the `llm` profile

The supported path is worker → ai-gateway (`:1975`) → your host model. Do not point the worker straight at Ollama or llama.cpp.

```sh
# Ollama defaults (adjust port/model id if yours differ)
export AIGW_OPENAI_BASE_URL=http://host.docker.internal:11434/v1
export MODEL_GATEWAY_MODEL=qwen3.5:4b

# MLX tag example (only if that is the id your server advertises):
# export MODEL_GATEWAY_MODEL=qwen3.5:4b-mlx

# llama.cpp example:
# export AIGW_OPENAI_BASE_URL=http://host.docker.internal:8081/v1
# export MODEL_GATEWAY_MODEL=local   # or whatever id your server accepts

# Optional if judgments are slow on CPU:
# export MODEL_GATEWAY_TIMEOUT=300s

docker compose --profile llm up -d --build
```

Wait until the API is healthy (same signal `platform-up` uses):

```sh
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/v1/me
# expect 401 (listening, unauthenticated) or 200
```

`mise run platform-up` always starts the `core` profile (stub upstream). For a real model you must export the env vars and use `--profile llm` as above. `mise run platform-llm-validate` only prints operator reminders. It does not download weights or run inference.

On Linux, Compose already maps `host.docker.internal` via `host-gateway` on `ai-gateway` and `coach-worker`. If the gateway still cannot reach the host, run the model in a container on the Compose network and set `AIGW_OPENAI_BASE_URL=http://<service>:<port>/v1` instead.

### 3. Re-run smoke

```sh
mise run platform-smoke
```

### What changes vs the stub

| | Path A (`core` + stub) | Path B (host Qwen) |
| --- | --- | --- |
| Agent judgment text | Canned, schema-valid | Model-generated rubric output |
| `source` tags | Still `deterministic` / `agent` | Same separation |
| `MODEL_GATEWAY_MODEL` | `local` (compose default) | Must match the server's model id (`qwen3.5:4b` or `qwen3.5:4b-mlx`) |
| Success bar | Both sources present | Same, plus you can read agent rationales as real model output |

Wrong model id, unreachable host URL, or schema-invalid JSON means missing `source=agent`, which means smoke fails.

---

## Path C: Scan a remote GitHub.com repository

Path C is how you baseline a repo that lives on GitHub without cloning it into the worker. Example target: `https://github.com/lousy-agents/coach` → params `repo_owner=lousy-agents`, `repo_name=coach`.

Do Path A first so you know the stack works. Path C adds real GitHub credentials to the same Compose services. It is slower, more setup, and still lab-only.

### What we require (and why)

Submit is allowed only when **both** are true (ADR-003):

1. The Coach GitHub App installation can read the repository.
2. The authenticated principal has a GitHub role in that repository (`admin`, `maintain`, `write`, `triage`, or `read`).

A nonexistent repo and an unauthorized one both return `403` with code `repo_not_authorized`. That is deliberate.

The worker never uses your OAuth token to fetch files. It mints short-lived **installation** tokens from the App private key and walks the tree via the Contents API. No `git clone` of the target.

### 1. Create a GitHub OAuth App (identity)

In GitHub → Settings → Developer settings → **OAuth Apps** → New OAuth App:

| Field | Local pilot value |
| --- | --- |
| Application name | e.g. `coach-local-pilot` |
| Homepage URL | `http://127.0.0.1:8080` |
| Authorization callback URL | `http://127.0.0.1:8080/oauth/github/callback` |

Register **no scopes**. Coach only needs public `id` and `login` from `GET /user` during login.

Save the **Client ID** and generate a **Client secret**.

### 2. Create and install a GitHub App (repository read)

In GitHub → Settings → Developer settings → **GitHub Apps** → New GitHub App:

| Field | Local pilot value |
| --- | --- |
| GitHub App name | e.g. `coach-local-pilot-app` (must be globally unique) |
| Homepage URL | `http://127.0.0.1:8080` |
| Webhook | Uncheck **Active** (groundwork does not ingest webhooks) |
| Repository permissions | **Contents**: Read-only; **Metadata**: Read-only (automatic) |
| Where can this GitHub App be installed? | Only on this account, or any account if you will install on an org |

Create the app, note the **App ID**, and generate a **private key** (`.pem` download).

Install the app on the account or org that owns the target repo (for `lousy-agents/coach`, install on the `lousy-agents` org, or on your user if you are scanning a personal fork). Grant access to the specific repository (or all repos, if you accept that blast radius for a lab install).

If submit later fails with `repo_not_authorized` while you are sure you are a collaborator, re-check installation target and Contents read permission first. Org-level permission quirks for the collaborators API are still a known edge (ADR-003).

### 3. Keep secrets off git

```sh
mkdir -p secrets
mv ~/Downloads/coach-local-pilot-app.*.private-key.pem secrets/github-app.pem
chmod 600 secrets/github-app.pem
```

`secrets/` and `compose.override.yaml` are gitignored. Do not commit PEMs, client secrets, or override files with real values.

### 4. Compose override (wire credentials into api + worker)

Repo-root `compose.yaml` is credential-free by default. Add a **gitignored** `compose.override.yaml` next to it. Docker Compose merges that file automatically.

```yaml
# compose.override.yaml  (local only; do not commit)
services:
  coach-api:
    environment:
      # Identity (OAuth)
      COACH_GITHUB_OAUTH_CLIENT_ID: "Iv1.xxxxxxxx"
      COACH_GITHUB_OAUTH_CLIENT_SECRET: "xxxxxxxx"
      COACH_GITHUB_OAUTH_REDIRECT_URI: "http://127.0.0.1:8080/oauth/github/callback"
      # Repo authz (same App as the worker)
      COACH_GITHUB_APP_ID: "123456"
      COACH_GITHUB_APP_PRIVATE_KEY_PATH: "/secrets/github-app.pem"
      # Keep fixture bypass so Path A smoke still works alongside live GitHub
      COACH_AUTHZ_BYPASS_OWNER: "coach-smoke"
      COACH_AUTHZ_BYPASS_REPO: "fixture-repo"
      # Lab convenience: leave test-mint on. Prefer OAuth for real-repo scans.
      COACH_AUTH_TEST_MINT: "1"
    volumes:
      - ./secrets/github-app.pem:/secrets/github-app.pem:ro

  coach-worker:
    environment:
      COACH_GITHUB_APP_ID: "123456"
      COACH_GITHUB_APP_PRIVATE_KEY_PATH: "/secrets/github-app.pem"
      # Optional: pin one installation. Prefer unset so the worker resolves
      # installation per owner/repo (needed if you scan across accounts).
      # COACH_GITHUB_INSTALLATION_ID: "987654321"
      #
      # Optional: raise budgets for large trees (defaults: 5000 files / 50 MiB
      # of supported-language source). coach itself can need headroom.
      # COACH_BASELINE_MAX_FILES: "20000"
      # COACH_BASELINE_MAX_TOTAL_BYTES: "104857600"
    volumes:
      - ./secrets/github-app.pem:/secrets/github-app.pem:ro
```

Bring the stack up (or recreate api/worker after editing the override):

```sh
docker compose --profile core up -d --build
# wait until API answers, same as Path A:
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/v1/me
```

You can combine this with Path B by also exporting `AIGW_OPENAI_BASE_URL` / `MODEL_GATEWAY_MODEL` and using `--profile llm`.

### 5. Sign in (Coach JWT)

Open in a browser:

```text
http://127.0.0.1:8080/oauth/github/start
```

Complete GitHub consent. The callback responds with JSON (not a polished UI):

```json
{
  "access_token": "<coach-jwt>",
  "token_type": "bearer"
}
```

Copy `access_token`. That value is the **Coach** JWT for `Authorization: Bearer …` on `/v1`. Your GitHub OAuth access token is not accepted on `/v1` and is never used for repo reads.

Confirm identity:

```sh
export TOKEN='paste-coach-jwt-here'
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/v1/me | jq .
# expect provider=github, your numeric subject, your login
```

Lab shortcut (not production identity): with test-mint still enabled you can mint a JWT whose `login` is your real GitHub login. Submit-time authz still calls GitHub as the App for that login. Prefer the OAuth path above so the subject is verified.

### 6. Submit a remote baseline scan

No local clone of the target. Name the GitHub owner and repo only:

```sh
# Example: https://github.com/lousy-agents/coach
JOB=$(curl -s -X POST http://127.0.0.1:8080/v1/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "kind": "repo_baseline_scan",
    "params": {
      "repo_owner": "lousy-agents",
      "repo_name": "coach",
      "ref": "main"
    }
  }' | jq -r .id)

echo "job_id=$JOB"
```

`ref` is optional. When omitted, the worker uses the repository default branch tip. Do not send `git_url`, `clone_url`, or any clone-style field; those return `400`.

Poll and fetch the report:

```sh
# Poll status until completed or failed
while true; do
  STATUS=$(curl -s -H "Authorization: Bearer $TOKEN" \
    "http://127.0.0.1:8080/v1/jobs/$JOB" | jq -r .status)
  echo "status=$STATUS"
  case "$STATUS" in
    completed|failed) break ;;
  esac
  sleep 2
done

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8080/v1/jobs/$JOB/report" | jq '{
    job_id, kind, commit_sha, summary,
    deterministic: [.findings[] | select(.source=="deterministic")] | length,
    agent: [.findings[] | select(.source=="agent")] | length,
    diagnostics: (.diagnostics|length)
  }'
```

Success means status `completed`, a pinned `commit_sha`, and findings you can inspect. Large repositories take longer: the worker walks the tree over the Contents API and analyzes every supported-language file within budget. If the job fails with a budget or size message, raise `COACH_BASELINE_MAX_FILES` / `COACH_BASELINE_MAX_TOTAL_BYTES` on the worker and recreate it, or scan a smaller repo first.

### Path C failure modes

| Symptom | Likely cause |
| --- | --- |
| `403` `repo_not_authorized` | You have no GitHub role in the repo; App not installed on that owner; App cannot see the repo; or the repo does not exist (same code on purpose) |
| `401` on `/v1/jobs` | Missing/expired Coach JWT; used a GitHub token instead of the Coach JWT from OAuth callback |
| OAuth callback `400` | Wrong callback URL on the OAuth App; stale `state`; user denied consent |
| Job `failed` / "no tree source configured" | Worker missing App id + private key |
| Job `failed` auth/not found during fetch | App installed but Contents permission missing, or installation cannot read that repo |
| Job `failed` budget / too large | Tree exceeds default 5000 files or 50 MiB supported source; raise worker budgets or pick a smaller target |
| Slow job on a monorepo | Expected: remote Contents walk is chatty. Prefer a focused repo for first Path C runs |
| `platform-smoke` fails ambient-credential guard | Host shell has `GITHUB_TOKEN` / `GH_TOKEN` / similar set. Path C credentials belong in Compose containers via override, not necessarily in your interactive shell. Unset ambient tokens before smoke, or skip smoke while debugging live GitHub |

### Path C honesty bounds

- Still lab Compose: fixed JWT signing key, optional test-mint, fixture bypass remain. Do not expose `:8080` beyond localhost.
- No GitHub writes (no PR comments, checks, or status).
- Scanning `lousy-agents/coach` only works if **you** have a role there **and** your pilot App is installed on that org with repo access. Being able to `git clone` a public URL is not enough.
- Cloud SGLang/AWS and `pr_history_scan` are still out of scope here.

---

## What the report means

### Deterministic vs agent

- `source=deterministic` means reproducible structural signals from `pkg/semantics` / `pkg/codesignal` (rule id and version). Same commit and same analyzer versions yield the same findings.
- `source=agent` means LLM-as-judge output for a versioned rubric (`rubric_id`, `rubric_version`, model identity). Opinion and context, never a replacement for deterministic evidence.

Our trust posture for this era is provenance over polish. Fewer, clearer findings beat exhaustive noise.

### Privacy on the smoke path

- The analysis target is a mounted fixture, not your working tree or private repos.
- There is no GitHub OAuth token and no GitHub App private key.
- Coach performs no GitHub writes (no PR comments, no checks).
- You pull results over HTTP on localhost.

Live "scan a real GitHub repo" is **Path C** above. Deeper env matrices live in [local-platform.md](./development/local-platform.md); contracts live under `.github/specs/` and ADR-001 through ADR-003.

---

## Limits (honest)

- Pilot and lab only. Compose enables test mint, a fixed local JWT signing key, and an authz bypass for the fixture pair. Path C adds real GitHub secrets to the same lab boundary. Do not expose this stack on a network or treat it as multi-tenant production.
- Not a linter, CI gate, or merge blocker. Advisory reports only.
- Not a public PR review bot. No GitHub writes in this era.
- Go, TypeScript, and TSX only. Other languages are skipped.
- `pr_history_scan`, web UI, harness hooks, SGLang, and AWS are out of scope here. Local Compose validation is the investment gate for cloud serving.
- Ollama and MLX are supported only as OpenAI-compatible upstreams behind the AI gateway, the same wire contract as llama.cpp. We do not vendor those runtimes.
- Compose is not a hostile-code sandbox. Do not point this stack at untrusted repositories expecting isolation.
- Path C is not anonymous OSS scanning. Relationship-gated authz is load-bearing.

We are still sharpening this path. Polished hosted login UX and cloud serving remain work in progress.

---

## Troubleshooting

| Symptom | Likely fix |
| --- | --- |
| `platform-smoke` connection refused | Run `mise run platform-up` (or wait for health) first |
| Smoke fails ambient-credential guard | Unset GitHub App, OAuth, `GITHUB_*`, and similar env vars in that shell |
| Report missing `source=agent` | Gateway cannot reach the model; wrong `AIGW_OPENAI_BASE_URL`; wrong `MODEL_GATEWAY_MODEL`; model returned non-schema JSON; timeout too low |
| Report missing `source=deterministic` | Worker fixture mount or owner-repo pair mismatch (defaults: `coach-smoke` / `fixture-repo`) |
| Host model unreachable from Docker | Use `host.docker.internal` (not `localhost` inside the container); on Linux prefer a compose-network model service if host-gateway fails |
| Stale Postgres schema after pulling main | `docker compose --profile core --profile llm down -v`, then bring the stack up again |
| Expecting your real GitHub repo in smoke | Smoke always scans the fixture; use Path C for remote GitHub.com repos |
| Believing stub text is "the model" | Path A always hits `model-stub` unless you switched aigw upstream (Path B) |
| Path C `403` on a public repo you only read | You need a GitHub **role** in that repo plus App install; see Path C failure modes |

---

## Tear down

```sh
mise run platform-down
```

Named Postgres volume is kept by default. Wipe data:

```sh
docker compose --profile core --profile llm down -v --remove-orphans
```

---

## FAQ

**Do I need a GitHub App or OAuth to try this?**  
No for Paths A and B. Test mint and the `coach-smoke/fixture-repo` fixture only. Path C (remote GitHub.com repos) needs both a GitHub OAuth App and a GitHub App installation.

**Does Coach post comments, checks, or write anything to GitHub?**  
No. Submit, poll, and fetch the report. Nothing is written back to GitHub in this era. Path C only **reads** via the App installation.

**Can I scan `https://github.com/lousy-agents/coach` without cloning it?**  
Yes, that is Path C. Submit `repo_owner=lousy-agents` and `repo_name=coach`. You still need a GitHub role in that repo and an App install that can read it. Public clone access alone is not enough.

**Stub vs `qwen3.5:4b` / `qwen3.5:4b-mlx`: what is the difference?**  
Stub returns canned, schema-valid agent judgments so the pipeline is testable without weights. Your local Qwen produces real rubric text. Both are still labeled `source=agent` and never mixed into deterministic evidence.

**Where does my code go on the smoke path?**  
It does not. Analysis runs against a small in-compose fixture. Traffic stays on your machine. Path C reads the remote tree over GitHub's API into the worker process; still no GitHub writes.

**What languages and job types work today?**  
Baseline scan over Go, TypeScript, and TSX. PR-history scan and other languages are not part of this quickstart.

**How do I know it actually worked?**  
Paths A/B: `platform-smoke: ok` and a completed report with both `source=deterministic` and `source=agent` findings. Path C: job status `completed`, a pinned `commit_sha` for the remote ref, and a report you pull from `/v1/jobs/{id}/report`. Containers up without that report is not success.

---

## Command cheat-sheet

```sh
# Path A: plumbing and stub judgments
mise run platform-up
mise run platform-smoke
mise run platform-down

# Path B: host Qwen via Ollama (example)
ollama pull qwen3.5:4b
export AIGW_OPENAI_BASE_URL=http://host.docker.internal:11434/v1
export MODEL_GATEWAY_MODEL=qwen3.5:4b
docker compose --profile llm up -d --build
mise run platform-smoke

# Path C: remote GitHub.com repo (after compose.override.yaml + App install)
open http://127.0.0.1:8080/oauth/github/start   # copy access_token from JSON
export TOKEN='…'
curl -s -X POST http://127.0.0.1:8080/v1/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"kind":"repo_baseline_scan","params":{"repo_owner":"lousy-agents","repo_name":"coach","ref":"main"}}'
```

---

## Related docs

| Doc | Audience |
| --- | --- |
| [local-platform.md](./development/local-platform.md) | Operators: full env matrix, aigw notes, CI smoke |
| [prd.md](./product/prd.md) | Product direction for the groundwork era |
| [system-overview.md](./architecture/system-overview.md) | Long-term architecture (webhook platform, SGLang/AWS) |
| Epic [#97](https://github.com/lousy-agents/coach/issues/97) | Baseline Scan platform epic |
