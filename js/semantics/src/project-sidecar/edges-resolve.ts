import type { Project } from "typescript/unstable/sync";
import type * as astns from "typescript/unstable/ast";

import {
  RESOLUTION_EXTERNAL,
  RESOLUTION_SNAPSHOT,
  RESOLUTION_UNRESOLVED,
} from "./protocol.js";
import { resolveSpecifier, type PathAliases } from "./resolve.js";
import type { ProjectSnapshot } from "./vfs.js";

/** A static module specifier: a string literal, or a template literal with no substitutions. */
export type SpecifierNode = astns.StringLiteral | astns.NoSubstitutionTemplateLiteral;

export function resolveTarget(
  project: Project,
  specifierNode: SpecifierNode,
  specifierText: string,
  snapshot: ProjectSnapshot,
  fromVirtualFsPath: string,
  useChecker: boolean,
): { to: string; resolution: string } {
  if (useChecker) {
    const viaChecker = resolveViaChecker(project, specifierNode, snapshot);
    if (viaChecker) return viaChecker;
  }
  return resolveViaManual(specifierText, snapshot, fromVirtualFsPath, pathAliasesOf(project, snapshot));
}

function resolveViaChecker(
  project: Project,
  specifierNode: SpecifierNode,
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
  aliases: PathAliases | undefined,
): { to: string; resolution: string } {
  const manualFrom = snapshot.canonicalizeVirtualPath(fromVirtualFsPath);
  const manual = resolveSpecifier(manualFrom, specifierText, snapshot, aliases);
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

function pathAliasesOf(project: Project, snapshot: ProjectSnapshot): PathAliases | undefined {
  const { paths } = project.compilerOptions;
  if (!paths || Object.keys(paths).length === 0) return undefined;
  return { paths, baseDirVirtual: pathsBaseDirVirtual(project, snapshot) };
}

/**
 * The native API reports `pathsBasePath` on the options at runtime without
 * declaring it in its typings; it is the directory of the config that
 * declared `paths`. Without it, fall back to the project's own config
 * directory, which is wrong only for inherited `paths` and then fails toward
 * "unresolved" rather than silently claiming an external package.
 */
function pathsBaseDirVirtual(project: Project, snapshot: ProjectSnapshot): string {
  const declared = (project.compilerOptions as { pathsBasePath?: unknown }).pathsBasePath;
  if (typeof declared === "string" && declared !== "") return declared;
  const config = snapshot.canonicalizeVirtualPath(project.configFileName);
  return config.slice(0, config.lastIndexOf("/"));
}
