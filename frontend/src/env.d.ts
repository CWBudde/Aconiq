/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Absent unless set at build time; empty means same-origin. */
  readonly VITE_API_BASE_URL?: string;
  /** Set to "true" when building the browser WASM demo (no HTTP backend). */
  readonly VITE_WASM_MODE?: string;
  /**
   * Bearer token for the local API, matching `aconiq serve --api-token`.
   * Absent unless the server was started with one; the API needs no credential
   * otherwise.
   */
  readonly VITE_API_TOKEN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

/**
 * What `backend/cmd/wasm/main.go` registers on `window.aconiq`. Every entry
 * point it sets belongs here: a missing one is not a type error at the call
 * site, it is a `TypeError: not a function` at runtime.
 */
interface Window {
  aconiq?: {
    /**
     * The second argument is optional and asks the kernel to report progress:
     * it is called with the receivers computed so far and the total, once per
     * chunk. Go refuses anything that is not a function, so a caller that does
     * not want progress passes one argument rather than `undefined`.
     */
    rls19Road: (
      json: string,
      onProgress?: (done: number, total: number) => void,
    ) => Promise<string>;
    /**
     * One Worker's share of a run. Same request as `rls19Road` plus a
     * `shard` of `{index, count}`, resolving to `{chunk, start, outputs}[]`
     * rather than a flat receiver list — the caller sorts by `chunk`, because
     * the order Workers reply in is the order the machine scheduled them.
     *
     * The request carries the *whole* run's receivers even so. See
     * `wasmkernel.ComputeRLS19RoadShard`: the grid's terrain elevation is
     * derived from the centroid of the receiver list the kernel is handed, so
     * a shard given only its own slice would compute over different ground.
     *
     * `onProgress` counts this shard's receivers, not the run's.
     */
    rls19RoadShard: (
      json: string,
      onProgress?: (done: number, total: number) => void,
    ) => Promise<string>;
    transform: (json: string) => Promise<string>;
    /** See `AconiqKernel.maskFootprints`. */
    maskFootprints: (json: string) => Promise<string>;
    /**
     * Two arguments, and the raster values are the first: they cross as raw
     * bytes rather than inside the JSON, because a 500x500 two-band grid is
     * four megabytes of float64. See `AconiqKernel.contours`.
     */
    contours: (payload: Uint8Array, json: string) => Promise<string>;
    /** Throws an `Error` if the descriptor cannot be marshalled. */
    standards: () => string;
    /**
     * The CRS is a required second argument: the Go GeoTIFF loader reads the
     * tie point and the pixel scale and no GeoKeyDirectory, so the raster does
     * not say what it is in, and a compute request carries bare numbers. See
     * `AconiqKernel.loadTerrain`.
     */
    loadTerrain: (data: Uint8Array, crs: string) => string;
    // Throws an `Error` on a raster Go cannot read, or a missing CRS. A
    // TypeScript signature cannot say so; `syncThrowSource` in
    // backend/cmd/wasm/main.go says why these two throw where the Promise-
    // returning exports reject.
    clearTerrain: () => void;
    defaultConfig: () => string;
    health: () => string;
    projectStatus: () => string;
  };
}
