# Research track (b): experimental rigour for Coach's evaluation

Source ledger for the science-grade evaluation of Coach (seeded fixture repositories with ground truth; an agentic eval in which `claude -p` uses `coach` to refactor a fixture, scored against that ground truth with a no-Coach control; reusable for the #260 precision census).

How this ledger was built: every quotation below was copied from the page named in its "Read" line, fetched on 2026-11-25, and each quotation was then checked as a verbatim substring of the fetched text. Where a source's PDF would not parse, the arXiv HTML rendering of the same version was read instead. Numbers marked **derived** are this ledger's own arithmetic from a source's formula, not a figure the source prints. Nothing is cited from memory.

## Summary of what the sources add up to

| Question | Answer the sources support | Source |
| --- | --- | --- |
| How do we report a pass rate? | Mean with a standard error, and a 95% interval of mean ± 1.96 SE; no bootstrap needed for a plain mean | 1 |
| What is the unit of independence? | The fixture (or task), not the trial or the finding. Trials of one fixture and findings in one file are clustered; clustered SEs ran up to 3.05× the naive ones in Miller's real data | 1 |
| How do we compare Coach with no-Coach? | Same fixtures, both arms, paired differences, with the SE of the difference. The paired form removes variance that both arms share | 1 |
| How many trials per fixture? | Report both pass@k and pass^k. They diverge as k grows (at 75% per trial, pass^3 ≈ 42%). Past a point, more trials per task stop shrinking the SE; more fixtures do | 1, 2, 3, 4 |
| How many fixtures? | Miller's worked example needs 969 independent questions to detect a 3-point difference at 80% power. Our effects are likely larger; see "Sizing" below | 1 |
| How do we keep the result honest? | Pre-register hypotheses, analysis plan, exclusions and stopping rule; label everything else exploratory; hold out fixtures the Coach rules were never tuned on | 5, 6, 7 |
| What can invalidate it? | Construct (is "refactor quality" what the score measures), internal (arm differences caused by something other than Coach), external (fixtures unlike real repositories), conclusion (the statistics) | 7 |
| What is contamination for us? | Fixtures, or the answers to them, that the model or Coach's own rules have seen: seeded fixtures built from public code, rules tuned on the fixtures they are scored on, and leaked ground truth in the repository itself | 5, 8 |

---

## 1. Miller, "Adding Error Bars to Evals: A Statistical Approach to Language Model Evaluations"

- **URL:** https://arxiv.org/abs/2411.00640 (Anthropic; submitted 1 Nov 2024)
- **Read:** https://arxiv.org/html/2411.00640v1, Sections 1–6 in full. The PDF would not parse. Appendices A–C (clustered-SE derivation, sample-size derivation, cluster-adjusted sample-size formula) were not read, so the cluster-adjusted power formula is cited only as existing ("Cluster-adjusted versions of Equations 9 and 10 are included in Appendix C").

Key findings:

1. **Report a standard error, from the Central Limit Theorem.** "we regard bootstrapping as unnecessary unless a complicated sampling scheme or estimator is being used." For a binary score the paper gives SE = √(s̄(1−s̄)/n) and a 95% interval of s̄ ± 1.96 × SE. The recommended reporting format puts the SE under each score, with the question count.
2. **Questions that share structure are not independent, and the naive SE is too small.** "Because the inclusion of questions is non-independent, a key assumption of the Central Limit Theorem (or a bootstrap) is violated". On real Anthropic-model results (Table 4) clustered versus naive SEs were DROP 1.34 vs 0.44 (ratio 3.05), MGSM 1.62% vs 0.86% (1.88), RACE-H 0.51% vs 0.46% (1.10); "clustered standard errors can be over 3X larger than naive standard errors."
3. **Resampling the model's answers has a ceiling.** In the paper's uniform-difficulty example, "Going from K = 1 (no resampling of answers) to K = 2, the total variance is reduced by 1/3. Increasing to K = 4, we have a variance reduction of 1/2, and setting K = 6, we reduce variance by 5/9. The upper limit on variance reduction via resampling in this example is 2/3." The paper also advises against lowering the sampling temperature to cut variance: "we advise against this practice".
4. **Compare models on paired differences.** "We therefore recommend using the paired version of the standard error estimate wherever practicable." Reason given: scores on the same questions are positively correlated "even across unrelated models", so pairing is a "free" reduction in variance. Worked example: with correlation 0.5, paired differences cut the estimator variance "by 1/3 in relative terms (that is, from 1/6 to 1/9 in absolute terms)".
5. **Power analysis sizes the eval before it runs.** The paper's example (δ = 0.03 to detect, 80% power, α = 0.05, ω² = 1/9, no answer-sampling variance) gives an eval that needs at least about 969 independent questions, and the conclusion that "new evals should contain at least 1,000 questions in order to have good signaling ability." A second example (n = 198, correlation 0.5) shows that increasing K from 1 to 10 for both models "reduces the Minimum Detectable Effect from 13.2% to 7.5%." Both are labelled fictional parameters.

**Design implication for Coach's eval:** make the *fixture* the unit of analysis; report each arm's mean pass rate with a clustered SE, and the Coach-minus-control difference as a paired per-fixture difference with its own SE; fix the number of fixtures from a power calculation made before any run, since repeating trials past a few per fixture cannot buy back a small fixture set.

## 2. Chen et al., "Evaluating Large Language Models Trained on Code" (Codex / HumanEval)

- **URL:** https://arxiv.org/abs/2107.03374
- **Read:** https://arxiv.org/html/2107.03374v2 (Section 2.1 and Appendix A). The PDF would not parse.

Key findings:

1. **Why not just report whether any of k samples passes.** The paper notes that the earlier way of computing pass@k "can have high variance", and instead draws n ≥ k samples per task, counts the c that pass, and uses the estimator 1 − C(n−c, k)/C(n, k), averaged over problems. The paper uses n = 200 with k ≤ 100.
2. **The tempting shortcut is biased.** The tempting shortcut of raising the pass@1 estimate p̂ to a power, 1 − (1 − p̂)^k, is biased: "we show that it is biased in Appendix A". Appendix A, Figure 13: it "underestimates the true value by a considerable margin", while "The unbiased estimator may have a slightly higher variance initially but allows for a fair comparison across different numbers of samples."
3. **Repeated sampling is itself a large effect.** "repeated sampling from the model is a surprisingly effective strategy for producing working solutions to difficult prompts. Using this method, we solve 70.2% of our problems with 100 samples per problem." (single-sample: 28.8%).

**Design implication for Coach's eval:** run n ≥ k trials per fixture and compute pass@k and pass^k from the (n, c) counts with the combinatorial estimators, never by raising a single pass@1 to a power; and state k, because a "best of k" arm and a "one attempt" arm are different products.

## 3. Yao et al., "τ-bench: A Benchmark for Tool-Agent-User Interaction in Real-World Domains" (origin of pass^k)

- **URL:** https://arxiv.org/abs/2406.12045
- **Read:** https://arxiv.org/html/2406.12045v1 (abstract, Section 1 and the metric definition in Section 3). The PDF would not parse.

Key findings:

1. **Definition.** "we propose a new metric – pass^k (pass hat k), defined as the chance that all k i.i.d. task trials are successful, averaged across tasks." It is the reliability counterpart of pass@k, "the chance that at least one out of k i.i.d. task trials is successful". With n trials and c successes the unbiased estimates are pass^k = E_task[C(c,k)/C(n,k)] and pass@k = 1 − E_task[C(n−c,k)/C(n,k)].
2. **The two metrics tell different stories.** The abstract: agents "succeed on <50% of the tasks, and are quite inconsistent (pass^8 <25% in retail)"; the introduction: "With increasing k, the chance of consistently solving a task drops rapidly, to as low as ∼25% for pass^8 on τ-retail for the same model" (pass^1 ≈ 61% there).
3. **What pass^k measures.** It "can capture the reliability of the agent at handling variations in conversations with the same underlying semantics", with variation coming from LM sampling of agent and user messages. The benchmark scores the end state, comparing "the database state at the end of a conversation with the annotated goal state" rather than the path.

**Design implication for Coach's eval:** a refactor that passes the ground truth on one run in three is not a tool a developer can rely on, so report pass^k beside pass@k for the Coach arm; and, like τ-bench, score the end state of the fixture against ground truth rather than the sequence of tool calls.

## 4. Anthropic, "Demystifying evals for AI agents"

- **URL:** https://www.anthropic.com/research/demystifying-evals-for-ai-agents (Engineering, published 9 Jan 2026; listed on https://www.anthropic.com/engineering)
- **Read:** the page in full.

Key findings:

1. **Trials, because runs differ.** "Each attempt at a task is a **trial**." The post also says model outputs vary between runs and "a task that passed on one eval run might fail on the next."
2. **pass@k vs pass^k, with a worked number.** pass@k "measures the likelihood that an agent gets at least one correct solution in k attempts"; pass^k "measures the probability that all k trials succeed." Example: "If your agent has a 75% per-trial success rate and you run 3 trials, the probability of passing all three is (0.75)³ ≈ 42%." "By k=10, they tell opposite stories: pass@k approaches 100% while pass^k falls to 0%." Which to use "depends on product requirements".
3. **Grade outcomes, and read transcripts.** "it's often better to grade what the agent produced, not the path it took." And: "a 0% pass rate across many trials (i.e. 0% pass@100) is most often a signal of a broken task, not an incapable agent", and "we do not take eval scores at face value until someone digs into the details of the eval and reads some transcripts."
4. **Isolation and size.** Each trial should be "isolated" by starting from a clean environment, and "20-50 simple tasks drawn from real failures is a great start". The post gives no recommended number of trials or value of k.

**Design implication for Coach's eval:** every trial starts from a fresh copy of the fixture with no state carried over; a fixture nobody (Coach arm or control) can pass is investigated as a broken fixture before it is counted; and a human reads a sample of transcripts from both arms before any number is published.

## 5. Kapoor et al., "AI Agents That Matter"

- **URL:** https://arxiv.org/abs/2407.01502
- **Read:** https://arxiv.org/html/2407.01502v1 (abstract, introduction, Section 5 and Appendix error-bar note). The PDF would not parse. Numbers the reading could not confirm verbatim (case-study reproduction gaps in Section 6) are left out.

Key findings:

1. **Cost belongs in the result.** "agent evaluations must be cost-controlled", because "Accuracy alone cannot identify progress because it can be improved by scientifically meaningless methods such as retrying." For "substantially similar accuracy, the cost can differ by almost two orders of magnitude."
2. **Holdouts.** "many agent benchmarks have inadequate holdout sets, and sometimes none at all." "In principle, a lookup table can achieve 100% accuracy on many agent benchmarks." Table 1 counts benchmarks with appropriate holdouts by generality level: distribution-specific 1/1, task-specific 3/6, domain-general 1/8, fully general 0/2. The authors add that developers "should consider keeping it secret to prevent LLM contamination or agent overfitting."
3. **Few runs, small intervals only with t.** In their own HumanEval study "We run each agent five times", and the error bars are "95% confidence intervals ... To calculate the 95% confidence intervals, we used the Student's t distribution given that we only have five runs per agent." They also state that "The high cost of evaluating agents makes it is hard to estimate confidence intervals."
4. **Non-standard evaluation inflates results.** "These errors inflate accuracy estimates and lead to overoptimism about agent capabilities."

**Design implication for Coach's eval:** record tokens and wall-clock per trial for both arms and report the Coach effect per unit of cost (a Coach arm that wins by spending several times the control's budget has not shown that Coach helps); keep a held-out fixture set the Coach rules and prompts were never tuned on; and with five-ish trials use t-based intervals, not the normal 1.96.

