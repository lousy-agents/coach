/**
 * Describes err for inclusion in a Response error message without ever
 * embedding a filesystem path (coach#326 Task 3): Node's own Error.message
 * for fs/module-resolution failures (ENOENT, EACCES, ERR_MODULE_NOT_FOUND,
 * ...) always embeds the absolute path it failed on — which here can be
 * the resolved compiler's on-disk location, its matching native package,
 * or this process's private materialized analyzer temp dir, none of which
 * may reach a serialized report.
 */
export function describeErrorWithoutPaths(err: unknown): string {
  if (!(err instanceof Error)) return typeof err === "string" ? err : Object.prototype.toString.call(err);
  const code = (err as NodeJS.ErrnoException).code;
  return code ? `${err.name} (${code})` : err.name;
}
