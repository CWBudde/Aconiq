// The wire between `kernel-client.ts` (main thread) and `kernel.worker.ts`.
//
// Nothing in here is a public API: it is one module talking to one worker, and
// both ends are in this directory so the two can be changed together. It is a
// separate file only because a message type declared in the worker would drag
// the worker's module — and with it the `new Worker` call — into the main
// bundle whenever the client imported the type.
//
// Everything crosses as JSON *strings*, exactly as the Go exports produce and
// consume them. The worker does not parse a result before sending it on: the
// client has to hold the parsed object anyway, and parsing twice to clone it
// once buys nothing. Bytes are the exception — see `payload` below.

/** The kernel entry points the worker dispatches to, by name. */
export type KernelMethod =
  | "rls19Road"
  | "rls19RoadShard"
  | "transform"
  | "contours"
  | "loadTerrain"
  | "clearTerrain";

/**
 * A call, main thread to worker. `id` correlates it with its
 * {@link KernelResult}; the client hands out monotonic ids and never reuses
 * one, so a reply that arrives after its caller gave up is dropped rather than
 * resolving somebody else's promise.
 */
export type KernelCall =
  | {
      id: number;
      method: "rls19Road";
      json: string;
      /**
       * Whether the caller passed an `onProgress`. The worker only hands Go a
       * progress callback when this is true, because the Go export still
       * refuses any second argument — see `kernel.worker.ts`.
       */
      progress: boolean;
    }
  | {
      id: number;
      method: "rls19RoadShard";
      /**
       * The whole run's request with a `shard` of `{index, count}` on it —
       * every receiver, not this shard's slice. The kernel derives the grid's
       * terrain elevation from the centroid of the list it is handed, so a
       * shard sent only its own receivers would compute over different
       * ground. See `wasmkernel.ComputeRLS19RoadShard`.
       */
      json: string;
      /** As `rls19Road`, but counting this shard's receivers. */
      progress: boolean;
    }
  | { id: number; method: "transform"; json: string }
  | {
      id: number;
      method: "contours";
      json: string;
      /**
       * The run's `.bin` payload, **transferred** rather than copied: a 500x500
       * two-band grid is four megabytes of float64, and the client owns a fresh
       * read from IndexedDB, so detaching it costs nothing.
       */
      payload: ArrayBuffer;
    }
  | {
      id: number;
      method: "loadTerrain";
      crs: string;
      /**
       * A GeoTIFF DTM, **copied** rather than transferred. The client retains
       * these bytes so it can replay the terrain into a respawned worker, and a
       * transfer would detach the very copy it needs.
       */
      payload: ArrayBuffer;
    }
  | { id: number; method: "clearTerrain" };

/**
 * The handshake. Sent once, when the WASM module is instantiated and its
 * exports are registered.
 *
 * `standards` and `defaultConfig` ride along because both Go exports are
 * synchronous and neither ever changes for the life of a worker. Shipping them
 * with the handshake means `AconiqKernel.standards()` answers from a cache
 * with no round trip, and the cache cannot outlive the worker that produced
 * it.
 */
export interface KernelReady {
  kind: "ready";
  /** `aconiq.standards()` verbatim — JSON, parsed by the client. */
  standards: string;
  /** `aconiq.defaultConfig()` verbatim — JSON, parsed by the client. */
  defaultConfig: string;
}

/**
 * The worker could not start at all: wasm_exec.js would not load, the module
 * would not instantiate, or it registered no exports.
 *
 * It exists so that a failed load is a rejection rather than a hang. Without
 * it a worker whose script 404s leaves `getKernel()` pending forever, which is
 * the one failure mode a user cannot tell apart from a slow computation.
 */
export interface KernelFatal {
  kind: "fatal";
  error: string;
}

/**
 * How far a long call has got. Throttled in the worker to roughly one message
 * per 100 ms, with the final `done === total` always sent.
 *
 * It does not settle the pending call — a progress message and a result are
 * different things, and the client keeps the entry in its map.
 */
export interface KernelProgress {
  kind: "progress";
  id: number;
  done: number;
  total: number;
}

/**
 * A settled call. `value` is the JSON the Go export returned, or `null` for
 * the ones that return nothing; `error` is Go's sentence verbatim, because
 * several of them are the exact words `aconiq run` prints and an Error does
 * not clone reliably. See `kernel-error.ts`.
 */
export type KernelResult =
  | { kind: "result"; id: number; ok: true; value: string | null }
  | { kind: "result"; id: number; ok: false; error: string };

/**
 * Everything the worker can say. Tagged uniformly with `kind` so a variant
 * added later — the progress track is the next one — cannot be mistaken for a
 * result by an older client, which ignores what it does not recognise.
 */
export type WorkerMessage =
  | KernelReady
  | KernelFatal
  | KernelProgress
  | KernelResult;

/** Called with the worker's progress reports for one `rls19Road` call. */
export type KernelProgressListener = (done: number, total: number) => void;
