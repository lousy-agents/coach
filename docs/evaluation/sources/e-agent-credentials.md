# Source ledger (e): running the agent on a subscription login

This is track (e) of the Coach evaluation design: how a headless Claude Code agent authenticates with the owner's subscription, and how it is isolated, when no API key is used.

- Every row was read from the official page named in it, on 26 Nov 2026.
- Quotes are verbatim and short.
- Where only a model-written summary of a page was available, the row says so and quotes nothing from it.
- Design documents cite these rows as `e.N`.

## e.1 Claude Code docs: authentication

- URL: https://code.claude.com/docs/en/authentication
- Key findings:
  1. A subscription token for scripts: "generate a one-year OAuth token with `claude setup-token`". "It can only make model requests, so it can't establish Remote Control sessions or fetch claude.ai connectors."
  2. Where credentials are stored:
     - "On macOS, credentials are stored in the encrypted macOS Keychain."
     - "On Linux, credentials are stored in `~/.claude/.credentials.json` with file mode `0600`."
  3. Which credential wins, in `claude -p`: "In non-interactive mode (`-p`), the key is always used when present". `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY` and `apiKeyHelper` all rank above `CLAUDE_CODE_OAUTH_TOKEN`, which ranks above `/login` credentials.
  4. Separate configuration directories: under `CLAUDE_CONFIG_DIR`, "Each directory has its own settings, session history, and claude.ai login or API key."
  5. Revoking a `setup-token` token: not stated on this page.
- Design implication for Coach:
  - The harness must unset every API-key variable in the agent's environment, or `-p` silently bills an API key instead of the subscription.
  - A `setup-token` token is a one-year subscription credential. Nothing on this page documents a way to revoke it.

## e.2 Claude Code docs: CLI reference and environment variables

- URLs: https://code.claude.com/docs/en/cli-reference , https://code.claude.com/docs/en/env-vars
- Key findings:
  1. `claude setup-token`: "Generate a long-lived OAuth token for CI and scripts. Prints the token to the terminal without saving it. Requires a Claude subscription."
  2. `CLAUDE_CODE_OAUTH_TOKEN`: "OAuth access token for claude.ai authentication. Alternative to `/login` for SDK and automated environments." It "Takes precedence over keychain-stored credentials."
  3. `CLAUDE_CONFIG_DIR`: "Override the configuration directory (default: `~/.claude`). All settings, session history, and plugins are stored under this path."
  4. Quieting the CLI:
     - `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` disables "nonessential network traffic: auto-updates, telemetry, error reporting".
     - `DISABLE_AUTOUPDATER`: "Set to `1` to disable automatic background updates."
