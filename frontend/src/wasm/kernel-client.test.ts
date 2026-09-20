// The RPC layer between the app and the kernel worker, driven without one.
//
// jsdom has no `Worker`, and even in a browser the real one would make these
// assertions untestable: the point is not that the kernel computes — the
// parity suites pin that against the real WASM module — but that the plumbing
// around it cannot lose a call. A fake port makes every failure mode reachable
// in a line: a reply out of order, a worker that dies, a cancellation.
//
// Three of these guard bugs that would be invisible in CI and obvious to a
// user: a mis-correlated reply gives one run another run's levels, a lost
// error string draws an empty red callout, and an unsettled promise is a run
// that never finishes and never fails.

import { describe, expect, it } from "vitest";

import {
  CancelledErrorName,
  connectKernel,
  KernelClient,
  type KernelWorkerEvent,
  type KernelWorkerPort,
  WorkerLostErrorName,
} from "./kernel-client";
import type { KernelCall, WorkerMessage } from "./protocol";

type Listener = (event: KernelWorkerEvent) => void;

/** A `Worker` with the wire cut: it records what it was sent, and says what the test tells it to. */
class FakePort implements KernelWorkerPort {
  readonly sent: KernelCall[] = [];
  readonly transferred: Transferable[][] = [];
  terminated = 0;
  private readonly listeners = new Map<string, Listener[]>();

  postMessage(message: KernelCall, transfer: Transferable[]): void {
    this.sent.push(message);
    this.transferred.push(transfer);
  }

  addEventListener(type: string, listener: Listener): void {
    const bucket = this.listeners.get(type) ?? [];
    bucket.push(listener);
    this.listeners.set(type, bucket);
  }

  terminate(): void {
    this.terminated += 1;
  }

  /** Deliver a worker → client message. */
  emit(message: WorkerMessage): void {
    for (const listener of this.listeners.get("message") ?? []) {
      listener({ data: message });
    }
  }

  /** Raise the worker's `error` event, as a script that failed to load does. */
  emitError(message: string): void {
    for (const listener of this.listeners.get("error") ?? []) {
      listener({ message });
    }
  }

  /** The handshake, with whatever `standards()` and `defaultConfig()` answered. */
  emitReady(standards = "[]", defaultConfig = "{}"): void {
    this.emit({ kind: "ready", standards, defaultConfig });
  }
}

/** A connected client plus the port behind it, handshake already done. */
async function connected(
  standards = "[]",
  defaultConfig = "{}",
): Promise<{ port: FakePort; client: KernelClient }> {
  const port = new FakePort();
  const pending = connectKernel(port);
  port.emitReady(standards, defaultConfig);
  return { port, client: await pending };
}

const emptyRequest = { receivers: [], sources: [], barriers: [] };

describe("kernel worker handshake", () => {
  it("answers standards and defaultConfig from the handshake, without a round trip", async () => {
    const { port, client } = await connected(
      '[{"id":"rls19-road"}]',
      '{"SegmentLengthM":10}',
    );

    expect(await client.standards()).toEqual([{ id: "rls19-road" }]);
    expect(await client.defaultConfig()).toEqual({ SegmentLengthM: 10 });
    // Both answered from the cache: nothing was posted to the worker at all.
    expect(port.sent).toEqual([]);
  });

  it("rejects rather than hanging when the worker cannot start", async () => {
    const port = new FakePort();
    const pending = connectKernel(port);
    port.emit({
      kind: "fatal",
      error: "Failed to load /Aconiq/aconiq.wasm. Run `just wasm-build`",
    });

    await expect(pending).rejects.toThrow(
      "Failed to load /Aconiq/aconiq.wasm. Run `just wasm-build`",
    );
  });

  it("rejects the handshake when the worker script itself fails", async () => {
    const port = new FakePort();
    const pending = connectKernel(port);
    port.emitError("Failed to fetch dynamically imported module");

    // A worker whose script 404s raises `error` and never handshakes. Without
    // this listener `getKernel()` would stay pending forever, which is the one
    // failure a user cannot tell apart from a slow computation.
    await expect(pending).rejects.toThrow(
      "Failed to fetch dynamically imported module",
    );
  });
});

