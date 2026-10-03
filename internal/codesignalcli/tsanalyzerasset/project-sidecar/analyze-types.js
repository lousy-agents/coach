/** Thrown only for genuine backend-startup failures (e.g. the bundled native tsgo binary failing to spawn); main.ts turns this into a Response.Error rather than crashing the process. */
export class SidecarBackendError extends Error {
}
//# sourceMappingURL=analyze-types.js.map