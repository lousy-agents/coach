import * as astns from "typescript/unstable/ast";

/**
 * Reports whether clause makes its whole import declaration type-only: the
 * clause-level `import type ...` phase modifier, or a pure named-import
 * list (no default binding) whose every specifier carries an inline
 * `type` modifier (`import { type T, type U } from "./x"`). A default
 * import (`clause.name`) is always a value binding, so its presence forces
 * a value `import` edge regardless of any inline-typed named specifiers
 * alongside it.
 */
export function isTypeOnlyImportClause(clause: astns.ImportClause | undefined): boolean {
  if (!clause) return false;
  if (clause.phaseModifier === astns.SyntaxKind.TypeKeyword) return true;
  if (clause.name) return false;
  const namedBindings = clause.namedBindings;
  if (namedBindings && astns.isNamedImports(namedBindings)) {
    return namedBindings.elements.length > 0 && namedBindings.elements.every((e) => e.isTypeOnly);
  }
  return false;
}

/**
 * Export-declaration counterpart of isTypeOnlyImportClause: the
 * clause-level `export type ... from`/`node.isTypeOnly`, or a named
 * export list whose every specifier carries an inline `type` modifier
 * (`export { type T } from "./x"`).
 */
export function isTypeOnlyExportDeclaration(node: astns.ExportDeclaration): boolean {
  if (node.isTypeOnly) return true;
  const exportClause = node.exportClause;
  if (exportClause && astns.isNamedExports(exportClause)) {
    return exportClause.elements.length > 0 && exportClause.elements.every((e) => e.isTypeOnly);
  }
  return false;
}
