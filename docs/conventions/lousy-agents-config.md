# `.lousy-agents` config

Each tool keeps one JSON file under `.lousy-agents/` at the git top-level.

| Tool | File | When |
| --- | --- | --- |
| coach | `.lousy-agents/coach.json` | now |
| doctor | `.lousy-agents/doctor.json` | later |
| lint | `.lousy-agents/lint.json` | later |

## Lookup

The same order applies to every tool. The selected file is read at the selected revision.

1. An explicit flag names one path. No other file is consulted.
2. Otherwise `.lousy-agents/<tool>.json` at the git top-level.
3. Otherwise a deprecated root fallback, and only when that file passes the tool's schema. The report always includes a `<tool>_config_deprecated` diagnostic. Coach's root fallback is `project.json`.

An invalid canonical file is an error. Coach does not fall back to `project.json` in that case.

On an interactive terminal, a TypeScript scan that finds an invalid canonical file offers guided policy authoring instead of only reporting the error. The candidate goes to standard output, no file is written, and the scan exits `2` with no report. If standard output is redirected to a file while a terminal is still attached, for example `coach codesignal ... > candidate.json`, the candidate lands in that file; Coach itself still writes none. The same offer follows the report when the canonical file is valid but declares no layers and the scan exits `0`; then the candidate goes to standard error and the scan keeps its own exit status. Off a terminal, with `--no-interactive`, or when `CI` is set, there is no offer, and an invalid canonical file exits `2` with empty standard output.

Diagnostic names are `<tool>_config_*`. Coach's names today:

- `project_config_deprecated`
- `project_config_uncommitted`
- `project_config_migration_skipped`
- `project_config_migration_failed`
- `project_config_invalid`

## Migration prompt

The migration prompt appears only for a scan (`coach codesignal --baseline` or `--base`) on an attended terminal. `--check-project`, `--prepare-compiler`, and `--suggest-project-config` do not prompt. A scan whose stdin is not a terminal emits `project_config_migration_skipped`. `--no-interactive`, or a non-empty `CI` environment variable, emits the same diagnostic, with this command:

```sh
mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json
```

No date is set for removing the root fallback. Tracking: [issue #510](https://github.com/lousy-agents/coach/issues/510). Removal: [issue #511](https://github.com/lousy-agents/coach/issues/511).
