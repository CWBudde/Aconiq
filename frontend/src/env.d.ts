/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string;
  /** Set to "true" when building the browser WASM demo (no HTTP backend). */
  readonly VITE_WASM_MODE?: string;
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
