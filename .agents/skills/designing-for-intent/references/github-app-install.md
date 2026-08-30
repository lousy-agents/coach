# Illustration: the GitHub App install consent moment

A worked example of the designing-for-intent procedure, grounded in Coach's
actual onboarding flow. Load this file only when a concrete example would
help, or when the artifact under review is this install/consent flow. Do not
treat it as the artifact under review unless the user asked to review it.

Before using the example, `Read` `docs/product/prd.md` and
`docs/architecture/system-overview.md` if those files are available in the
current workspace.

Before a repository can be scanned, "the Coach GitHub App must also be
installed for that repository — part of pilot onboarding."

- **Outcome**: the engineer wants a trustworthy signal report on their own
  work, fast, without granting more access than that requires.
- **Constraints**: temporal — they are usually doing this right before or
  right after opening a PR, not as a separate errand; capacity — they will
  not read a permissions essay, they will skim it in seconds.
- **Delegation boundary**: they are willing to hand off "read this
  repository's contents for analysis"; they are not implicitly agreeing to
  "write to this repository" or "scan every repository I can see." The
  architecture's own posture agrees: repository-content mutation is
  "prohibited in v1; explicit developer activation in Next"
  (`docs/architecture/system-overview.md`).
- **Implicit signal / privacy cost**: installing the App on one repository
  could be read as "trust Coach with all my repositories." That inference is
  wrong and costly if surfaced (it would make an engineer distrust the tool
  for over-reaching); the install screen should scope its language to the
  single repository being onboarded, not to the person's account-wide trust.
- **Confidence-to-response policy**: even at high confidence that the
  engineer wants read access to more of their repositories (e.g. because
  their `coach codesignal` runs keep hitting a second repo they don't have
  installed), the correct response is to propose "install Coach on this
  repository too," never to broaden scope silently.
- **Agency risks found**: A1 (Major) coherence break — the architecture's
  "system-owned, non-blocking advisory feedback" posture
  (`docs/architecture/system-overview.md`) is stated in docs but never
  restated at the point where the first report appears, so a first-time user
  could predictably misread an advisory finding as a hard gate; A2 (Minor)
  hidden reasoning — the install screen does not say why Contents-API read
  access is needed rather than a broader scope, though the granted scope
  itself is still stated correctly.
