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

interface Window {
  aconiq?: {
    rls19Road: (json: string) => Promise<string>;
    defaultConfig: () => string;
    health: () => string;
    projectStatus: () => string;
  };
}