## 6. Bell and Kampman, "Perspectives on Machine Learning from Psychology's Reproducibility Crisis" (supplemented by Ernst and Baldassarre, "Registered Reports in Software Engineering")

- **URLs:** https://arxiv.org/abs/2104.08878 (Bell and Kampman, v2, 23 Apr 2021); https://arxiv.org/abs/2302.03649 (Ernst and Baldassarre, 2023)
- **Read:** https://arxiv.org/html/2104.08878v2 and https://arxiv.org/html/2302.03649v1, in full. Nosek et al., "The preregistration revolution" (PNAS) could not be read: the publisher returned HTTP 403.

Key findings:

1. **What preregistration is.** "With preregistration, researchers publicly declare their hypotheses, experiments, and analysis plans before running their experiments. The final paper can then reference the preregistration, as a foil against practices such as HARKing, p-hacking, and selectively reporting only experiments with desirable results."
2. **Why an eval needs it.** "In machine learning, HARKing may take the form of testing various ideas against a benchmark, and only selecting the most promising for publication. It may also apply to our choice of performance metrics, or to our choice of problem dataset." And: "Plunging repeatedly into the data to search for a result, commonly known as 'p-hacking', invalidates the core assumption of a statistical hypothesis test". The authors suggest hypotheses "clearly state the search space of hyperparameters under consideration."
3. **Pre-registered still needs a robustness check.** Multiverse analysis: "A result that holds for only select leaves cannot be considered robust." Exploratory work is to be "clearly delineated from any confirmatory hypothesis testing".
4. **Registered reports (SE).** "Registered reports are scientific publications which begin the publication process by first having the detailed research protocol, including key research questions, reviewed and approved by peers." The authors state that the results "can be negative" and the study is still published "if the protocol is adhered to (or minor deviations are thoroughly justified)", and that deviations seen to date included changes to participant recruitment and to "study constructs".

