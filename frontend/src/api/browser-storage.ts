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
    let settled = false;
    request.onupgradeneeded = () => {
      request.result.createObjectStore(STORE_NAME);
    };
    request.onsuccess = () => {
      // `onblocked` may already have rejected; the open still completes once
      // the other tab closes. A connection nobody holds would sit open with
      // no `onversionchange` handler and block every future upgrade itself.
      if (settled) {
        request.result.close();
        return;
      }
      settled = true;
      resolve(request.result);
    };
    request.onerror = () => {
      settled = true;
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
      settled = true;
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

/**
 * Runs several writes inside one transaction and resolves when it commits.
 *
 * IndexedDB transactions are all-or-nothing, which is the only way two records
 * that describe one change can stay consistent: the document naming a run and
 * the bytes keyed beside it. Split across two transactions there is no safe
 * order — delete first and a failed document write leaves a run whose raster is
 * gone, delete second and the write that was supposed to make room runs while
 * the space it needs is still occupied.
 *
 * Resolves on `oncomplete` rather than on the last request, because a quota
 * failure can surface at commit time after every `put` has already reported
 * success — and still listens on each request, because it can also surface
 * there, on the one write that did not fit.
 *
 * `operations` returns the requests it made so both can be watched.
 */
async function withWriteTransaction(
  context: string,
  operations: (store: IDBObjectStore) => IDBRequest<unknown>[],
): Promise<void> {
  const db = await openDatabase();
  return new Promise<void>((resolve, reject) => {
    let settled = false;
    const settle = (fn: () => void) => {
      if (settled) return;
      settled = true;
      fn();
    };
    const fail = (error: unknown) => {
      settle(() => {
        reject(toStorageError(error, "unavailable", context));
      });
    };

    let tx: IDBTransaction;
    try {
      tx = db.transaction(STORE_NAME, "readwrite");
    } catch (error) {
      db.close();
      reject(toStorageError(error, "unavailable", context));
      return;
    }

    tx.onabort = () => {
      db.close();
      fail(tx.error);
    };
    tx.onerror = () => {
      fail(tx.error);
    };
    tx.oncomplete = () => {
      db.close();
      settle(resolve);
    };

    let requests: IDBRequest<unknown>[];
    try {
      requests = operations(tx.objectStore(STORE_NAME));
    } catch (error) {
      fail(error);
      return;
    }

    for (const request of requests) {
      request.onerror = () => {
        fail(request.error);
      };
    }
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

/**
 * Stores the document alone. Kept separate from the pair below rather than
 * delegating to it: the callers that give nothing up — the localStorage
 * migration, and every test that seeds a store — should not be reaching
 * through a function whose subject is what a write removes.
 */
export async function savePersistedState(value: unknown): Promise<void> {
  await withWriteTransaction("Browser-mode runs cannot be stored", (store) => [
    store.put(value, STATE_KEY),
  ]);
}

/**
 * Stores the document and removes the byte records it stopped naming, as one
 * change.
 *
 * Every caller that drops a run — the run cap, quota eviction, `deleteRun` —
 * has exactly this pair to write, and the eviction case is why they must
 * commit together: it writes a smaller document *in order to* free space, and
 * doing so while the evicted run's raster still occupies the quota is the one
 * situation eviction exists for and the one where it would achieve nothing.
 */
export async function savePersistedStateForgetting(
  value: unknown,
  artifactIds: readonly string[],
): Promise<void> {
  await withWriteTransaction("Browser-mode runs cannot be stored", (store) => [
    // Deletes before the put: within one transaction that is the order that
    // gives the document the space the eviction just freed.
    ...artifactIds.map((artifactId) =>
      store.delete(artifactBytesKey(artifactId)),
    ),
    store.put(value, STATE_KEY),
  ]);
}

/**
 * Artifact bytes live under their own key, one record per artifact, rather than
 * inside the state document.
 *
 * The document is written *whole* on every change — `persist` structured-clones
 * it, runs and all — so a raster binary stored inside it would be re-cloned on
 * every save. A 250 000-cell grid over two bands is 4 MB, and the store keeps
 * twenty runs. Keeping the bytes beside the document costs one extra request on
 * the rare path that reads them and nothing at all on the common path that does
 * not.
 *
 * This is not the per-run record split PLAN.md's Phase F describes; it is one
 * key space for the one thing that is actually large.
 */
const ARTIFACT_BYTES_PREFIX = "artifact-bytes:";

function artifactBytesKey(artifactId: string): string {
  return ARTIFACT_BYTES_PREFIX + artifactId;
}

export async function saveArtifactBytes(
  artifactId: string,
  bytes: ArrayBuffer,
): Promise<void> {
  await withStore(
    "readwrite",
    "Browser-mode raster data cannot be stored",
    (store) => store.put(bytes, artifactBytesKey(artifactId)),
  );
}

/**
 * Whether a value read back from the store is an `ArrayBuffer`.
 *
 * **Not `instanceof`.** IndexedDB hands back a structured clone, and the clone
 * can be constructed in a different realm from the one this module runs in —
 * `fake-indexeddb` does exactly that under vitest. A cross-realm `ArrayBuffer`
 * has every internal slot and fails `instanceof` anyway, so the branded tag is
 * what actually answers the question.
 */
function isArrayBuffer(value: unknown): value is ArrayBuffer {
  return Object.prototype.toString.call(value) === "[object ArrayBuffer]";
}

/** Resolves with the stored bytes, or `null` when nothing is stored. */
export async function loadArtifactBytes(
  artifactId: string,
): Promise<ArrayBuffer | null> {
  const value: unknown = await withStore(
    "readonly",
    "Browser-mode raster data cannot be read",
    (store) => store.get(artifactBytesKey(artifactId)),
  );
  if (isArrayBuffer(value)) return value;
  // A view is not what saveArtifactBytes stores, but a store that normalises
  // buffers to typed arrays would otherwise silently read as "nothing stored".
  if (ArrayBuffer.isView(value)) {
    return value.buffer.slice(
      value.byteOffset,
      value.byteOffset + value.byteLength,
    ) as ArrayBuffer;
  }
  return null;
}

/**
 * Removes the bytes of artifacts whose run is gone. A failure here is not worth
 * failing the caller over — the record is orphaned, not corrupting — so it is
 * reported through the returned promise and callers may ignore it.
 */
export async function deleteArtifactBytes(
  artifactIds: readonly string[],
): Promise<void> {
  await withWriteTransaction(
    "Browser-mode raster data cannot be removed",
    (store) =>
      artifactIds.map((artifactId) =>
        store.delete(artifactBytesKey(artifactId)),
      ),
  );
}

export async function clearPersistedState(): Promise<void> {
  await withStore(
    "readwrite",
    "Stored browser-mode runs cannot be removed",
    (store) => store.delete(STATE_KEY),
  );
  // The artifact byte records outlive the document that referenced them, so
  // clearing only STATE_KEY would leave every raster this origin ever stored
  // occupying quota with nothing able to name it again.
  await withStore(
    "readwrite",
    "Stored browser-mode raster data cannot be removed",
    (store) =>
      store.delete(
        IDBKeyRange.bound(
          ARTIFACT_BYTES_PREFIX,
          ARTIFACT_BYTES_PREFIX + "\uffff",
        ),
      ),
  );
}
