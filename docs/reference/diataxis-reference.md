---
title: "Diátaxis Reference (agent-oriented)"
compiled: 2026-11-23
source_work: "Diátaxis"
source_author: "Daniele Procida"
source_url: "https://diataxis.fr/"
source_license: "CC BY-SA 4.0"
source_license_url: "https://creativecommons.org/licenses/by-sa/4.0/"
source_license_evidence: "https://github.com/evildmp/diataxis-documentation-framework/blob/main/LICENSE.rst"
license: "CC BY-SA 4.0"
provenance: "AI-generated paraphrase and synthesis of diataxis.fr, read through a summarizing fetch rather than verbatim; not reviewed or endorsed by the author"
secondary_sources:
  - url: "https://pasqualepillitteri.it/en/news/5528/diataxis-framework-documentation-ai"
    status: "provisional; body not reached"
changes: "Restructured and condensed for coding-agent use; sections tagged [derived] are new synthesis, not from the source"
---

# Diátaxis Reference (agent-oriented)

Provenance tags used throughout:
- [site] stated on a diataxis.fr page, paraphrased
- [article] stated in the pasqualepillitteri.it article, unverified
- [derived] synthesis written for agent use; not stated by the source

Fidelity note: diataxis.fr pages were read through a summarizing fetch, not verbatim. Re-check exact wording against the originals before quoting anything.

---

## Contents

1. Purpose and how to use this document
2. Decision procedure (start here)
3. The model: two dimensions, four needs
4. Tutorials
5. How-to guides
6. Reference
7. Explanation
8. Keeping types apart: blur, collapse, and tie-breakers
9. Quality
10. Working method
11. Applying Diátaxis to AI agents and RAG (provisional)
12. Agent checklists
13. Coverage and verification log
14. Sources

---

## 1. Purpose and how to use this document

- Diátaxis is a framework for planning, writing, structuring, and improving technical documentation. [site]
- It identifies four kinds of documentation, each matched to a distinct user need: tutorials, how-to guides, reference, and explanation. [site]
- Each type has one job. Most documentation problems come from mixing jobs within one piece. [site][derived]
- Use section 2 to classify a piece of content, section 4 to 7 to write each type, section 8 to find blur, and section 12 to review.
- Homepage framing [site, homepage]: Diátaxis calls itself practical and systematic. It gives guidance on content, style, and architecture, grounded in what documentation users need. It imposes no implementation constraints, so teams can adopt it without changing tools or build process. It calls itself lightweight, easy to grasp, and straightforward to apply. It names two beneficiaries: documentation users, and the people who create and maintain documentation. It does not mention reviewers. It describes the framework as bringing "an active principle of quality" to documentation, a way for maintainers to reason about their own work. [site]
- Start-here framing [site, start-here]: The site is a handbook or toolbox to consult when needed. A reader does not need to read all of it to begin, and the author advises against trying. The recommended first step is to apply the framework to a documentation problem, even a small one. [site]
- Belief is optional [site, start-here]: the author presents Diátaxis as pragmatic. A reader need not believe in it or study its theory, and there is no test. Take the ideas that help and leave the rest. [site]

---

## 2. Decision procedure (start here)

### 2.1 Two diagnostic questions [site, compass page]

1. **Action or cognition?** Does the content give practical steps to perform, or knowledge to think about?
2. **Acquisition or application?** Is the user studying to learn, or working to get something done?

### 2.2 Mapping the answers to a type

| Q1 answer | Q2 answer | Type |
|---|---|---|
| Action | Acquisition (study) | Tutorial |
| Action | Application (work) | How-to guide |
| Cognition (knowledge) | Application (work) | Reference |
| Cognition (knowledge) | Acquisition (study) | Explanation |

Apply the questions at the level of a whole document or an individual sentence. [site]

Flexible prompts the compass page offers for the same check [site, compass]:
- Am I writing for one purpose or the other?
- Is this text serving one purpose or the other?
- Does the user need one or the other?
- Do I want to do one or the other?

Here "purpose" means the pair of answers above (for example, study or work), so the prompts test whether one piece is serving a single purpose. [derived]

### 2.3 Quick profile of each type

