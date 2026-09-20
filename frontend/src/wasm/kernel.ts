// The Aconiq WASM computation kernel, as the app sees it.
//
// Usage:
//   const kernel = await getKernel();
//   const outputs = await kernel.rls19Road({ receivers, sources, barriers });
//
// The kernel runs in a Web Worker (`kernel.worker.ts`), and there is exactly
// one production path into it. There is deliberately no main-thread fallback:
// `cmd/wasm/main.go` computes inside a Promise executor, which runs
// synchronously, so a main-thread kernel does not yield for the length of a
// run and freezes the tab — the defect this file exists to have fixed. A
// second implementation kept "for tests" would be a third reading of what the
// kernel does, exercised by nothing, and no test needs one: every suite either
// mocks this module or imports `kernel-node.ts` directly.

import type { StandardDescriptor } from "@/standards/descriptor";
import { connectKernel, type KernelClient } from "./kernel-client";
import { RunDiagnostics, describeRun } from "./kernel-diagnostics";
import { ShardProgress, mergeChunks, poolSize } from "./kernel-pool";
import type { KernelProgressListener } from "./protocol";
import { spawnKernelWorker } from "./spawn-worker";
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

export type { KernelProgressListener } from "./protocol";

export interface AconiqKernel {
  /**
   * Compute RLS-19 road traffic noise levels for all receivers.
   *
   * `onProgress` is called with how many receivers are done out of how many
   * there are, throttled by the worker. It is optional and ignored by the
   * kernels that cannot report — including today's WASM build, whose Go
   * export still takes exactly one argument.
   */
  rls19Road(
    req: ComputeRequest,
    onProgress?: KernelProgressListener,
  ): Promise<ReceiverOutput[]>;
  /**
   * Project a batch of coordinates between two CRS.
   *
   * This is the frontend's only way into a map projection, and deliberately
   * so: the kernel already links `internal/geo` in, and a second
   * transverse-Mercator implementation in TypeScript would be a second answer
   * to where a model sits. See `@/model/compute-crs`, which is the only caller.
   */
  transform(req: TransformRequest): Promise<TransformResponse>;
  /**
   * Trace ISO-band contour lines over a run's raster.
   *
   * `payload` is the raw `.bin` the run wrote — band-major, row-major within a
   * band, little-endian float64 — and `req.raster` is the `.json` sidecar that
   * says how to read it. The values stay out of the JSON deliberately: a
   * 500x500 two-band grid is four megabytes of float64, and rendering those as
   * JSON numbers and parsing them back would cost more than the marching
   * squares does. Nothing here decodes the bytes; Go does, through the one
   * decoder `aconiq export` uses, so the browser never carries a second reading
   * of the byte contract.
   *
   * Those same four megabytes are *transferred* to the worker rather than
   * copied, so `payload`'s buffer is detached when this returns. The one
   * caller reads it fresh out of IndexedDB for this call and never looks at it
   * again; a caller that needs to keep its bytes must pass a copy.
   *
   * Every band is traced in one call and told apart by
   * {@link ContourLine.band_name}; a caller showing one band filters rather
   * than asking again.
   *
   * The tracing itself is `internal/report/contour`, which
   * `GET /api/v1/runs/{id}/contours` and `aconiq export --format
   * contour-geojson` also call — where a 55 dB line falls is read as an
   * assessment, so the three must not be able to disagree.
   */
  contours(payload: Uint8Array, req: ContourRequest): Promise<ContourResult>;
  /**
   * The standards this kernel can actually run, in the same shape `GET
   * /api/v1/standards` answers with.
   *
   * It comes from the kernel rather than from a hardcoded list so that the
   * evidence tier, the parameter defaults and the surface enum are declared
   * once, by the Go module. The hardcoded copy this replaced had drifted twice.
   *
   * A Promise despite answering from a cache: the worker ships the list with
   * its handshake, so this costs no round trip, but one calling convention for
   * the whole interface is worth more than saving an `await` at two call
   * sites that are already inside `await getKernel()`.
   */
  standards(): Promise<StandardDescriptor[]>;
  /**
   * Load a GeoTIFF DTM into the kernel, replacing whatever was loaded before.
   *
   * `crs` names the CRS the raster's own coordinates are in, and it is
   * required. The Go loader reads the tie point and the pixel scale and no
   * GeoKeyDirectory, so the file does not say; and a {@link ComputeRequest}
   * carries bare numbers, so the request does not say either. Without it every
   * elevation query would be a query into an unknown CRS — which is how a DTM
   * in degrees comes to be asked about metres, miss on every lookup, and be
   * read as sea level.
   *
   * A run over a loaded terrain must therefore also send
   * {@link ComputeRequest.projection}; the kernel refuses it otherwise.
   *
   * The bytes are retained, not consumed: a worker that is terminated and
   * respawned is reloaded with the last DTM, so a cancelled run does not
   * silently drop the terrain out from under the next one.
   */
  loadTerrain(data: Uint8Array, crs: string): Promise<TerrainInfo>;
  /** Drop the loaded terrain. Runs afterwards compute without one. */
  clearTerrain(): Promise<void>;
  /** Return the default PropagationConfig. */
  defaultConfig(): Promise<PropagationConfig>;
}

