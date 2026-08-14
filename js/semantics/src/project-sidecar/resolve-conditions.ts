import { isRecord } from "./vfs.js";

const CONDITION_PRIORITY = ["import", "require", "node", "default"] as const;

export function resolveConditions(value: unknown): string | undefined {
  if (typeof value === "string") return stripLeadingDot(value);
  if (!isRecord(value)) return undefined;
  for (const condition of CONDITION_PRIORITY) {
    const nested = value[condition];
    if (typeof nested === "string") return stripLeadingDot(nested);
    const deeper = nestedRecord(nested);
    if (deeper !== undefined) return deeper;
  }
  return undefined;
}

function nestedRecord(nested: unknown): string | undefined {
  if (!isRecord(nested)) return undefined;
  return resolveConditions(nested);
}

export function stripLeadingDot(p: string): string {
  return p.replace(/^\.\//, "");
}
