export const meta = {
  name: 'implement-issue-plan',
  description: 'Read-only planner: turns a GitHub issue into a task DAG for the main session to execute',
  whenToUse: 'Invoked by the /implement-issue command. Plans only -- it never edits code, and never runs the implement/review loop, which must stay in the main session so the review and PR hooks fire.',
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
          files: { type: 'array', items: { type: 'string' }, description: 'Every file the task may touch.' },
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
          kind: { type: 'string', enum: ['false-parallelism', 'uncovered-criterion', 'unbuildable-order', 'scope-creep'] },
          detail: { type: 'string' },
        },
      },
    },
  },
}

const issue = args && args.issue ? String(args.issue) : String(args || '')

phase('Ingest')

// Three lenses, because a plan goes wrong in three different places: what the
// issue actually asks for, what the code makes possible, and what this
// repository requires of any change.
const [spec, code, conventions] = await parallel([
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
])

phase('Plan')

const plan = await agent(
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
)

phase('Self-check')

// One adversarial pass, not a loop: the executor reviews every task's diff
// anyway, so planning past the point of obvious defects buys nothing.
const audits = await parallel([
  () => agent(
    `Find tasks marked independent that are not. Two tasks conflict if they share a file, or if one consumes what the other produces.\n\nPLAN:\n${JSON.stringify(plan)}`
    + NO_MUTATION,
    { label: 'false parallelism', phase: 'Self-check', agentType: READ_ONLY, schema: AUDIT_SCHEMA },
  ),
  () => agent(
    `Find acceptance criteria no task covers, and tasks whose work no criterion justifies. Check by ID against the plan's own criteria list.\n\nPLAN:\n${JSON.stringify(plan)}`
    + NO_MUTATION,
    { label: 'coverage', phase: 'Self-check', agentType: READ_ONLY, schema: AUDIT_SCHEMA },
  ),
])

const defects = audits.filter(Boolean).flatMap((a) => a.defects || [])
if (!defects.length) {
  log('self-check found no defects in the task DAG')
  return { issue, plan, defects: [] }
}

log(`self-check found ${defects.length} defect(s); repairing`)

const repaired = await agent(
  `Repair this task DAG. Return the corrected DAG in full -- same schema, every field.\n\nPLAN:\n${JSON.stringify(plan)}\n\nDEFECTS:\n${JSON.stringify(defects)}\n\n` +
  `Fix only what the defects name. Preserve the conventions text verbatim.`
  + NO_MUTATION,
  { label: 'repair', phase: 'Self-check', agentType: READ_ONLY, schema: PLAN_SCHEMA },
)

return { issue, plan: repaired || plan, defects }
