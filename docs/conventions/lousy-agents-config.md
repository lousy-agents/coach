# `.lousy-agents` config

Each tool keeps one JSON file under `.lousy-agents/` at the git top-level.

| Tool | File | When |
| --- | --- | --- |
| coach | `.lousy-agents/coach.json` | now |
| coach (TypeScript, `js/semantics`) | `.lousy-agents/coach.typescript-js-semantics.json` | now, with an explicit `--project-config` only |
| doctor | `.lousy-agents/doctor.json` | later |
| lint | `.lousy-agents/lint.json` | later |

## Lookup

The same order applies to every tool. The selected file is read at the selected revision. For an explicit flag, Coach first checks the path on disk, and reads the content at the revision ([ADR-008](../architecture/ADR-008-explicit-policy-disk-and-revision.md)).

1. An explicit flag names one path. No other file is consulted. A path that does not exist on disk exits `2` with `project_config_missing`, and neither falls back to the committed copy. A committed policy that is gone gets the one `git restore` command that brings it back; when a file stands where a parent directory should be, the message states that cause and offers no restore command and no authoring. A symbolic link, a directory or other file that is not a regular file, a path through a symbolic link, a file that cannot be read, or a parent that cannot be searched, exits `2` with `project_config_invalid`.
2. Otherwise `.lousy-agents/<tool>.json` at the git top-level.
3. Otherwise a deprecated root fallback, and only when that file passes the tool's schema. The report always includes a `<tool>_config_deprecated` diagnostic. Coach's root fallback is `project.json`.

An invalid canonical file is an error. Coach does not fall back to `project.json` in that case. A canonical path that the analyzed revision commits as a symbolic link, a directory or a submodule is an error as well, and it is reported the same way in a scan (exit `2`, naming what is committed) and in `--check-project` (`policy_invalid` with a `cause`); a new file would not replace what is committed there, so no authoring is offered.

A TypeScript scan that finds an invalid canonical document opens no guided policy authoring, with or without a terminal, because the only session it has ends by asking the user to save a candidate over the file. It exits `2` with no report and empty standard output, and says in words to fix the policy, or move it aside and commit the move, then run guided policy authoring to write a new one. On an interactive terminal, a TypeScript scan offers guided policy authoring after the report when the canonical file is valid but declares no layers and the scan exits `0`; then the candidate goes to standard error, no file is written, and the scan keeps its own exit status. Off a terminal, with `--no-interactive`, or when `CI` is set, there is no offer.

Diagnostic names are `<tool>_config_*`. Coach's names today:

- `project_config_deprecated`
- `project_config_uncommitted`
- `project_config_unusable`
- `project_config_migration_skipped`
- `project_config_migration_failed`
- `project_config_invalid`
- `project_config_missing`

## Migration prompt

The migration prompt appears only for a scan (`coach codesignal --baseline` or `--base`) on an attended terminal. `--check-project`, `--prepare-compiler`, and `--suggest-project-config` do not prompt. A scan whose stdin is not a terminal emits `project_config_migration_skipped`. `--no-interactive`, or a non-empty `CI` environment variable, emits the same diagnostic, with this command:

```sh
top="$(git rev-parse --show-toplevel)" && mkdir -p "$top"/.lousy-agents && git -C "$top" mv -- project.json .lousy-agents/coach.json
```

No date is set for removing the root fallback. Tracking: [issue #510](https://github.com/lousy-agents/coach/issues/510). Removal: [issue #511](https://github.com/lousy-agents/coach/issues/511).