- Design implication for Coach:
  - Anthropic documents a subscription token "for CI and scripts".
  - The agent environment sets `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and a fresh `CLAUDE_CONFIG_DIR`.

## e.3 Claude Code docs: headless mode

- URL: https://code.claude.com/docs/en/headless
- Key findings:
  1. "In bare mode, Claude Code never reads OAuth credentials or the system keychain."
  2. "Set `ANTHROPIC_API_KEY` before running it, because bare mode doesn't use your subscription login".
- Design implication for Coach: `--bare` cannot be used on a subscription login. The page says nothing more about plain `claude -p` with a subscription.

## e.4 Claude Code docs: development containers

- URL: https://code.claude.com/docs/en/devcontainer
- Key findings:
  1. Commands "execute inside it rather than on the host machine".
  2. Egress is restricted by an optional reference script: "an `init-firewall.sh` script that limits outbound traffic to the destinations the script allows". It needs "`NET_ADMIN` and `NET_RAW` capabilities".
  3. Credentials persist only if mounted: "mount a named volume at `~/.claude` and set `CLAUDE_CONFIG_DIR` to the same path".
  4. The warning: dev containers "do not prevent a malicious project from exfiltrating anything accessible inside the container, including the Claude Code credentials stored in `~/.claude`."
- Design implication for Coach: in a plain container, the agent's tools can read any credential the CLI in that container can read. An egress allowlist narrows where the credential can be sent, but does not remove it from the container.

## e.5 Claude Code docs: native Bash sandbox

- URL: https://code.claude.com/docs/en/sandboxing (about 40k of 112k characters read verbatim; the "Security limitations" section only as a summary, not quoted)
- Key findings:
  1. Scope: "The sandbox covers shell commands only." The file tools "follow permission rules instead".
  2. Reads: by default the sandbox can read "Most of the machine, including credential files such as `~/.ssh` and `~/.aws/credentials`".
  3. "The sandbox is off by default."
- Design implication for Coach: running the host `claude` under the native sandbox leaves the oracle and the credential on the same machine as the agent, separated only by rules. This design rejects that option.

## e.6 Docker docs: Docker Sandboxes

- URLs: https://docs.docker.com/ai/sandboxes/security , https://docs.docker.com/ai/sandboxes/security/defaults , https://docs.docker.com/ai/sandboxes/architecture/ , https://docs.docker.com/ai/sandboxes/agents/claude-code , https://docs.docker.com/ai/sandboxes/configuration/credentials/ , https://docs.docker.com/ai/sandboxes/get-started/ , https://docs.docker.com/ai/sandboxes/install/
- Two linked usage pages returned HTTP 404, so nothing about mounting extra directories was verified.
- Key findings:
  1. **Isolation:** agents run "in microVMs". The "Hypervisor isolation: separate kernel per sandbox." And "Outside the workspace and shared skills store, the agent cannot access your host filesystem."
  2. **Subscription login with the credential kept on the host:**
     - "If no API key is set, use the `/login` command inside Claude Code to authenticate via OAuth."
     - "The real credential stays on the host when proxy management is active; the sandbox sees only a sentinel value."
     - "The session token stays on your host and is never stored inside the sandbox."
  3. **Host configuration is not inherited:** "Sandboxes don't pick up user-level configuration from your host, such as `~/.claude`."
  4. **Network:**
     - "All outbound TCP traffic, including HTTP, HTTPS, and SSH, is blocked unless an explicit rule allows the destination."
     - Rules are added with "`sbx policy allow`".
     - A caveat: "The default allowed domains include broad wildcards."
  5. **Workspace:** "With `--clone`, your repository is mounted read-only and the agent works on a private clone."
  6. **Platforms:**
     - macOS "Sonoma version 14 or later" on "Apple silicon";
     - "Windows 11";
     - Linux "Ubuntu 24.04 or later" with KVM;
     - "You don't need Docker Desktop or Docker Engine to use `sbx`."
  7. **Default startup:** the default command is "claude --dangerously-skip-permissions".
- Not stated in the pages read:
  - how to run `claude -p` with chosen flags instead of the default command;
  - how to use a custom image that carries `coach` and `jq`;
  - whether one `/login` serves many sandboxes;
  - how to remove the default wildcard allowances.
- Design implication for Coach:
  - Docker Sandboxes is the only documented option in which the subscription credential never enters the agent's boundary.
  - The four unstated mechanisms must be proven in the pilot before the design can depend on them.

## e.7 Anthropic Consumer Terms and Claude Code plan support

- URLs: https://www.anthropic.com/legal/consumer-terms (effective 8 Oct 2025), https://support.claude.com/en/articles/11145838-using-claude-code-with-your-pro-or-max-plan
- Key findings:
  1. Consumer Terms §3, item 7, prohibits: "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it, to access the Services through automated or non-human means, whether through a bot, script, or otherwise."
  2. Usage limits are shared: "These limits are shared across Claude and Claude Code, meaning all activity in both tools counts against the same limits." Also: "Pro and Max plans have a five-hour session limit and a weekly limit."
  3. The support article does not mention scripted, headless or evaluation use.
- Design implication for Coach:
  - The owner must decide whether e.2's documented "for CI and scripts" token counts as "where we otherwise explicitly permit it" for unattended evaluation runs. This design does not assume an answer.
  - Evaluation runs draw on the same five-hour and weekly limits as the owner's own Claude use.
