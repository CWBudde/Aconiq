/**
 * The one persisted document of browser mode, kept in IndexedDB.
 *
 * Browser-mode runs used to live in localStorage. That store is synchronous
 * and capped at a few megabytes per origin, and a run carries its whole
 * receiver table, CSV and export bundle — a handful of runs filled it, and the
 * write threw with the run already computed. IndexedDB stores structured
 * values without a string round-trip, its quota is orders of magnitude larger
 * and, being asynchronous, it never blocks the UI thread on a write.
 *
 * This wrapper knows nothing about the document's shape; it moves one value
 * under one key and classifies failures. Every function is safe to call where
 * IndexedDB is missing — it rejects with `unavailable` rather than throwing.
 */

const DB_NAME = "aconiq-browser";
const DB_VERSION = 1;
const STORE_NAME = "documents";
const STATE_KEY = "backend-state";

export type BrowserStorageReason = "quota" | "unavailable" | "corrupt";

export class BrowserStorageError extends Error {
  readonly reason: BrowserStorageReason;

  constructor(
    reason: BrowserStorageReason,
    message: string,
    options?: ErrorOptions,
  ) {
    super(message, options);
    this.name = "BrowserStorageError";
    this.reason = reason;
  }
}

export function isBrowserStorageError(
  error: unknown,
  reason?: BrowserStorageReason,
): error is BrowserStorageError {
  return (
    error instanceof BrowserStorageError &&
    (reason === undefined || error.reason === reason)
  );
}

/**
 * Maps whatever IndexedDB reported to a `BrowserStorageError`. A quota
 * failure is the one case the caller can act on (evict, retry), so it is the
 * one that gets its own reason; everything else is `fallback`.
 */
function toStorageError(
  error: unknown,
  fallback: BrowserStorageReason,
  context: string,
): BrowserStorageError {
  if (error instanceof BrowserStorageError) return error;
  if (error instanceof DOMException && error.name === "QuotaExceededError") {
    return new BrowserStorageError(
      "quota",
      `${context}: the browser's storage quota is exhausted`,
      { cause: error },
    );
  }
  const detail = error instanceof Error ? error.message : String(error);
  return new BrowserStorageError(fallback, `${context}: ${detail}`, {
    cause: error,
  });
}

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === "undefined") {
      reject(
        new BrowserStorageError(
          "unavailable",
          "IndexedDB is not available in this browser",
        ),
      );
      return;
    }
    let request: IDBOpenDBRequest;
    try {
      request = indexedDB.open(DB_NAME, DB_VERSION);
    } catch (error) {
      reject(
        toStorageError(error, "unavailable", "IndexedDB cannot be opened"),
      );
      return;
    }
    request.onupgradeneeded = () => {
      request.result.createObjectStore(STORE_NAME);
    };
    request.onsuccess = () => {
      resolve(request.result);
    };
    request.onerror = () => {
      reject(
        toStorageError(
          request.error,
          "unavailable",
          "IndexedDB cannot be opened",
        ),
      );
    };
    // Only fires when another tab holds an older schema open. The schema has
    // one version, so in practice it never does; surfacing it is cheaper than
    // hanging.
    request.onblocked = () => {
      reject(
        new BrowserStorageError(
          "unavailable",
          "IndexedDB is blocked by another open tab",
        ),
      );
    };
  });
}

/**
 * Runs one request inside its own transaction and closes the connection
 * afterwards. A read settles as soon as its request does; a write waits for
 * the transaction to commit, because a quota failure can surface at commit
 * time after every `put` has already reported success.
 */
async function withStore<T>(
  mode: IDBTransactionMode,
  context: string,
  operation: (store: IDBObjectStore) => IDBRequest<T>,
): Promise<T> {
  const db = await openDatabase();
  return new Promise<T>((resolve, reject) => {
    let settled = false;
    const settle = (fn: () => void) => {
      if (settled) return;
      settled = true;
      fn();
    };
    let tx: IDBTransaction;
    try {
      tx = db.transaction(STORE_NAME, mode);
    } catch (error) {
      db.close();
      reject(toStorageError(error, "unavailable", context));
      return;
    }
    const fail = (error: unknown) => {
      settle(() => {
        reject(toStorageError(error, "unavailable", context));
      });
    };
    tx.onabort = () => {
      db.close();
      fail(tx.error);
    };
    tx.onerror = () => {
      fail(tx.error);
    };

    let request: IDBRequest<T>;
    try {
      request = operation(tx.objectStore(STORE_NAME));
    } catch (error) {
      fail(error);
      return;
    }
    const succeed = () => {
      settle(() => {
        resolve(request.result);
      });
    };
    request.onerror = () => {
      fail(request.error);
    };
    if (mode === "readonly") request.onsuccess = succeed;
    tx.oncomplete = () => {
      db.close();
      if (mode !== "readonly") succeed();
    };
  });
}

/** Resolves with the stored document, or `null` when nothing is stored. */
export async function loadPersistedState(): Promise<unknown> {
  const value: unknown = await withStore(
    "readonly",
    "Stored browser-mode runs cannot be read",
    (store) => store.get(STATE_KEY),
  );
  return value ?? null;
}

export async function savePersistedState(value: unknown): Promise<void> {
  await withStore("readwrite", "Browser-mode runs cannot be stored", (store) =>
    store.put(value, STATE_KEY),
  );
}

export async function clearPersistedState(): Promise<void> {
  await withStore(
    "readwrite",
    "Stored browser-mode runs cannot be removed",
    (store) => store.delete(STATE_KEY),
  );
}
