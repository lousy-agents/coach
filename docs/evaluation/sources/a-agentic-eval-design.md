# Source ledger (a): agentic evaluation design

Track (a) of the Coach evaluation design: how to build and grade an evaluation in which an agent runs Coach on a fixture repository and refactors it. Each row was read from the primary source on 25 Nov 2026. Quotes are verbatim and short. Where a fetch read part of a page only as a summary, the row says so, and nothing from that part is quoted. Design documents cite these rows as `a.N`.

## a.1 Anthropic, "Demystifying evals for AI agents" (engineering post)

- URL: https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents (published 9 Jan 2026)
- Key findings:
  1. An eval is built from tasks, trials, graders and transcripts. A task "is a single test with defined inputs and success criteria", and "Because model outputs vary between runs, we run multiple trials to produce more consistent results." A transcript "is the complete record of a trial, including outputs, tool calls, reasoning, intermediate results".
  2. Graders: "Agent evaluations typically combine three types of graders: code-based, model-based, and human." The post recommends "choosing deterministic graders where possible". A model-based grader "Requires calibration with human graders for accuracy", and "LLM-as-judge graders should be closely calibrated with human experts". It also advises: "grade each dimension with an isolated LLM-as-judge rather than using one to grade all dimensions."
  3. Consistency metrics: pass@k "measures the likelihood that an agent gets at least one correct solution in k attempts"; pass^k "measures the probability that all k trials succeed". The post recommends "pass^k for agents where consistency is essential". Also: "a 0% pass rate across many trials ... is most often a signal of a broken task, not an incapable agent".
  4. Outcome over path, and isolation: "it's often better to grade what the agent produced, not the path it took." "Make your graders resistant to bypasses or hacks." "Each trial should be "isolated" by starting from a clean environment", because "Shared state can also artificially inflate performance" (the example is Claude "examining the git history from previous trials").
  5. Read the transcripts: "we do not take eval scores at face value until someone digs into the details of the eval".
- Design implication for Coach:
  - Grade each refactor trial by its outcome, the fixture's final structure, with code-based graders.
  - Start every trial from a fresh copy of the fixture, with no git history from other trials.
  - Report pass^k as well as the mean.
  - Calibrate any LLM judge against human labels, one dimension at a time.
  - Read a sample of transcripts before trusting the numbers.
- Superseded: parts of this implication are replaced by automation in `agentic-eval-design.md` §1, "Departures from the sources".

## a.2 Anthropic docs, "Define success criteria and build evaluations"

- URL: https://platform.claude.com/docs/en/test-and-evaluate/develop-tests (redirected from https://docs.anthropic.com/en/docs/test-and-evaluate/develop-tests; no date shown)
- Key findings:
  1. Choose the cheapest reliable grader: "choose the fastest, most reliable, most scalable method". Code-based grading is "Fastest and most reliable, extremely scalable, but also lacks nuance". Human grading is "Most flexible and high quality, but slow and expensive. Avoid if possible."
  2. LLM grading: "Test to ensure reliability first, then scale." "Have detailed, clear rubrics". "instruct the LLM to output only 'correct' or 'incorrect', or to judge from a scale of 1–5."
  3. Judge independence: "Generally best practice to evaluate with a different model from the one used to generate the evaluated output".
  4. Task realism: "Design evals that mirror your real-world task distribution."
- Design implication for Coach:
  - The refactor task must look like real pilot work, a plausible change to a real-looking repository.
  - Any LLM judge must be a different model from the agent under test.
  - Each judge gets a closed label set with a rubric, and is tested for reliability before it is used at scale.
- Superseded: parts of this implication are replaced by automation in `agentic-eval-design.md` §1, "Departures from the sources".

## a.3 Inspect AI documentation (UK AI Security Institute)

