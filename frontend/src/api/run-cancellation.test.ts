/**
 * What a browser-mode run reports while it computes, and what is left behind
 * when it is stopped.
 *
 * A file of its own, with a kernel stub whose `rls19Road` never settles on its
 * own: `browser-backend.test.ts` drives `startRun` to completion everywhere,
 * and a run that has already finished cannot be cancelled or observed
 * mid-flight. The stub is the only way to hold a run open long enough to ask
 * either question.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { browserBackend, resetBrowserBackendForTests } from "./browser-backend";
import { isRunCancelled } from "./backend";
import * as storage from "./browser-storage";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";
import type { ComputeRequest } from "@/wasm/types";
import { CancelledErrorName } from "@/wasm/kernel-client";
import type { KernelProgressListener } from "@/wasm/protocol";

/** The receivers the stubbed kernel was asked to compute, with its reporter. */
interface KernelCall {
  request: ComputeRequest;
  onProgress: KernelProgressListener | undefined;
  /** Settles the call, as the worker's reply would. */
  finish: () => void;
  /** Settles it the other way: a compute that finished before the abort. */
  succeed: () => void;
}

const calls: KernelCall[] = [];
let cancels = 0;

/**
 * A kernel whose compute hangs until the test releases it.
 *
 * `cancelKernel` only counts here: what it does to a real worker — terminate
 * it and reject every pending call — is `kernel-client.test.ts`'s subject.
 * What this file has to see is that `startRun` reaches for it at all, and
 * that the rejection which follows leaves the store as it found it. So the
 * stub rejects the pending call itself, exactly as a torn-down worker does,
 * with the name the client uses.
 */
vi.mock("@/wasm/kernel", () => ({
  cancelKernel: () => {
    cancels += 1;
    for (const call of calls.splice(0)) call.finish();
  },
  getKernel: () =>
    Promise.resolve({
      rls19Road: (req: ComputeRequest, onProgress?: KernelProgressListener) =>
        // Never resolved: a completed run is `browser-backend.test.ts`'s
        // subject, and this file only ever ends one by cancelling it.
        new Promise((resolve, reject) => {
          calls.push({
            request: req,
            onProgress,
            finish: () => {
              const cancelled = new Error("kernel cancelled");
              cancelled.name = CancelledErrorName;
              reject(cancelled);
            },
            succeed: () => {
              resolve(
                req.receivers.map((receiver) => ({
                  Receiver: receiver,
                  Indicators: { lr_day: 50, lr_night: 40 },
                })),
              );
            },
          });
        }),
      transform: (req: { source_crs: string; coordinates: number[] }) =>
        Promise.resolve({
          source_crs: req.source_crs,
          target_crs: req.source_crs,
          applied: false,
          coordinates: req.coordinates,
        }),
      standards: () =>
        Promise.resolve([
          {
            id: "rls19-road",
            description: "rls19-road (stub)",
            default_version: "2019",
            versions: [],
            evidence_tier: "normative",
          },
        ]),
      defaultConfig: () => Promise.resolve({}),
    }),
}));

const ROAD: ModelFeature = {
  id: "road",
  kind: "source",
  sourceType: "line",
  properties: { traffic_day_pkw: 300 },
  geometry: {
    type: "LineString",
    coordinates: [
      [0, 0],
      [100, 0],
    ],
  },
};

const RUN_SPEC = {
  standardId: "rls19-road",
  version: "2019",
  profile: "default",
  params: { surface_type: "SMA" },
  receiverMode: "custom",
} as const;

