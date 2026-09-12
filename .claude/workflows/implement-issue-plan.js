export const meta = {
  name: 'implement-issue-plan',
  description: 'Read-only planner: turns a GitHub issue into a task DAG for the main session to execute',
  whenToUse: 'Invoked by the /implement-issue command. Plans only -- it never edits code, and never runs the implement/review loop, which must stay in the main session so the review-fidelity hooks fire.',
  phases: [
    { title: 'Ingest', detail: 'read the issue, its linked specs, and the code it touches' },
    { title: 'Plan', detail: 'decompose into a task DAG with per-task acceptance criteria' },
    { title: 'Self-check', detail: 'adversarially probe the DAG for false parallelism and uncovered criteria' },
  ],
}

// `Explore` carries no Edit, Write, or NotebookEdit tool. It does carry Bash,
// so "cannot mutate" is not something the agent type guarantees on its own --
// every prompt below therefore states the prohibition explicitly.
//
// This matters more than ordinary tidiness: the review gate runs in the main
// session against the tasks this workflow hands back, so anything the planner
// changed itself would reach the branch without any reviewer ever seeing it.
//
// `Explore` is also one of the built-in agent types that do NOT get CLAUDE.md
// injected at start. That is why the conventions are read by an ingest agent
// and passed through the prompt rather than assumed: an agent here knows only
// what its prompt carries.
const READ_ONLY = 'Explore'

// Appended to every planner prompt. Bash is in reach; this says not to use it
// for anything that writes.
const NO_MUTATION = '\n\nYou are planning, not implementing. Do not modify, create, or delete any file, ' +
  'and do not run any command that writes to the repository, the index, or the working tree. ' +
  'Read and report only.'

const CRITERION = {
  type: 'object',
  required: ['id', 'text'],
  additionalProperties: false,
  properties: {
    id: { type: 'string', description: 'Stable ID, e.g. AC-1. Referenced by tasks and by the PR evidence table.' },
    text: { type: 'string' },
  },
}

const PLAN_SCHEMA = {
  type: 'object',
  required: ['acceptanceCriteria', 'tasks', 'conventions'],
  additionalProperties: false,
  properties: {
    acceptanceCriteria: { type: 'array', minItems: 1, items: CRITERION },
    conventions: {
      type: 'string',
      minLength: 1,
      description: 'Verbatim conventions and validation commands an implementer needs. Subagents share no context with the orchestrator, so anything omitted here is unavailable to them.',
    },
    tasks: {
      type: 'array',
      minItems: 1,
      items: {
        type: 'object',
        required: ['id', 'title', 'files', 'criteriaIds', 'dependsOn', 'acceptanceTest'],
        additionalProperties: false,
        properties: {
          id: { type: 'string' },
          title: { type: 'string' },
          // minItems guards the independence rule below: two tasks that each
          // name no file share no file, so both read as parallelizable.
          files: { type: 'array', minItems: 1, items: { type: 'string' }, description: 'Every file the task may touch.' },
          criteriaIds: { type: 'array', minItems: 1, items: { type: 'string' } },
          dependsOn: { type: 'array', items: { type: 'string' } },
          acceptanceTest: {
            type: 'string',
            description: 'The externally observable behavior whose absence the implementer must demonstrate as a failing test first.',
          },
        },
      },
    },
  },
}

const AUDIT_SCHEMA = {
  type: 'object',
  required: ['defects'],
  additionalProperties: false,
  properties: {
    defects: {
      type: 'array',
      items: {
        type: 'object',
        required: ['kind', 'detail'],
        additionalProperties: false,
        properties: {
          kind: { type: 'string', enum: ['false-parallelism', 'uncovered-criterion', 'unbuildable-order', 'scope-creep', 'unscoped-task'] },
          detail: { type: 'string' },
        },
      },
    },
  },
}

