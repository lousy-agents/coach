// Functional tests for the implement-issue planner workflow.
//
// The workflow is a script the Workflow tool executes, so nothing else in this
// repository would notice a syntax error, a renamed binding, or a control-flow
// regression until a human ran /implement-issue and the tool rejected the file.
// These tests import it under a fake harness -- agent()/parallel()/phase()/log()
// are stubs -- so the failure modes a real run hits (an agent returning null, a
// caller passing the wrong args shape, a model emitting an unbuildable graph)
// are reachable offline and deterministically.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, writeFileSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(join(here, 'implement-issue-plan.js'), 'utf8')

// The workflow body uses top-level await and a top-level return, which the
// Workflow runtime supplies but a bare module does not. Wrapping it in an async
// function is what makes it importable -- and is also, on its own, a parse check.
const wrapped = join(mkdtempSync(join(tmpdir(), 'wf-')), 'wrapped.mjs')
writeFileSync(wrapped,
  `export async function run(args, phase, agent, parallel, log) {\n${source.replace(/^export const meta/m, 'const meta')}\n}\n`)
const { run } = await import(wrapped)

const PLAN = {
  acceptanceCriteria: [{ id: 'AC-1', text: 'does a thing' }],
  conventions: 'Ginkgo v2 acceptance tests required. Validate with mise run ci-all.',
  tasks: [{ id: 'T1', title: 't', files: ['a.go'], criteriaIds: ['AC-1'], dependsOn: [], acceptanceTest: 'x' }],
}
const withTasks = (tasks) => ({ ...PLAN, tasks })

// drive runs the workflow against a responder keyed by agent label, capturing
// every prompt so tests can assert on what agents were actually asked.
// deliberate asserts the run stopped by design: an intentional refusal names
// the workflow and says why, where an incidental TypeError from a missing guard
// tells an operator nothing and disappears if the crashing line changes.
function deliberate(error, expected) {
  assert.ok(error, 'expected the run to stop')
  assert.ok(!(error instanceof TypeError),
    `expected a deliberate refusal, got an incidental crash: ${error.message}`)
  assert.match(error.message, /implement-issue-plan/)
  if (expected) assert.match(error.message, expected)
}

async function drive(args, responder) {
  const prompts = [], logs = []
  const agent = async (prompt, opts) => { prompts.push({ prompt, opts }); return responder(opts, prompt) }
  const parallel = async (thunks) =>
    Promise.all(thunks.map(async (f) => { try { return await f() } catch { return null } }))
  let result, error
  try { result = await run(args, () => {}, agent, parallel, (m) => logs.push(m)) } catch (e) { error = e }
  return { result, error, prompts, logs }
}

const planning = (plan) => (opts) => {
  if (!opts?.schema) return 'ingest text'
  if (opts.label === 'task DAG') return plan
  return { defects: [] }
}

test('a well-formed request produces a plan', async () => {
  const { result, error, prompts } = await drive({ issue: '250' }, planning(PLAN))
  assert.equal(error, undefined)
  assert.equal(result.plan.tasks.length, 1)
  assert.equal(result.issue, '250')
  assert.equal(result.repairApplied, false, 'a plan that needed no repair must not claim one was applied')
  assert.deepEqual(result.defects, [])
  assert.ok(prompts[0].prompt.includes('#250'), 'the issue number reaches the ingest prompts')
})

test('every agent is read-only and told not to mutate', async () => {
  const { prompts } = await drive({ issue: '250' }, planning(PLAN))
  for (const { prompt, opts } of prompts) {
    assert.equal(opts.agentType, 'Explore')
    assert.ok(/Read and report only/.test(prompt), 'Explore carries Bash, so the prohibition must be stated')
  }
})

test('no agent in the workflow is an implementer or reviewer', async () => {
  // These must be spawned by the main session; inside a workflow their
  // SubagentStop hooks never fire.
  const { prompts } = await drive({ issue: '250' }, planning(PLAN))
  for (const { opts } of prompts) {
    assert.ok(!['task-implementer', 'task-reviewer', 'workflow-integration-reviewer'].includes(opts.agentType))
  }
})

