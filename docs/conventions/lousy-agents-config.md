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

Diagnostic names are `<tool>_config_*`. Coach's names today:

- `project_config_deprecated`
- `project_config_uncommitted`
- `project_config_migration_skipped`
- `project_config_migration_failed`
- `project_config_invalid`

## Migration prompt

The migration prompt appears only for a scan (`coach codesignal --baseline` or `--base`) on an attended terminal. `--check-project`, `--prepare-compiler`, and `--suggest-project-config` do not prompt. Non-interactive use does not prompt. `--no-interactive`, or a non-empty `CI` environment variable, emits `project_config_migration_skipped` with this command:

```sh
mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json
```

No date is set for removing the root fallback. Tracking: [issue #510](https://github.com/lousy-agents/coach/issues/510). Removal: [issue #511](https://github.com/lousy-agents/coach/issues/511).
