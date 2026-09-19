/** Set by the root-scope-missing mode to the one root computeRootScopes must
 * omit from its response; undefined leaves computeRootScopes unaffected. */
export declare let testHookDropRoot: string | undefined;
/** The post-preflight failure/degrade modes AC-RUN-9's controls provoke. */
export declare function applyTestHook(argv: readonly string[]): Promise<void>;
export declare function readTestDelayHook(): number | undefined;
//# sourceMappingURL=test-hook.d.ts.map