for (const [label, args] of [
  ['an object with no issue', {}],
  ['nothing at all', undefined],
  ['the wrong key', { issueNumber: 250 }],
  ['a non-numeric issue', { issue: 'main' }],
]) {
  test(`a caller passing ${label} fails before any agent runs`, async () => {
    const { error, prompts } = await drive(args, planning(PLAN))
    deliberate(error, /numeric issue reference/)
    assert.equal(prompts.length, 0, 'no agent turn may be spent on an unresolved issue')
  })
}

test('a failed planning agent aborts instead of returning nothing', async () => {
  const { result, error, logs } = await drive({ issue: '250' },
    (opts) => (opts?.label === 'task DAG' ? null : (opts?.schema ? { defects: [] } : 'text')))
  deliberate(error, /planning agent returned no usable result/)
  assert.equal(result, undefined)
  assert.ok(!logs.some((l) => /no defects/.test(l)), 'must not report a clean self-check on a plan that does not exist')
})

test('a failed auditor is not reported as a clean self-check', async () => {
  const { error } = await drive({ issue: '250' },
    (opts) => (opts?.label === 'task DAG' ? PLAN : (opts?.schema ? null : 'text')))
  deliberate(error, /unaudited/)
})

test('a failed repair is not reported as repaired', async () => {
  const { result, error } = await drive({ issue: '250' }, (opts) => {
    if (opts?.label === 'task DAG') return PLAN
    if (opts?.label === 'false parallelism') return { defects: [{ kind: 'false-parallelism', detail: 'T1/T2 share a.go' }] }
    if (opts?.label === 'coverage') return { defects: [] }
    if (opts?.label === 'repair') return null
    return 'text'
  })
  deliberate(error, /repair agent returned no usable result/)
  assert.equal(result, undefined)
})

test('a successful repair is reported as applied', async () => {
  const { result } = await drive({ issue: '250' }, (opts) => {
    if (opts?.label === 'task DAG') return PLAN
    if (opts?.label === 'false parallelism') return { defects: [{ kind: 'false-parallelism', detail: 'x' }] }
    if (opts?.label === 'repair') return PLAN
    if (opts?.schema) return { defects: [] } // coverage, and both re-check lenses
    return 'text'
  })
  assert.equal(result.repairApplied, true)
  assert.equal(result.defects.length, 1, 'the defects are still surfaced so the executor knows where the plan was fragile')
})

// The executor may not start a task until everything in its dependsOn is
// COMPLETE, so an unbuildable graph does not fail -- it hangs.
test('a dependency cycle is caught before the executor can deadlock', async () => {
  const { result } = await drive({ issue: '250' }, planning(withTasks([
    { id: 'T1', title: 'a', files: ['a.go'], criteriaIds: ['AC-1'], dependsOn: ['T2'], acceptanceTest: 'x' },
    { id: 'T2', title: 'b', files: ['b.go'], criteriaIds: ['AC-1'], dependsOn: ['T1'], acceptanceTest: 'x' },
  ])))
  const kinds = result ? result.defects.map((d) => d.kind) : []
  assert.ok(kinds.includes('unbuildable-order'), `expected an unbuildable-order defect, got ${JSON.stringify(kinds)}`)
})

test('a dependency on a task that does not exist is caught', async () => {
  const { result } = await drive({ issue: '250' }, planning(withTasks([
    { id: 'T1', title: 'a', files: ['a.go'], criteriaIds: ['AC-1'], dependsOn: ['T7'], acceptanceTest: 'x' },
  ])))
  assert.ok(result.defects.some((d) => d.kind === 'unbuildable-order' && /T7/.test(d.detail)))
})

test('a task citing a criterion that does not exist is caught', async () => {
  const { result } = await drive({ issue: '250' }, planning(withTasks([
    { id: 'T1', title: 'a', files: ['a.go'], criteriaIds: ['AC-9'], dependsOn: [], acceptanceTest: 'x' },
  ])))
  assert.ok(result.defects.some((d) => d.kind === 'uncovered-criterion' && /AC-9/.test(d.detail)))
})