| Type | Need served | User state | Documentation informs | One-line purpose [derived] |
|---|---|---|---|---|
| Tutorial | Learning | Study | Action | Teach by guided doing toward a small achievable goal |
| How-to guide | Goals | Work | Action | Help a competent user finish a specific real task |
| Reference | Information | Work | Cognition | Describe the machinery accurately and neutrally |
| Explanation | Understanding | Study | Cognition | Build context and "why" beside the work |

### 2.4 Tie-breakers when the compass is unclear [site, reference-explanation page]

- Dull, easily forgotten content, or lists of classes, methods, attributes, and tables: probably reference.
- Content you could enjoy reading away from the work: probably explanation.
- If a friend asking "can you tell me more about this?" would prompt the answer: probably explanation.
- Test: would someone consult this while doing a task, or after stepping away from the work to think? Consulted during work points to reference or how-to. Read away from the product points to explanation or tutorial.

---

## 3. The model: two dimensions, four needs

### 3.0 Why a theory at all [site, theory page]

- The "Understanding Diátaxis" section opens with an epigraph that calls Diátaxis "the Grand Unified Theory of Documentation", attributed to David Laing. [site]
- Documentation discussion is mostly practical. Advice takes the form of heuristics, rules of thumb, and principles that have been found to work. Practitioners can say what to do and how, but say little about why it works, and the theory side of the discipline gets much less attention. [site]
- The page does not dismiss heuristics or practitioner judgment. It treats them as valuable but incomplete, because they do not explain why what works works. Its stated aim is a more rigorous theoretical basis for documentation practice. [site]
- Sub-pages introduced: foundations (why it works), the map (two dimensions), quality (towards a theory of quality), and the two common-problem pages. [site]

### 3.1 The two dimensions [site, foundations and map pages]

- **Action / cognition:** Skill has a practical side (knowing how, doing) and a theoretical side (knowing that, thinking). The two are inseparable yet distinct.
- **Acquisition / application:** Practitioners both acquire a skill (study) and apply it (work). These are distinct but linked.
- Because the two dimensions define the territory, there are exactly four quadrants. The site argues this is a complete set, not an arbitrary count. [site]
- Intended audience: practitioners in a domain of skill. A craft can be a tool or product, a whole discipline or profession, or a specific activity such as flying one aircraft. [site, foundations]
- Reliability claim: the map rests on a systematic analysis of generalised user needs. Because the map covers the whole territory of craft, there is no other category of need to address. [site, foundations]

### 3.2 Needs and matching documentation [site, foundations page]

- Learning → tutorials (user acquires the craft; documentation informs action)
- Goals → how-to guides (user applies the craft; documentation informs action)
- Information → reference (user applies the craft; documentation informs cognition)
- Understanding → explanation (user acquires the craft; documentation informs cognition)

### 3.3 The map and its neighbours [site, map page]

- The map is a two-dimensional structure, not a list. Each type occupies its own region, and the boundaries show how types differ.
- Adjacent quadrants share a dimension, which is why neighbours blur. The named pairs are tutorial with how-to guide, and reference with explanation. [site]
- The user's cycle around the map runs tutorial → how-to guide → reference → explanation → back to tutorial. [site]
- Users do not follow a strict reading order. They can enter anywhere and move between types as needs change. [site]
- Learning cycle: learn by doing (tutorial), apply to work (how-to), consult facts when knowledge is missing (reference), reflect away from the work to see the whole (explanation). The cycle then restarts on something new or deeper. [site]

### 3.4 Comparison of the four types [site, map page comparison table]

| Attribute | Tutorials | How-to guides | Reference | Explanation |
|---|---|---|---|---|
| What they do | Teach and guide newcomers | Steer the reader toward a goal | State facts and inform | Clarify and discuss ideas |
| Question answered | How do I learn to do this? | How do I accomplish this task? | What is this thing? | Why is it this way? |
| Oriented to | Learning | Goals | Information | Understanding |
| Purpose | Give a learning experience | Help reach a specific aim | Describe how things work | Shed light on a topic |
| Form | A guided lesson | Ordered steps | Plain factual description | Free-flowing discussion |
| Analogy | Teaching a child to cook | A cookbook recipe | Facts on food packaging | An essay on food history |

Blur table for guide actions, from the same map page [site]:

