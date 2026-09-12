import { IDBFactory } from "fake-indexeddb";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  BrowserStorageError,
  clearPersistedState,
  isBrowserStorageError,
  loadPersistedState,
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

  it("isBrowserStorageError matches by reason when one is given", () => {
    const error = new BrowserStorageError("corrupt", "bad");
    expect(isBrowserStorageError(error)).toBe(true);
    expect(isBrowserStorageError(error, "corrupt")).toBe(true);
    expect(isBrowserStorageError(error, "quota")).toBe(false);
    expect(isBrowserStorageError(new Error("bad"))).toBe(false);
  });
});
