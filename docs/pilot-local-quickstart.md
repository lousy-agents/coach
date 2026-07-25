# Local Coach quickstart

Run Coach on your laptop with Docker Compose. You submit a baseline scan, wait for the job to finish, and download a report. The report labels each finding as either **deterministic** (rule-based, same inputs → same result) or **agent** (model judgment).

This guide covers three paths:

| Path | What you do | What you need |
| --- | --- | --- |
| **A** | Smoke test with a built-in sample repo and a fake model | Docker, mise |
| **B** | Same smoke test with a real local model (Qwen 3.5 or Gemma 4) | Path A plus Ollama or similar |
| **C** | Scan a real GitHub.com repo without cloning it | Path A plus GitHub OAuth App and GitHub App |

Path A takes about 10–15 minutes the first time (image builds dominate). Paths B and C add setup on top.

For deterministic analysis of a local git checkout with no Docker, use the [`coach codesignal` CLI](../README.md#coach-codesignal-cli-preview) instead.

---

## What you get

- API on `http://127.0.0.1:8080`
- Background worker, Postgres, Redis, and a model gateway
- Job type: `repo_baseline_scan` (Go, TypeScript, and TSX files only)
- No writes to GitHub (no PR comments, checks, or status updates)
- Paths A and B analyze a small fixture repo (`coach-smoke` / `fixture-repo`), not your code
- Path C reads a remote repo over the GitHub API into the worker on your machine

This stack is for local experiments. It uses fixed lab credentials and test login shortcuts. Do not expose port 8080 on a public network.

---

## Prerequisites

### All paths

- Docker with Compose
- [mise](https://mise.jdx.dev/)
- Free ports: **8080**, **1975**, **1064**, **5432**, **6379**
- A clone of this repository

```sh
git clone https://github.com/lousy-agents/coach.git
cd coach
mise install
```

Before Path A or B smoke tests, unset GitHub and cloud tokens in that shell (`GITHUB_TOKEN`, `GH_TOKEN`, and similar). The smoke check fails if those are set, so a pass cannot depend on credentials already on your machine.

### Path B only

- Enough RAM for a ~4B–12B chat model (Apple Silicon works well)
- One OpenAI-compatible server on the host:
  - [Ollama](https://ollama.com/) with `qwen3.5:4b` (usual Qwen 3.5 choice)
  - Ollama or another server with `qwen3.5:4b-mlx` on Apple Silicon
  - Ollama (or similar) with a **Gemma 4** id your server exposes (example tag below)
  - llama.cpp with a Qwen- or Gemma-family GGUF, if you already run that

Coach talks to the model only through the compose gateway (`POST /v1/chat/completions`). You do not configure the worker to call Ollama directly.

### Path C only

Two different GitHub apps (both required):

| App | Purpose |
| --- | --- |
| **OAuth App** | Sign you in; Coach issues its own API token |
| **GitHub App** | Lets Coach read repository files and check that you have access |

Also required:

- You have a **role** on the target repo (owner, collaborator, or org/team access). Being able to view a public repo is not enough; you will get `403` `repo_not_authorized`.
- The GitHub App is **installed** on the account or org that owns the repo, with access to that repo.
- You do **not** clone the target repo. You only pass `repo_owner`, `repo_name`, and optional `ref`.

---

## Path A: Smoke test (no GitHub, fake model)

Starts the stack, runs one scan against the fixture repo, and checks that the report has both kinds of findings.

### Start

```sh
mise run platform-up
```

Builds images if needed, starts the default profile, and waits until the API responds.

### Smoke

```sh
mise run platform-smoke
```

You want:

```text
platform-smoke: ok
```

If nothing is listening, the command exits non-zero (connection refused or timeout). That is expected.

### What "ok" means

1. A short-lived lab token was created (`POST /v1/auth/test-mint`; local testing only).
2. A `repo_baseline_scan` for `coach-smoke` / `fixture-repo` was accepted.
3. The job reached `completed`.
4. The report has at least one `source=deterministic` finding and at least one `source=agent` finding.

Containers running without that report is not a pass. The fake model returns fixed, valid agent JSON so you can verify the pipeline without downloading weights. Those agent texts are not real analysis.

### Optional: inspect a report yourself

```sh
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/v1/auth/test-mint \
  -H 'Content-Type: application/json' \
  -d '{"subject":"1","login":"platform-smoke"}' | jq -r .token)

JOB=$(curl -s -X POST http://127.0.0.1:8080/v1/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"kind":"repo_baseline_scan","params":{"repo_owner":"coach-smoke","repo_name":"fixture-repo"}}' \
  | jq -r .id)

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1:8080/v1/jobs/$JOB/report" | jq .
```

Findings look like:

```json
{ "source": "deterministic", "…": "…" }
{ "source": "agent", "rubric_id": "…", "model_identity": "…", "…": "…" }
```

Agent findings never remove or change deterministic ones. If the model is down or returns invalid JSON, deterministic findings can still appear and the problem is recorded as a diagnostic. The smoke command still requires agent findings, so a broken model path fails smoke.

---

## Path B: Real local model

Same stack and smoke as Path A. Point the gateway at a model on your machine.

**Recreate containers after changing model/gateway/judgment env.** Compose does not hot-reload `AIGW_OPENAI_BASE_URL`, `MODEL_GATEWAY_*`, or worker judgment knobs into already-running containers. After exports change, run `docker compose --profile llm up -d --build` again (or `down` then `up`) so worker and ai-gateway pick up the new values.

### 1. Start a model server

**Qwen 3.5 via Ollama (`qwen3.5:4b`):**

```sh
ollama pull qwen3.5:4b
ollama serve   # default http://127.0.0.1:11434
curl -s http://127.0.0.1:11434/v1/models | jq .
```

**Qwen 3.5 MLX (`qwen3.5:4b-mlx`):** run any OpenAI-compatible server that serves that id (often still port 11434 with Ollama). Note the exact `model` string the server expects.

**Gemma 4 via Ollama (example id — use the tag your server lists):**

```sh
# Example; confirm the id with: curl -s http://127.0.0.1:11434/v1/models | jq .
ollama pull gemma4:12b
ollama serve
curl -s http://127.0.0.1:11434/v1/models | jq .
```

**llama.cpp:** serve OpenAI-compatible chat on a port you choose (example: 8081).

### 2. Start compose with that upstream

```sh
# --- Qwen 3.5 (Ollama) ---
export AIGW_OPENAI_BASE_URL=http://host.docker.internal:11434/v1
export MODEL_GATEWAY_MODEL=qwen3.5:4b

# --- Gemma 4 (Ollama; id must match /v1/models) ---
# export AIGW_OPENAI_BASE_URL=http://host.docker.internal:11434/v1
# export MODEL_GATEWAY_MODEL=gemma4:12b

# MLX example (only if your server uses this id):
# export MODEL_GATEWAY_MODEL=qwen3.5:4b-mlx

# llama.cpp example:
# export AIGW_OPENAI_BASE_URL=http://host.docker.internal:8081/v1
# export MODEL_GATEWAY_MODEL=local

# Local-LLM recommendations (Path B):
export MODEL_GATEWAY_TIMEOUT=120s          # raise to 300s if judgments time out
export MODEL_GATEWAY_DISABLE_THINKING=1    # Ollama-style think:false when supported

# Optional judgment packing / budget knobs (worker; defaults shown):
# export COACH_JUDGMENT_MAX_WALL_TIME=10m                 # min 5m
# export COACH_MAX_HIDDEN_MUTATION_JUDGMENTS=16           # 0=default 16; negative=unlimited
# export COACH_MAX_FINDINGS_PER_JUDGMENT_PACK=4
# export COACH_MAX_JUDGMENT_PROMPT_TOKENS=3500
# export COACH_JUDGMENT_FILE_AFFINITY_MIN_FINDINGS=5
# export COACH_JUDGMENT_EVIDENCE_WINDOW_LINES=15

docker compose --profile llm up -d --build
```

Wait for the API:

```sh
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/v1/me
# 401 or 200 means it is listening
```

`mise run platform-up` always uses the fake model. For a real model, set the env vars and use `--profile llm` as above.

On Linux, if the gateway cannot reach the host, run the model in a container on the same compose network and set `AIGW_OPENAI_BASE_URL=http://<service>:<port>/v1`.

### Path B env reference

| Variable | Typical Path B | Purpose |
| --- | --- | --- |
| `AIGW_OPENAI_BASE_URL` | `http://host.docker.internal:11434/v1` | aigw upstream (Ollama/llama.cpp) |
| `MODEL_GATEWAY_MODEL` | `qwen3.5:4b` or `gemma4:12b` | Must match the server’s model id |
| `MODEL_GATEWAY_TIMEOUT` | `120s` (try `300s` if slow) | Outbound judgment HTTP timeout |
| `MODEL_GATEWAY_DISABLE_THINKING` | `1` | Adds `think: false` on chat completions (best-effort; ignored if unsupported) |
| `COACH_JUDGMENT_MAX_WALL_TIME` | `10m` (min `5m`) | Judgment-phase wall budget (separate from analyze) |
| `COACH_MAX_HIDDEN_MUTATION_JUDGMENTS` | `16` (`0`→16; negative→unlimited) | Cap on judged hidden-mutation signals |
| `COACH_MAX_FINDINGS_PER_JUDGMENT_PACK` | `4` | Max findings per model call |
| `COACH_MAX_JUDGMENT_PROMPT_TOKENS` | `3500` | Soft pack split (chars/4 estimator) |
| `COACH_JUDGMENT_FILE_AFFINITY_MIN_FINDINGS` | `5` | Dense paths get dedicated packs |
| `COACH_JUDGMENT_EVIDENCE_WINDOW_LINES` | `15` | ±N lines of evidence around the signal |

If jobs hit the judgment wall with few agent rows, lower `COACH_MAX_HIDDEN_MUTATION_JUDGMENTS` or raise `COACH_JUDGMENT_MAX_WALL_TIME` / `MODEL_GATEWAY_TIMEOUT`—do not only shrink pack size.

### 3. Smoke again

```sh
mise run platform-smoke
```

| | Path A | Path B |
| --- | --- | --- |
| Agent text | Fixed sample JSON | From your local model |
| `source` labels | `deterministic` / `agent` | Same |
| `MODEL_GATEWAY_MODEL` | `local` (default) | Must match the server’s model id (Qwen 3.5 or Gemma 4 example above) |

Wrong model id, unreachable URL, or invalid model JSON → missing `source=agent` → smoke fails.

---

## Path C: Scan a GitHub.com repository

Scan a remote repo (for example `https://github.com/lousy-agents/coach`) without cloning it. Finish Path A first.

Coach accepts the job only if:

1. Your GitHub App installation can read the repository.
2. The signed-in user has a role on that repository (`admin`, `maintain`, `write`, `triage`, or `read`).

Missing repo and unauthorized repo both return `403` `repo_not_authorized`. Coach reads files with the App installation, not with your OAuth login token.

### 1. GitHub OAuth App (sign-in)

GitHub → Settings → Developer settings → **OAuth Apps** → New OAuth App:

| Field | Value |
| --- | --- |
| Application name | e.g. `coach-local` |
| Homepage URL | `http://127.0.0.1:8080` |
| Authorization callback URL | `http://127.0.0.1:8080/oauth/github/callback` |

Leave scopes empty. Save the **Client ID** and create a **Client secret**.

### 2. GitHub App (repo read)

Use the manifest in this repo so permissions match what Coach needs:

| Permission | Why |
| --- | --- |
| Contents (read) | List and read source files |
| Metadata (read) | Basic repo metadata |
| Administration (read) | Check your access level on the repo |

Webhooks are inactive. The App is private to you.

| File | Use |
| --- | --- |
| [`deploy/compose/platform/github-app.manifest.json`](../deploy/compose/platform/github-app.manifest.json) | Permission definition |
| [`deploy/compose/platform/create-github-app.html`](../deploy/compose/platform/create-github-app.html) | Browser button to register the App |
| [`deploy/compose/platform/complete-github-app-manifest.sh`](../deploy/compose/platform/complete-github-app-manifest.sh) | Saves App ID and private key after GitHub redirects |

```sh
cd deploy/compose/platform
python3 -m http.server 8765
```

Open http://127.0.0.1:8765/create-github-app.html

1. Register on your user account, or under an org you admin.
2. Confirm the name on GitHub (change it if taken) and create the App.
3. You return to the local page with `?code=…`. Within one hour, from the **repo root**:

   ```sh
   ./deploy/compose/platform/complete-github-app-manifest.sh <code>
   ```

4. That creates (gitignored):

   - `secrets/github-app.pem`
   - `secrets/github-app.json` (includes App ID)

5. **Install** the App on the owner of the target repo (link is printed by the script). Grant that repo (or all repos, if you accept the wider access).

**Manual fallback:** create a GitHub App in the UI with the same homepage, inactive webhook, Contents / Metadata / Administration read, not public. Download a private key to `secrets/github-app.pem`.

```sh
chmod 600 secrets/github-app.pem
```

Do not commit `secrets/` or `compose.override.yaml`.

### 3. Compose override

Default compose has no GitHub credentials. Add a gitignored `compose.override.yaml` at the repo root (Compose loads it automatically):

```yaml
# compose.override.yaml  (local only; do not commit)
services:
  coach-api:
    environment:
      COACH_GITHUB_OAUTH_CLIENT_ID: "Iv1.xxxxxxxx"
      COACH_GITHUB_OAUTH_CLIENT_SECRET: "xxxxxxxx"
      COACH_GITHUB_OAUTH_REDIRECT_URI: "http://127.0.0.1:8080/oauth/github/callback"
      # App ID from complete-github-app-manifest.sh / secrets/github-app.json
      COACH_GITHUB_APP_ID: "123456"
      COACH_GITHUB_APP_PRIVATE_KEY_PATH: "/secrets/github-app.pem"
      # Keep fixture access so Path A still works
      COACH_AUTHZ_BYPASS_OWNER: "coach-smoke"
      COACH_AUTHZ_BYPASS_REPO: "fixture-repo"
      COACH_AUTH_TEST_MINT: "1"
    volumes:
      - ./secrets/github-app.pem:/secrets/github-app.pem:ro

  coach-worker:
    environment:
      COACH_GITHUB_APP_ID: "123456"
      COACH_GITHUB_APP_PRIVATE_KEY_PATH: "/secrets/github-app.pem"
      # Optional: larger repos (defaults 5000 files / 50 MiB of Go/TS/TSX source)
      # COACH_BASELINE_MAX_FILES: "20000"
      # COACH_BASELINE_MAX_TOTAL_BYTES: "104857600"
    volumes:
      - ./secrets/github-app.pem:/secrets/github-app.pem:ro
```

```sh
docker compose --profile core up -d --build
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/v1/me
```

Combine with Path B by exporting `AIGW_OPENAI_BASE_URL` / `MODEL_GATEWAY_MODEL` and using `--profile llm`.

### 4. Sign in

Open http://127.0.0.1:8080/oauth/github/start

After GitHub consent you get JSON (no fancy UI):

```json
{
  "access_token": "<coach-token>",
  "token_type": "bearer"
}
```

Use that `access_token` as `Authorization: Bearer …` on `/v1`. A raw GitHub token will not work.

```sh
export TOKEN='paste-coach-token-here'
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/v1/me | jq .
```

You should see your GitHub login. Prefer this OAuth flow for real repos. Lab-only alternative: test-mint with your real GitHub `login` string still checks access via the App, but identity is not verified by GitHub login.

### 5. Submit, poll, fetch report

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

`ref` is optional (default branch if omitted). Do not send `git_url` or `clone_url` (rejected with `400`).

Success: status `completed`, a `commit_sha`, and findings you can read. Large repos are slow and may hit size limits; raise `COACH_BASELINE_MAX_*` on the worker or pick a smaller repo.

### Path C problems

| Symptom | Likely cause |
| --- | --- |
| `403` `repo_not_authorized` | No role on the repo; App not installed; App cannot see the repo; or repo does not exist |
| `401` on `/v1/jobs` | Missing/expired Coach token, or you used a GitHub token |
| OAuth callback `400` | Wrong callback URL; expired login attempt; you denied access |
| Job failed / no tree source | Worker missing App ID or private key |
| Job failed auth during fetch | App lacks Contents access or is not installed on that repo |
| Job failed budget / too large | Over 5000 files or 50 MiB of supported source; raise limits or use a smaller repo |
| Slow monorepo job | Normal for remote file-by-file fetch; try a smaller repo first |
| Smoke fails credential check | `GITHUB_TOKEN` / `GH_TOKEN` set in your shell; unset them, or skip smoke while using Path C |

---

## Reading the report

- **`source=deterministic`**: rule-based finding. Same commit and same Coach version should reproduce it.
- **`source=agent`**: model judgment for a named rubric. Extra context, not a replacement for deterministic rows.

Paths A and B only touch the fixture repo. Path C fetches the remote tree over GitHub’s API; Coach still does not write to GitHub.

---

## Limits

- Local lab only (test login, fixed signing key, fixture bypass). Keep it on localhost.
- Advisory reports only; not a CI gate or merge blocker.
- No GitHub writes.
- Go, TypeScript, TSX only.
- No PR-history scan or web UI in this guide.
- You cannot scan arbitrary public OSS without a role on the repo and an App install.

---

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| Smoke: connection refused | `mise run platform-up` first |
| Smoke: credential check failed | Unset `GITHUB_*` / `GH_TOKEN` in that shell |
| No `source=agent` | Model URL/id wrong, gateway cannot reach host, bad JSON, or timeout |
| No `source=deterministic` | Fixture pair mismatch (`coach-smoke` / `fixture-repo`) |
| Host model unreachable | Use `host.docker.internal`, not `localhost`, from inside Docker |
| Postgres errors after git pull | `docker compose --profile core --profile llm down -v` then start again |
| Smoke never sees your GitHub repo | Expected; use Path C |
| Path C `403` on a public repo | Need a role + App install, not just public read |

---

## Tear down

```sh
mise run platform-down
```

Drop database volume too:

```sh
docker compose --profile core --profile llm down -v --remove-orphans
```

---

## FAQ

**Do I need GitHub apps for the first try?**  
No. Paths A and B use the fixture only. Path C needs OAuth App + GitHub App.

**Does Coach comment on PRs?**  
No. You pull the report from the local API.

**Can I scan github.com/lousy-agents/coach without cloning?**  
Yes (Path C). You need a role on that repo and an installed App that can read it.

**Fake model vs Qwen / Gemma?**  
Fake model proves the pipeline. Qwen 3.5 or Gemma 4 produces real agent text. Both stay labeled `source=agent`.

**What languages?**  
Go, TypeScript, TSX. Baseline scan only in this guide.

**How do I know it worked?**  
A/B: `platform-smoke: ok` and both finding sources. C: job `completed`, `commit_sha` set, report from `/v1/jobs/{id}/report`.

---

## Commands

```sh
# Path A
mise run platform-up
mise run platform-smoke
mise run platform-down

# Path B (Qwen 3.5; for Gemma 4 use MODEL_GATEWAY_MODEL=gemma4:12b or your server id)
ollama pull qwen3.5:4b
export AIGW_OPENAI_BASE_URL=http://host.docker.internal:11434/v1
export MODEL_GATEWAY_MODEL=qwen3.5:4b
export MODEL_GATEWAY_TIMEOUT=120s
export MODEL_GATEWAY_DISABLE_THINKING=1
docker compose --profile llm up -d --build   # recreate after env changes
mise run platform-smoke

# Path C (after App registration + compose.override.yaml)
cd deploy/compose/platform && python3 -m http.server 8765
# browser → register App → ./deploy/compose/platform/complete-github-app-manifest.sh <code>
open http://127.0.0.1:8080/oauth/github/start
export TOKEN='…'
curl -s -X POST http://127.0.0.1:8080/v1/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"kind":"repo_baseline_scan","params":{"repo_owner":"lousy-agents","repo_name":"coach","ref":"main"}}'
```