| Guide action | Absorbed when tutorials blur | Absorbed when how-to guides blur |
|---|---|---|
| Acquiring skill | Tutorials | Explanation |
| Applying skill | Reference | How-to guides |
| Holding factual knowledge | Reference | Explanation |

Reading the blur table: each row names a kind of guide action. The two right-hand columns show which neighbouring type tends to absorb that action when the named type's boundary breaks. [derived]

### 3.5 The problem of structure [site, map page]

- Poor structure also damages content. Organising documentation around product features leads to inconsistency across a portfolio of docs.
- Any clear set of content categories helps authors and users, but authors often find content that does not fit the scheme and may question why one scheme is preferred.
- A clear structure sets reader expectations about each piece's purpose and gives authors guidance on how to write it and where to place it.
- Each type has one distinct job, set apart from the others.

---

## 4. Tutorials

### 4.1 Definition [site, tutorials page]

A tutorial is a practical, learning-oriented experience run under a tutor's guidance. The learner builds skill and knowledge by doing something meaningful toward an achievable goal. It aims at learning, not at getting a task done.

### 4.2 Roles [site]

- The teacher carries most responsibility for what is learned, the activities chosen, and the learner's success.
- The learner's only duty is to pay attention and follow directions.
- Exercises must be meaningful, achievable, logically ordered, and complete enough to cover every action, concept, and tool the learner needs.

### 4.3 Why tutorials are hard [site]

- Writers cannot correct or respond to a learner in real time, as a live teacher could.
- Tutorials take substantial effort to write and maintain, especially where the product changes quickly.
- Changes cascade through the whole learning path, unlike other docs that can be revised in isolation.
- Writers must separate what the learner needs to understand from what the learner needs to do, and design one path that covers both.

### 4.4 Principles [site, eleven principles]

1. **Don't try to teach.** Provide experiences to learn from; avoid telling and explaining.
2. **Show the destination early.** State what the learner will build; avoid promising what they will "learn".
3. **Deliver visible results early and often.** Each step should produce a result the learner can see and understand.
4. **Maintain a narrative of expectations.** Describe expected output, warn about likely failure points, prepare for surprises.
5. **Point out what to notice.** Learners are usually too focused to notice environmental changes on their own.
6. **Target the feeling of doing.** Tie purpose to action so the task is satisfying and flows.
7. **Encourage repetition.** Let learners repeat successful steps; where operations are irreversible, find ways to allow it anyway.
8. **Minimise explanation.** Keep explanation brief and link to fuller discussion. Explanation belongs when the learner asks for it.
9. **Focus on the concrete.** Lead from one specific action and result to the next; general patterns emerge from concrete experience.
10. **Ignore options and alternatives.** Stay on the path; leave side paths for later.
11. **Aspire to perfect reliability.** The tutorial must work for every user every time, because the writer cannot rescue learners. Find gaps through user testing and observation.

### 4.5 Language patterns [site, plus derived framing]

- First-person plural builds a sense of shared effort ("we will build...").
- State what the learner will accomplish to set expectations.
- Clear, unambiguous, sequential instructions.
- Brief basic explanations with links to deeper material.
- Show expected output as a benchmark.
- Point out what to notice.
- Briefly acknowledge achievement to reinforce confidence.

### 4.6 Pitfalls [derived from 4.3 to 4.5]

- Drifting into how-to territory by offering choices, branches, or edge cases.
- Explanation creep: long background sections inside a lesson.
- Untested paths. A tutorial that fails for some users cannot be rescued by the writer.
- Mixing in reference-style exhaustive option lists.

---

## 5. How-to guides

### 5.1 Definition and scope [site, how-to-guides page]

- A how-to guide is a goal-oriented direction that helps a reader work through a problem or reach a result. It guides action and concerns practical work.
- Good scope: narrow, specific tasks such as calibrating a device or configuring one setting.
- Poor scope: broad, open-ended topics such as "building a web application".
- Value: a list of how-to guides signals what a product can do, and well-chosen guides tend to be the most-read documentation.

### 5.2 Orientation [site]

