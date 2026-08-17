# GitHub Surface Reference

> Load this during the GitHub surface probe (before Phase 1). Reload in Phase 7 only if
> the recorded binding has fallen out of context. SKILL.md names only abstract operations.
> This file is the only place that binds them to concrete tools. Example MCP names below
> are examples, not required calls — bind whichever tools the harness actually exposes
> after reading their schemas.

## Resolve the target

Do this before any GitHub call.

1. **Full PR URL** → parse `owner`, `repo`, and `number` from the URL. The URL wins over ambient context.
2. **`#N` or bare `N`** → take `number` from the argument. Resolve `owner`/`repo` from a single unambiguous `origin` remote (`git remote get-url origin`).
3. **Ambiguous or absent repository** — more than one candidate remote, no remote, a fork whose upstream also matches, or no PR number — **stop and ask**. Do not guess.

## Detect a CCR-proxy session

Run this check before considering `gh`. It prints a label or nothing — it does not print token values.

```bash
{ [ "$CLAUDE_CODE_REMOTE" = true ] || [ "$GH_TOKEN" = proxy-injected ] || [ "$GITHUB_TOKEN" = proxy-injected ]; } && echo ccr-proxy || true
```

Output means CCR-proxy. Treat the session as CCR-proxy. `proxy-injected` is not a PAT. Never `curl` GitHub with it, never pass it to a script as a token, and never `apt install gh` as this skill's fix.

If you did not run this check, do not bind `gh`. Continue to harness tools.

## Probe (once)

Probe in this order. Record the outcome of every step. Stop binding once one read path and one write path are chosen.

1. Run the CCR check above. If it prints anything, `gh` is **not** a candidate — skip step 2 and go to step 3 even if `gh` is installed.
2. If **not** CCR-proxy: `command -v gh`, `command -v jq`, and `gh auth status`. All three must succeed for `gh` to be a candidate. If `gh` is a candidate, skip steps 3-4 and bind `gh` directly at step 5 — do not enumerate harness tools when `gh` will win anyway. Missing `gh`, missing `jq`, or failed auth means `gh` is absent — continue to step 3. Do not abort.
3. List the harness's GitHub tools (official GitHub MCP, Claude built-in GitHub tools, or any other connector). Read each schema. Do not infer parameters from a name.
4. Score each harness surface by how many of these ops it can actually perform: `read_pr`, `list_review_comments`, `list_reviews`, `reply_in_thread`, `resolve_thread`.
5. Bind **exactly one** read path and **exactly one** write path:
   - Prefer `gh` when it is a candidate (local least toil).
   - Otherwise prefer the higher-scoring harness surface. Ties go to official GitHub MCP over built-in tools (MCP exposes thread resolve; built-ins often do not).
6. Record: surface name, ops bound, ops degraded. Do not re-probe. Do not mix surfaces mid-run.

**Zero read surfaces:** abort. Report every probe attempted. Mutate nothing on GitHub.

## Mid-run failure

Do not switch surfaces if a bound call fails.

- `read_pr`, `list_review_comments`, or `list_reviews` fails → abort Discovery. Nothing was implemented or resolved.
- `reply_in_thread` or `resolve_thread` fails (missing tool, 403, GraphQL pin) → degrade that op, disclose it, continue the rest of Phase 7.

## Pagination

Exhaust every page. `gh` uses `--paginate`. MCP `get_review_comments` is cursor-based — keep passing `after` until there is no `endCursor` / `hasNextPage` is false. Other MCP list methods use `page` / `perPage` — increment until a short page.

## Binding table

For the `gh` path, see [`gh` binding](#gh-binding) below — every operation maps to one of its snippets.

| Operation | Official MCP (examples) | Built-in / other |
| --- | --- | --- |
| `read_pr` | `pull_request_read` `method=get` | PR-read tool from schema |
| `list_review_comments` | `pull_request_read` `method=get_review_comments` (threads include `isResolved` and comments) | review-comment list tool |
| `list_reviews` | `pull_request_read` `method=get_reviews` | reviews list tool |
| `reply_in_thread` | `add_reply_to_pull_request_comment` with **numeric** `commentId` — see MCP ids below | comment/reply tool |
| `resolve_thread` | `pull_request_review_write` `method=resolve_thread` + `threadId` (`PRRT_…`) — see MCP ids below | resolve tool if the schema has one; else degrade |

`jq` is required only on the `gh` path. Probe it before binding `gh`. Do not use `gh api --jq` together with `--paginate`: `gh` applies `--jq` per page, which breaks a whole-list `sort_by`. Pipe the paginated array to `jq` instead.

### MCP comment and thread ids

`pull_request_read` `method=get_review_comments` returns GraphQL threads. Flatten every thread's `comments` before triaging.

- **Reply** needs the REST numeric id. Take it from the comment `url` (`#discussion_r` followed by digits). Do not pass GraphQL `id` (`PRRC_…` / `PRRT_…`) to `add_reply_to_pull_request_comment`. If a comment has no `#discussion_r` id, skip that reply and disclose it.
- **Resolve** needs the thread GraphQL id (`PRRT_…` on the thread, not on a comment). Use that only for `resolve_thread`.

### `gh` binding

```bash
gh pr view {number} --json title,body,headRefName,baseRefName,state,url

gh api repos/{owner}/{repo}/pulls/{number}/comments --paginate \
  | jq '[.[] | {id, user: .user.login, path, line, body, created_at, in_reply_to_id}]
        | sort_by(.created_at) | reverse'

gh api repos/{owner}/{repo}/pulls/{number}/reviews --paginate \
  | jq '[.[] | {id, user: .user.login, state, body, submitted_at}]
        | sort_by(.submitted_at) | reverse'

gh api repos/{owner}/{repo}/pulls/{number}/comments/{comment_id}/replies \
  -f body="Fixed in {sha}. {Brief description of what changed and why.}"

gh api graphql -f query='{
  repository(owner: "{owner}", name: "{repo}") {
    pullRequest(number: {number}) {
      reviewThreads(last: 50) {
        nodes {
          id
          isResolved
          comments(first: 1) { nodes { databaseId path } }
        }
      }
    }
  }
}'

gh api graphql -f query='mutation {
  resolveReviewThread(input: {threadId: "{thread_node_id}"}) {
    thread { isResolved }
  }
}'
```

If GraphQL returns `This GraphQL query is not enabled for this session` (or any 403 naming a REST fallback), degrade `resolve_thread`. Do not retry the same query. Do not invent a REST resolve — GitHub has none. Reply in-thread if `reply_in_thread` is bound, leave the thread open, disclose the degradation.
