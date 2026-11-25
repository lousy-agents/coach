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

## Review work before you commit

`--worktree` reviews the changes you have not committed yet: edited, staged and new files, read from disk and compared with your last commit. Run it from the repository root, before each commit or after each turn of a coding agent:

```sh
coach codesignal --worktree
```

Findings in the files you changed are marked `lifecycle: introduced`. Nothing leaves your machine, and nothing is written to your repository. `--worktree` covers uncommitted work only; review committed changes with `--base` in the pull request.

### Run the review when your coding agent finishes

One script, [`scripts/coach-self-review.sh`](./docs/cli-codesignal.md#the-check), decides whether the review is clean, for any agent. It runs `coach codesignal --worktree --no-interactive --fail-on-incomplete-coverage --format json` and calls the review clean when:

- Coach ran and exited `0` (exit `1`, `2` or `127` means the review did not run);
- every in-scope file was read (exit `3` means part of the project or working tree went unread);
- the project policy on disk is the committed one (`scope.policy.matches_head` is not `false`), so an agent cannot loosen `.lousy-agents/coach.json` and review its work against the loosened policy.

Findings stay advisory: the hook does not block on them and does not pass them to the agent. When the review is not clean, the hook keeps the agent working for one more turn, to tell you why. It needs Coach v0.13.0 or later, the first release that reports `scope.policy`; with an older Coach the hook blocks and says so.

Each agent wires the script to its own end-of-turn event:

| Agent | Event | Config file | How "not clean" is reported |
| --- | --- | --- | --- |
| Claude Code | `Stop` | `.claude/settings.json` | exit `2`, reason on stderr (`exit2` mode) |
| OpenAI Codex | `Stop` | `.codex/hooks.json` | exit `2`, reason on stderr (`exit2` mode) |
| GitHub Copilot (CLI and cloud agent) | `agentStop` | `.github/hooks/coach-self-review.json` | `{"decision":"block","reason":...}` on stdout (`json` mode) |

**Claude Code**, in `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sh",
            "args": ["${CLAUDE_PROJECT_DIR}/scripts/coach-self-review.sh", "exit2"]
          }
        ]
      }
    ]
  }
}
```

**OpenAI Codex**, in `.codex/hooks.json` (review and trust the hook with `/hooks` before it runs):

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sh \"$(git rev-parse --show-toplevel)/scripts/coach-self-review.sh\" exit2",
            "timeout": 600
          }
        ]
      }
    ]
  }
}
```

**GitHub Copilot**, in `.github/hooks/coach-self-review.json` (the cloud agent reads it from the default branch, and needs `coach` installed in its environment):

```json
{
  "version": 1,
  "hooks": {
    "agentStop": [
      {
        "type": "command",
        "bash": "sh scripts/coach-self-review.sh json",
        "cwd": ".",
        "timeoutSec": 600
      }
    ]
  }
}
```

The script, why each flag is there, the full exit-code handling, and how to test the hook by hand are in the [agent self-review hook](./docs/cli-codesignal.md#agent-self-review-hook-preview) section of the CLI contract.

## Learn more

- [CLI contract](./docs/cli-codesignal.md): every flag, report ranking and narrowing, exit codes, JSON schema, diff mode (`--base`), and rule IDs.
- [Project policy](./docs/cli-codesignal.md#project-policy): declare layers and forbidden imports, and what Coach reports when a policy is missing or invalid.
- [TypeScript setup](./docs/cli-codesignal.md#typescript-project-readiness): readiness checks, `--prepare-compiler`, and unattended runs.
- [Product requirements](./docs/product/prd.md).
- [Local quickstart](./docs/pilot-local-quickstart.md): the end-user steps for the local CLI.
- [Local platform lab](./docs/development/local-platform.md): an optional Docker stack that adds agent judgments. It is not the default install.
- Libraries: [`pkg/semantics`](./docs/cli-codesignal.md#pkgsemantics-quickstart) and `@lousy-agents/coach-semantics`. You do not need them to run `coach`.