beforeEach(async () => {
  await storage.clearPersistedState();
  window.localStorage.clear();
  resetBrowserBackendForTests();
  calls.length = 0;
  cancels = 0;
  useModelStore.setState({
    features: [ROAD],
    receivers: [
      {
        id: "R1",
        heightM: 4,
        geometry: { type: "Point", coordinates: [5, 2] },
      },
      {
        id: "R2",
        heightM: 4,
        geometry: { type: "Point", coordinates: [50, 20] },
      },
    ],
    calcArea: null,
    crs: "EPSG:4326",
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

/**
 * Resolves once the run has reached the kernel and is waiting there.
 *
 * Macrotasks rather than microtasks: `startRun` reads the persisted document
 * on its way in, and fake-indexeddb settles its requests on the task queue, so
 * draining the microtask queue never gets past the store.
 */
async function pendingCall(): Promise<KernelCall> {
  for (let attempt = 0; attempt < 100 && calls.length === 0; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  const call = calls[0];
  if (!call) throw new Error("the run never reached the kernel");
  return call;
}

describe("browserBackend.startRun progress", () => {
  it("hands the caller's reporter to the kernel", async () => {
    const reported: [number, number][] = [];
    const run = browserBackend.startRun(RUN_SPEC, {
      onProgress: (done, total) => {
        reported.push([done, total]);
      },
    });

    const call = await pendingCall();
    // The same function, not a wrapper: nothing between the UI and the worker
    // needs to rewrite what the kernel counts, and a layer that did would be
    // a second opinion about what "done" means.
    expect(call.onProgress).toBeTypeOf("function");
    call.onProgress?.(1, 2);
    expect(reported).toEqual([[1, 2]]);

    call.finish();
    await expect(run).rejects.toThrow();
  });

  it("asks for no reporter when the caller passed none", async () => {
    const run = browserBackend.startRun(RUN_SPEC);
    const call = await pendingCall();
    expect(call.onProgress).toBeUndefined();

    call.finish();
    await expect(run).rejects.toThrow();
  });
});

describe("browserBackend.startRun cancellation", () => {
  it("terminates the kernel and writes nothing when the signal aborts", async () => {
    const controller = new AbortController();
    const run = browserBackend.startRun(RUN_SPEC, {
      signal: controller.signal,
    });
    await pendingCall();

    controller.abort();

    await expect(run).rejects.toSatisfy(isRunCancelled);
    expect(cancels).toBe(1);
    // The whole point of cancelling inside the store lock: the run id was
    // minted, and nothing else happened. No run, no artifact bytes, no log —
    // and `run_count` agrees, so the project does not report a run the list
    // cannot show.
    await expect(browserBackend.getRuns()).resolves.toEqual([]);
    await expect(storage.listArtifactBytesIDs()).resolves.toEqual([]);
    const status = await browserBackend.getProjectStatus();
    expect(status.run_count).toBe(0);
  });

  it("keeps nothing when the abort lands after the compute but before the save", async () => {
    const controller = new AbortController();
    const saving = vi.spyOn(storage, "saveArtifactBytes");
    const run = browserBackend.startRun(RUN_SPEC, {
      signal: controller.signal,
    });
    const call = await pendingCall();

    // Order matters, and it is the order of the bug: the kernel answers
    // first, so cancelling afterwards terminates a worker that is already
    // idle and rejects nothing. Aborting before `succeed()` would instead be
    // the stub's `cancelKernel` rejecting the pending call, which is the case
    // above — and would pass whether or not this side checks anything.
    call.succeed();
    controller.abort();

    await expect(run).rejects.toSatisfy(isRunCancelled);
    await expect(browserBackend.getRuns()).resolves.toEqual([]);
    await expect(storage.listArtifactBytesIDs()).resolves.toEqual([]);
    const status = await browserBackend.getProjectStatus();
    expect(status.run_count).toBe(0);

    // And the raster was never written in the first place. The guard after
    // the save would have cleaned it up either way, so this is what makes
    // the earlier one worth having: on a real grid that write is tens of
    // megabytes into IndexedDB for a run the user has already abandoned.
    expect(saving).not.toHaveBeenCalled();
  });

  it("leaves no orphaned raster behind when the abort lands during the save", async () => {
    const controller = new AbortController();
    // The raster write is the wide one — tens of megabytes on a real grid —
    // so it is the step most likely to be interrupted. Aborting inside it
    // leaves bytes already in the store, which nothing would ever name again.
    const saving = vi
      .spyOn(storage, "saveArtifactBytes")
      .mockImplementation(async (id, bytes) => {
        controller.abort();
        saving.mockRestore();
        await storage.saveArtifactBytes(id, bytes);
      });

    const run = browserBackend.startRun(RUN_SPEC, {
      signal: controller.signal,
    });
    const call = await pendingCall();
    call.succeed();

    await expect(run).rejects.toSatisfy(isRunCancelled);
    await expect(storage.listArtifactBytesIDs()).resolves.toEqual([]);
    await expect(browserBackend.getRuns()).resolves.toEqual([]);
  });

  it("refuses a run whose signal is already aborted, without loading anything", async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(
      browserBackend.startRun(RUN_SPEC, { signal: controller.signal }),
    ).rejects.toSatisfy(isRunCancelled);
    // Nothing to terminate, so nothing was terminated: a pre-aborted signal
    // must not tear down a kernel another run may be using.
    expect(cancels).toBe(0);
    expect(calls).toEqual([]);
  });

  it("stops listening once the run has settled", async () => {
    const controller = new AbortController();
    const run = browserBackend.startRun(RUN_SPEC, {
      signal: controller.signal,
    });
    const call = await pendingCall();
    call.finish();
    await expect(run).rejects.toThrow();

    // A signal outlives the run it was given to — one component, many runs —
    // so a listener left behind would let a stale abort terminate the kernel
    // a *later* run is computing in.
    controller.abort();
    expect(cancels).toBe(0);
  });

  it("releases the store lock, so the next run gets the id the cancelled one minted", async () => {
    const controller = new AbortController();
    const cancelled = browserBackend.startRun(RUN_SPEC, {
      signal: controller.signal,
    });
    await pendingCall();
    controller.abort();
    await expect(cancelled).rejects.toSatisfy(isRunCancelled);

    const next = browserBackend.startRun(RUN_SPEC);
    const call = await pendingCall();
    call.onProgress?.(0, 0);
    // Completing this one is the other file's business; what matters here is
    // that it reached the kernel at all — a lock the rejected body failed to
    // release would have deadlocked it.
    expect(call.request.receivers).toHaveLength(2);
    call.finish();
    await expect(next).rejects.toThrow();
  });
});
