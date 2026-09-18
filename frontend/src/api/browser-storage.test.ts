import { IDBFactory } from "fake-indexeddb";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  BrowserStorageError,
  clearPersistedState,
  deleteArtifactBytes,
  isBrowserStorageError,
  listArtifactBytesIDs,
  loadArtifactBytes,
  loadPersistedState,
  saveArtifactBytes,
  savePersistedState,
} from "./browser-storage";

describe("browser storage", () => {
  beforeEach(() => {
    // A fresh factory per test: the fake database is process-wide state, and
    // a document left behind by one test must not become another's fixture.
    vi.stubGlobal("indexedDB", new IDBFactory());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("resolves null when nothing has been stored", async () => {
    await expect(loadPersistedState()).resolves.toBeNull();
  });

  it("round-trips a structured value", async () => {
    const value = { version: 1, state: { runs: [{ id: "run-0001" }] } };
    await savePersistedState(value);
    await expect(loadPersistedState()).resolves.toEqual(value);
  });

  it("overwrites the previous document", async () => {
    await savePersistedState({ n: 1 });
    await savePersistedState({ n: 2 });
    await expect(loadPersistedState()).resolves.toEqual({ n: 2 });
  });

  it("clears the document", async () => {
    await savePersistedState({ n: 1 });
    await clearPersistedState();
    await expect(loadPersistedState()).resolves.toBeNull();
  });

  it("clearing an empty store is not an error", async () => {
    await expect(clearPersistedState()).resolves.toBeUndefined();
  });

  it("reports a quota failure as such", async () => {
    // fake-indexeddb has no quota, so the put request is made to fail the way
    // a real one does: with a DOMException named QuotaExceededError.
    vi.spyOn(IDBObjectStore.prototype, "put").mockImplementation(function () {
      const request = {
        error: new DOMException("quota", "QuotaExceededError"),
        onerror: null as (() => void) | null,
        onsuccess: null as (() => void) | null,
        result: undefined,
      };
      queueMicrotask(() => request.onerror?.());
      return request as unknown as IDBRequest<IDBValidKey>;
    });

    const failure = await savePersistedState({ n: 1 }).catch(
      (error: unknown) => error,
    );
    expect(failure).toBeInstanceOf(BrowserStorageError);
    expect(isBrowserStorageError(failure, "quota")).toBe(true);
    expect((failure as BrowserStorageError).message).toMatch(/quota/);
  });

  it("reports a quota failure raised at commit, after the put succeeded", async () => {
    // Browsers may accept every `put` and only abort the transaction when it
    // commits. The write must wait for that, so a fake transaction lets the
    // request succeed and then aborts with the quota error. Only the write
    // transaction is faked: the open itself runs a `versionchange` one
    // through the same method, and that has to stay real.
    await savePersistedState({ n: 0 });
    // eslint-disable-next-line @typescript-eslint/unbound-method -- only ever invoked through `.call(this, ...)` below
    const realTransaction = IDBDatabase.prototype.transaction;
    vi.spyOn(IDBDatabase.prototype, "transaction").mockImplementation(function (
      this: IDBDatabase,
      names,
      mode,
      options,
    ) {
      if (mode !== "readwrite") {
        return realTransaction.call(this, names, mode, options);
      }
      const tx = {
        error: new DOMException("quota", "QuotaExceededError"),
        onabort: null as (() => void) | null,
        onerror: null as (() => void) | null,
        oncomplete: null as (() => void) | null,
        objectStore: () => ({
          put: () => {
            const request = {
              onsuccess: null as (() => void) | null,
              onerror: null as (() => void) | null,
              result: "backend-state",
            };
            queueMicrotask(() => {
              request.onsuccess?.();
              queueMicrotask(() => tx.onabort?.());
            });
            return request;
          },
        }),
      };
      return tx as unknown as IDBTransaction;
    });

    const failure = await savePersistedState({ n: 1 }).catch(
      (error: unknown) => error,
    );
    expect(isBrowserStorageError(failure, "quota")).toBe(true);
  });

  describe("without IndexedDB", () => {
    beforeEach(() => {
      vi.stubGlobal("indexedDB", undefined);
    });

    it("rejects every operation with `unavailable` instead of throwing", async () => {
      for (const operation of [
        () => loadPersistedState(),
        () => savePersistedState({}),
        () => clearPersistedState(),
      ]) {
        // The call itself must not throw — only the promise may reject —
        // because callers `await` inside try/catch blocks written for that.
        const promise = operation();
        expect(promise).toBeInstanceOf(Promise);
        const failure = await promise.catch((error: unknown) => error);
        expect(isBrowserStorageError(failure, "unavailable")).toBe(true);
      }
    });
  });

  it("classifies an open failure as `unavailable`", async () => {
    vi.stubGlobal("indexedDB", {
      open: () => {
        throw new Error("blocked by policy");
      },
    });
    const failure = await loadPersistedState().catch((error: unknown) => error);
    expect(isBrowserStorageError(failure, "unavailable")).toBe(true);
    expect((failure as BrowserStorageError).message).toMatch(
      /blocked by policy/,
    );
  });

  /*
   * The byte records are the one thing in this store that a caller can lose
   * track of: they are keyed beside the document rather than inside it, so a
   * tab that dies between writing bytes and writing the document leaves a
   * record nobody named. Reclaiming those needs a listing, and a listing needs
   * to agree with the range delete `clearPersistedState` already does —
   * otherwise one of the two would miss records the other removes.
   */
  describe("artifact bytes", () => {
    const bytes = (n: number) => new Uint8Array([n]).buffer;

    it("round-trips bytes under their own key", async () => {
      await saveArtifactBytes("a", bytes(7));

      const read = await loadArtifactBytes("a");
      expect(read).not.toBeNull();
      expect(new Uint8Array(read as ArrayBuffer)).toEqual(new Uint8Array([7]));
      await expect(loadArtifactBytes("absent")).resolves.toBeNull();
    });

    it("lists the ids it holds, with the key prefix stripped", async () => {
      await saveArtifactBytes("artifact-run-0001-raster-bin", bytes(1));
      await saveArtifactBytes("artifact-run-0002-raster-bin", bytes(2));

      await expect(listArtifactBytesIDs()).resolves.toEqual([
        "artifact-run-0001-raster-bin",
        "artifact-run-0002-raster-bin",
      ]);
    });

    it("lists nothing when only the document is stored", async () => {
      // The listing is a range over one key prefix, and `backend-state` sorts
      // *before* it. A listing that walked every key would return the document
      // as though it were an artifact, and the sweep above it would delete the
      // one record it must never touch.
      await savePersistedState({ version: 1, state: {} });

      await expect(listArtifactBytesIDs()).resolves.toEqual([]);
    });

    it("deletes the ids it is given and leaves the rest", async () => {
      await saveArtifactBytes("keep", bytes(1));
      await saveArtifactBytes("drop", bytes(2));

      await deleteArtifactBytes(["drop", "never-stored"]);

      await expect(listArtifactBytesIDs()).resolves.toEqual(["keep"]);
    });

    it("clearing removes every byte record, not only the document", async () => {
      await savePersistedState({ version: 1, state: {} });
      await saveArtifactBytes("a", bytes(1));
      await saveArtifactBytes("b", bytes(2));

      await clearPersistedState();

      await expect(listArtifactBytesIDs()).resolves.toEqual([]);
      await expect(loadPersistedState()).resolves.toBeNull();
    });

    it("reads back a stored typed-array view as a buffer", async () => {
      // Not what `saveArtifactBytes` writes, but a store that normalises
      // buffers to views would otherwise read as "nothing stored" — and an
      // absent raster is reported as a corrupted run.
      await savePersistedState({ version: 1, state: {} });
      const view = new Uint8Array([1, 2, 3, 4]);
      await saveArtifactBytes("view", view as unknown as ArrayBuffer);

      const read = await loadArtifactBytes("view");
      expect(read).not.toBeNull();
      expect(new Uint8Array(read as ArrayBuffer)).toEqual(view);
    });

    it("rejects with `unavailable` where IndexedDB is missing", async () => {
      vi.stubGlobal("indexedDB", undefined);
      const failure = await listArtifactBytesIDs().catch(
        (error: unknown) => error,
      );
      expect(isBrowserStorageError(failure, "unavailable")).toBe(true);
    });
  });

  it("isBrowserStorageError matches by reason when one is given", () => {
    const error = new BrowserStorageError("corrupt", "bad");
    expect(isBrowserStorageError(error)).toBe(true);
    expect(isBrowserStorageError(error, "corrupt")).toBe(true);
    expect(isBrowserStorageError(error, "quota")).toBe(false);
    expect(isBrowserStorageError(new Error("bad"))).toBe(false);
  });
});
