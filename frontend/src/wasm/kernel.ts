// Lazy loader for the Aconiq WASM computation kernel.
//
// Usage:
//   const kernel = await getKernel();
//   const outputs = await kernel.rls19Road({ receivers, sources, barriers });

import type { StandardDescriptor } from "@/standards/descriptor";
import type {
  ComputeRequest,
  PropagationConfig,
  ReceiverOutput,
  TerrainInfo,
  TransformRequest,
  TransformResponse,
} from "./types";

export interface AconiqKernel {
  /** Compute RLS-19 road traffic noise levels for all receivers. */
  rls19Road(req: ComputeRequest): Promise<ReceiverOutput[]>;
  /**
   * Project a batch of coordinates between two CRS.
   *
   * This is the frontend's only way into a map projection, and deliberately
   * so: the kernel already links `internal/geo` in, and a second
   * transverse-Mercator implementation in TypeScript would be a second answer
   * to where a model sits. See `@/model/compute-crs`, which is the only caller.
   */
  transform(req: TransformRequest): Promise<TransformResponse>;
  /**
   * The standards this kernel can actually run, in the same shape `GET
   * /api/v1/standards` answers with.
   *
   * It comes from the kernel rather than from a hardcoded list so that the
   * evidence tier, the parameter defaults and the surface enum are declared
   * once, by the Go module. The hardcoded copy this replaced had drifted twice.
   */
  standards(): StandardDescriptor[];
  /**
   * Load a GeoTIFF DTM into the kernel, replacing whatever was loaded before.
   *
   * `crs` names the CRS the raster's own coordinates are in, and it is
   * required. The Go loader reads the tie point and the pixel scale and no
   * GeoKeyDirectory, so the file does not say; and a {@link ComputeRequest}
   * carries bare numbers, so the request does not say either. Without it every
   * elevation query would be a query into an unknown CRS — which is how a DTM
   * in degrees comes to be asked about metres, miss on every lookup, and be
   * read as sea level.
   *
   * A run over a loaded terrain must therefore also send
   * {@link ComputeRequest.projection}; the kernel refuses it otherwise.
   */
  loadTerrain(data: Uint8Array, crs: string): TerrainInfo;
  /** Drop the loaded terrain. Runs afterwards compute without one. */
  clearTerrain(): void;
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
    async transform(req: TransformRequest): Promise<TransformResponse> {
      const json = await exports.transform(JSON.stringify(req));
      return JSON.parse(json) as TransformResponse;
    },
    standards(): StandardDescriptor[] {
      return JSON.parse(exports.standards()) as StandardDescriptor[];
    },
    loadTerrain(data: Uint8Array, crs: string): TerrainInfo {
      return JSON.parse(exports.loadTerrain(data, crs)) as TerrainInfo;
    },
    clearTerrain(): void {
      exports.clearTerrain();
    },
    defaultConfig(): PropagationConfig {
      return JSON.parse(exports.defaultConfig()) as PropagationConfig;
    },
  };
}
