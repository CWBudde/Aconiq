// The Aconiq WASM kernel, running off the UI thread.
//
// It used to run on it. `cmd/wasm/main.go` builds its results inside
// `new Promise(executor)`, and a Promise executor runs *synchronously*, so
// `await kernel.rls19Road(...)` never yielded: the whole RLS-19 computation
// happened before the microtask queue was reached and the tab froze for its
// duration. Nothing on the Go side can fix that — a WASM module has one stack
// and no way to hand control back mid-call — so the fix is to give it a thread
// of its own, which is this file.
//
// It is a **module** worker. That is what makes `import()` available, and
// `import()` is how `public/wasm_exec.js` gets loaded: the file is a
// side-effecting IIFE that assigns `globalThis.Go` and imports and exports
// nothing, so evaluating it as a module does exactly what evaluating it as a
// classic script does. `vite.config.ts` sets `worker: { format: "es" }` to
// match, and `spawn-worker.ts` passes `{ type: "module" }`.
//
// What this file is not: a second implementation of anything. It marshals
// arguments in, calls the same Go exports `kernel-node.ts` calls, and marshals
// results out. Every number is still decided by the Go module.

import { serializeKernelError } from "./kernel-error";
import type {
  KernelCall,
  KernelProgressListener,
  WorkerMessage,
} from "./protocol";

/**
 * What `backend/cmd/wasm/main.go` registers on the worker's global object.
 *
 * The same shape `src/env.d.ts` declares on `window`, restated here because
 * that declaration is `interface Window` and a worker has no window. Keep the
 * two in step: a missing entry is not a type error at the call site, it is a
 * `TypeError: not a function` at runtime.
 */
interface AconiqExports {
  /**
   * The second argument is optional because the Go export distinguishes one
   * argument from two: it hands the walk a progress callback only when it is
   * given one, so a caller that does not want progress must pass a single
   * argument rather than `undefined`.
   */
  rls19Road(json: string, onProgress?: KernelProgressListener): Promise<string>;
  /**
   * One shard of a run, resolving to `{chunk, start, outputs}[]` rather than
   * a flat receiver list. Same one-or-two-argument rule as `rls19Road`.
   */
  rls19RoadShard(
    json: string,
    onProgress?: KernelProgressListener,
  ): Promise<string>;
  transform(json: string): Promise<string>;
  contours(payload: Uint8Array, json: string): Promise<string>;
  standards(): string;
  loadTerrain(data: Uint8Array, crs: string): string;
  clearTerrain(): void;
  defaultConfig(): string;
}

/**
 * The slice of `DedicatedWorkerGlobalScope` this file touches.
 *
 * Declared rather than pulled in with `/// <reference lib="webworker" />`:
 * that directive merges the worker lib into the *whole* program, and this
 * tsconfig already has `DOM`, so the two would collide on every shared global.
 * `wasm-env.d.ts` declares the Go runtime the same way, for the same reason.
 */
interface KernelWorkerScope {
  postMessage(message: WorkerMessage): void;
  addEventListener(
    type: "message",
    listener: (event: MessageEvent<KernelCall>) => void,
  ): void;
  readonly location: { readonly href: string };
  /** Installed by wasm_exec.js. `GoWasmRuntime` is global — see wasm-env.d.ts. */
  Go?: new () => GoWasmRuntime;
  /** Installed by the Go module's own `main()`. */
  aconiq?: AconiqExports;
}

const scope = self as unknown as KernelWorkerScope;

/**
 * Roughly one progress message per tenth of a second.
 *
 * Fast enough that a progress bar looks live, slow enough that a run over
 * 250 000 receivers does not spend its time in `postMessage`. The final
 * report is exempt: a bar that stops at 97 % because the last message was
 * throttled is worse than no bar.
 */
const ProgressIntervalMS = 100;

function post(message: WorkerMessage): void {
  scope.postMessage(message);
}

/**
 * Where a file in `public/` lives at runtime.
 *
 * `BASE_URL` is `/Aconiq/` for the gh-pages build and `/` for a local one, and
 * Vite substitutes it into this chunk at build time. Resolving it against the
 * worker's own location rather than using it bare is what keeps that correct:
 * `importScripts`-style relative resolution inside a worker is relative to the
 * worker script, which lives under `assets/`, not to the document.
 */