- URLs: https://inspect.aisi.org.uk/ , https://inspect.aisi.org.uk/tasks.html , https://inspect.aisi.org.uk/scorers.html , https://inspect.aisi.org.uk/model-graded.html , https://inspect.aisi.org.uk/metrics.html , https://inspect.aisi.org.uk/sandboxing.html , https://inspect.aisi.org.uk/eval-logs.html
- Key findings:
  1. The unit of evaluation: "Tasks provide a recipe for an evaluation consisting minimally of a dataset, a solver, and a scorer" (tasks.html). "The core data type underlying the use of datasets with Inspect is the Sample" (datasets.html).
  2. Isolation: "each sample gets its own sandbox instance, even if the sandbox is defined at Task level" (sandboxing.html). The network limit is narrow: "The network_mode: none entry applies only to processes inside the container." The page also warns: "Don't count on a non-root container user to contain an adversarial model".
  3. Repeated trials: "If a task is run over more than one epoch, multiple scores will be generated for each sample." Reducers include `pass_at_{k}` ("Probability of at least 1 correct sample given k epochs") and `pass_k_{k}` ("Probability that all k epoch attempts succeed") (metrics.html).
  4. Model graders are not deterministic: "setting temperature=0 reduces, but does not always eliminate, run-to-run disagreement". By default the grader "runs with the provider's default generation settings" (model-graded.html).
  5. Audit trail: a log keeps "Each sample evaluated, including its input, output, target, and score", and "When you edit a score, a ScoreEditEvent is automatically added to the sample's event log" (eval-logs.html).
- Design implication for Coach:
  - Model the eval in Inspect's terms: a sample is a fixture plus a prompt, the solver is the agent run (`claude -p`), and the scorer is the ground-truth grader. Inspect itself or a thin harness of the same shape can run it.
  - Run one sandbox per trial, and run each sample over several epochs, reported with both `pass_at` and `pass_k`.
  - Pin the grader model's settings, and keep every transcript and score edit for adjudication.

## a.4 Jimenez et al., "SWE-bench: Can Language Models Resolve Real-World GitHub Issues?" (ICLR 2024)

- URL: https://arxiv.org/abs/2310.06770 (HTML v3 read; Sections 1–5 verbatim, the rest only as a summary and not quoted)
- Key findings:
  1. Tasks come from real work: "We create candidate tasks by selecting the merged PRs that (1) resolve a GitHub issue and (2) make changes to the test files of the repository".
  2. Execution-based grading, with two test sets. FAIL_TO_PASS: "there is at least one fail-to-pass test which was used to test the reference solution". PASS_TO_PASS: "a median of 51 additional tests run to check whether prior functionality is properly maintained".
  3. The success rule: "If the patch applies successfully and all of these tests pass we consider the proposed solution to have successfully resolved the issue."
- Design implication for Coach:
  - Every fixture carries behaviour tests in two groups.
  - Must-keep-passing tests guard behaviour, so a refactor that deletes or breaks code fails.
  - Structural target checks play the part of FAIL_TO_PASS: they fail on the seeded fixture and pass after a correct refactor.
  - A trial succeeds only if both groups pass.

## a.5 OpenAI, "Introducing SWE-bench Verified"

- URL: https://openai.com/index/introducing-swe-bench-verified/ (13 Aug 2024, updated 24 Feb 2025)
- Key findings:
  1. Graders can be wrong in both directions: "The unit tests used to evaluate the correctness of a solution are often overly specific, and in some cases are even unrelated to the issue." "Many samples have an issue description that is underspecified".
  2. Human validation at scale: "We worked with 93 software developers experienced in Python to manually screen SWE-bench samples for quality", and "each sample is labeled 3 times by separate annotators". The labels were combined conservatively, "taking the highest-severity label amongst the 3 annotators".
  3. The cost of skipping validation: "68.3% of SWE-bench samples being filtered out due to underspecification, unfair unit tests, or other issues", which left "500 samples verified to be non-problematic by our human annotators".
  4. Contamination: models trained on internet text "are likely to be contaminated on the tasks".
- Design implication for Coach:
  - Every fixture and its target state need human validation before use, with at least two independent humans and the stricter label kept.
  - The target checks must accept every valid refactor, not only the one the author had in mind.
  - Fixtures are written fresh for this eval, so they are not in any training set.
- Superseded: parts of this implication are replaced by automation in `agentic-eval-design.md` §1, "Departures from the sources".

## a.6 Zhu et al., "Establishing Best Practices for Building Rigorous Agentic Benchmarks" (the Agentic Benchmark Checklist, ABC)