// Singleton promise — the kernel is only loaded once.
let kernelPromise: Promise<AconiqKernel> | null = null;

/**
 * The live client, when there is one. Held beside {@link kernelPromise}
 * because cancelling has to reach the worker *now*, not after whatever
 * `kernelPromise` is still waiting for.
 */
let activeClient: KernelClient | null = null;

/**
 * The extra workers a run is currently spread across, beside
 * {@link activeClient}.
 *
 * They live for one run: spawned when a run turns out to be big enough to be
 * worth splitting, terminated in the `finally` after it. Holding four
 * instantiated Go heaps open between runs would cost tens of megabytes each
 * for a user who may never start another, and the browser's compiled-module
 * cache makes the respawn cheap -- it is Go runtime init, not a 4.6 MB fetch.
 *
 * They are tracked at module scope rather than inside the run so that
 * {@link cancelKernel} can reach them: terminating is the only way to stop a
 * WASM computation, and a cancel that stopped only the primary would leave
 * three workers burning a core each until they finished work nobody wants.
 */
let poolExtras: KernelClient[] = [];

/**
 * The last DTM handed to {@link AconiqKernel.loadTerrain}, kept so a respawned
 * worker can be given it again.
 *
 * A terrain is not part of a request — it is state inside the WASM module —
 * so terminating the worker would otherwise lose it, and the next run would
 * quietly compute over sea level. That is precisely the "miss every lookup and
 * read as flat ground" failure `loadTerrain`'s CRS argument exists to prevent,
 * arriving by another route.
 */
let retainedTerrain: { data: Uint8Array; crs: string } | null = null;

export function getKernel(): Promise<AconiqKernel> {
  if (kernelPromise === null) {
    const loading = loadKernel();
    kernelPromise = loading;
    // A kernel that failed to load must not be the answer for the rest of the
    // session. `kernelPromise` is the *promise*, so without this a rejected
    // load stays cached and every later getKernel() re-reads the same
    // rejection: one blip fetching four megabytes of WASM and browser mode
    // never recovers. Clearing it lets the next caller try again.
    void loading.catch(() => {
      if (kernelPromise === loading) kernelPromise = null;
    });
  }

  return kernelPromise;
}

/**
 * Drop a kernel that died on its own, so the next {@link getKernel} builds a
 * new one.
 *
 * A `KernelClient` that has failed rejects every later call with the reason it
 * failed for, so holding on to one is holding on to a broken browser mode.
 * Unlike {@link cancelKernel} this does not pre-warm a replacement: nobody
 * asked for the teardown, so there is no reason to think a run is coming, and
 * a worker that died once may well die again on sight.
 */
function releaseLostKernel(lost: KernelClient): void {
  if (activeClient !== lost) return;

  activeClient = null;
  kernelPromise = null;
}

/**
 * Tear the kernel down, failing everything in flight, and start a fresh one.
 *
 * Terminating the worker is the only way to stop a WASM computation: the
 * module has one stack and no cancellation point, so a run ends when its
 * thread does. Every pending call rejects with an `Error` named
 * `KernelCancelled`.
 *
 * A replacement is started immediately rather than on the next `getKernel()`,
 * because instantiating the module takes long enough to be felt and the user
 * who just cancelled a run is the user most likely to start another.
 */