- Write from the user's point of view, not the system's. Define guides by human needs and projects, not by the operations a tool can perform.
- Critique: guides that walk through tool functions add little value. Instructions that ignore purpose are low-value even when technically correct.
- Better approach: explain what the user needs to decide or set for a real-world goal.
- Tools are supporting players. A guide may focus on one tool or span several, depending on the goal.

### 5.3 What how-to guides are not [site]

- Not tutorials. The two serve different needs, and mixing them causes many documentation problems.
- Not just procedures. Real problems branch, overlap, have several entry and exit points, and require the user's judgment.

### 5.4 Principles [site]

- **Stay on the goal.** Task-focused; assume the user knows what they want; contain action only.
- **Exclude** explanation and full reference material. Link to them instead.
- **Assume a competent user** who can follow instructions correctly.
- **Address real-world complexity.** No guide covers every case; make guidance adaptable so users can tailor it.
- **Prioritise practical usefulness over completeness.** A how-to guide need not be end-to-end; start and end at sensible points.
- **Provide an executable solution:** an "if this situation, then these steps" structure. Steps can be actions, including mental work such as judgment and problem-solving.
- **Describe a logical sequence.** Order follows dependencies, or one step prepares the user for the next.
- **Seek flow.** Base sequencing on how users think and work. Avoid forced context switches, long open thoughts, and unnecessary backtracking.
- **Name precisely.** Titles say exactly what the guide shows. Vague titles mislead, for example when a doc covers whether to do something rather than how. Clear titles also help search ranking.

### 5.5 Language [site]

- Open by stating the problem or task the guide solves.
- Use conditional imperatives: "if you want X, do Y".
- Point to reference for complete option lists instead of listing everything.

### 5.6 Pitfalls [site and derived]

- Walking through tool functions without stating the user's purpose.
- Including background, history, or teaching material.
- Vague or search-unfriendly titles.

---

## 6. Reference

### 6.1 Definition [site, reference page]

- Reference is a technical description of a product's machinery and how to operate it.
- It is information-oriented and serves users during their work.
- The product drives the content, not user needs. In software, reference covers APIs, classes, functions, and how to use them.
- Users need reliable, authoritative material to work with confidence.

### 6.2 Principles [site]

- **Describe, and only describe.** Neutral, objective, factual, structured like the machinery itself.
- **Austere.** Readers consult it rather than read it. Leave no room for doubt.
- **Explain how things work or how to use them correctly, but do not show how to perform tasks.**
- **Adopt standard patterns.** Consistency makes reference usable. Put information where users expect it, in familiar formats.
- **Respect the structure of the machinery.** Doc structure should mirror product structure so readers can navigate both together. Do not force an unnatural structure; the conceptual relationships in the code should guide it.
- **Provide short examples,** such as a command in use, without drifting into instruction or explanation.
- **Link, don't inline.** Link to how-to guides, explanations, and tutorials rather than including them.

### 6.3 Content [site]

- Include: facts about behaviour and the machinery, lists of commands, options, operations, features, flags, limitations, error messages, warnings, brief illustrative examples.
- Exclude: opinion, speculation, instruction, interpretation, anything not factual description.
- Language: clear, firm wording for requirements and prohibitions; warnings where they apply.

### 6.4 Pitfalls [site]

- Drifting into explanation or instruction when description feels inadequate.
- Relying on auto-generated reference alone. Generation helps keep reference accurate to the code, but it is not the whole job.
- Inconsistent or creative presentation.
- Expansive examples that grow into explanations (see 8.3).

### 6.5 Analogy [site]

Reference is like a product label: standard formats, facts only, no marketing or recipes mixed in. The site argues reference deserves the same seriousness.

---

## 7. Explanation

### 7.1 Definition and scope [site, explanation page]

- A discursive, understanding-oriented treatment of a subject that invites reflection. It adds depth, breadth, and context.
- It takes a higher, wider view than the other types. Its scope is a bounded area of knowledge.
- It answers questions of the form "can you tell me about...?"
- It is suited to reading away from the product.

### 7.2 Value and place [site]

- Explanation is less urgent than the other types because it does not directly guide daily work, but it is just as important. Practitioners need it to connect their knowledge.
- Alternative names: discussion, background, conceptual guides, topics.
- Explanation reveals what is implicit or obscured. Without it, knowledge stays fragmented and work feels anxious.
- Explanation is often missing as a distinct section; it tends to be scattered across other sections.

