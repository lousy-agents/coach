# Source ledger (f): ground truth and validation without humans

Track (f) of the Coach evaluation design. It covers how fixtures, ground truth and their validation can be produced and checked with no human in the loop, and what an automated-only path can establish.

How the sources were read:
- Rows were read from the sources named, on 26 Nov 2026.
- Two papers (LAVA, SZZ) were read from their author PDFs, extracted to text. Where a quote spans two PDF columns it is joined, and no word is changed.
- Some rows were read only from the arXiv abstract page. They say so, and quote only the abstract.
- Design documents cite these rows as `f.N`.

## f.1 Dolan-Gavitt et al., "LAVA: Large-scale Automated Vulnerability Addition" (IEEE S&P 2016)

- URL: https://www.ieee-security.org/TC/SP2016/papers/0824a110.pdf (full text, extracted)
- Key findings:
  1. Without ground truth, a tool cannot be measured: "There is currently no way to measure fundamental figures of merit such as miss and false alarm rate for a bug finding tool."
  2. Hand-built corpora are expensive: "a corpus of fourteen very well annotated historic bugs with triggering inputs took about six months to construct".
  3. Injection gives ground truth by construction: "Each validated bug is guaranteed to come with a triggering input". The paper's first requirement for corpus bugs is that they "Be cheap and plentiful".
  4. Injected bugs are artificial, and the paper says so: "These vulnerabilities are synthetic but, we argue, still realistic".
- Design implication for Coach:
  - Seed violations by program transformation, so that each one carries its exact ground truth.
  - Accept every seeded violation only after a mechanical check that it is present.
  - Keep c.6's warning beside this row: injected defects are found at different rates from natural ones.

## f.2 Stryker Mutator, "Mutant states and metrics"

- URL: https://stryker-mutator.io/docs/mutation-testing-elements/mutant-states-and-metrics/ (full page)
- Key findings:
  1. Killed: "When at least one test failed while this mutant was active, the mutant is killed."
  2. Survived: "When all tests passed while this mutant was active, the mutant survived. You're missing a test for it."
  3. The mutation score is detected ÷ valid × 100. Compile and runtime errors are excluded: "It is not represented in your mutation score."
- Design implication for Coach:
  - A generated hidden suite is accepted only when its mutation score, defined this way, reaches a preregistered threshold.
  - Restricting the score to code reachable from the API surface, and adding a coverage floor, are this design's own additions. They are not from f.2.

## f.3 Foster et al. (Meta), "Mutation-Guided LLM-based Test Generation at Meta" (submitted to FSE 2025 Industry)

- URL: https://arxiv.org/abs/2501.12862 (abstract only)
- Key findings:
  1. "From these currently uncaught faults, ACH generates tests that can catch them, thereby 'killing' the mutants."
  2. Equivalent mutants are detected automatically, imperfectly: the detector "achieves a precision of 0.79 and a recall of 0.47 (rising to 0.95 and 0.96 with simple pre-processing)."
- Design implication for Coach:
  - Generated tests can be judged by mutants they kill, without a human.
  - It is this design's inference, not a statement in f.3, that equivalent mutants make a score of 100% unreachable, so the threshold sits below 1.

## f.4 Alshahwan et al. (Meta), "Automated Unit Test Improvement using Large Language Models at Meta" (FSE 2024)

- URL: https://arxiv.org/abs/2402.09171 (abstract only)
- Key findings:
  1. Acceptance is a set of automated filters: "TestGen-LLM verifies that its generated test classes successfully clear a set of filters". The filters are there "that assure measurable improvement over the original test suite".
  2. Many candidates fail them: "75% of TestGen-LLM's test cases built correctly, 57% passed reliably, and 25% increased coverage."
- Design implication for Coach:
  - Every generated artefact passes automated filters or is regenerated.
  - Expect many rejections, and record the seed of each attempt.

## f.5 Panickssery, Bowman and Feng, "LLM Evaluators Recognize and Favor Their Own Generations" (2024)

- URL: https://arxiv.org/abs/2404.13076 (abstract only)
- Key findings:
  1. The bias is called "self-preference, where an LLM evaluator scores its own outputs higher than others'".
  2. Models recognise their own output: "LLMs such as GPT-4 and Llama 2 have non-trivial accuracy at distinguishing themselves from other LLMs and humans."
- Design implication for Coach: this is evidence of bias in LLM **evaluators**. Applying it to LLM **generators** is this design's inference. It is part of the reason the design uses no LLM to generate or judge anything.

## f.6 Zheng et al., "Judging LLM-as-a-Judge with MT-Bench and Chatbot Arena" (NeurIPS 2023 Datasets and Benchmarks)