function publishedAsset(name: string): string {
  return new URL(`${import.meta.env.BASE_URL}${name}`, scope.location.href)
    .href;
}

/**
 * Load wasm_exec.js, instantiate the kernel, and hand back its exports.
 *
 * If a host ever serves `public/wasm_exec.js` with a Content-Type a module
 * import refuses, the fallback is to fetch the text and evaluate it —
 * `new Function(await (await fetch(url)).text())()` inside the catch below.
 * Not implemented, because no host in use does that and an untaken fallback
 * path is an untested one.
 */
async function start(): Promise<AconiqExports> {
  const execURL = publishedAsset("wasm_exec.js");
  try {
    await import(/* @vite-ignore */ execURL);
  } catch {
    throw new Error(
      `Failed to load ${execURL}. Run \`just wasm-build\` to generate the WASM kernel.`,
    );
  }

  const GoRuntime = scope.Go;
  if (GoRuntime === undefined) {
    throw new Error(
      "wasm_exec.js was loaded but did not define Go. Re-run `just wasm-build` to regenerate a matching wasm_exec.js.",
    );
  }

  const go = new GoRuntime();
  const wasmURL = publishedAsset("aconiq.wasm");

  let module: WebAssembly.WebAssemblyInstantiatedSource;
  try {
    module = await WebAssembly.instantiateStreaming(
      fetch(wasmURL),
      go.importObject,
    );
  } catch {
    throw new Error(
      `Failed to load ${wasmURL}. Run \`just wasm-build\` to generate the WASM kernel.`,
    );
  }

  // Fire-and-forget, as on the main thread: Go's main() ends in select{} so
  // this promise never resolves, and every export is registered synchronously
  // before the scheduler first yields.
  void go.run(module.instance);

  const exports = scope.aconiq;
  if (exports === undefined) {
    throw new Error(
      "WASM kernel not initialised: the Go module ran but registered no aconiq global. Make sure public/aconiq.wasm is the current build (`just wasm-build`).",
    );
  }

  return exports;
}

const ready = start();

// The handshake. `standards` and `defaultConfig` are synchronous Go exports
// that never change for the life of a worker, so they ride along and the
// client answers both from a cache with no round trip.
void ready.then(
  (exports) => {
    try {
      post({
        kind: "ready",
        standards: exports.standards(),
        defaultConfig: exports.defaultConfig(),
      });
    } catch (cause) {
      post({ kind: "fatal", error: serializeKernelError(cause) });
    }
  },
  (cause: unknown) => {
    post({ kind: "fatal", error: serializeKernelError(cause) });
  },
);

/** A progress callback for one call, rate-limited to {@link ProgressIntervalMS}. */
function throttledProgress(id: number): KernelProgressListener {
  let lastSentAt = 0;
  return (done, total) => {
    const now = Date.now();
    if (done < total && now - lastSentAt < ProgressIntervalMS) return;
    lastSentAt = now;
    post({ kind: "progress", id, done, total });
  };
}

async function dispatch(call: KernelCall): Promise<string | null> {
  const exports = await ready;

  switch (call.method) {
    case "rls19Road":
      return call.progress
        ? exports.rls19Road(call.json, throttledProgress(call.id))
        : exports.rls19Road(call.json);
    case "rls19RoadShard":
      return call.progress
        ? exports.rls19RoadShard(call.json, throttledProgress(call.id))
        : exports.rls19RoadShard(call.json);
    case "transform":
      return exports.transform(call.json);
    case "contours":
      // The bytes arrived transferred, so this view owns them outright; Go
      // copies them in with js.CopyBytesToGo either way.
      return exports.contours(new Uint8Array(call.payload), call.json);
    case "loadTerrain":
      return exports.loadTerrain(new Uint8Array(call.payload), call.crs);
    case "clearTerrain":
      exports.clearTerrain();
      return null;
  }
}

scope.addEventListener("message", (event) => {
  const call = event.data;
  void dispatch(call).then(
    (value) => {
      post({ kind: "result", id: call.id, ok: true, value });
    },
    (cause: unknown) => {
      // Go rejects with a bare string, and several of those strings are the
      // exact sentences `aconiq run` prints. They cross as text and become an
      // Error again on the client — see kernel-error.ts.
      post({
        kind: "result",
        id: call.id,
        ok: false,
        error: serializeKernelError(cause),
      });
    },
  );
});