### 7.3 Boundaries [site]

- Explanation has no natural limits. The other types have them (learning goals, tasks, product scope).
- A real or imagined "why" question can set the boundary. Otherwise the writer draws a reasonable perimeter.

### 7.4 Writing good explanation [site]

- **Make connections** to related ideas, including ones outside the immediate scope.
- **Provide context:** why things are the way they are. Cover design decisions, history, constraints, implications, and examples.
- **Talk about the subject:** the bigger picture, history, choices and alternatives, reasons. Titles should read naturally after "About", e.g. "About user authentication".
- **Admit opinion and perspective.** Human creations involve judgment and other viewpoints exist. Consider alternatives, counterexamples, multiple approaches. Treat it as discussion, not instruction or fact list.
- **Keep it closely bounded.** No drifting into instructions or technical reference.

### 7.5 Language [site]

Typical moves: explaining reasons (including historical ones); offering judgments where suitable; analogies and context; weighing alternatives and trade-offs; showing how a system's internals produce its behaviour.

### 7.6 Analogy [site]

A cooking-history book with no recipes and no cooking steps: it places food in its historical, scientific, and social context. It is read when reflecting on cooking, not while cooking.

---

## 8. Keeping types apart: blur, collapse, and tie-breakers

### 8.1 Tutorial vs how-to guide [site, tutorials-how-to page]

- Core difference: tutorials serve a user studying to build basic competence (learning-oriented). How-to guides serve a competent user working on a specific task (task-oriented).
- Path: tutorial is one managed line with no choices; how-to guide branches into alternatives.
- Setting: tutorial is a controlled environment that removes surprises; how-to guide applies to the messy real world and must warn about the unexpected.
- Safety: tutorial must be safe so the learner can always restart; how-to guide cannot promise safety, and the user often has one chance.
- Responsibility: in a tutorial the teacher fixes the learner's problems; in a how-to guide the user gets out of trouble.
- Detail: tutorial spells out basics; how-to guide assumes them.
- Generality: tutorial is concrete with prepared tools and conditions; how-to guide must be general because real cases vary.
- Assumptions: tutorial may not anticipate what the learner does not yet know to ask; how-to guide assumes the right questions.
- Myth rejected: "tutorials are for basics, how-to guides for advanced topics" is wrong. Both can cover basic or advanced material.
- Consequence of mixing: the most common conflation in software documentation. It confuses newcomers and pushes users to other products; the site also argues it can be dangerous in a medical context.

### 8.2 Reference vs explanation [site, reference-explanation page]

- Both sit in the theory half of the map. Neither gives the reader steps. Both hold theoretical knowledge.
- Core distinction: reference supports applying knowledge (work); explanation supports acquiring it (study). This mirrors tutorial vs how-to.
- The distinction is "mostly" straightforward, but intuition is not reliable enough on its own. Use the tie-breakers in 2.4.

### 8.3 How reference and explanation blur [site]

- Reference becomes expansive: examples are appropriate in reference, but writers tend to expand them into explanations of why, what-if scenarios, or origins.
- Explanation then scatters through reference, interrupting and obscuring it.
- Blurring also harms explanation, because explanatory content never develops properly.

### 8.4 Partial vs total collapse [site, map page]

- Partial collapse: two types partly merge.
- Total collapse: tutorials and how-to guides merge completely, and neither need can be met.

### 8.5 Flow disruption [site, quality page]

Interrupting one type with another breaks the reader's purpose. The canonical example is explanation dropped into a how-to guide.

---

## 9. Quality

### 9.1 Two kinds of quality [site, quality page]

- **Functional quality:** accuracy, completeness, consistency, usefulness, precision. These are independent, objectively measurable, and must be met consistently. Users notice when any is missed.
- **Deep quality:** feels good to use, has flow, fits human needs, is beautiful, anticipates the user. It cannot be measured, but it can be recognised.
- Functional quality acts as a constraint measured against the world. Deep quality is interdependent, assessed against human needs, and depends on functional quality: it cannot exist without accuracy, completeness, and consistency.
- Analogy [site, quality]: functional quality is fit to purpose; deep quality is how the thing feels to use. Users may sense deep quality without naming it, but creators need a clear understanding of what makes documentation good.

