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

Findings in the files you changed are marked `lifecycle: introduced`. Coach sends nothing anywhere and writes nothing to your repository. `--worktree` covers uncommitted work only; review committed changes in the pull request with `--base`.

### Run the review when your coding agent finishes

Copy [`scripts/coach-self-review.sh`](./docs/cli-codesignal.md#the-check) from the CLI contract into your repository. It runs `coach codesignal --worktree --no-interactive --fail-on-incomplete-coverage --format json` and calls the review clean when:

- Coach ran and exited `0` (exit `1`, `2` or `127` means the review did not run);
- every in-scope file was read (exit `3` means part of the project or working tree went unread);
- the project policy on disk is the committed one (`scope.policy.matches_head` is not `false`), so an agent cannot loosen `.lousy-agents/coach.json` and have its work reviewed against the loosened policy while the edit is uncommitted. With no policy, this check always passes.

A policy or code change the agent commits is not flagged; review committed changes in the pull request with `--base`. Findings stay advisory: the hook does not block on them and does not pass them to the agent. When the review is not clean, the hook asks the agent for at most one extra turn, to tell you why. It needs Coach v0.13.0 or later, the first release that reports `scope.policy`, on the `PATH` of the shell the agent runs its hooks in, which may not read `~/.bashrc`. `sh` and `bash` read no startup file when they run a hook command, so with mise, start the hook command with `export PATH=\"$HOME/.local/share/mise/shims:$PATH\";` (written with its quotes escaped, as it must be inside the JSON `command` string), or replace `coach` in the script with its absolute path.

The script takes one argument, how the agent expects "not clean" to be reported: `exit2` for Claude Code and Codex, `json` for Copilot. Each snippet below checks that the script is there and, if it is not, exits `1`, which Claude Code and Copilot document as non-blocking; the Codex documentation names only exit `0` and `2` for `Stop`. The Claude Code wiring has been run in Claude Code; the Codex and Copilot wiring is written from their documentation and has not been run in those agents.

| Agent | Event | Config file | How "not clean" is reported |
| --- | --- | --- | --- |
| Claude Code | `Stop` | `.claude/settings.json` | exit `2`, reason on stderr (`exit2`) |
| OpenAI Codex | `Stop` | `.codex/hooks.json` | exit `2`, reason on stderr (`exit2`) |
| GitHub Copilot (CLI and cloud agent) | `agentStop` | `.github/hooks/coach-self-review.json` | `{"decision":"block","reason":...}` on stdout (`json`) |

**Claude Code**, in `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "f=\"${CLAUDE_PROJECT_DIR}/scripts/coach-self-review.sh\"; if [ -f \"$f\" ]; then sh \"$f\" exit2; else echo \"coach self-review: $f not found\" >&2; exit 1; fi"
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
            "command": "f=\"$(git rev-parse --show-toplevel 2>/dev/null)/scripts/coach-self-review.sh\"; if [ -f \"$f\" ]; then sh \"$f\" exit2; else echo \"coach self-review: $f not found\" >&2; exit 1; fi",
            "timeout": 600
          }
        ]
      }
    ]
  }
}
```

**GitHub Copilot**, in `.github/hooks/coach-self-review.json`. The cloud agent runs on GitHub's infrastructure: it reads this file from the default branch, needs `coach` installed in its environment (for example in its setup steps), and counts the extra turn against the job's timeout. The cloud agent [pushes its changes to its pull request](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/cloud-agent/use-cloud-agent-on-github), so this hook may find little uncommitted work there; review its pull request with `coach codesignal --base <default branch>`. Copilot hooks in Visual Studio Code are not covered.

```json
{
  "version": 1,
  "hooks": {
    "agentStop": [
      {
        "type": "command",
        "bash": "f=scripts/coach-self-review.sh; if [ -f \"$f\" ]; then sh \"$f\" json; else echo \"coach self-review: $f not found\" >&2; exit 1; fi",
        "cwd": ".",
        "timeoutSec": 600
      }
    ]
  }
}
```

The script, why each flag is there, the full exit-code handling, and how to test the hook by hand are in the [agent self-review hook](./docs/cli-codesignal.md#agent-self-review-hook) section of the CLI contract.

## Learn more

- [CLI contract](./docs/cli-codesignal.md): every flag, report ranking and narrowing, exit codes, JSON schema, diff mode (`--base`), and rule IDs.
- [Project policy](./docs/cli-codesignal.md#project-policy): declare layers and forbidden imports, and what Coach reports when a policy is missing or invalid.
- [TypeScript setup](./docs/cli-codesignal.md#typescript-project-readiness): readiness checks, `--prepare-compiler`, and unattended runs.
- [Product requirements](./docs/product/prd.md).
- [Local quickstart](./docs/pilot-local-quickstart.md): the end-user steps for the local CLI.
- [Local platform lab](./docs/development/local-platform.md): an optional Docker stack that adds agent judgments. It is not the default install.
- Libraries: [`pkg/semantics`](./docs/cli-codesignal.md#pkgsemantics-quickstart) and `@lousy-agents/coach-semantics`. You do not need them to run `coach`.