test('a long dependency chain is not mistaken for a cycle', async () => {
  const { result } = await drive({ issue: '250' }, planning(withTasks([
    { id: 'T1', title: 'a', files: ['a.go'], criteriaIds: ['AC-1'], dependsOn: [], acceptanceTest: 'x' },
    { id: 'T2', title: 'b', files: ['b.go'], criteriaIds: ['AC-1'], dependsOn: ['T1'], acceptanceTest: 'x' },
    { id: 'T3', title: 'c', files: ['c.go'], criteriaIds: ['AC-1'], dependsOn: ['T2'], acceptanceTest: 'x' },
  ])))
  assert.deepEqual(result.defects, [], 'a valid chain must not be flagged')
})

// --- Ingest integrity -------------------------------------------------------
//
// The three ingest results are interpolated into the planning prompt as text.
// A null one does not stop the run -- it stringifies, so the planner reads
// "CONVENTIONS: null" and plans confidently from nothing. `conventions` is the
// worst case: /implement-issue copies that string verbatim into every
// implementer prompt, so losing it silently strips the acceptance-test-first
// policy from agents that share no other context.

for (const label of ['issue #250', 'affected code', 'conventions']) {
  test(`a failed ${label} ingest agent aborts instead of planning from "null"`, async () => {
    const { error, prompts } = await drive({ issue: '250' }, (opts) =>
      (opts?.label === label ? null : planning(PLAN)(opts)))
    deliberate(error, /no usable result/)
    assert.ok(!prompts.some((p) => p.opts.label === 'task DAG'),
      'no planning turn may be spent on an ingest result that does not exist')
  })
}

test('an empty ingest result is refused like a missing one', async () => {
  const { error } = await drive({ issue: '250' }, (opts) =>
    (opts?.label === 'conventions' ? '   ' : planning(PLAN)(opts)))
  deliberate(error, /no usable result/)
})

test('a plan with empty conventions is refused rather than silently stripping policy', async () => {
  const { error } = await drive({ issue: '250' }, planning({ ...PLAN, conventions: '  ' }))
  deliberate(error, /conventions/)
})

// --- Structural checks the schema cannot express ----------------------------

test('duplicate task ids are caught before the executor collides on them', async () => {
  const { result } = await drive({ issue: '250' }, planning(withTasks([
    { id: 'T1', title: 'a', files: ['a.go'], criteriaIds: ['AC-1'], dependsOn: [], acceptanceTest: 'x' },
    { id: 'T1', title: 'b', files: ['b.go'], criteriaIds: ['AC-1'], dependsOn: [], acceptanceTest: 'x' },
  ])))
  assert.ok(result.defects.some((d) => /duplicate/i.test(d.detail) && /T1/.test(d.detail)),
    `expected a duplicate-id defect, got ${JSON.stringify(result.defects)}`)
})

test('duplicate acceptance criterion ids are caught', async () => {
  const { result } = await drive({ issue: '250' }, planning({
    ...PLAN,
    acceptanceCriteria: [{ id: 'AC-1', text: 'a' }, { id: 'AC-1', text: 'b' }],
  }))
  assert.ok(result.defects.some((d) => /duplicate/i.test(d.detail) && /AC-1/.test(d.detail)),
    `expected a duplicate-criterion defect, got ${JSON.stringify(result.defects)}`)
})

// Set subtraction is exact in code and a guess in a prompt, and this is the
// invariant the PR template's acceptance-criteria -> evidence table rests on.
test('an acceptance criterion no task covers is caught in code, not left to an auditor', async () => {
  const { result } = await drive({ issue: '250' }, (opts) => {
    if (!opts?.schema) return 'ingest text'
    if (opts.label === 'task DAG') {
      return { ...PLAN, acceptanceCriteria: [{ id: 'AC-1', text: 'a' }, { id: 'AC-2', text: 'b' }] }
    }
    return { defects: [] } // every auditor reports clean; only checkGraph can catch this
  })
  const defects = result ? result.defects : []
  assert.ok(defects.some((d) => d.kind === 'uncovered-criterion' && /AC-2/.test(d.detail)),
    `expected AC-2 to be reported uncovered, got ${JSON.stringify(defects)}`)
})

test('a task whose files list is empty is caught', async () => {
  const { result } = await drive({ issue: '250' }, planning(withTasks([
    { id: 'T1', title: 'a', files: [], criteriaIds: ['AC-1'], dependsOn: [], acceptanceTest: 'x' },
  ])))
  assert.ok(result.defects.some((d) => /T1/.test(d.detail) && /file/i.test(d.detail)),
    `two tasks that each name no file share no file, so both read as independent; got ${JSON.stringify(result.defects)}`)
})