describe("kernel worker call correlation", () => {
  it("keeps concurrent calls apart when their replies come back out of order", async () => {
    const { port, client } = await connected();

    const first = client.rls19Road(emptyRequest);
    const second = client.transform({
      source_crs: "EPSG:4326",
      target_crs: "auto",
      coordinates: [],
    });

    const [firstCall, secondCall] = port.sent;
    expect(firstCall?.method).toBe("rls19Road");
    expect(secondCall?.method).toBe("transform");
    expect(firstCall?.id).not.toBe(secondCall?.id);

    // Second first, on purpose: the worker settles calls in whatever order
    // they finish, and a client that assumed FIFO would hand each caller the
    // other one's answer — a wrong noise map with no error anywhere.
    port.emit({
      kind: "result",
      id: secondCall?.id ?? -1,
      ok: true,
      value:
        '{"source_crs":"EPSG:4326","target_crs":"EPSG:25832","applied":true,"coordinates":[]}',
    });
    port.emit({
      kind: "result",
      id: firstCall?.id ?? -1,
      ok: true,
      value:
        '[{"Receiver":{"id":"r1"},"Indicators":{"lr_day":55,"lr_night":45}}]',
    });

    await expect(second).resolves.toMatchObject({ target_crs: "EPSG:25832" });
    await expect(first).resolves.toMatchObject([
      { Indicators: { lr_day: 55, lr_night: 45 } },
    ]);
  });

  it("routes progress to the call that asked for it without settling it", async () => {
    const { port, client } = await connected();

    const seen: [number, number][] = [];
    const call = client.rls19Road(emptyRequest, (done, total) => {
      seen.push([done, total]);
    });
    const id = port.sent[0]?.id ?? -1;

    // `progress: true` is what tells the worker it may hand Go a callback.
    expect(port.sent[0]).toMatchObject({ method: "rls19Road", progress: true });

    port.emit({ kind: "progress", id, done: 40, total: 100 });
    port.emit({ kind: "progress", id, done: 100, total: 100 });
    port.emit({ kind: "result", id, ok: true, value: "[]" });

    await expect(call).resolves.toEqual([]);
    expect(seen).toEqual([
      [40, 100],
      [100, 100],
    ]);
  });

  it("asks for no progress when the caller passed no listener", async () => {
    const { port, client } = await connected();

    const call = client.rls19Road(emptyRequest);
    const id = port.sent[0]?.id ?? -1;
    // The Go export still refuses a second argument, so the worker must be
    // told not to pass one.
    expect(port.sent[0]).toMatchObject({ progress: false });

    port.emit({ kind: "result", id, ok: true, value: "[]" });
    await expect(call).resolves.toEqual([]);
  });

  it("transfers the contour payload instead of copying it", async () => {
    const { port, client } = await connected();

    const payload = new Uint8Array(64);
    const call = client.contours(payload, {
      raster: { width: 1, height: 1, bands: 1, nodata: -999, units: {} },
      target_crs: "EPSG:25832",
    });

    const sent = port.sent[0];
    expect(sent?.method).toBe("contours");
    // Four megabytes of float64 is exactly what structured clone should not be
    // duplicating, so the buffer travels in the transfer list.
    expect(port.transferred[0]).toEqual([payload.buffer]);

    port.emit({
      kind: "result",
      id: sent?.id ?? -1,
      ok: true,
      value: '{"crs":"EPSG:25832","interval":5,"lines":[]}',
    });
    await expect(call).resolves.toMatchObject({ interval: 5 });
  });

  it("copies the terrain bytes rather than transferring them", async () => {
    const { port, client } = await connected();

    const dtm = new Uint8Array(16);
    const call = client.loadTerrain(dtm, "EPSG:25832");

    // `kernel.ts` retains these bytes so a respawned worker can be given the
    // DTM again; transferring would detach the copy it is retaining.
    expect(port.transferred[0]).toEqual([]);
    expect(dtm.byteLength).toBe(16);

    port.emit({
      kind: "result",
      id: port.sent[0]?.id ?? -1,
      ok: true,
      value: '{"bounds":[0,0,1,1],"pixel_size":[1,1],"grid_size":[2,2]}',
    });
    await expect(call).resolves.toMatchObject({ grid_size: [2, 2] });
  });
});

describe("kernel worker error fidelity", () => {
  it("rebuilds Go's refusal as an Error carrying the sentence verbatim", async () => {
    const { port, client } = await connected();

    // Verbatim matters: this is `geo.ComputeCRSForGeographic`'s refusal, which
    // `aconiq run` prints word for word, and browser mode must refuse a site
    // in the same words. It is also why it crosses as a string — Go rejects
    // with a bare string, not an Error, and an Error does not clone reliably.
    const refusal =
      "model spans UTM zones 32 and 33; set an explicit project CRS";

    const call = client.transform({
      source_crs: "EPSG:4326",
      target_crs: "auto",
      coordinates: [],
    });
    port.emit({
      kind: "result",
      id: port.sent[0]?.id ?? -1,
      ok: false,
      error: refusal,
    });

    // `instanceof Error` and `.message`, both: `run/setup-dialog.tsx` renders
    // `{error.message}`, and a bare string there draws an empty red callout —
    // which is what browser mode did before the kernel moved into a worker.
    await expect(call).rejects.toBeInstanceOf(Error);
    await expect(call).rejects.toThrow(refusal);
    await call.catch((cause: unknown) => {
      expect((cause as Error).message).toBe(refusal);
    });
  });
});

describe("kernel worker teardown", () => {
  it("rejects everything in flight when the worker dies", async () => {
    const { port, client } = await connected();

    const first = client.rls19Road(emptyRequest);
    const second = client.transform({
      source_crs: "EPSG:4326",
      target_crs: "auto",
      coordinates: [],
    });

    port.emitError("worker terminated unexpectedly");

    for (const call of [first, second]) {
      await expect(call).rejects.toMatchObject({
        name: WorkerLostErrorName,
        message: "worker terminated unexpectedly",
      });
    }

    // And a call made afterwards fails immediately rather than posting into a
    // worker that is gone.
    await expect(client.rls19Road(emptyRequest)).rejects.toMatchObject({
      name: WorkerLostErrorName,
    });
  });

  it("cancels by terminating, and says so", async () => {
    const { port, client } = await connected();

    const call = client.rls19Road(emptyRequest);
    client.cancel();

    await expect(call).rejects.toMatchObject({ name: CancelledErrorName });
    // Terminating is the only way to interrupt a WASM call: the module has one
    // stack and no cancellation point.
    expect(port.terminated).toBeGreaterThan(0);
  });

  it("drops a reply that arrives after its call was cancelled", async () => {
    const { port, client } = await connected();

    const call = client.rls19Road(emptyRequest);
    const id = port.sent[0]?.id ?? -1;
    client.cancel();
    await expect(call).rejects.toMatchObject({ name: CancelledErrorName });

    // Ids are never reused, so a late reply has nothing to resolve. It must
    // not throw on the way past either.
    expect(() => {
      port.emit({ kind: "result", id, ok: true, value: "[]" });
    }).not.toThrow();
  });
});
