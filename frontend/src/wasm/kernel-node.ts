// Test-only loader for the Aconiq WASM computation kernel.
//
// `kernel.ts` is the production loader and is browser-only by construction: it
// injects wasm_exec.js through `document.createElement`, resolves URLs against
// `import.meta.env.BASE_URL`, and instantiates via `fetch`. None of that works
// from vitest, so the parity tests need their own way in — but they must reach
// the *same* kernel, or they would be pinning a second implementation.
//
// This file therefore loads the identical artifacts `just wasm-build` produces
// and returns the identical `AconiqKernel` interface. It reads them from disk
// and evaluates wasm_exec.js with `node:vm` rather than importing it: the file
// is a side-effecting IIFE living in `public/`, outside the module graph, and
// it assigns `globalThis.Go` instead of exporting anything.
//
// Nothing in `src/` may import this module — it exists for `*.test.ts` only.

import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { runInThisContext } from "node:vm";

import type { StandardDescriptor } from "@/standards/descriptor";
import type { AconiqKernel } from "./kernel";
import { withKernelErrors } from "./kernel-error";
import type {
  ComputeRequest,
  ContourRequest,
  ContourResult,
  PropagationConfig,
  ReceiverOutput,
  TerrainInfo,
  TransformRequest,
  TransformResponse,
} from "./types";

/**
 * Set by `just fe-test-wasm` and by the frontend CI job. Without it a missing
 * kernel is a skip, which is right on a developer machine that has never run
 * `just wasm-build`. With it a missing kernel is a failure, because a parity
 * suite that silently checked nothing is the failure mode these tests exist to
 * prevent. This mirrors `acceptance.StrictSuiteEnv` on the Go side.
 */
export const RequireKernelEnv = "ACONIQ_REQUIRE_WASM";

const artifacts = {
  wasmExec: "../../public/wasm_exec.js",
  module: "../../public/aconiq.wasm",
} as const;

function artifactPath(relative: string): string {
  return fileURLToPath(new URL(relative, import.meta.url));
}

/**
 * The Go runtime shim `wasm_exec.js` installs. `wasm-env.d.ts` declares the
 * same shape on `window`; this module reads it off `globalThis` instead,
 * because `runInThisContext` assigns there and the two are not guaranteed to be
 * the same object under a jsdom test environment.
 */
interface GoRuntimeGlobal {
  Go?: new () => {
    readonly importObject: WebAssembly.Imports;
    run(instance: WebAssembly.Instance): Promise<void>;
  };
  aconiq?: {
    rls19Road: (json: string) => Promise<string>;
    transform: (json: string) => Promise<string>;
    contours: (payload: Uint8Array, json: string) => Promise<string>;
    standards: () => string;
    loadTerrain: (data: Uint8Array, crs: string) => string;
    clearTerrain: () => void;
    defaultConfig: () => string;
  };
}

/**
 * The reason the kernel cannot be loaded, or null when both artifacts are in
 * place. Both are gitignored build outputs, so their absence is the normal
 * state of a fresh checkout rather than an error.
 */
export function missingKernelReason(): string | null {
  const missing = Object.values(artifacts)
    .filter((relative) => !existsSync(artifactPath(relative)))
    .map((relative) => relative.replace("../../", "frontend/"));

  if (missing.length === 0) {
    return null;
  }

  return `WASM kernel not built (${missing.join(", ")}) — run \`just wasm-build\``;
}

/**
 * The reason to skip, or null to run. Throws instead of returning a reason when
 * {@link RequireKernelEnv} is set, so CI cannot pass by skipping.
 */
export function kernelSkipReason(): string | null {
  const reason = missingKernelReason();
  if (reason === null) {
    return null;
  }

  if (process.env[RequireKernelEnv]) {
    throw new Error(`${reason} (${RequireKernelEnv} is set, so this is fatal)`);
  }

  return reason;
}

let kernelPromise: Promise<AconiqKernel> | null = null;

/** Load the kernel once per vitest worker and hand back the shared instance. */
export function getNodeKernel(): Promise<AconiqKernel> {
  kernelPromise ??= loadNodeKernel();
  return kernelPromise;
}

async function loadNodeKernel(): Promise<AconiqKernel> {
  const reason = missingKernelReason();
  if (reason !== null) {
    throw new Error(reason);
  }

  const host = globalThis as unknown as GoRuntimeGlobal;

  if (host.Go === undefined) {
    const execPath = artifactPath(artifacts.wasmExec);
    runInThisContext(readFileSync(execPath, "utf8"), { filename: execPath });
  }

  const GoRuntime = host.Go;
  if (GoRuntime === undefined) {
    throw new Error(
      "wasm_exec.js was evaluated but did not define Go — regenerate it with `just wasm-build`",
    );
  }

  const go = new GoRuntime();
  const { instance } = await WebAssembly.instantiate(
    readFileSync(artifactPath(artifacts.module)),
    go.importObject,
  );

  // Fire-and-forget, exactly as kernel.ts does: Go's main() ends in select{} so
  // this promise never resolves, and every export is registered synchronously
  // before the scheduler first yields.
  void go.run(instance);

  const exports = host.aconiq;
  if (exports === undefined) {
    throw new Error(
      "the Go module ran but registered no aconiq global — public/aconiq.wasm is not a kernel build",
    );
  }

  // Every call goes through `withKernelErrors`, exactly as the worker client
  // does. `cmd/wasm/main.go` rejects with a bare string rather than an Error,
  // so without this the two kernels would differ in what they throw — and the
  // parity suites would be pinning the wrong one.
  return {
    async rls19Road(req: ComputeRequest): Promise<ReceiverOutput[]> {
      const json = await withKernelErrors(
        exports.rls19Road(JSON.stringify(req)),
      );
      return JSON.parse(json) as ReceiverOutput[];
    },
    async transform(req: TransformRequest): Promise<TransformResponse> {
      const json = await withKernelErrors(
        exports.transform(JSON.stringify(req)),
      );
      return JSON.parse(json) as TransformResponse;
    },
    async contours(
      payload: Uint8Array,
      req: ContourRequest,
    ): Promise<ContourResult> {
      const json = await withKernelErrors(
        exports.contours(payload, JSON.stringify(req)),
      );
      return JSON.parse(json) as ContourResult;
    },
    // Asynchronous although the Go exports below them are not: `AconiqKernel`
    // is what the worker-backed kernel can offer, and it cannot offer a
    // synchronous answer. Mirroring the interface here is what keeps the
    // parity suites driving the same calling convention the app uses.
    standards(): Promise<StandardDescriptor[]> {
      return Promise.resolve(
        JSON.parse(exports.standards()) as StandardDescriptor[],
      );
    },
    loadTerrain(data: Uint8Array, crs: string): Promise<TerrainInfo> {
      return Promise.resolve(
        JSON.parse(exports.loadTerrain(data, crs)) as TerrainInfo,
      );
    },
    clearTerrain(): Promise<void> {
      exports.clearTerrain();
      return Promise.resolve();
    },
    defaultConfig(): Promise<PropagationConfig> {
      return Promise.resolve(
        JSON.parse(exports.defaultConfig()) as PropagationConfig,
      );
    },
  };
}