- URL: https://arxiv.org/abs/2507.02825 (v5, 7 Aug 2025; Sections 1–5 read verbatim, Appendix F only as a summary and not quoted)
- Key findings:
  1. Two validity conditions. Task validity: "a task should be solvable if and only if the agent possesses the target capability." Outcome validity: "the evaluation result (e.g., tests or checks) truly indicates task success."
  2. Trivial agents pass broken benchmarks: "a trivial agent that returns empty responses is considered successful on intentionally impossible tasks" (τ-bench, 38%), and "an agent can score 100% on SWE-Lancer without resolving any tasks" by replacing tests "with a trivial assertion (e.g., assert 1 == 1)".
  3. Checklist items that apply here:
     - "it is important to fully isolate agents from the ground truth results (T.5)";
     - "Providing an automatic oracle solver can help demonstrate the correctness of the task configuration (T.9)";
     - "the state space should be complex enough (O.g.3)";
     - "manually verifying the correctness and quality of test cases (O.d.1)";
     - "conducting pilot experiments to assess the accuracy and self-consistency of LLM judges (O.c.1)".
  4. The size of the errors: such issues "can lead to under- or overestimation of agents' performance by up to 100% in relative terms"; "SWE-bench-Verified uses insufficient test cases".
  5. Reporting: the paper calls for "reporting benchmark results rigorously, including measures of statistical significance (R.10), clear interpretation guidelines (R.11), and appropriate baseline comparisons (R.12-13)", and finds "80% of the benchmarks fail to acknowledge weaknesses in their design or implementation".
- Design implication for Coach:
  - Isolate the agent from the manifest and the graders, and make test files read-only, or check their hashes.
  - Run two baseline agents: a do-nothing agent and a delete-the-code agent. Both must score zero.
  - Each fixture ships a reference ("oracle") refactor that must score full marks.
  - Report confidence intervals, the trivial-agent baselines, and a threats-to-validity section.

## a.7 Miller, "Adding Error Bars to Evals: A Statistical Approach to Language Model Evaluations" (Anthropic, 2024)

- URLs: https://arxiv.org/abs/2411.00640 (v1, 1 Nov 2024; HTML body Sections 1–6 read verbatim, Appendices A–C not read and not cited), https://www.anthropic.com/research/statistical-approach-to-model-evals (19 Nov 2024)
- Key findings:
  1. Report uncertainty with every score: "We suggest reporting the standard error of the mean alongside (beneath) the mean when reporting eval scores." The post adds: "A 95% confidence interval can be calculated from the SEM by adding and subtracting 1.96 × SEM from the mean score." (For 0/1 scores the paper's SE is the square root of s̄(1−s̄)/n.)
  2. Cluster on the unit of randomization: "we recommend clustering standard errors on the unit of randomization". Without that, related questions give standard errors that "will yield inconsistent standard errors", and "clustered standard errors on popular evals can be over three times as large as naive standard errors."
  3. Resample within a question, then average: "we recommend resampling answers from the same model several times, and using the question-level averages as the question scores". The paper warns that "computing a pooled standard error across all KN answers will be inconsistent", and advises against lowering the temperature to cut variance ("we advise against this practice").
  4. Compare conditions on the same questions: "paired differences represent a 'free' reduction in estimator variance", and "We therefore recommend using the paired version of the standard error estimate wherever practicable."
  5. Size the eval in advance with power analysis: "Using power analysis to determine whether an eval (or a random subsample) is capable of testing a hypothesis of interest". Its worked example concludes that "new evals should contain at least 1,000 questions"; raising per-question samples from 1 to 10 "reduces the Minimum Detectable Effect from 13.2% to 7.5%."
- Design implication for Coach:
  - The unit of analysis is the fixture task, not the single trial: average each fixture's K trials, then compute the standard error across fixtures, clustered by fixture family where tasks share a seeded repository.
  - Compare Coach against the no-Coach control as paired differences on the same fixtures, and report the mean difference, its paired SE, the 95% CI and the correlation.
  - Run a power analysis before the main run. With tens of fixtures rather than 1,000 questions, the design must state the minimum detectable effect it can see, and not claim more.
  - Keep the agent's sampling settings at their defaults; do not lower temperature to make the numbers steadier.