export function cancelKernel(): void {
  const client = activeClient;
  const wasLoaded = kernelPromise !== null;

  activeClient = null;
  kernelPromise = null;
  client?.cancel();
  releasePoolExtras();

  if (!wasLoaded) return;

  // Pre-warm. The `catch` is not error handling — whoever awaits `getKernel()`
  // sees the rejection — it only keeps a respawn nobody has asked for yet from
  // surfacing as an unhandled rejection.
  const warm = loadKernel();
  void warm.catch(() => undefined);
  kernelPromise = warm;
}

async function loadKernel(): Promise<AconiqKernel> {
  if (typeof Worker === "undefined") {
    throw new Error(
      "The Aconiq kernel runs in a Web Worker, and this environment has none. Browser mode needs a browser; a test wanting the real kernel loads it with getNodeKernel() from src/wasm/kernel-node.ts.",
    );
  }

  const client = await connectKernel(spawnKernelWorker());
  activeClient = client;
  client.onLost(() => {
    releaseLostKernel(client);
  });

  // Replay whatever terrain the previous worker held. Done before the kernel
  // is handed out so that no run can observe the gap.
  if (retainedTerrain !== null) {
    await client.loadTerrain(retainedTerrain.data, retainedTerrain.crs);
  }

  return retainingTerrain(client);
}

/**
 * The client, with the terrain bookkeeping {@link retainedTerrain} needs.
 *
 * A wrapper rather than state inside `KernelClient`, because the retention has
 * to outlive the client it was recorded against — that is the whole point of
 * it.
 */
function retainingTerrain(client: KernelClient): AconiqKernel {
  return {
    rls19Road: (req, onProgress) => runOverPool(client, req, onProgress),
    transform: (req) => client.transform(req),
    contours: (payload, req) => client.contours(payload, req),
    standards: () => client.standards(),
    defaultConfig: () => client.defaultConfig(),
    async loadTerrain(data, crs) {
      const info = await client.loadTerrain(data, crs);
      retainedTerrain = { data, crs };
      return info;
    },
    async clearTerrain() {
      await client.clearTerrain();
      retainedTerrain = null;
    },
  };
}

/**
 * Compute a run, spread across a Worker pool when it is large enough to pay
 * for one.
 *
 * Below the threshold this is `client.rls19Road(req, onProgress)` and nothing
 * else — byte for byte the path every run took before the pool existed, which
 * is what keeps a small run from paying for machinery it cannot use.
 *
 * Above it, every worker receives the *whole* request plus its
 * `{index, count}` and derives its own receiver window. That is deliberate
 * and not an oversight: `wasmkernel.ComputeRLS19RoadShard` explains why a
 * shard handed only its own slice would compute over different ground.
 *
 * The extras are terminated in the `finally`, including when the run throws,
 * so a failed or cancelled run does not leave workers computing results
 * nobody will read.
 */
async function runOverPool(
  client: KernelClient,
  req: ComputeRequest,
  onProgress?: KernelProgressListener,
): Promise<ReceiverOutput[]> {
  const size = poolSize(navigator.hardwareConcurrency, req.receivers.length);

  // Started before the pool is, so the elapsed time the console reports is
  // the time the user has actually been waiting — worker startup and scene
  // preparation included. Those are invisible to the progress bar, and on a
  // large model they are a noticeable part of the wait.
  const diagnostics = new RunDiagnostics(describeRun(req, size));

  // The kernel is now asked to report progress whether or not the caller
  // wanted a bar, because the diagnostics are a consumer in their own right:
  // a run with no bar is exactly the one where the console is all there is.
  // The cost is bounded by `kernel.worker.ts`'s throttle — about ten messages
  // a second carrying two integers.
  const report: KernelProgressListener = (done, total) => {
    diagnostics.progress(done, total);
    onProgress?.(done, total);
  };

  try {
    const outputs = await computeOverPool(client, req, size, report);
    diagnostics.finished();

    return outputs;
  } catch (cause) {
    diagnostics.failed(cause);

    throw cause;
  }
}

