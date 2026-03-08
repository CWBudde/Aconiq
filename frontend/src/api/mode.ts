export const IS_WASM_MODE = import.meta.env.VITE_WASM_MODE === "true";

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(
  /\/$/,
  "",
);

export function apiURL(path: string): string {
  return `${API_BASE_URL}${path}`;
}