// Resolve the issue reference before spending a single agent turn. Without
// this, `{}` interpolates as "#[object Object]" and a missing args as "#",
// and the ingest agents research a nonexistent issue rather than failing.
const issue = String((args && args.issue) || args || '').trim()
if (!/^\d+$/.test(issue)) {
  throw new Error(
    `implement-issue-plan needs a numeric issue reference; got ${JSON.stringify(args)}. ` +
    'Call it as: Workflow({name: "implement-issue-plan", args: {issue: "248"}}).')
}

// mustHave turns a failed agent() into a stop. agent() resolves to null when
// the subagent dies on a terminal API error, when the operator stops it
// mid-run, and when auto mode's classifier blocks it before it starts -- none
// of which raise. Every downstream consumer here reads its input as if the
// agent succeeded, so a silent null becomes a confident report about nothing;
// an ingest null is worse than a plan null, because it stringifies into the
// planning prompt as the word "null" instead of stopping anything.
function mustHave(value, what) {
  const unusable = value === null || value === undefined ||
    (typeof value === 'string' && value.trim() === '')
  if (unusable) {
    throw new Error(`implement-issue-plan: the ${what} agent returned no usable result; aborting rather than planning from nothing.`)
  }
  return value
}

// auditDefects reads a self-check round. An auditor that failed found nothing
// because it never ran; reporting that as a clean bill of health is the same
// error as reporting a null plan as a good one, so refuse instead.
function auditDefects(results, what) {
  const found = []
  for (const result of results) {
    if (!result || !Array.isArray(result.defects)) {
      throw new Error(`implement-issue-plan: a ${what} auditor returned no usable result, so the plan is unaudited. Aborting rather than reporting it clean.`)
    }
    found.push(...result.defects)
  }
  return found
}

// checkGraph reports the defects that are decidable from the plan alone. They
// belong in code rather than in an auditor prompt for two reasons: an LLM asked
// to do set subtraction over IDs guesses where a loop is exact, and several of
// these do not fail the executor -- they hang it, whose rule is that a task may
// not start until everything in its dependsOn is COMPLETE. A cycle leaves no
// task eligible; a dangling reference never completes.
function checkGraph(candidate) {
  const tasks = (candidate && candidate.tasks) || []
  const allCriteria = ((candidate && candidate.acceptanceCriteria) || []).map((c) => c.id)
  const ids = new Set(tasks.map((t) => t.id))
  const criteria = new Set(allCriteria)
  const found = []

  // The executor keys task state by id, so a repeated id is not a cosmetic
  // duplicate -- two tasks collide on one status and one of them is never run.
  const duplicates = (values) => [...new Set(values.filter((v, i) => values.indexOf(v) !== i))]
  for (const id of duplicates(tasks.map((t) => t.id))) {
    found.push({ kind: 'unbuildable-order', detail: `duplicate task id ${id}: the executor tracks tasks by id and cannot hold two` })
  }
  for (const id of duplicates(allCriteria)) {
    found.push({ kind: 'uncovered-criterion', detail: `duplicate acceptance criterion id ${id}: a task citing it names two different requirements` })
  }

  const cited = new Set()
  for (const task of tasks) {
    for (const dep of task.dependsOn || []) {
      if (!ids.has(dep)) {
        found.push({ kind: 'unbuildable-order', detail: `task ${task.id} depends on ${dep}, which no task defines` })
      }
    }
    for (const cid of task.criteriaIds || []) {
      cited.add(cid)
      if (!criteria.has(cid)) {
        found.push({ kind: 'uncovered-criterion', detail: `task ${task.id} cites ${cid}, which no acceptance criterion defines` })
      }
    }
    // A task naming no file is outside the independence rule the plan is built
    // on: two such tasks share no file, so both read as safely parallel.
    if (!(task.files || []).length) {
      found.push({ kind: 'unscoped-task', detail: `task ${task.id} names no file, so nothing constrains what it may touch or what it conflicts with` })
    }
  }

  // The invariant the PR evidence table rests on: every criterion reaches a
  // task. The coverage auditor is asked this too; only this loop is exact.
  for (const cid of criteria) {
    if (!cited.has(cid)) {
      found.push({ kind: 'uncovered-criterion', detail: `acceptance criterion ${cid} is covered by no task` })
    }
  }

  // Iteratively strip tasks whose dependencies are all satisfied; whatever
  // cannot be stripped is in, or behind, a cycle.
  const pending = new Map(tasks.map((t) => [t.id, (t.dependsOn || []).filter((d) => ids.has(d))]))
  let progressed = true
  while (progressed) {
    progressed = false
    for (const [id, deps] of pending) {
      if (deps.every((d) => !pending.has(d))) {
        pending.delete(id)
        progressed = true
      }
    }
  }
  if (pending.size) {
    found.push({ kind: 'unbuildable-order', detail: `dependency cycle among tasks: ${[...pending.keys()].join(', ')}` })
  }
  return found
}