/** {@link runOverPool} without the narration: pick a pool, use it, merge. */
async function computeOverPool(
  client: KernelClient,
  req: ComputeRequest,
  size: number,
  report: KernelProgressListener,
): Promise<ReceiverOutput[]> {
  const total = req.receivers.length;

  if (size <= 1) {
    return client.rls19Road(req, report);
  }

  // Zero of n once, for the run — not once per shard. Each shard reporting
  // its own zero would make the bar jump as the others checked in. Same
  // reasoning as the single-worker path in `KernelClient.rls19Road`, which is
  // why that one does it and the shard call does not.
  report(0, total);

  const progress = new ShardProgress(total, report);

  let clients: KernelClient[];

  try {
    clients = [client, ...(await growPool(size - 1))];
  } catch {
    // A pool that will not spawn is not a failed run. One worker is already
    // there and can do the whole thing, slower.
    return client.rls19Road(req, report);
  }

  try {
    const settled = await Promise.all(
      clients.map((shardClient, index) =>
        shardClient.rls19RoadShard(
          req,
          { index, count: clients.length },
          progress.forShard(index),
        ),
      ),
    );

    return mergeChunks(settled.flat(), total);
  } finally {
    releasePoolExtras();
  }
}

/**
 * Which generation of the pool is current.
 *
 * {@link releasePoolExtras} bumps it. A {@link growPool} still in flight
 * compares the value it started under against the current one and throws its
 * workers away if they no longer belong to anybody — without it, a
 * `cancelKernel()` during startup clears the list and a late-completing
 * `growPool` then republishes workers the cancellation can no longer reach.
 */
let poolGeneration = 0;

/**
 * Start `count` extra workers, each carrying whatever terrain the primary
 * holds.
 *
 * The terrain replay is not optional. A DTM lives inside a worker's own
 * linear memory, so an extra that skipped it would compute its share over sea
 * level while the primary computed over real ground — and the two halves of
 * the raster would disagree with nothing to say why.
 *
 * # Nothing started here may escape unreferenced
 *
 * `Promise.allSettled` rather than `Promise.all`, and it matters. `all`
 * rejects on the first failure while its siblings keep running, so a worker
 * that connects a moment later lands in no list anybody holds: it cannot be
 * terminated, and its Go heap stays until the tab closes. Repeated partial
 * failures then leak whole WASM instances — and since the likeliest reason a
 * spawn failed in the first place is memory pressure, that is the worst
 * possible thing to respond with. `allSettled` waits for every outcome, so by
 * the time the failure is inspected `started` is complete and every one of
 * them can be cancelled.
 *
 * The same reasoning covers the cancellation window, via
 * {@link poolGeneration}: the workers are published to `poolExtras` only
 * after both checks pass, and thrown away if a cancel arrived meanwhile.
 */
async function growPool(count: number): Promise<KernelClient[]> {
  const generation = poolGeneration;
  const started: KernelClient[] = [];

  const outcomes = await Promise.allSettled(
    Array.from({ length: count }, async () => {
      const extra = await connectKernel(spawnKernelWorker());

      // Recorded before the replay, so a terrain failure still leaves the
      // worker reachable for the cleanup below.
      started.push(extra);

      if (retainedTerrain !== null) {
        await extra.loadTerrain(retainedTerrain.data, retainedTerrain.crs);
      }
    }),
  );

  const failure = outcomes.find((outcome) => outcome.status === "rejected");
  const cancelled = poolGeneration !== generation;

  if (failure !== undefined || cancelled) {
    for (const extra of started) {
      extra.cancel();
    }

    if (failure !== undefined) {
      // A rejection is not guaranteed to be an Error — connectKernel rejects
      // with one, but nothing in the type system says so.
      throw failure.reason instanceof Error
        ? failure.reason
        : new Error(String(failure.reason));
    }

    throw new Error(
      "kernel pool: the run was cancelled while the pool started",
    );
  }

  poolExtras = [...poolExtras, ...started];

  return started;
}

/**
 * Terminate every extra worker and forget them.
 *
 * Bumping the generation is what makes this reach a pool that has not
 * finished starting yet; see {@link poolGeneration}.
 */
function releasePoolExtras(): void {
  const extras = poolExtras;
  poolExtras = [];
  poolGeneration += 1;

  for (const extra of extras) {
    extra.cancel();
  }
}