// --- The repair is verified, not trusted ------------------------------------

const repairing = (repaired, recheck) => (opts) => {
  if (!opts?.schema) return 'ingest text'
  if (opts.label === 'task DAG') return PLAN
  if (opts.label === 'repair') return repaired
  // Only the false-parallelism lens speaks; the coverage lens reports clean in
  // both rounds, so a residual count is attributable to exactly one auditor.
  if (!/false parallelism/.test(opts.label)) return { defects: [] }
  if (/recheck/.test(opts.label)) return recheck(opts)
  return { defects: [{ kind: 'false-parallelism', detail: 'T1/T2 share a.go' }] }
}

test('the repair is re-audited, not only structurally re-checked', async () => {
  const { result, prompts } = await drive({ issue: '250' }, repairing(PLAN, () => ({ defects: [] })))
  const rechecks = prompts.filter((p) => /recheck/.test(p.opts.label))
  assert.equal(rechecks.length, 2, 'both self-check lenses must re-run against the repaired DAG')
  assert.ok(rechecks.every((p) => p.prompt.includes(JSON.stringify(PLAN))),
    'the re-audit must read the repaired DAG, not the original')
  assert.deepEqual(result.residualDefects, [])
  assert.equal(result.repairApplied, true)
})

test('a repair that does not fix the defect is surfaced, not reported clean', async () => {
  const { result, logs } = await drive({ issue: '250' },
    repairing(PLAN, () => ({ defects: [{ kind: 'false-parallelism', detail: 'T1/T2 still share a.go' }] })))
  assert.equal(result.repairApplied, true)
  assert.equal(result.residualDefects.length, 1,
    'a defect the repair failed to fix must reach the executor rather than vanish')
  assert.ok(logs.some((l) => /unresolved|residual|remain/i.test(l)),
    `the operator must be told the repair was incomplete; logs were ${JSON.stringify(logs)}`)
})

test('a failed repair re-check auditor is not reported as clean', async () => {
  const { error } = await drive({ issue: '250' }, repairing(PLAN, () => null))
  deliberate(error, /unaudited/)
})

// A clean run must still declare the field, so the executor can read it
// unconditionally instead of distinguishing "none" from "not checked".
test('the result always carries residualDefects', async () => {
  const { result } = await drive({ issue: '250' }, planning(PLAN))
  assert.deepEqual(result.residualDefects, [])
})

// --- What is fatal in a repaired DAG, and what is only reported --------------
//
// checkGraph decides two different things. A dependency cycle or a dangling
// dependsOn HANGS the executor -- it waits forever for a task that can never
// complete, and cannot report why -- so a repair that leaves one is fatal. A
// coverage gap or an unscoped task DEGRADES the plan instead: the executor runs
// it and its per-task review still fires. Throwing the whole plan away for the
// second kind costs more than it saves, and hands the operator nothing.

test('a repair that leaves the graph deadlocked is fatal', async () => {
  const { error } = await drive({ issue: '250' }, repairing(withTasks([
    { id: 'T1', title: 'a', files: ['a.go'], criteriaIds: ['AC-1'], dependsOn: ['T2'], acceptanceTest: 'x' },
    { id: 'T2', title: 'b', files: ['b.go'], criteriaIds: ['AC-1'], dependsOn: ['T1'], acceptanceTest: 'x' },
  ]), () => ({ defects: [] })))
  deliberate(error, /deadlock/i)
})

test('a repair that leaves a coverage gap is reported, not thrown away', async () => {
  const { result, error } = await drive({ issue: '250' }, repairing({
    ...PLAN,
    acceptanceCriteria: [{ id: 'AC-1', text: 'a' }, { id: 'AC-2', text: 'b' }],
  }, () => ({ defects: [] })))
  assert.equal(error, undefined, 'a plan the executor can still run must not be discarded')
  assert.ok(result.residualDefects.some((d) => d.kind === 'uncovered-criterion' && /AC-2/.test(d.detail)),
    `expected AC-2 reported as residual, got ${JSON.stringify(result.residualDefects)}`)
  assert.equal(result.repairApplied, true)
})
