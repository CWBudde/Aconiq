// Lazy loader for the Aconiq WASM computation kernel.
//
// Usage:
//   const kernel = await getKernel();
//   const outputs = await kernel.rls19Road({ receivers, sources, barriers });

import type {
  ComputeRequest,
  PropagationConfig,
  ReceiverOutput,
} from "./types";

export interface AconiqKernel {
  /** Compute RLS-19 road traffic noise levels for all receivers. */
  rls19Road(req: ComputeRequest): Promise<ReceiverOutput[]>;
  /** Return the default PropagationConfig. */
  defaultConfig(): PropagationConfig;
}

// Singleton promise — WASM is only loaded once.
let kernelPromise: Promise<AconiqKernel> | null = null;

export function getKernel(): Promise<AconiqKernel> {
  if (!kernelPromise) {
    kernelPromise = loadKernel();
  }
  return kernelPromise;
}

async function loadWasmExecScript(): Promise<void> {
  if (typeof window.Go !== "undefined") return;
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = `${import.meta.env.BASE_URL}wasm_exec.js`;
    script.onload = () => {
      resolve();
    };
    script.onerror = () => {
      reject(
        new Error(
          "Failed to load wasm_exec.js. Run `just wasm-build` to generate the WASM kernel.",
        ),
      );
    };
    document.head.appendChild(script);
  });
}

async function loadKernel(): Promise<AconiqKernel> {
  await loadWasmExecScript();

  const GoRuntime = window.Go;
  if (!GoRuntime) {
    throw new Error(
      "wasm_exec.js was loaded but did not define window.Go. Re-run `just wasm-build` to regenerate a matching wasm_exec.js.",
    );
  }

  const go = new GoRuntime();
  const wasmUrl = `${import.meta.env.BASE_URL}aconiq.wasm`;

  let result: WebAssembly.WebAssemblyInstantiatedSource;
  try {
    result = await WebAssembly.instantiateStreaming(
      fetch(wasmUrl),
      go.importObject,
    );
  } catch {
    throw new Error(
      `Failed to load ${wasmUrl}. Run \`just wasm-build\` to generate the WASM kernel.`,
    );
  }

  // Fire-and-forget: Go's main() blocks on select{} so this promise never resolves.
  // All JS exports are registered synchronously before the scheduler yields.
  void go.run(result.instance);

  // Captured once: `window.aconiq` is undefined until the Go module registers
  // its exports, so narrowing it here is what keeps the returned methods safe.
  const exports = window.aconiq;
  if (!exports) {
    throw new Error(
      "WASM kernel not initialised: the Go module ran but did not register window.aconiq. Load the kernel via getKernel() and make sure public/aconiq.wasm is the current build (`just wasm-build`).",
    );
  }

  return {
    async rls19Road(req: ComputeRequest): Promise<ReceiverOutput[]> {
      const json = await exports.rls19Road(JSON.stringify(req));
      return JSON.parse(json) as ReceiverOutput[];
    },
    defaultConfig(): PropagationConfig {
      return JSON.parse(exports.defaultConfig()) as PropagationConfig;
    },
  };
}