**Design implication for Coach's eval:** commit a dated protocol file before the first scored run, stating hypotheses, the primary metric and its estimator, the number of fixtures and trials, exclusion rules, and the stopping rule; label every later analysis as confirmatory or exploratory; and publish a negative result if that is what the protocol yields. Record any deviation in the repository with the reason.

## 7. Gomes et al., "The Hidden Methodological Layer: Uncovering Threats to Validity in LLM-Assisted Empirical Software Engineering" (with the ACM SIGSOFT Empirical Standards)

- **URLs:** https://arxiv.org/abs/2611.20033 (cs.SE, 15 Nov 2026); https://arxiv.org/abs/2010.03525 (Ralph, ed., "Empirical Standards for Software Engineering Research", v0.2.0)
- **Read:** https://arxiv.org/html/2611.20033v1 in full; the Empirical Standards PDF. **Limitation:** the standard reference for the four validity types, Wohlin et al., *Experimentation in Software Engineering*, was not readable (the Springer link redirects to a login page). The definitions below come from this open paper's own statement of the four dimensions it attributes to Wohlin et al. (2024), and from the Standards' glossary, not from the book.

Key findings:

1. **The four dimensions.** "Among the most widely adopted are the validity dimensions articulated by Wohlin et al. ... : construct validity, internal validity, external validity, and conclusion validity. These dimensions provide a structured lens for reasoning about whether a study accurately captures the intended concepts, establishes credible causal relationships, generalizes beyond the studied context, and supports statistically sound conclusions". (In that order: construct, internal, external, conclusion.)
2. **Construct validity, as applied.** "construct validity is usually threatened by problems in instruments, coding schemes, measurement procedures, or operational definitions."
3. **Glossary definitions.** The Empirical Standards define "External validity: Can the conclusions be generalized to the target population? The characteristics and size of the sample should represent the extended population." and "Internal validity: Are the relationships between the investigated factors examined?"
4. **LLM-in-the-loop studies add their own threats, and mostly omit them.** "LLM-related threats remain substantially underreported", with recurring threats "including classification errors, prompt-dependent interpretation, workflow instability, evaluator bias, downstream propagation of outputs, and model dependence." On run-to-run variation: "identical inputs may yield different outputs across executions." External validity was "rarely reported" although studies "rely on specific models, versions, prompts, and configurations whose behavior may not generalize across tools, domains, datasets, or future model releases." The authors argue for "making prompts, model configurations, interaction protocols, validation procedures, and downstream reuse of LLM-generated outputs visible as part of the research method."

