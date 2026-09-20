// The main-thread half of the worker RPC: an `AconiqKernel` backed by
// `postMessage`.
//
// It is deliberately separate from `kernel.ts`, which owns the singleton and
// the terrain it replays, and from `spawn-worker.ts`, which owns the one
// `new Worker(...)` expression. What is left here is the part worth testing on
// its own: correlating replies with calls, turning Go's strings back into
// Errors, and making sure a worker that dies takes every pending promise with
// it instead of leaving them hanging.
//
// That last one is why this file exists at all. A `Map` of pending promises is
// a leak waiting to happen: every path out of the map — a reply, a worker
// error, a cancellation — has to settle its entry, and the one that does not
// is a run that never finishes and never fails.

import type { StandardDescriptor } from "@/standards/descriptor";
import type { AconiqKernel } from "./kernel";
import { reviveKernelError } from "./kernel-error";
import type {
  KernelCall,
  KernelProgressListener,
  WorkerMessage,
} from "./protocol";
import type {
  ComputeRequest,
  ContourRequest,
  ContourResult,
  PropagationConfig,
  ReceiverOutput,
  TerrainInfo,
  TransformRequest,
  TransformResponse,
} from "./types";

/**
 * The slice of `Worker` this client uses.
 *
 * Narrow on purpose: a test drives the client with a handful of lines rather
 * than a `Worker` polyfill, and jsdom has no `Worker` to polyfill from. A real
 * `Worker` satisfies it structurally, so `kernel.ts` passes one straight in.
 */
export interface KernelWorkerPort {
  /**
   * `transfer` is required rather than optional so that a real `Worker`'s
   * overload set matches this one signature. Pass `[]` when nothing moves.
   */
  postMessage(message: KernelCall, transfer: Transferable[]): void;
  addEventListener(
    type: "message" | "error" | "messageerror",
    listener: (event: KernelWorkerEvent) => void,
  ): void;
  terminate(): void;
}

/**
 * The two event shapes the client reads, in one type.
 *
 * `MessageEvent` carries `data` and `ErrorEvent` carries `message`; the client
 * registers a separate listener per event name, so each handler already knows
 * which of the two it has.
 */
export interface KernelWorkerEvent {
  readonly data?: WorkerMessage;
  readonly message?: string;
}

/** `Error.name` when {@link KernelClient.cancel} tore the worker down. */
export const CancelledErrorName = "KernelCancelled";

/** `Error.name` when the worker died on its own. */
export const WorkerLostErrorName = "KernelWorkerLost";

interface PendingCall {
  resolve: (value: string | null) => void;
  reject: (cause: Error) => void;
  onProgress?: KernelProgressListener;
}

function namedError(name: string, message: string): Error {
  const error = new Error(message);
  error.name = name;
  return error;
}

/**
 * The buffer to put on the wire for a view, without copying when we do not
 * have to.
 *
 * `postMessage` transfers whole `ArrayBuffer`s, not views, so a `Uint8Array`
 * that is a window onto a larger buffer has to be copied first — otherwise the
 * transfer would move bytes the caller still owns.
 */
function bufferOf(view: Uint8Array): ArrayBuffer {
  const { buffer, byteOffset, byteLength } = view;
  if (
    buffer instanceof ArrayBuffer &&
    byteOffset === 0 &&
    byteLength === buffer.byteLength
  ) {
    return buffer;
  }
  return view.slice().buffer;
}

/**
 * An `AconiqKernel` whose every method is a round trip to a worker.
 *
 * Constructed by {@link connectKernel}, which does not hand it back until the
 * worker's handshake has arrived — so by the time a caller holds one, the WASM
 * module is instantiated and `standards()` and `defaultConfig()` are already
 * answerable from the handshake.
 */
export class KernelClient implements AconiqKernel {
  private readonly pending = new Map<number, PendingCall>();
  private nextID = 1;
  /**
   * Set once the client is unusable, and the reason. Every later call rejects
   * with it rather than posting into a worker that is gone.
   */
  private failure: Error | null = null;
  private standardsCache: StandardDescriptor[] = [];
  private defaultConfigCache: PropagationConfig | null = null;
  private readonly handshake: Promise<void>;
  private settleHandshake: {
    resolve: () => void;
    reject: (cause: Error) => void;
  } | null = null;

  constructor(private readonly port: KernelWorkerPort) {
    this.handshake = new Promise<void>((resolve, reject) => {
      this.settleHandshake = { resolve, reject };
    });

    port.addEventListener("message", (event) => {
      if (event.data !== undefined) this.receive(event.data);
    });
    // Both are the worker dying rather than a call failing, so they settle
    // everything at once. Without them a worker script that fails to load
    // leaves `getKernel()` pending forever — the one failure a user cannot
    // tell apart from a slow computation.
    port.addEventListener("error", (event) => {
      this.fail(
        namedError(
          WorkerLostErrorName,
          event.message ?? "the Aconiq kernel worker stopped unexpectedly",
        ),
      );
    });
    port.addEventListener("messageerror", () => {
      this.fail(
        namedError(
          WorkerLostErrorName,
          "the Aconiq kernel worker sent a message this browser could not read",
        ),
      );
    });
  }

  /** Resolves when the worker has handshaked; rejects if it never will. */
  ready(): Promise<void> {
    return this.handshake;
  }

  /**
   * Stop the worker and settle everything outstanding as cancelled.
   *
   * Terminating is the only way to interrupt a WASM call: the module has one
   * stack and no cancellation point, so a run in flight ends when its thread
   * does. `kernel.ts` spawns a replacement.
   */
  cancel(): void {
    this.fail(
      namedError(CancelledErrorName, "the Aconiq kernel run was cancelled"),
    );
  }