phase('Ingest')

// Three lenses, because a plan goes wrong in three different places: what the
// issue actually asks for, what the code makes possible, and what this
// repository requires of any change. `parallel` is the right shape here even
// though pipeline() is the default: planning is one barrier that needs all
// three at once, not three independent chains.
const INGEST_LENSES = ['issue-reading', 'code-mapping', 'conventions']
const [spec, code, conventions] = (await parallel([
  () => agent(
    `Read GitHub issue #${issue} and every spec or issue it links. Return its explicit acceptance criteria verbatim, each with a stable ID (AC-1, AC-2, ...). ` +
    `Where a criterion is ambiguous, record the ambiguity and the most defensible reading rather than resolving it silently. ` +
    `Do not design an implementation.`
    + NO_MUTATION,
    { label: `issue #${issue}`, phase: 'Ingest', agentType: READ_ONLY },
  ),
  () => agent(
    `For GitHub issue #${issue}, map the code that a fix must touch: the packages, files, and existing tests. ` +
    `Report what exists, not what should be built. Note any file that several plausible tasks would all need to edit -- that is what makes tasks non-parallelizable.`
    + NO_MUTATION,
    { label: 'affected code', phase: 'Ingest', agentType: READ_ONLY },
  ),
  () => agent(
    `Read AGENTS.md and any nested AGENTS.md or package-level convention docs. Return, verbatim, the conventions and validation commands an implementer must follow: ` +
    `the acceptance-test-first policy and its required Ginkgo form, the architecture dependency rules, the outbound-HTTP and fail-closed policies, the Go comment policy, and the exact validation commands. ` +
    `Quote them; do not paraphrase or soften them.`
    + NO_MUTATION,
    { label: 'conventions', phase: 'Ingest', agentType: READ_ONLY },
  ),
])).map((value, i) => mustHave(value, INGEST_LENSES[i]))

phase('Plan')

const plan = mustHave(await agent(
  `Decompose GitHub issue #${issue} into a task DAG.\n\n` +
  `ACCEPTANCE CRITERIA:\n${spec}\n\nAFFECTED CODE:\n${code}\n\nCONVENTIONS (pass through verbatim):\n${conventions}\n\n` +
  `Rules:\n` +
  `- Every acceptance criterion must be covered by at least one task.\n` +
  `- Two tasks may only be independent (neither in the other's dependsOn) if they share no file and neither consumes the other's output.\n` +
  `- Each task states the externally observable behavior an implementer must first demonstrate as a FAILING test, per the acceptance-test-first policy.\n` +
  `- Scope each task to what the issue asks for. Do not add adjacent improvements.\n` +
  `- The 'conventions' field is copied verbatim into implementer prompts, which share no context with anyone. Omitting something makes it unavailable.`
  + NO_MUTATION,
  { label: 'task DAG', phase: 'Plan', agentType: READ_ONLY, schema: PLAN_SCHEMA },
), 'planning')

