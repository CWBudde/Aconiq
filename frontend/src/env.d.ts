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
    rls19Road: (json: string) => Promise<string>;
    transform: (json: string) => Promise<string>;
    standards: () => string;
    loadTerrain: (data: Uint8Array) => string;
    clearTerrain: () => void;
    defaultConfig: () => string;
    health: () => string;
    projectStatus: () => string;
  };
}
