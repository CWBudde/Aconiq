// The one `new Worker(...)` in the frontend, alone in its own module.
//
// Alone because of what the expression is: Vite rewrites `new Worker(new
// URL("./x.ts", import.meta.url))` at build time into a reference to a
// separate chunk, and that rewrite only happens for this literal shape. Any
// indirection — a variable holding the URL, a helper that takes the specifier
// — and the worker silently stops being bundled. Keeping it in a file of its
// own makes that shape impossible to refactor away by accident, and gives
// tests one module to `vi.mock`.
//
// `type: "module"` rather than a classic worker: the kernel loads
// `public/wasm_exec.js` with a dynamic `import()` of the published asset, and
// a module worker is the only kind that has one. `importScripts` would work in
// a classic worker but is not in eslint's `globals.browser`, and
// `new URL(..., import.meta.url)` resolves against the importing chunk at
// runtime, which is what keeps the `--base=/Aconiq/` gh-pages build correct.
// `vite.config.ts` sets `worker: { format: "es" }` to match.

/**
 * Start a fresh kernel worker.
 *
 * Every call makes a new one. The single-instance policy lives in
 * `kernel.ts`, which is also what decides when a dead worker is replaced.
 */
export function spawnKernelWorker(): Worker {
  return new Worker(new URL("./kernel.worker.ts", import.meta.url), {
    type: "module",
    name: "aconiq-kernel",
  });
}
