/**
 * Test-only affordances of the sidecar process, kept in one module so a
 * reader can see every hook that can alter its behavior. Coach never passes
 * the flag and scrubs the environment before spawning (AC-RUN-4); the
 * failure modes exist so AC-RUN-9's controls can provoke them.
 */
const TEST_HOOK_FLAG_PREFIX = "--coach-test-hook=";
const TEST_HOOK_CRASH_PARTWAY = "crash-partway";
const TEST_HOOK_HANG = "hang";

/** Long enough that the caller's wall-clock budget always expires first. */
const TEST_HOOK_HANG_MS = 600_000;

/** The two post-preflight failure modes AC-RUN-9's controls provoke. */
export async function applyTestHook(argv: readonly string[]): Promise<void> {
  const flag = argv.find((a) => a.startsWith(TEST_HOOK_FLAG_PREFIX));
  if (!flag) return;
  switch (flag.slice(TEST_HOOK_FLAG_PREFIX.length)) {
    case TEST_HOOK_CRASH_PARTWAY:
      setImmediate(() => {
        throw new Error("coach-ts-project-sidecar test hook: crashing partway through analysis");
      });
      await sleep(TEST_HOOK_HANG_MS);
      return;
    case TEST_HOOK_HANG:
      await sleep(TEST_HOOK_HANG_MS);
      return;
    default:
      return;
  }
}

export function readTestDelayHook(): number | undefined {
  const raw = process.env.COACH_TS_SIDECAR_TEST_DELAY_MS;
  if (!raw) return undefined;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
