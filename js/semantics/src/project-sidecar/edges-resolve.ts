import type { Project } from "typescript/unstable/sync";
import type * as astns from "typescript/unstable/ast";

import {
  RESOLUTION_EXTERNAL,
  RESOLUTION_SNAPSHOT,
  RESOLUTION_UNRESOLVED,
} from "./protocol.js";
import { resolveSpecifier } from "./resolve.js";
import type { ProjectSnapshot } from "./vfs.js";

export function resolveTarget(
  project: Project,
  specifierNode: astns.StringLiteral,
  specifierText: string,
  snapshot: ProjectSnapshot,
  fromVirtualFsPath: string,
  useChecker: boolean,
): { to: string; resolution: string } {
  if (useChecker) {
    const viaChecker = resolveViaChecker(project, specifierNode, snapshot);
    if (viaChecker) return viaChecker;
  }
  return resolveViaManual(specifierText, snapshot, fromVirtualFsPath);
}

function resolveViaChecker(
  project: Project,
  specifierNode: astns.StringLiteral,
  snapshot: ProjectSnapshot,
): { to: string; resolution: string } | undefined {
  const symbol = project.checker.getSymbolAtLocation(specifierNode);
  const declPath = symbol?.declarations?.[0]?.path;
  if (!declPath) return undefined;
  const repoPath = snapshot.toRepoPath(declPath);
  if (repoPath === undefined) return undefined;
  return { to: `file:${repoPath}`, resolution: RESOLUTION_SNAPSHOT };
}

function resolveViaManual(
  specifierText: string,
  snapshot: ProjectSnapshot,
  fromVirtualFsPath: string,
): { to: string; resolution: string } {
  const manualFrom = snapshot.canonicalizeVirtualPath(fromVirtualFsPath);
  const manual = resolveSpecifier(manualFrom, specifierText, snapshot);
  if (manual.kind === "snapshot" && manual.virtualPath) {
    const repoPath = snapshot.toRepoPath(manual.virtualPath);
    if (repoPath !== undefined) {
      return { to: `file:${repoPath}`, resolution: RESOLUTION_SNAPSHOT };
    }
  }
  if (manual.kind === "external") {
    return { to: `external:${specifierText}`, resolution: RESOLUTION_EXTERNAL };
  }
  return { to: `unresolved:${specifierText}`, resolution: RESOLUTION_UNRESOLVED };
}
