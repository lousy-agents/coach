#!/usr/bin/env python3
"""Static lint helper for feature specifications.

This script performs deterministic markdown checks that are useful before an
LLM adversarial audit. It intentionally avoids external dependencies.

Exit codes: 0 when no High findings were produced (Medium and Low findings may
still be present in the output), 1 when at least one High finding was produced,
2 when the spec file could not be read (missing, a directory, or denied).

Known limitations:

- Only fenced code blocks are recognized as code. Indented (four-space) code
  blocks are linted as prose.
- YAML front matter and HTML comments are linted as prose. `---` is also the
  spec template's thematic break, so front matter cannot be detected by
  delimiter alone.
- Finding ids (`SL-001`, ...) are run-local ordinals, not stable identifiers.
  They shift whenever the document or the check set changes.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Iterable, Iterator, List, Tuple


class Severity:
    """Finding severities. `HIGH` is the one the exit code depends on."""

    HIGH = "High"
    MEDIUM = "Medium"
    LOW = "Low"


@dataclass
class Finding:
    id: str
    severity: str
    category: str
    line: int | None
    title: str
    evidence: str
    recommendation: str


REQUIRED_SECTIONS = [
    "Problem Statement",
    "Personas",
    "Value Assessment",
    "User Stories",
    "Design",
    "Tasks",
    "Out of Scope",
    "Future Considerations",
]

# `feature-to-plan`'s SKILL.md sanctions cosmetic title variation when the target
# repo has an established convention. Only the alternates it names are accepted;
# this table is deliberately not user-configurable, because a configurable alias
# list would let a spec silence a real missing-section finding by renaming it.
SECTION_ALIASES = {
    "Personas": ("personas", "stakeholders"),
    "Acceptance Criteria": ("acceptance criteria", "acceptance"),
}

AMBIGUOUS_TERMS = [
    "appropriate",
    "as needed",
    "better",
    "easy",
    "efficient",
    "fast",
    "handle",
    "improve",
    "intuitive",
    "optimize",
    "robust",
    "seamless",
    "simple",
    "support",
    "user-friendly",
]

EARS_STARTS = (
    "the ",
    "when ",
    "while ",
    "where ",
    "if ",
)

REQUIRED_TASK_FIELDS = [
    "Objective",
    "Context",
    "Affected files",
    "Requirements",
    "Verification",
    "Done when",
]

HEADING_RE = re.compile(r"^(#{1,6})\s+(.+?)\s*$")
FENCE_RE = re.compile(r"^(?P<indent> {0,3})(?P<delim>`{3,}|~{3,})(?P<info>.*)$")
BULLET_RE = re.compile(r"^\s*[-*]\s+")
# `?` is not a word character, so `\b\?\?\?\b` can never match a standalone
# placeholder. The marker alternation is kept separate from the word-boundary group.
PLACEHOLDER_RE = re.compile(r"\b(?:TODO|TBD|FIXME|XXX)\b|\?{3,}", re.IGNORECASE)
# Matches the template's `### Story: <Title>` and `### Story 1: <Title>` forms
# without matching unrelated headings such as `### Storybook integration`.
STORY_RE = re.compile(r"^Story(?:\s+\d+)?\s*:", re.IGNORECASE)
TASK_TITLE_RE = re.compile(r"^Task\s+\d+", re.IGNORECASE)
FIELD_RE = re.compile(r"^[ \t]*\*\*(?P<label>[^*\n]+?)\*\*[ \t]*:", re.M)
CHECKED_BOX_RE = re.compile(r"\[[xX]\]")
# A value made only of whitespace and bare list markers carries no content.
BARE_MARKER_RE = re.compile(r"\[[ xX]\]|[-*\s]")
FLOWCHART_RE = re.compile(r"^(?:flowchart|graph)\s+(?:TB|TD|BT|LR|RL)\b", re.IGNORECASE)
SEQUENCE_RE = re.compile(r"^sequenceDiagram\b", re.IGNORECASE)


@dataclass(frozen=True)
class Fence:
    """A top-level fenced block. CommonMark fences do not nest, so every fence
    recognized by the scanner is top-level by construction."""

    open_line: int
    # None when the fence never closed, which CommonMark runs to end of document.
    # No check reads this yet; it is what distinguishes the two cases in a report.
    close_line: int | None
    info: str
    body: List[str] = field(default_factory=list)


@dataclass(frozen=True)
class Heading:
    line: int
    level: int
    raw_title: str
    title: str


@dataclass(frozen=True)
class Section:
    start: int  # 1-based line of the heading itself
    end: int  # 1-based line of the next sibling-or-shallower heading, exclusive


def accepted_titles(canonical: str) -> Tuple[str, ...]:
    return SECTION_ALIASES.get(canonical, (canonical.lower(),))


def read_lines(path: Path) -> List[str]:
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        text = path.read_text(encoding="utf-8", errors="replace")
    # `str.splitlines()` also breaks on form feed and U+2028, which would shift
    # reported line numbers away from what the author's editor shows.
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    return lines


def scan_fences(lines: List[str]) -> Tuple[List[bool], List[Fence]]:
    """Classify every line as fenced or not, following CommonMark closing rules.

    A closing fence must use the same character as its opener, be at least as
    long, and carry no info string. That is what keeps a three-backtick
    ```mermaid block nested inside a four-backtick example fence from being
    read as a real diagram.
    """
    in_code = [False] * len(lines)
    fences: List[Fence] = []

    open_at: int | None = None
    open_char = ""
    open_len = 0
    open_info = ""
    body: List[str] = []

    for idx, line in enumerate(lines, start=1):
        match = FENCE_RE.match(line)
        if open_at is None:
            if not match:
                continue
            delim = match.group("delim")
            info = match.group("info").strip()
            # An opening backtick fence may not carry a backtick in its info string.
            if delim[0] == "`" and "`" in info:
                continue
            open_at, open_char, open_len, open_info, body = idx, delim[0], len(delim), info, []
            in_code[idx - 1] = True
            continue

        in_code[idx - 1] = True
        if match:
            delim = match.group("delim")
            if (
                delim[0] == open_char
                and len(delim) >= open_len
                and match.group("info").strip() == ""
            ):
                fences.append(Fence(open_line=open_at, close_line=idx, info=open_info, body=body))
                open_at = None
                continue
        body.append(line)

    if open_at is not None:
        # An unterminated fence runs to the end of the document, per CommonMark.
        fences.append(Fence(open_line=open_at, close_line=None, info=open_info, body=body))

    return in_code, fences


def scan_headings(lines: List[str], in_code: List[bool]) -> List[Heading]:
    found: List[Heading] = []
    for idx, line in enumerate(lines, start=1):
        if in_code[idx - 1]:
            continue
        match = HEADING_RE.match(line)
        if match:
            raw_title = match.group(2).strip()
            found.append(
                Heading(
                    line=idx,
                    level=len(match.group(1)),
                    raw_title=raw_title,
                    title=raw_title.rstrip(":").strip(),
                )
            )
    return found


@dataclass
class Doc:
    """The document model every check shares.

    Fence state is computed once here rather than re-derived by each check, so
    the seven scanners cannot drift from one another on the fence rules.
    """

    lines: List[str]
    in_code: List[bool]
    fences: List[Fence]
    headings: List[Heading]

    def prose(self) -> Iterator[Tuple[int, str]]:
        for idx, line in enumerate(self.lines, start=1):
            if not self.in_code[idx - 1]:
                yield idx, line

    def span(self, heading: Heading, ignore: Iterable[Heading] = ()) -> Section:
        """Range of a heading, up to the next sibling-or-shallower heading.

        `ignore` exempts headings that belong to the construct being scoped —
        a `## Acceptance` or a `## Task 1` written at level 2 would otherwise
        terminate the very section it is part of.
        """
        ignored = {h.line for h in ignore}
        end = len(self.lines) + 1
        seen = False
        for other in self.headings:
            if other.line == heading.line:
                seen = True
                continue
            if seen and other.level <= heading.level and other.line not in ignored:
                end = other.line
                break
        return Section(start=heading.line, end=end)

    def prose_in(self, section: Section) -> Iterator[Tuple[int, str]]:
        for idx in range(section.start + 1, section.end):
            if idx - 1 < len(self.lines) and not self.in_code[idx - 1]:
                yield idx, self.lines[idx - 1]

    def find_section(self, canonical: str, level: int = 2, ignore: Iterable[Heading] = ()) -> Section | None:
        accepted = accepted_titles(canonical)
        for heading in self.headings:
            if heading.level == level and heading.title.lower() in accepted:
                return self.span(heading, ignore=ignore)
        return None


def build_doc(lines: List[str]) -> Doc:
    in_code, fences = scan_fences(lines)
    return Doc(lines=lines, in_code=in_code, fences=fences, headings=scan_headings(lines, in_code))


def finding(severity: str, category: str, line: int | None, title: str, evidence: str, recommendation: str) -> Finding:
    """A finding without an id. `run_lint` numbers them once, in report order."""
    return Finding(
        id="",
        severity=severity,
        category=category,
        line=line,
        title=title,
        evidence=evidence,
        recommendation=recommendation,
    )


def acceptance_headings(doc: Doc) -> List[Heading]:
    accepted = accepted_titles("Acceptance Criteria")
    return [h for h in doc.headings if h.title.lower() in accepted]


def lint_required_sections(doc: Doc) -> List[Finding]:
    found: List[Finding] = []
    for section in REQUIRED_SECTIONS:
        accepted = accepted_titles(section)
        if any(h.level == 2 and h.title.lower() in accepted for h in doc.headings):
            continue
        misplaced = next((h for h in doc.headings if h.title.lower() in accepted), None)
        if misplaced is None:
            line, evidence = None, "section not found"
        else:
            line = misplaced.line
            evidence = (
                f"`{misplaced.raw_title}` appears at heading level {misplaced.level}, "
                "not as a level-2 section"
            )
        found.append(
            finding(
                Severity.HIGH,
                "Structure",
                line,
                f"Missing required section: {section}",
                evidence,
                f"Add a `## {section}` section or map the repository's equivalent heading explicitly.",
            )
        )
    return found


def lint_todo_markers(doc: Doc) -> List[Finding]:
    return [
        finding(
            Severity.MEDIUM,
            "Completeness",
            idx,
            "Unresolved placeholder marker",
            line.strip(),
            "Convert the placeholder into a bounded Open Question or resolve it before implementation.",
        )
        for idx, line in doc.prose()
        if PLACEHOLDER_RE.search(line)
    ]


def lint_ambiguous_terms(doc: Doc) -> List[Finding]:
    term_pattern = re.compile(r"\b(" + "|".join(re.escape(t) for t in AMBIGUOUS_TERMS) + r")\b", re.IGNORECASE)
    found: List[Finding] = []
    for idx, line in doc.prose():
        stripped = line.strip()
        if not stripped or stripped.startswith("|"):
            continue
        match = term_pattern.search(stripped)
        if match:
            found.append(
                finding(
                    Severity.LOW,
                    "Ambiguity",
                    idx,
                    f"Potentially ambiguous term: {match.group(1)}",
                    stripped,
                    "Replace subjective language with an observable behavior, threshold, or verification condition.",
                )
            )
    return found


def is_ears_like(bullet: str) -> bool:
    text = re.sub(r"^\s*[-*]\s+(\[.\]\s+)?", "", bullet).strip().lower()
    return text.startswith(EARS_STARTS) and re.search(r"\bshall\b", text) is not None


def lint_criteria_sections(doc: Doc) -> List[Finding]:
    """Each acceptance-criteria section has bullets, and they read as EARS."""
    found: List[Finding] = []
    total = 0
    ears_like = 0

    for heading in acceptance_headings(doc):
        bullets = [
            (idx, line)
            for idx, line in doc.prose_in(doc.span(heading))
            if BULLET_RE.match(line)
        ]
        if not bullets:
            found.append(
                finding(
                    Severity.HIGH,
                    "Acceptance Criteria",
                    heading.line,
                    "Empty acceptance criteria section",
                    "no bullet criteria found",
                    "Add EARS-style criteria with trigger/condition and observable system response.",
                )
            )
            continue
        for idx, line in bullets:
            total += 1
            if is_ears_like(line):
                ears_like += 1
            else:
                found.append(
                    finding(
                        Severity.MEDIUM,
                        "Acceptance Criteria",
                        idx,
                        "Acceptance criterion may not be EARS-like",
                        line.strip(),
                        "Rewrite as `When/While/Where/If/The <system> shall <observable response>`.",
                    )
                )

    if total == 0:
        found.append(
            finding(
                Severity.HIGH,
                "Acceptance Criteria",
                None,
                "No acceptance criteria bullets found",
                "no `Acceptance Criteria` bullets detected",
                "Add acceptance criteria for each user story using EARS-style syntax.",
            )
        )
    elif ears_like / total < 0.5:
        found.append(
            finding(
                Severity.MEDIUM,
                "Acceptance Criteria",
                None,
                "Most acceptance criteria are not EARS-like",
                f"{ears_like}/{total} bullets look EARS-like",
                "Rewrite weak criteria so each includes a condition or trigger and a shall statement.",
            )
        )
    return found


def story_headings(doc: Doc) -> List[Heading]:
    """Stories at any level. Requiring level 3 here would let a misplaced story
    escape every check; the level itself is reported by `lint_story_headings`."""
    return [h for h in doc.headings if STORY_RE.match(h.raw_title)]


def lint_story_headings(doc: Doc) -> List[Finding]:
    return [
        finding(
            Severity.MEDIUM,
            "Structure",
            story.line,
            "Story heading must be level 3",
            f"`{'#' * story.level} {story.raw_title}` is at heading level {story.level}",
            "Write each story as a level-3 `### Story N: <Title>` heading under `## User Stories`.",
        )
        for story in story_headings(doc)
        if story.level != 3
    ]


def criteria_range(doc: Doc, stories: List[Heading], pos: int, criteria: List[Heading]) -> int:
    """Line at which a story's criteria stop counting as its own.

    Criteria headings are exempt from ending the story: `## Acceptance` is a
    sanctioned title, and at level 2 it would otherwise terminate the story it
    belongs to and produce a false "no acceptance criteria".
    """
    end = doc.span(stories[pos], ignore=criteria).end
    if pos + 1 < len(stories):
        end = min(end, stories[pos + 1].line)
    return end


def lint_story_coverage(doc: Doc) -> List[Finding]:
    """Every story has criteria of its own.

    A document-wide bullet count lets one well-covered story hide every other
    story that has none, which is the false negative this check closes.
    """
    criteria = acceptance_headings(doc)
    stories = story_headings(doc)
    found: List[Finding] = []

    for pos, story in enumerate(stories):
        end = criteria_range(doc, stories, pos, criteria)
        covering = [h for h in criteria if story.line < h.line < end]
        if not covering:
            found.append(
                finding(
                    Severity.HIGH,
                    "Acceptance Criteria",
                    story.line,
                    "Story has no acceptance criteria",
                    story.raw_title,
                    "Add an `#### Acceptance Criteria` subsection with EARS-style criteria for this story.",
                )
            )
            continue
        # Criteria at or above the story's own level are ambiguous: nothing
        # distinguishes "this story's criteria under a sanctioned alias" from
        # "a separate section that merely follows the story". Report the
        # structure rather than guessing which was meant.
        if all(h.level <= story.level for h in covering):
            unnested = covering[0]
            found.append(
                finding(
                    Severity.MEDIUM,
                    "Acceptance Criteria",
                    unnested.line,
                    "Acceptance criteria are not nested under their story",
                    f"`{'#' * unnested.level} {unnested.raw_title}` is at level "
                    f"{unnested.level}, at or above `{story.raw_title}` at level {story.level}",
                    "Nest each story's criteria one level deeper, as `#### Acceptance Criteria`.",
                )
            )
    return found


def lint_checkboxes(doc: Doc) -> List[Finding]:
    return [
        finding(
            Severity.LOW,
            "Task Hygiene",
            idx,
            "Completed checkbox in draft spec",
            line.strip(),
            "Use unchecked `[ ]` boxes in draft specs; implementers should mark completion.",
        )
        for idx, line in doc.prose()
        if CHECKED_BOX_RE.search(line)
    ]


def task_fields(block: str) -> dict[str, str]:
    """Map each `**Label**:` in a task block to the text that follows it."""
    matches = list(FIELD_RE.finditer(block))
    fields: dict[str, str] = {}
    for pos, match in enumerate(matches):
        end = matches[pos + 1].start() if pos + 1 < len(matches) else len(block)
        fields[match.group("label").strip().lower()] = block[match.end() : end]
    return fields


def is_blank_value(text: str) -> bool:
    """True when a field carries no content — whitespace and bare list markers
    (`-`, `*`, `[ ]`) do not count as a value."""
    return BARE_MARKER_RE.sub("", text) == ""


def task_block(doc: Doc, start_line: int, end_line: int) -> str:
    """Prose lines of a task. A `**Objective**:` shown inside a fenced example
    is an illustration, not a populated field."""
    return "\n".join(
        doc.lines[idx - 1]
        for idx in range(start_line, end_line)
        if idx - 1 < len(doc.lines) and not doc.in_code[idx - 1]
    )


def lint_tasks(doc: Doc) -> List[Finding]:
    task_like = [h for h in doc.headings if TASK_TITLE_RE.match(h.raw_title)]
    section = doc.find_section("Tasks", level=2, ignore=task_like)
    if section is None:
        return []

    candidates = [h for h in task_like if section.start < h.line < section.end]
    if not candidates:
        return [
            finding(
                Severity.HIGH,
                "Tasks",
                section.start,
                "No numbered task headings found",
                "Tasks section exists but no `### Task N` headings detected",
                "Split implementation into numbered, ordered tasks sized for one coding-agent session.",
            )
        ]

    found: List[Finding] = [
        finding(
            Severity.MEDIUM,
            "Tasks",
            heading.line,
            "Task heading must be level 3",
            f"`{'#' * heading.level} {heading.raw_title}` is at heading level {heading.level}",
            "Write each task as a level-3 `### Task N: <Title>` heading under `## Tasks`.",
        )
        for heading in candidates
        if heading.level != 3
    ]

    for pos, heading in enumerate(candidates):
        next_start = candidates[pos + 1].line if pos + 1 < len(candidates) else section.end
        fields = task_fields(task_block(doc, heading.line, next_start))
        for label in REQUIRED_TASK_FIELDS:
            key = label.lower()
            if key not in fields:
                found.append(
                    finding(
                        Severity.MEDIUM,
                        "Tasks",
                        heading.line,
                        f"Task missing `{label}` field",
                        heading.raw_title,
                        f"Add a `**{label}**:` field so agents know scope and completion criteria.",
                    )
                )
            elif is_blank_value(fields[key]):
                found.append(
                    finding(
                        Severity.MEDIUM,
                        "Tasks",
                        heading.line,
                        f"Task `{label}` field is empty",
                        heading.raw_title,
                        f"Give `**{label}**:` real content; an empty label tells an agent nothing.",
                    )
                )
    return found


def diagram_directive(fence: Fence) -> str | None:
    """The Mermaid directive line, or None when the block declares no type."""
    body = list(fence.body)
    pos = 0
    while pos < len(body) and not body[pos].strip():
        pos += 1
    # Mermaid permits a `---` YAML front-matter block ahead of the directive.
    if pos < len(body) and body[pos].strip() == "---":
        pos += 1
        while pos < len(body) and body[pos].strip() != "---":
            pos += 1
        pos += 1
    for line in body[pos:]:
        stripped = line.strip()
        if not stripped or stripped.startswith("%%"):
            continue
        return stripped
    return None


def is_mermaid_fence(fence: Fence) -> bool:
    tokens = fence.info.split()
    return bool(tokens) and tokens[0].lower() == "mermaid"


def lint_mermaid(doc: Doc) -> List[Finding]:
    mermaid = [f for f in doc.fences if is_mermaid_fence(f)]
    if not mermaid:
        return [
            finding(
                Severity.LOW,
                "Design",
                None,
                "No Mermaid diagrams found",
                "no mermaid code fences detected",
                "Add diagrams when data flow, sequence, state, or architecture would reduce agent ambiguity.",
            )
        ]

    directives = [d for d in (diagram_directive(f) for f in mermaid) if d]
    declared = ", ".join(d.split()[0] for d in directives) or "none typed"
    evidence = f"mermaid blocks present: {declared}"
    first_fence = mermaid[0].open_line

    required = [
        (FLOWCHART_RE, "No Mermaid data-flow diagram (`flowchart` / `graph`)",
         "Add a `flowchart TB` or `flowchart LR` diagram showing how data moves through the feature."),
        (SEQUENCE_RE, "No Mermaid sequence diagram (`sequenceDiagram`)",
         "Add a `sequenceDiagram` showing the call order between the components involved."),
    ]
    return [
        finding(Severity.LOW, "Design", first_fence, title, evidence, recommendation)
        for pattern, title, recommendation in required
        if not any(pattern.match(d) for d in directives)
    ]


def run_lint(path: Path) -> List[Finding]:
    doc = build_doc(read_lines(path))
    findings = [
        *lint_required_sections(doc),
        *lint_todo_markers(doc),
        *lint_story_headings(doc),
        *lint_criteria_sections(doc),
        *lint_story_coverage(doc),
        *lint_checkboxes(doc),
        *lint_tasks(doc),
        *lint_mermaid(doc),
        *lint_ambiguous_terms(doc),
    ]
    for ordinal, item in enumerate(findings, start=1):
        item.id = f"SL-{ordinal:03d}"
    return findings


def render_markdown(path: Path, findings: Iterable[Finding]) -> str:
    findings = list(findings)
    lines = [f"# Static Spec Audit Lint: {path}", ""]
    if not findings:
        lines.append("No deterministic lint findings. Continue with adversarial semantic audit.")
        return "\n".join(lines)
    for finding in findings:
        location = f"line {finding.line}" if finding.line else "global"
        lines.extend(
            [
                f"## {finding.id} — {finding.title}",
                "",
                f"- **Severity**: {finding.severity}",
                f"- **Category**: {finding.category}",
                f"- **Location**: {location}",
                f"- **Evidence**: {finding.evidence}",
                f"- **Recommendation**: {finding.recommendation}",
                "",
            ]
        )
    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description="Run deterministic lint checks against a feature spec markdown file.")
    parser.add_argument("spec", type=Path, help="Path to the spec markdown file")
    parser.add_argument("--format", choices=["json", "markdown"], default="json", help="Output format")
    args = parser.parse_args()

    if not args.spec.exists():
        parser.error(f"spec file not found: {args.spec}")

    try:
        findings = run_lint(args.spec)
    except OSError as exc:
        # Exit 1 is reserved for "High findings were produced". A read failure
        # that escaped as a traceback would be indistinguishable from one.
        print(f"error: could not read {args.spec}: {exc}", file=sys.stderr)
        return 2
    if args.format == "json":
        payload = {
            "artifact_audited": str(args.spec),
            "finding_count": len(findings),
            "findings": [asdict(f) for f in findings],
        }
        print(json.dumps(payload, indent=2))
    else:
        print(render_markdown(args.spec, findings))
    return 1 if any(f.severity == Severity.HIGH for f in findings) else 0


if __name__ == "__main__":
    raise SystemExit(main())