### 9.2 What Diátaxis does and does not do [site]

- Diátaxis cannot address functional quality directly.
- It touches some aspects of deep quality, mainly fit to user needs (its categories derive from user needs) and flow.
- It offers principles, not a formula. It helps with overall form but does not make documentation beautiful; UX, interaction, and visual design are still needed.
- Using it does not guarantee deep quality. It only creates conditions in which deep quality becomes possible.

### 9.3 Diagnostic uses [site]

- Reference: structure mirrors the code, which makes gaps visible.
- Tutorials: removing explanatory material can expose places where the learner is left to work things out alone.
- The framework works analytically, revealing gaps and errors that were previously hidden.

---

## 10. Working method

### 10.1 Use as a guide, not a plan [site, how-to-use-diataxis page]

- Treat the framework as a map for checking where you are and which way you are heading, not a checklist of sections to fill.
- Do not spend effort on top-level structure first. Follow the prompts; structure emerges from improvements.
- Do not create empty sections for the four quadrants. Structure changes from the inside as material moves under the right headings.

### 10.2 Working cycle [site]

1. **Choose** any piece of documentation at hand. Do not hunt for problems; choose at random if nothing comes to mind.
2. **Assess** at a small scale (a paragraph or sentence): what user need it serves, how well, what to add, move, remove, or change, and whether its language and logic fit its type.
3. **Decide** on the single next action that gives an immediate improvement.
4. **Do it**, then publish or commit. Do not add work to make it "worthy".
5. Return to step 1.

This cycle removes the difficulty of deciding what to do next without a plan.

### 10.3 Principles of pace [site]

- Work one small step at a time, even on poor docs. Publish each improvement right away rather than batching.
- Good structure grows from healthy internal parts, as in a living organism, rather than being imposed from above.
- Documentation is never finished, but at every stage it can be complete: useful, suited to its stage, structurally healthy, and ready for the next stage.

### 10.3a Start-here loop (four steps) [site, start-here]

The start-here page gives a shorter loop than the working cycle in 10.2:
1. Look at the documentation in front of you, or at its absence if you have not started.
2. Ask whether it could be improved.
3. Choose one small change that would help.
4. Make that change, then repeat.

The two loops agree on the core: small steps, repeated. The working cycle adds an explicit "assess at small scale" step and a "publish or commit" step. [derived]

### 10.4 Pragmatism [site, start-here and application pages]

- Readers do not need to adopt the whole approach, believe in it, or study its theory. Take what proves useful.
- Understanding comes from applying it. Alternate between doing the work and reflecting on it.
- The site gives no separate step-by-step for adopting the framework in an existing doc set versus a new one. [site]

---

## 11. Applying Diátaxis to AI agents and RAG (provisional)

### 11.1 What the article is and what it claims [article, provisional]

- Title: "Diátaxis: The 4 Types of Technical Documentation and Their Use with AI" by Pasquale Pillitteri. A tag page lists it with a date of 19 June 2026.
- Thesis: the framework's structure makes documentation easier for AI assistants and RAG systems to digest, and it is suited to RAG and AI agents. [article]
- The Italian version states that a clean reference reduces hallucinations in RAG. [article, Italian version, unverified in full]
- The article credits the framework's creation to Daniele Procida in 2020. [article]
- Section headings (from the table of contents of the English version): the two axes and compass; how to use it in practice; why it matters for AI and RAG systems, with practical benefits; common mistakes; FAQ.

### 11.2 Limits of this enrichment

- The article body was not read. Fetching it hit an anti-scraper check page, which was not bypassed.
- Claims about specific recommendations are therefore unverified. Only the thesis, the TOC, and one sentence from the Italian snippet are used.
- Recheck once the body is available, or ask the user to paste the text.

### 11.3 Secondary source, lower weight [secondary, unverified]

A byteiota industry piece argues that AI agents retrieve fragments rather than whole pages, so reference should stay free of narrative. It gives an example of a tutorial hedge ("for most use cases") that should not appear in a reference fragment. Treat as opinion, not evidence.

### 11.4 Implications for agents [derived]