**Design implication for Coach's eval:** write a validity section against the four headings before the first run. Construct: refactor correctness is operationalised as hidden-test pass plus seeded-defect resolution, and the seeds define what is counted. Internal: both arms get identical prompt, model, tool permissions, budget and fixture copy, differing only in Coach. External: fixtures are seeded, not natural repositories, so claims stop at the fixtures' language, size and defect families. Conclusion: clustered, paired statistics, a pre-set number of fixtures and a stated α. Pin and publish the model ID, prompts, flags and Coach version for every run.

## 8. Aleithan et al., "SWE-Bench+: Enhanced Coding Benchmark for LLMs" (benchmark leakage; with LiveCodeBench and a contamination survey)

- **URL:** https://arxiv.org/abs/2410.06992 (submitted 9 Oct 2024)
- **Read:** https://arxiv.org/html/2410.06992v2. Also read, in part: LiveCodeBench, https://arxiv.org/html/2403.07974v2 (Jain et al.), and the survey by Xu et al., https://arxiv.org/html/2406.04244, whose definition and method taxonomy were read but whose per-method evidence was not.

Key findings:

1. **Ground truth leaked into the task.** "32.67% of the successful patches involve 'cheating' as the solutions were directly provided in the issue report or the comments. We refer to as 'solution leakage' problem."
2. **Weak tests passed wrong answers.** "31.08% of the passed patches are suspicious patches due to weak test cases, i.e., the tests were not adequate to verify the correctness of a patch."
3. **The effect on the headline number was large.** "When we filtered out these problematic issues, the resolution rate of SWE-Agent+GPT-4 drops from 12.47% to 3.97%." On the new post-cutoff set the same agent resolved 0.55%. "over 94% of the issues were created before LLM's knowledge cutoff dates, posing potential data leakage issues."
4. **Mitigation by time.** SWE-Bench+ collected "GitHub issues that were created after the training cutoff dates of the LLMs"; LiveCodeBench marks problems with release dates, so "For newer models, we can detect and avoid contamination by only evaluating on time-windows after the model's cutoff date."
5. **Definition.** The survey defines the problem as models inadvertently incorporating "evaluation benchmark information from their training data, leading to inaccurate or unreliable performance during the evaluation phase".

