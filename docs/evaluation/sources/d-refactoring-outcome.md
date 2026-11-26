# Track (d): Measuring refactoring outcomes

Researched 2026-11-25. Each row says what was read:
- d.2 and d.5 were read from their arXiv abstract pages only;
- d.3 was read from the paper's landing-page abstract and the RMTool documentation, not the paper body;
- the other rows were read from the sources named.

Quotes are verbatim. Where one is cut short, either the page was truncated or the quote was too long to give in full. Rows are cited as `d.N`, where N is the section number.

## 1. Campbell, "Cognitive Complexity: a new way of measuring understandability" (SonarSource white paper, v1.7, 29 Aug 2023)
URL: https://www.sonarsource.com/docs/CognitiveComplexity.pdf
- Motivation is set against Cyclomatic Complexity, whose "underlying mathematical model is unsatisfactory at producing a value that measures" maintainability. The new metric "breaks from the use of mathematical models."
- Three rules: "Ignore structures that allow multiple statements to be readably shorthanded"; "Increment (add one) for each break in the linear flow"; "Increment when flow-breaking structures are nested."
- It departs from cyclomatic counting on purpose: "A switch and all its cases combined incurs a single structural increment."
- Design implication: the metric is defined by these rules, not by a theory. Grade it against Coach's own implementation of the rules, not against raw numbers from other tools, and report changes per function rather than a repository total.

## 2. Muñoz Barón, Wyrich and Wagner, "An Empirical Validation of Cognitive Complexity as a Measure of Source Code Understandability" (ESEM 2020)
URL: https://arxiv.org/abs/2007.12520 (abstract only)
- "Cognitive Complexity positively correlates with comprehension time and subjective ratings of understandability."
- "The metric showed mixed results for the correlation with the correctness of comprehension tasks", and also for physiological measures.
- The authors call it "the first validated and solely code-based metric" that is "able to reflect at least some aspects of code understandability."
- Design implication: a lower score is evidence of better understandability, not proof. Use it as one signal, and never as the only measure of a refactoring's success.

## 3. Murphy, Notkin and Sullivan, "Software Reflexion Models: Bridging the Gap Between Source and High-Level Models" (FSE 1995)
URL: https://www.cs.ubc.ca/~murphy/papers/rm/fse95.html (RMTool docs: https://www.cs.ubc.ca/~murphy/software/rmtool/rms.html). Read: the landing-page abstract and the RMTool page; the paper body was not read.
- The problem: high-level models "are almost always inaccurate with respect to the system's source code."
- The method: "an engineer defines a high-level model and specifies how the model maps to the source. A tool then computes a software reflexion model that shows where the engineer's high-level model agrees with and where it differs from a model of the source."
- Three classes: convergences (edges both predicted and present), divergences (edges present but not predicted), and absences (edges predicted but missing).
- Design implication: Coach's `.lousy-agents/coach.json` rings are already a high-level model. Grade architecture conformance on two counts:
  - divergences, which should reach 0;
  - absences, which catch an agent that "fixes" a layer violation by deleting the dependency and the feature with it.

## 4. Gautam et al., "RefactorBench: Evaluating Stateful Reasoning in Language Agents Through Code" (ICLR 2025)
URL: https://arxiv.org/abs/2503.07832 (method: https://arxiv.org/html/2503.07832v1)
- "100 large handcrafted multi-file refactoring tasks", each "defined by 3 natural language instructions of varying specificity". Baseline agents end up "solving only 22% of tasks with base instructions."
- Grading checks structure, not text. Each AST test "verifies that changes have the correct broad code structure and syntax necessitated by that subtask," "without dependence on exact line match." "A generated patch is considered successful, if all of the relevant AST tests pass."
- "Reference solutions are withheld to prevent overfitting to the task" (they "do not release gold reference solutions (only the testing files)").
- Design implication: express the target structure as AST or graph predicates broken into subtests, not as a gold diff, and keep reference refactorings private.

## 5. Cordeiro, Noei and Zou, "An Empirical Study on the Code Refactoring Capability of Large Language Models" (arXiv 2411.02320, 2024)
URL: https://arxiv.org/abs/2411.02320 (abstract only)
- "StarCoder2 reduces code smells by 20.1% more than developers"
- "One-shot prompting increases the unit test pass rate by 6.15%"; "Generating five refactorings per input further increases the pass rate by 28.8%"
- The abstract reports smell reduction and test pass rate as separate measures. On smells, StarCoder2 leads, "excelling in systematic issues like Long Statement and Magic Number, while developers handle complex, context-dependent issues better."
- Design implication:
  - Report behaviour preservation beside any metric improvement, because the abstract reports them separately.
  - Report a single generation (pass@1) as the headline, because generating several refactorings per input raised the pass rate by 28.8%.