- URL: https://arxiv.org/abs/2306.05685 (abstract only)
- Key finding: LLM judges show "position, verbosity, and self-enhancement biases, as well as limited reasoning ability".
- Design implication for Coach: the listed biases are another reason this design uses no LLM judge.

## f.7 White et al., "LiveBench: A Challenging, Contamination-Free LLM Benchmark" (ICLR 2025)

- URL: https://arxiv.org/abs/2406.19314 (abstract only). The ICLR 2025 title is the one above. The arXiv page's latest version, v2 of 18 Apr 2025, reads "Contamination-Limited".
- Key findings:
  1. It "scores answers automatically according to objective ground-truth values".
  2. LLM judges and crowdsourcing "can introduce significant biases, and break down when scoring hard questions".
- Design implication for Coach: prefer ground truth defined by construction and checked by code over any judged label.

## f.8 Śliwerski, Zimmermann and Zeller, "When Do Changes Induce Fixes?" (MSR 2005), the SZZ algorithm

- URLs: https://www.st.cs.uni-saarland.de/papers/msr2005/ (abstract page), https://www.st.cs.uni-saarland.de/papers/msr2005/msr2005.pdf (full text, extracted)
- Key findings:
  1. "We show how to automatically locate fix-inducing changes by linking a version archive (such as CVS) to a bug database (such as BUGZILLA)."
  2. The mechanism is blame on the revision before the fix: "we call the CVS annotate command for revision r1 = 1.17 because this was the last revision without the fix".
  3. A stated limit: the "CVS annotate command is only reliable for text files".
- Design implication for Coach:
  - History can show, with no new human labels, whether code that Coach flags is later implicated in **bug fixes**.
  - SZZ identifies fix-inducing changes, not refactors.
  - The bug labels come from the linked tracker or commit messages, which are themselves human-written.

## f.9 Rosa et al., "Evaluating SZZ Implementations Through a Developer-informed Oracle" (ICSE 2021)

- URL: https://arxiv.org/abs/2102.03300 (abstract only)
- Key findings:
  1. SZZ's accuracy is unsettled: "fairly evaluating those implementations on a reliable oracle is an open problem."
  2. "Ideally, there should be a golden set created by the original developers of the studied systems."
- Design implication for Coach: the abstract says only that a reliable oracle for SZZ is an open problem. So the error rate of any SZZ signal used here is unknown, and the signal is reported as association, never as precision.

## f.10 gocognit (Go cognitive complexity)

- URL: https://github.com/uudashr/gocognit (MIT; no release tag shown on the page)
- Key findings:
  1. It "Calculates cognitive complexities of functions (and methods) in Go source code."
  2. It cites the specification without claiming to implement it in full: "these rules are specific for Go, please see the original whitepaper for more complete reference."
  3. It has machine-readable output: `-json` will "encode the output as JSON". `-over N` shows only functions above N and exits 1.
- Design implication for Coach: pin it as the external grader for Go `complexity.cognitive_complexity`. Its counts follow its own Go reading of the specification, and may differ from Coach's (d.1).

## f.11 ESLint core rule `max-depth`

- URL: https://eslint.org/docs/latest/rules/max-depth
- Key findings:
  1. "This rule enforces a maximum depth that blocks can be nested to reduce code complexity."
  2. The option is `max` (default 4).
- Design implication for Coach: pin it as the external grader for TS `complexity.max_nesting_depth`.
- Not retrieved: the TypeScript cognitive-complexity rule in eslint-plugin-sonarjs. Its GitHub page returned 404 and its npm page 403. Nothing is cited from it, and a TypeScript cognitive-complexity grader stays a follow-up.

## f.12 Gremlins (Go mutation testing)

- URLs: https://github.com/go-gremlins/gremlins (Apache-2.0), https://gremlins.dev/latest/
- Key findings:
  1. Each mutant is reported as one of RUNNABLE, NOT COVERED, KILLED, LIVED, TIMED OUT or NOT VIABLE.
  2. "Gremlins is still in its 0.x.x release, which, as per SemVer, doesn't guarantee backward compatibility."
- Design implication for Coach:
  - Pin an exact version.
  - Compute f.2's score from the statuses: detected = KILLED + TIMED OUT, valid = detected + LIVED + NOT COVERED. NOT VIABLE is excluded, like f.2's compile errors.

## f.13 StrykerJS (TypeScript mutation testing)

- URLs: https://stryker-mutator.io/docs/stryker-js/introduction/ , https://stryker-mutator.io/docs/stryker-js/configuration/ (Apache-2.0)
- Key findings:
  1. It "supports most JavaScript projects, including TypeScript".
  2. Thresholds: "mutation score < break: Error! Stryker will exit with exit code 1". The `json` reporter writes `reports/mutation/mutation.json`.
- Design implication for Coach: pin an exact version, set `thresholds.break` from the protocol, and read the JSON report.