**Design implication for Coach's eval:** the seeded ground truth must not be readable from the fixture the agent sees (no comments, commit messages, issue text, test names or file layout that name the seeded defect or its fix), hidden tests must be able to fail a wrong-but-plausible patch (verify with at least one mutated or deliberately wrong solution per fixture, so a weak test is found before a run), and fixtures should be written fresh or from material dated after the model's training cutoff rather than lifted from public repositories the model may have seen.

---

## Sizing: how many trials and fixtures

All of the following is **derived** here from the normal-approximation formulas in sources 1 and 3 (z = 1.96 for α = 0.05 two-sided; z = 0.84 for 80% power). Miller's own figure of 969 reproduces from his formula, which is the check that the arithmetic matches his.

| Question | Inputs | Result |
| --- | --- | --- |
| Half-width of a 95% interval on one arm's pass rate, if n independent fixtures | p = 0.5 | ±10 points needs n ≈ 97; ±5 points needs n ≈ 385 |
| Same, easier tasks | p = 0.8 | ±10 points needs n ≈ 62; ±5 points needs n ≈ 246 |
| Fixtures to detect a Coach-vs-control gap, two independent arms, 80% power, α = 0.05 | p₁ = 0.5 | gap of 20 points: ≈ 91 per arm; gap of 10 points: ≈ 385 per arm |
| Same gap, paired design (same fixtures in both arms), using Miller's n = (z_α/2 + z_β)² · ω²/δ² with ω² approximated by the share q of fixtures where the arms disagree | q = 0.4, δ = 0.20 / q = 0.3, δ = 0.10 | ≈ 79 fixtures / ≈ 236 fixtures |
| pass^k at 75% per-trial rate | k = 1, 3, 5, 8, 10 | 0.75, 0.42, 0.24, 0.10, 0.056 (this matches Anthropic's (0.75)³ ≈ 42%) |

Reading it: unless the effect Coach has is very large, a few dozen fixtures cannot separate the arms, and trials per fixture cannot fix that. Extra trials per fixture reduce only the sampling part of the variance, and Miller's example reaches a ceiling of 2/3 of that part. The q values above are placeholders for the pre-registration, to be replaced by a pilot's measured disagreement rate. The cluster-adjusted power formula (Miller, Appendix C) should be used once the number of trials per fixture is fixed; it was not read here.

## What was not obtainable

- Wohlin et al., *Experimentation in Software Engineering* (Springer): login wall. Validity definitions come from source 7 as above.
- Nosek et al., "The preregistration revolution" (PNAS 2018): HTTP 403. Sources 6 cover preregistration instead.
- Miller Appendices A–C, and the body of Xu et al.'s contamination survey beyond its definition and method taxonomy.
- No source read here gives a recommended number of trials per task for agentic evals; Anthropic's post says so explicitly, and source 5 used five.
