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
export declare function isTypeOnlyImportClause(clause: astns.ImportClause | undefined): boolean;
/**
 * Export-declaration counterpart of isTypeOnlyImportClause: the
 * clause-level `export type ... from`/`node.isTypeOnly`, or a named
 * export list whose every specifier carries an inline `type` modifier
 * (`export { type T } from "./x"`).
 */
export declare function isTypeOnlyExportDeclaration(node: astns.ExportDeclaration): boolean;
//# sourceMappingURL=type-only.d.ts.map