// The conventions string is the one field with no recoverable failure: step 2
// of /implement-issue copies it verbatim into every implementer prompt, and
// those agents share no other context, so a blank one silently strips the
// acceptance-test-first policy from the run rather than breaking it.
if (!String(plan.conventions || '').trim()) {
  throw new Error(
    'implement-issue-plan: the plan carries no conventions text. Implementers share no context with the orchestrator, ' +
    'so shipping this would run the whole issue with the acceptance-test-first policy silently absent. Aborting.')
}

phase('Self-check')

// Two lenses over the same plan, so this is a barrier by nature: the repair
// needs every defect at once. They are the only two the executor cannot
// recover from on its own -- everything else its per-task review still catches.
const auditRound = (candidate, pass) => parallel([
  () => agent(
    `Find tasks marked independent that are not. Two tasks conflict if they share a file, or if one consumes what the other produces.\n\nPLAN:\n${JSON.stringify(candidate)}`
    + NO_MUTATION,
    { label: `false parallelism${pass}`, phase: 'Self-check', agentType: READ_ONLY, schema: AUDIT_SCHEMA },
  ),
  () => agent(
    `Find acceptance criteria no task covers, and tasks whose work no criterion justifies. Check by ID against the plan's own criteria list.\n\nPLAN:\n${JSON.stringify(candidate)}`
    + NO_MUTATION,
    { label: `coverage${pass}`, phase: 'Self-check', agentType: READ_ONLY, schema: AUDIT_SCHEMA },
  ),
])

const defects = [...checkGraph(plan), ...auditDefects(await auditRound(plan, ''), 'self-check')]
if (!defects.length) {
  log('self-check found no defects in the task DAG')
  return { issue, plan, defects: [], residualDefects: [], repairApplied: false }
}

// One repair pass, then a re-audit -- deliberately not a loop, and said out
// loud so a clean-looking result is not read as more convergence than it is.
log(`self-check found ${defects.length} defect(s); one repair pass follows, then a re-audit -- the executor reviews every task diff again regardless`)

const repaired = mustHave(await agent(
  `Repair this task DAG. Return the corrected DAG in full -- same schema, every field.\n\nPLAN:\n${JSON.stringify(plan)}\n\nDEFECTS:\n${JSON.stringify(defects)}\n\n` +
  `Fix only what the defects name. Preserve the conventions text verbatim.`
  + NO_MUTATION,
  { label: 'repair', phase: 'Self-check', agentType: READ_ONLY, schema: PLAN_SCHEMA },
), 'repair')

// The repair is verified, not trusted: it is the same kind of agent that
// produced the defects. What checkGraph finds in it splits two ways.
//
// An `unbuildable-order` defect is fatal. It does not fail the executor -- it
// hangs it, waiting on a task that can never complete, with nothing to report.
// Everything else checkGraph decides only degrades the plan: the executor still
// runs it, and its per-task review still fires. Discarding a runnable plan over
// one of those would cost a whole planning round and hand the operator nothing.
const recheckedGraph = checkGraph(repaired)
const deadlocks = recheckedGraph.filter((d) => d.kind === 'unbuildable-order')
if (deadlocks.length) {
  throw new Error(
    `implement-issue-plan: the repaired DAG would deadlock the executor, which waits for every dependsOn to complete: ${JSON.stringify(deadlocks)}`)
}

// The semantic lenses run again too. Checking only the graph would let a repair
// that fixed a cycle while introducing false parallelism return as clean.
// Anything left is reported rather than looped on, so the executor knows which
// tasks to be skeptical of instead of inheriting a silent pass.
const residualDefects = [
  ...recheckedGraph.filter((d) => d.kind !== 'unbuildable-order'),
  ...auditDefects(await auditRound(repaired, ' (recheck)'), 'repair re-check'),
]
if (residualDefects.length) {
  log(`repair left ${residualDefects.length} unresolved defect(s); reporting them for the executor rather than repairing again`)
} else {
  log('repair re-check found no remaining defects')
}

return { issue, plan: repaired, defects, residualDefects, repairApplied: true }