## 6. Zhong, Raghunathan and Carlini, "ImpossibleBench: Measuring LLMs' Propensity of Exploiting Test Cases" (arXiv 2510.20270, 2025)
URL: https://arxiv.org/html/2510.20270v1
- The benchmark builds tasks whose tests conflict with the specification. On those tasks the pass rate is the "cheating rate", because any pass implies a shortcut.
- It names three shortcuts: "Modify Test Cases: The model directly modifies tests despite being explicitly instructed not to"; "Overload Comparison Operators"; "Special Casing: The model special-cases the test cases to pass them."
- "Hiding tests from agents reduces cheating success rate to near zero". "Read-only access provides a middle ground." "We recommend either hiding test files entirely or restricting them to read-only access."
- Design implication: keep the behavioural oracle out of the agent's sandbox, or make it read-only. Add one or more "impossible" fixtures where Coach's target cannot be reached without breaking behaviour; a high score on those means the metric is being gamed.

## 7. Krakovna et al., "Specification gaming: the flip side of AI ingenuity" (DeepMind, 2020)
URL: https://deepmind.google/discover/blog/specification-gaming-the-flip-side-of-ai-ingenuity/
- Specification gaming "satisfies the literal specification of an objective without achieving the intended outcome."
- The cause is "misspecification of the intended task, rather than any flaw in the RL algorithm."
- Examples: the Coast Runners boat loops to collect reward instead of finishing the race, and the Lego block is flipped instead of stacked.
- Design implication: if "Coach findings go down" is the agent's only objective, deleting or hiding code satisfies it. The grader needs checks that hold the intended outcome fixed: behaviour preserved, features still present, and size kept within bounds.

## 8. Tsantalis et al., RefactoringMiner
URL: https://github.com/tsantalis/RefactoringMiner
- Detects about 107 refactoring types, including "From Fowler's book (40)" such as Extract Method and Move Class.
- Supports TypeScript and JavaScript among other languages. The README does not list Go.
- Its precision and recall figures are in documentation/accuracy.md, which was not read.
- Design implication: for TypeScript fixtures, a target can be stated as "the diff contains Extract Method out of X" rather than as exact code. Go fixtures will need Coach's own AST or graph predicates.

## Proposed grader for refactoring outcome
- **Gate 1, behaviour preserved (pass/fail).** A grader-held test suite for the fixture's public API runs against the agent's tree; it stays hidden, as ImpossibleBench recommends. Visible tests are checked by hash, and any edit to them fails the gate. Report pass@1 only (sources 5 and 6).
- **Gate 2, no degenerate solution (pass/fail).** Fail the run if any of these holds:
  - an exported symbol or entry point present in the fixture is missing or unreachable;
  - non-test lines of code fall outside a band, for example below 70% of the original;
  - new stubs, `panic("TODO")` or `any` were added;
  - code special-cases test inputs;
  - suppression comments were added.

  These checks hold the intended outcome fixed against specification gaming (sources 6 and 7).
- **The target is a set of properties, not a diff.** Each fixture ships 3 to 8 private predicates, scored as RefactorBench-style subtests. Examples: "function F has cognitive complexity ≤ N"; "no edge from package `usecases` to `adapters`"; "the check and the act on file X are one atomic call"; "no caller mutates its argument." The score is the fraction of predicates met, and is counted only when both gates pass (source 4).
- **Architecture conformance as reflexion classes.** Count divergences from `coach.json` before and after, with a target of 0, and count absences of required edges separately, so that removing a feature to remove an edge is caught (source 3).
- **Metric changes are supporting evidence only.** Report the change in cognitive complexity, nesting depth and branch density per function, as Coach computes them. Use them to rank runs that already pass, never to decide pass or fail, because the metric's validity is only partial (sources 1 and 2).
- **Calibrate against gaming.** Include "impossible" fixtures, where the target cannot be reached without changing behaviour, and a do-nothing baseline run. A good grader gives both near-zero credit (source 6).
- **Name the refactoring where tooling allows.** On TypeScript fixtures, run RefactoringMiner to confirm the intended kind of change (Extract or Move). Record the result as a diagnostic, not a gate (source 8).
- **Report the axes together.** Show the predicate score, both gates and the metric changes side by side, because studies show smell reduction and correctness can move in different directions (source 5).