  private fail(cause: Error): void {
    // First one wins: a cancellation followed by the `error` event that
    // terminating may raise should still read as a cancellation.
    this.failure ??= cause;
    const reason = this.failure;

    const settle = this.settleHandshake;
    this.settleHandshake = null;
    settle?.reject(reason);

    for (const call of this.pending.values()) call.reject(reason);
    this.pending.clear();

    this.port.terminate();
  }

  private receive(message: WorkerMessage): void {
    switch (message.kind) {
      case "ready": {
        this.standardsCache = JSON.parse(
          message.standards,
        ) as StandardDescriptor[];
        this.defaultConfigCache = JSON.parse(
          message.defaultConfig,
        ) as PropagationConfig;
        const settle = this.settleHandshake;
        this.settleHandshake = null;
        settle?.resolve();
        return;
      }
      case "fatal":
        this.fail(reviveKernelError(message.error));
        return;
      case "progress": {
        // Deliberately does not delete the entry: progress is not a result.
        this.pending.get(message.id)?.onProgress?.(message.done, message.total);
        return;
      }
      case "result": {
        const call = this.pending.get(message.id);
        // No entry means the call was already settled by a cancellation. Ids
        // are never reused, so dropping the reply is the whole handling.
        if (call === undefined) return;
        this.pending.delete(message.id);
        if (message.ok) call.resolve(message.value);
        else call.reject(reviveKernelError(message.error));
        return;
      }
    }
  }

  private call(
    build: (id: number) => KernelCall,
    transfer: Transferable[],
    onProgress?: KernelProgressListener,
  ): Promise<string | null> {
    if (this.failure !== null) return Promise.reject(this.failure);

    const id = this.nextID;
    this.nextID += 1;

    return new Promise<string | null>((resolve, reject) => {
      this.pending.set(id, {
        resolve,
        reject,
        // Conditional spread because `exactOptionalPropertyTypes` is on: an
        // explicit `undefined` is not the same as an absent property.
        ...(onProgress === undefined ? {} : { onProgress }),
      });
      this.port.postMessage(build(id), transfer);
    });
  }

  /**
   * The JSON a call answered with, refusing the `null` the void methods send.
   *
   * `null` here would mean the worker replied to the wrong method, which is a
   * bug in this directory rather than anything a user did — so it says so
   * instead of reaching `JSON.parse` as the string "null".
   */
  private static json(value: string | null, method: string): string {
    if (value === null) {
      throw new Error(`the kernel worker answered ${method} with no payload`);
    }
    return value;
  }

  async rls19Road(
    req: ComputeRequest,
    onProgress?: KernelProgressListener,
  ): Promise<ReceiverOutput[]> {
    const value = await this.call(
      (id) => ({
        id,
        method: "rls19Road",
        json: JSON.stringify(req),
        progress: onProgress !== undefined,
      }),
      [],
      onProgress,
    );
    return JSON.parse(
      KernelClient.json(value, "rls19Road"),
    ) as ReceiverOutput[];
  }

  async transform(req: TransformRequest): Promise<TransformResponse> {
    const value = await this.call(
      (id) => ({ id, method: "transform", json: JSON.stringify(req) }),
      [],
    );
    return JSON.parse(
      KernelClient.json(value, "transform"),
    ) as TransformResponse;
  }

  async contours(
    payload: Uint8Array,
    req: ContourRequest,
  ): Promise<ContourResult> {
    // Transferred, not copied: the caller reads these bytes fresh out of
    // IndexedDB for this one call, and four megabytes of float64 is exactly
    // the thing structured clone should not be duplicating.
    const buffer = bufferOf(payload);
    const value = await this.call(
      (id) => ({
        id,
        method: "contours",
        json: JSON.stringify(req),
        payload: buffer,
      }),
      [buffer],
    );
    return JSON.parse(KernelClient.json(value, "contours")) as ContourResult;
  }

  standards(): Promise<StandardDescriptor[]> {
    return Promise.resolve(this.standardsCache);
  }

  defaultConfig(): Promise<PropagationConfig> {
    if (this.defaultConfigCache === null) {
      return Promise.reject(
        new Error("the Aconiq kernel worker published no default config"),
      );
    }
    return Promise.resolve(this.defaultConfigCache);
  }

  async loadTerrain(data: Uint8Array, crs: string): Promise<TerrainInfo> {
    // Copied rather than transferred, unlike `contours`: `kernel.ts` retains
    // these bytes so it can replay the DTM into a respawned worker, and a
    // transfer would detach the copy it is retaining.
    const buffer = data.slice().buffer;
    const value = await this.call(
      (id) => ({ id, method: "loadTerrain", crs, payload: buffer }),
      [],
    );
    return JSON.parse(KernelClient.json(value, "loadTerrain")) as TerrainInfo;
  }

  async clearTerrain(): Promise<void> {
    await this.call((id) => ({ id, method: "clearTerrain" }), []);
  }
}

/**
 * Attach a client to a worker and wait for its handshake.
 *
 * Rejects rather than resolving a half-built kernel: a caller that holds a
 * `KernelClient` can call `standards()` without awaiting a round trip, and
 * that is only true once the handshake has landed.
 */
export async function connectKernel(
  port: KernelWorkerPort,
): Promise<KernelClient> {
  const client = new KernelClient(port);
  await client.ready();
  return client;
}