- Tag every chunk of documentation with its type (tutorial, how-to, reference, explanation). Typed metadata lets retrieval return a single-purpose fragment.
- Keep reference chunks free of opinion, instruction, and narrative. A retrieved reference chunk should be safe to cite as fact.
- Keep each how-to step paired with the condition it applies to, since fragments lose surrounding context.
- Route "why" questions to explanation and "what are the parameters" questions to reference. Route "how do I do X" to how-to, and "teach me" to tutorial.
- Avoid mixed chunks. A chunk that mixes steps with background is harder to retrieve and to trust.

---

## 12. Agent checklists [derived from sections 4 to 9]

### 12.1 Classify before editing
- [ ] Run the two compass questions (section 2).
- [ ] If the answers disagree across paragraphs, split the content by type.
- [ ] If unclear, apply the consultation test (section 2.4).

### 12.2 Per-type review
- Tutorial: each step yields a visible result; destination stated up front; no branching; expected output shown; no explanation longer than a brief link; works on every run.
- How-to guide: title states the task; opens with the problem; conditional "if X, do Y" structure; assumes a competent user; no background; links to reference for options.
- Reference: neutral wording; structure mirrors the code; facts, options, errors, and limits listed; no instructions or opinion; short examples only; links out to other types.
- Explanation: "About X" framing; connects to related ideas; gives context and reasons; admits alternatives; no steps; no reference tables.

### 12.3 Document-level review
- [ ] One dominant type per document (or clearly separated sections).
- [ ] Cross-type content is linked, not inlined.
- [ ] No flow breaks (for example, explanation inside a how-to guide).
- [ ] Check for partial collapse (two types merging) and total collapse (tutorials and how-to guides indistinguishable).

---

## 13. Coverage and verification log

### 13.1 Pages read (14)
start-here, foundations, map, quality, compass, application, how-to-use-diataxis, theory, tutorials, how-to-guides, reference, explanation, tutorials-how-to, reference-explanation.

### 13.2 Excluded and why
- colophon, translation, and the Polish translation (/pl/): meta or localised, no framework content.
- news: checked. Two entries (August 2026): an Atom feed and a Polish translation. No framework or English page changes.
- homepage: read as the navigation index.

### 13.3 Double-check results
- Homepage navigation and the theory page's sub-links agree, including "Towards a theory of quality" (the /quality/ page).
- The sitemap lists only the homepage, so it cannot confirm completeness. Coverage rests on the navigation and the theory and quadrant pages' own links.
- Second audit (session 2): re-captured the map comparison table (now section 3.4), the map's problem-of-structure section (3.5), the theory section's argument (3.0), foundations' audience and reliability claim (3.1), start-here's introduction and four-step loop (10.3a), the homepage's framing (section 1), and the compass's flexible prompts (section 2). These were previously missing or partial.
- The "question each type answers" column in section 2.3 is now confirmed by the map table in 3.4.
- All pages were read through summaries. Minor sections may still be condensed. Verify exact wording against originals before quoting.

### 13.4 Open items
- Article body (pasqualepillitteri.it) not read; enrichment is provisional.
- Verify exact wording against originals before quoting.

---

## 14. Sources

Primary (diataxis.fr):
- https://diataxis.fr/
- https://diataxis.fr/start-here/
- https://diataxis.fr/foundations/
- https://diataxis.fr/map/
- https://diataxis.fr/quality/
- https://diataxis.fr/compass/
- https://diataxis.fr/application/
- https://diataxis.fr/how-to-use-diataxis/
- https://diataxis.fr/theory/
- https://diataxis.fr/tutorials/
- https://diataxis.fr/how-to-guides/
- https://diataxis.fr/reference/
- https://diataxis.fr/explanation/
- https://diataxis.fr/tutorials-how-to/
- https://diataxis.fr/reference-explanation/
- https://diataxis.fr/news/ (checked; excluded)

Secondary (provisional):
- https://pasqualepillitteri.it/en/news/5528/diataxis-framework-documentation-ai (body not reached)
- https://pasqualepillitteri.it/news/5527/diataxis-framework-documentazione-ai (Italian version, snippet only)
- https://byteiota.com/diataxis-the-documentation-framework-ai-agents-need/ (opinion, unverified)
