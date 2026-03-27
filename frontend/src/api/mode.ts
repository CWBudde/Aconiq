export const IS_WASM_MODE = import.meta.env.VITE_WASM_MODE === "true";

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(
  /\/$/,
  "",
);

export const API_BASE_URL_OVERRIDE_KEY = "aconiq.api_base_url";

function readLocalStorageOverride(): string | null {
  try {
    const value = localStorage.getItem(API_BASE_URL_OVERRIDE_KEY);
    if (!value) return null;
    const normalized = value.trim().replace(/\/$/, "");
    return normalized.length > 0 ? normalized : null;
  } catch {
    return null;
  }
}

export function getAPIBaseURL(): string {
  return readLocalStorageOverride() ?? API_BASE_URL;
}

export function hasAPIBaseURLOverride(): boolean {
  return readLocalStorageOverride() !== null;
}

export function setAPIBaseURLOverride(value: string): void {
  try {
    const normalized = value.trim().replace(/\/$/, "");
    if (normalized.length === 0) {
      localStorage.removeItem(API_BASE_URL_OVERRIDE_KEY);
      return;
    }
    localStorage.setItem(API_BASE_URL_OVERRIDE_KEY, normalized);
  } catch {
    // Storage unavailable or full. Ignore silently.
  }
}

export function clearAPIBaseURLOverride(): void {
  try {
    localStorage.removeItem(API_BASE_URL_OVERRIDE_KEY);
  } catch {
    // Storage unavailable. Ignore silently.
  }
}

export function apiURL(path: string): string {
  return `${getAPIBaseURL()}${path}`;
}
