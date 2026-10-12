# What EARS Does and Does Not Establish

> The `to-ears` skill loads this file whenever a report, PR, or user conversation is about to make a claim about what EARS achieved. Every claim must stay within the evidence summarized here.

## Do not claim

EARS alone does not:
- find all omitted behavior
- prove safety
- resolve conflicts
- establish traceability
- guarantee test coverage
- make a requirement correct, complete, unambiguous, feasible, or testable

EARS is a syntax aid. It makes conditions, triggers, states, feature applicability, and required responses **visible**, and that visibility helps reviewers catch problems. The review itself still has to happen.

## 2009 EARS study (Mavin, Wilkinson, Harwood, Novak — IEEE RE'09)

- The study interpreted 36 requirements from an aero-engine certification document, producing 47 interpreted requirements.
- Average words per requirement fell from 36.9 to 25.6.
- The authors reported improvement across eight categories of natural-language problems. Ambiguity, vagueness, and wordiness were **reduced, not eliminated**.
- Omissions apparently disappeared in that sample. The authors warned that this does not show every missing requirement was captured.
- The study was small, focused on high-level safety-related requirements, and relied on expert interpretation.
- The authors positioned EARS mainly for high-level stakeholder requirements. They did not claim it applies universally.

## 2025 PLC study (Ebrahimi Salari, Enoiu, Afzal, Seceleanu — SN Computer Science 6:314)

**Controlled experiment**
- 10 participants worked on three short requirements after a brief EARS tutorial.
- Participants chose **different patterns for the same requirement**.
- Participants reported four difficulties:
  - unclear or incomplete source requirements
  - choosing one pattern versus several
  - identifying the system perspective
  - deciding test coverage
- The task did not ask participants to write testable requirements.
- The authors, not the participants, did the later concretization and test design.

**Industrial case**
- One program, CraneNumberCheck.
- The paper reports 100% code coverage.
- It compares 119 lines of an existing test script with 26 CODESYS Test Actions. These are different units of effort.

**Caveat:** some pattern counts and examples in the paper are inconsistent. Rely on the method it describes, not on every table label.

## How to use this

Treat both studies as evidence that the approach is feasible and as a source of useful workflow ideas. They are not broad proof of effectiveness across domains, tools, teams, or system complexity.

When reporting work, separate three things:
1. what the source establishes
2. what you inferred
3. what you propose

## References

- Mavin, A., Wilkinson, P., Harwood, A., and Novak, M. "The Easy Approach to Requirements Syntax (EARS)." 2009 17th IEEE International Requirements Engineering Conference, pp. 317–322. DOI: 10.1109/RE.2009.9.
- Ebrahimi Salari, M., Enoiu, E. P., Afzal, W., and Seceleanu, C. "An Empirical Investigation of Requirements Engineering and Testing Utilizing EARS Notation in PLC Programs." *SN Computer Science* 6, 314 (2025). DOI: 10.1007/s42979-025-03843-3.
