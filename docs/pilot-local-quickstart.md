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
| Success | `platform-smoke: ok` and a report with both `source=deterministic` and `source=agent` findings |

Languages analyzed today: Go, TypeScript, TSX. The job kind in this guide is `repo_baseline_scan` against an in-compose fixture (`coach-smoke/fixture-repo`). On this path there are no GitHub credentials, no GitHub writes, and no cloud Coach backend. Traffic stays on your machine.

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

Live "scan a real GitHub repo" needs GitHub OAuth (who you are) and a GitHub App installation (server-side repo read). That is an operator path, not this first-run guide. See [local-platform.md](./development/local-platform.md) and the baseline platform specs under `.github/specs/`.

---

## Limits (honest)

- Pilot and lab only. Compose enables test mint, a fixed local JWT signing key, and an authz bypass for the fixture pair. Do not expose this stack on a network or treat it as multi-tenant production.
- Not a linter, CI gate, or merge blocker. Advisory reports only.
- Not a public PR review bot. No GitHub writes in this era.
- Go, TypeScript, and TSX only. Other languages are skipped.
- `pr_history_scan`, web UI, harness hooks, SGLang, and AWS are out of scope here. Local Compose validation is the investment gate for cloud serving.
- Ollama and MLX are supported only as OpenAI-compatible upstreams behind the AI gateway, the same wire contract as llama.cpp. We do not vendor those runtimes.
- Compose is not a hostile-code sandbox. Do not point this stack at untrusted repositories expecting isolation.

We are still sharpening this path. Live GitHub onboarding and cloud serving remain work in progress.

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
| Expecting your real GitHub repo in smoke | Smoke always scans the fixture; live GitHub is a separate operator setup |
| Believing stub text is "the model" | Path A always hits `model-stub` unless you switched aigw upstream (Path B) |

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
No for Paths A and B. Test mint and the `coach-smoke/fixture-repo` fixture only. Real-repo scans need GitHub identity and App install later.

**Does Coach post comments, checks, or write anything to GitHub?**  
No. Submit, poll, and fetch the report. Nothing is written back to GitHub in this era.

**Stub vs `qwen3.5:4b` / `qwen3.5:4b-mlx`: what is the difference?**  
Stub returns canned, schema-valid agent judgments so the pipeline is testable without weights. Your local Qwen produces real rubric text. Both are still labeled `source=agent` and never mixed into deterministic evidence.

**Where does my code go on the smoke path?**  
It does not. Analysis runs against a small in-compose fixture. Traffic stays on your machine.

**What languages and job types work today?**  
Baseline scan over Go, TypeScript, and TSX. PR-history scan and other languages are not part of this quickstart.

**How do I know it actually worked?**  
`platform-smoke: ok` and a completed report with both `source=deterministic` and `source=agent` findings. Containers up without that report is not success.

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
```

---

## Related docs

| Doc | Audience |
| --- | --- |
| [local-platform.md](./development/local-platform.md) | Operators: full env matrix, aigw notes, CI smoke |
| [prd.md](./product/prd.md) | Product direction for the groundwork era |
| [system-overview.md](./architecture/system-overview.md) | Long-term architecture (webhook platform, SGLang/AWS) |
| Epic [#97](https://github.com/lousy-agents/coach/issues/97) | Baseline Scan platform epic |
