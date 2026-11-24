# coach

Coach gives you, or your coding agent, a private and repeatable report on structural risk in Go and TypeScript code before a person has to read the diff.

## Why use it

- **Local and private.** The default command calls no model, contacts no GitHub, writes nothing to your repository, and sends no telemetry. The same input gives the same report.
- **Every finding can be acted on.** Each finding gives its rule, severity, file and line, the evidence, why it matters, and a suggested fix. Findings keep their identity when lines move or severity changes, so you can tell what is new.
- **It reports where code breaks the layers you define.** Commit a layer policy, and coach reports the file and line where a forbidden import breaks it.

## Who it's for

Go and TypeScript developers, and the coding agents working for them, who review changes locally. Coach is experimental and stays on the 0.x line.

**Not for yet.** Merge gates, CI checks, security assurance, or a hosted service. Findings are advisory. A completed scan exits `0` even when it finds signals, and an empty report means no rule matched, not that the code is clean.

## Install

Install [mise](https://mise.jdx.dev/) if you do not have it (`curl https://mise.run | sh` puts it in `~/.local/bin`). Then:

```sh
mise use -g github:lousy-agents/coach
echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc && eval "$(~/.local/bin/mise activate bash)"
```

The first line changes your global mise config. Coach needs `git` 2.43 or later on your `PATH`. Other shells, pinned versions, release archives, building from source, and signed archives are in the [CLI contract](./docs/cli-codesignal.md#install).

## First run

From the root of a Git repository:

```sh
coach codesignal --baseline --top 10
```

This scans the Go, TypeScript, and TSX files that ship at `HEAD` and prints the ten highest-ranked findings. The default `--scope production` leaves out test-only and excluded files. `--scope all` analyzes every tracked Go, TypeScript, and TSX file. Each finding shows its rule, severity, path, line, evidence, why it matters, and a recommendation. The header counts the files analyzed and the signals found, and a line says how many signals `--top` withheld.

If no rule matches, the report says `No active CodeSignal findings.` That means no rule matched the committed code, not that the code is clean. [What you should see](./docs/cli-codesignal.md#what-you-should-see) describes the full output.

## Agent self-review (preview)

`--worktree` reviews uncommitted edits against `HEAD`, so a coding agent can check its own changes before commit. Run it from the repository root:

```sh
coach codesignal --worktree --no-interactive --fail-on-incomplete-coverage --format json
```

The hook needs a coach build that reports `scope.policy`. v0.12.0 does not, so there the hook blocks and says so. `--worktree` is a documented flag of the CLI contract. The PRD change that brought this job into scope is [#730](https://github.com/lousy-agents/coach/pull/730). Coach ships the CLI behaviour. The hook below is an example to copy. Coach does not run a hosted hook service. The hook reports only whether the review ran and was clean. It does not pass findings to the agent.

To run the review when Claude Code finishes a response, add this to `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash",
            "args": ["${CLAUDE_PROJECT_DIR}/.claude/hooks/coach-self-review.sh"]
          }
        ]
      }
    ]
  }
}
```

and save this as `.claude/hooks/coach-self-review.sh`:

```sh
#!/bin/sh
# "Clean" means the review ran, no in-scope file went unread, and the project policy on disk is the committed one.
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 2
block=2
case "$(cat | tr -d ' \n')" in *'"stop_hook_active":true'*) block=0 ;; esac
report=$(coach codesignal --worktree --no-interactive --fail-on-incomplete-coverage --format json); rc=$?
flat=$(printf '%s' "$report" | tr -d ' \n')
problem=""
case $rc in
  0) case $flat in
       *'"policy":null'*|*'"matches_head":true'*) ;;
       *'"matches_head":false'*) problem="the project policy on disk differs from HEAD" ;;
       *) problem="coach did not report scope.policy, so the policy check cannot run" ;;
     esac ;;
  1|2|127) problem="the review did not run (exit $rc)" ;;
  *) problem="the review is incomplete: a project or working-tree input was not analyzed (exit $rc)" ;;
esac
[ -z "$problem" ] && exit 0
echo "coach self-review is not clean: $problem. Tell the user." >&2
exit $block
```

An uncommitted policy edit counts as not clean. The hook covers uncommitted work only: a policy or code change the agent has already committed is not flagged, so review committed changes in the pull request. The reasoning and the exit-code handling are in the [agent self-review hook](./docs/cli-codesignal.md#agent-self-review-hook-preview) section.

## Learn more

- [CLI contract](./docs/cli-codesignal.md): every flag, report ranking and narrowing, exit codes, JSON schema, diff mode (`--base`), and rule IDs.
- [Project policy](./docs/cli-codesignal.md#project-policy): declare layers and forbidden imports, and what Coach reports when a policy is missing or invalid.
- [TypeScript setup](./docs/cli-codesignal.md#typescript-project-readiness): readiness checks, `--prepare-compiler`, and unattended runs.
- [Product requirements](./docs/product/prd.md).
- [Local quickstart](./docs/pilot-local-quickstart.md): the end-user steps for the local CLI.
- [Local platform lab](./docs/development/local-platform.md): an optional Docker stack that adds agent judgments. It is not the default install.
- Libraries: [`pkg/semantics`](./docs/cli-codesignal.md#pkgsemantics-quickstart) and `@lousy-agents/coach-semantics`. You do not need them to run `coach`.
