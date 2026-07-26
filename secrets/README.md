# Local secrets (gitignored)

Compose mounts this directory read-only at `/secrets` in `coach-api` and
`coach-worker`.

| Host path | Container path | Set by |
| --- | --- | --- |
| `secrets/github-app.pem` | `/secrets/github-app.pem` | `complete-github-app-manifest.sh` or manual download |
| `secrets/github-app.json` | `/secrets/github-app.json` | manifest script (metadata only; not required at runtime) |

Path C `.env` should set:

```dotenv
COACH_GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem
COACH_GITHUB_APP_ID=<id from github-app.json>
```

Do not commit PEM files or OAuth secrets. Use repo-root `.env` (from `.env.example`)
for App ID and OAuth client values.
