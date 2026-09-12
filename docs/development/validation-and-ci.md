# Validation and CI

Reference detail behind AGENTS.md's Validation section. AGENTS.md carries the
rules and the traps; this page carries the shape and the measurements, which an
agent can otherwise derive from `mise.toml` and `.github/workflows/ci.yml`.

## Local task closure

`mise tasks` lists every task with its description. The composites:

| Task | Closure | Runs the sidecar suite? | Runs wasm? |
| --- | --- | --- | --- |
| `ci-gate` | gofmt / go-vet / style | no | no |
| `ci-fast` | Go slice + agent-tooling suites, sidecar built first | yes | no |
| `ci` | `ci-go` + `js-ci` | no — sidecar is not built first | no |
| `ci-all` | sidecar, then `ci-go` + `js-ci` + `wasm-build` | yes | yes |

`ci-all` reaches `js-install` through `project-sidecar-build`, so the sidecar and TypeScript-backend specs do run inside `test` rather than being absent. What `ci-all` lacks is the proof: `test` carries no `-ginkgo.fail-on-empty`, so those specs skip gracefully when their compiler preconditions fail and `ci-all` stays green anyway. The `projectmodel-sidecar` and `ts-project-backend` leaves add that flag, which is what turns a silent skip into a failure. Run `mise run ts-project-backend-acceptance` directly when you touch `cmd/coach` or `internal/codesignalcli`; it needs `strace` on PATH, because the specs assert on the analyzer subtree's file syscalls.

Two gaps `mise run ci` alone does not close, which is why `ci-all` exists:

- `wasm-build` is in no task's closure, so a `GOOS=js GOARCH=wasm` break passes `ci`.
- `ci` runs `test` before anything builds the sidecar, so `pkg/projectmodel`'s
  TypeScript sidecar acceptance suite skips silently unless Node and the binary
  are already present. A green `ci` does not mean that suite ran.

`ci-gate` deliberately runs no tests and no `tidy-check`, because `tidy-check`
rewrites `go.mod`/`go.sum` in place and a smoke check should not mutate the tree.

`ci-all` deliberately excludes `test-acceptance-fast` (its ambient-credential
preflight cannot pass where `GITHUB_TOKEN`/`GH_TOKEN` or `~/.aws/config` are
present, and `test` already runs every acceptance suite unfiltered) and
`platform-smoke` (Docker plus live services), which has no local equivalent at
all. No local composite pins Node 26 either, so `ts-project-backend-node26` is
reproducible locally only by writing that `mise.local.toml` yourself.

## Why the exhaustive gate is not local

A serial local `ci-all` measured ~910s on a CCR container. CI proves a strict
superset — the same atomic tasks as parallel leaf jobs, plus `platform-smoke` —
in ~426s wall clock, on compute that is not the session's.

`cmd/coach` dominates `mise run test`'s wall time: `go test -race ./cmd/coach/...`
measured ~239s, of which roughly 180s is deliberate blocking on
`tsSidecarWallTime` (60s, `internal/codesignalcli/project_ts_backend.go`) and
`snapshotGitTimeout` (30s, `internal/codesignalcli/project_snapshot.go`), each
paid twice across the project-backend and no-findings-verdict acceptance specs.
Read `ci-fast` as "narrower than CI", not as "quick".

## Cross-language acceptance gates

A gate whose acceptance criteria assert on the rendered CLI report across both
the Go and TypeScript project backends runs the mandatory Validation Suite plus
`mise run test-acceptance-fast` directly, rather than only through `mise run test`.
That layer is named and scoped in
[`docs/architecture/acceptance-harness.md`](../architecture/acceptance-harness.md)
and carries its own preflight guard (`go run ./cmd/acceptance-guard-preflight`),
independent of the wider `test` superset.

In a credentialed environment, `test-acceptance-fast`'s ambient-credential
preflight refusal is the documented, expected result, not a gate failure —
`mise run test` already ran the same `*Acceptance` suites unfiltered.

## GitHub Actions shape

GHA is a parallel scheduler of atomic `mise run <task>` steps. The local
`ci` / `ci-fast` / `ci-all` composites are serial bundles of the same tasks; the
workflow does not invoke those composites.

Seven independent leaf jobs plus a `status` aggregator:

- `verify` — `ci-go`: gofmt / go-vet / tidy-check / acceptance-style-check /
  test / test-examples. mise installs only Go; the runner image may still have
  Node, so the sidecar suite usually skips because the sidecar is not built,
  not because `node` is missing. Toolchain comes from `mise.toml`
  (`go = "1.26.6"`), not `go.mod`'s language version.
- `js-verify` — `mise run js-ci` only.
- `projectmodel-sidecar` — `mise run projectmodel-sidecar-acceptance` (builds
  the sidecar, then the real suite). Parallel with `js-verify`.
- `ts-project-backend` — `mise run ts-project-backend-acceptance`, the
  TS-dependent `cmd/coach` and `internal/codesignalcli` specs run against this
  repo's own installed `js/semantics` compiler. Installs `strace` first,
  because the specs assert that the analyzer subtree's file syscalls stay
  inside a frozen allowlist. Allow 40 minutes.
- `ts-project-backend-node26` — the same `mise run ts-project-backend-acceptance` task, with a
  `mise.local.toml` written first to pin Node 26. It asserts the resolved major is 26 before
  running, because a mise regression that declined the override would otherwise re-run the
  Node 24 leg and stay green, leaving the supported {24,26} set half-proven.
- `wasm-build` — `mise run wasm-build`.
- `platform-smoke` — `platform-up` / `platform-smoke` / `platform-down` as three
  steps, so teardown still runs on failure.
- `status` — `if: always()` plus `needs` on every leaf; inspects `toJSON(needs)`
  and fails unless each result is `success`.

`platform-smoke` has no local composite; run the three platform tasks directly.
Adding a leaf means adding it to `status.needs`, or the new leaf can fail while
the required check stays green.
