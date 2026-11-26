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
- Design implication for Coach: a generated hidden suite is accepted only when its mutation score, defined this way over the code it covers, reaches a preregistered threshold.

## f.3 Foster et al. (Meta), "Mutation-Guided LLM-based Test Generation at Meta" (submitted to FSE 2025 Industry)

- URL: https://arxiv.org/abs/2501.12862 (abstract only)
- Key findings:
  1. "From these currently uncaught faults, ACH generates tests that can catch them, thereby 'killing' the mutants."
  2. Equivalent mutants are detected automatically, imperfectly: the detector "achieves a precision of 0.79 and a recall of 0.47 (rising to 0.95 and 0.96 with simple pre-processing)."
- Design implication for Coach:
  - Generated tests can be judged by mutants they kill, without a human.
  - Equivalent mutants make a 100% score unreachable, so the threshold has to sit below 1.

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
- Design implication for Coach:
  - Never let the agent's model, or its family, generate the tasks or judge the outcomes.
  - Keep every gate deterministic.

## f.6 Zheng et al., "Judging LLM-as-a-Judge with MT-Bench and Chatbot Arena" (NeurIPS 2023 Datasets and Benchmarks)

- URL: https://arxiv.org/abs/2306.05685 (abstract only)
- Key finding: LLM judges show "position, verbosity, and self-enhancement biases, as well as limited reasoning ability".
- Design implication for Coach: an LLM judge may only produce a labelled secondary measure, and never a gate or an input to a claim.

## f.7 White et al., "LiveBench: A Challenging, Contamination-Free LLM Benchmark" (ICLR 2025)

- URL: https://arxiv.org/abs/2406.19314 (abstract only)
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
- Design implication for Coach: history alone can show, with no human labels, whether code that Coach flags is later implicated in fixes. This gives an automated proxy for the value of findings.

## f.9 Rosa et al., "Evaluating SZZ Implementations Through a Developer-informed Oracle" (ICSE 2021)

- URL: https://arxiv.org/abs/2102.03300 (abstract only)
- Key findings:
  1. SZZ's accuracy is unsettled: "fairly evaluating those implementations on a reliable oracle is an open problem."
  2. "Ideally, there should be a golden set created by the original developers of the studied systems."
- Design implication for Coach:
  - An SZZ-based signal is a noisy proxy whose error rate is not known.
  - Report it as association, never as precision. It does not replace #260's human-labelled precision.
