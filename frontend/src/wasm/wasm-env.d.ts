// Ambient declarations for the globals that Go's WASM support code installs on
// `window`. This file has no imports/exports on purpose so that it stays a
// global script and its `interface Window` merges with the one in `src/env.d.ts`
// (which declares `window.aconiq`, registered by the Go module itself).

/**
 * The Go runtime shim defined by `wasm_exec.js` (copied from
 * `$(go env GOROOT)/lib/wasm/wasm_exec.js` by `just wasm-build`).
 *
 * Only the members `src/wasm/kernel.ts` relies on are declared.
 */
interface GoWasmRuntime {
  /** Imports that the Go-compiled WASM module must be instantiated with. */
  readonly importObject: WebAssembly.Imports;
  /** Command-line arguments visible to Go's `os.Args`. */
  argv: string[];
  /** Environment visible to Go's `os.Getenv`. */
  env: Record<string, string>;
  /**
   * Starts the Go program. Resolves when Go's `main()` returns — a kernel build
   * that blocks on `select{}` never resolves this, by design.
   */
  run(instance: WebAssembly.Instance): Promise<void>;
}

interface Window {
  /**
   * Optional: only defined once `wasm_exec.js` has been loaded into the page.
   * `src/wasm/kernel.ts` injects that script lazily, so callers must guard.
   */
  Go?: new () => GoWasmRuntime;
}
