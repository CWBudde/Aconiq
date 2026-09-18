/**
 * A run's result raster, fetched, coloured and placed — or why there is none.
 *
 * One hook rather than logic inside the layer component, so the whole path
 * from artifact to image is reachable by `renderHook` without a map. What is
 * left in `result-layers.tsx` is the MapLibre call, which jsdom cannot run
 * anyway.
 *
 * Every refusal has a status of its own. A raster that quietly does not appear
 * is indistinguishable from one that failed, and the two want different things
 * from the reader — so the panel says which, and the receiver circles keep
 * drawing either way.
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { backend } from "@/api/backend";
import type { RasterMetadata, RunSummary } from "@/api/client";
import { useArtifactBytes, useRasterMetadata } from "@/api/hooks";
import { NOISE_LEVEL_RAMP } from "./color-ramp";
import { DISPLAY_CRS } from "./display-model";
import { encodeRasterPNG } from "./raster-canvas";
import {
  cornerCoordinates,
  rasterCornerExtent,
  type RasterCorners,
} from "./raster-extent";
import { rasterToRGBA } from "./raster-image";
import { readRasterBand } from "@/model/raster-bin";
import { declaresLevels } from "./result-units";

/**
 * The largest raster drawn as one image.
 *
 * Not a performance number, a correctness one. An `image` source becomes a GL
 * texture, and a dimension over the device's `MAX_TEXTURE_SIZE` fails at upload
 * with nothing on screen and nothing in the panel to say so. Reading the real
 * limit means reaching into MapLibre's own canvas, and a device-dependent cap
 * would make the same project draw on one machine and not another, which this
 * app does not do anywhere else. 4096 is the floor every WebGL2 implementation
 * clears.
 *
 * It is not redundant with the receiver cap, because only one mode has one.
 * `cli.maxDummyReceivers` holds API-mode auto-grids to 250 000 cells, which no
 * square grid can breach — but `buildReceiverGrid` in `browser-backend.ts`
 * divides the calculation area by the resolution and stops there, so a long
 * thin area at a fine resolution reaches a width in the tens of thousands with
 * a height of two.
 */
const MAX_RASTER_DIMENSION = 4096;

/**
 * Whether this raster is past what one image can carry.
 *
 * Shared by the memo below and by `resolveRaster`, and that sharing is the
 * point: the cap used to live only in the resolver, so an oversized grid was
 * fully read, colourised and PNG-encoded *before* anything returned
 * `too-large`. A long thin browser grid — the case the cap exists for, since
 * only API mode has a receiver cap — allocated hundreds of megabytes to put a
 * one-line refusal on screen. A guard that runs after the work it guards is
 * not a guard.
 */
function exceedsOneImage(metadata: RasterMetadata): boolean {
  return (
    metadata.width > MAX_RASTER_DIMENSION ||
    metadata.height > MAX_RASTER_DIMENSION
  );
}

/** Placed pixels, or the reason there are none. */
export type ResultRaster =
  /** No run, or a run that wrote no raster — explicit receivers place no grid. */
  | { status: "none" }
  | { status: "loading" }
  /** The sidecar carries no georeference: the receivers were not a grid. */
  | { status: "not-a-grid" }
  /** The values are not decibels, so the noise ramp may not paint them. */
  | { status: "not-levels"; unit: string }
  /** `band_names` does not name the indicator the picker is on. */
  | { status: "no-such-band"; indicator: string }
  | { status: "too-large"; width: number; height: number }
  /** No projector in this mode — see `BackendCapabilities.canReprojectForDisplay`. */
  | { status: "unsupported"; crs: string }
  /** The sidecar never recorded which CRS its cells are in. */
  | { status: "unknown-crs" }
  | { status: "failed" }
  | {
      status: "ready";
      url: string;
      /** Top-left, top-right, bottom-right, bottom-left, in {@link DISPLAY_CRS}. */
      coordinates: RasterCorners;
    };

const NONE: ResultRaster = { status: "none" };
const LOADING: ResultRaster = { status: "loading" };

/** The corner extent in {@link DISPLAY_CRS}, or why it could not be reached. */
type PlacedCorners =
  | { status: "idle" }
  | { status: "projecting" }
  | { status: "unsupported"; crs: string }
  | { status: "unknown-crs" }
  | { status: "failed" }
  | { status: "ready"; coordinates: RasterCorners };

const CORNERS_IDLE: PlacedCorners = { status: "idle" };

/**
 * The raster's four corners in {@link DISPLAY_CRS}.
 *
 * The CRS comes off the **sidecar**, not off the run summary's `compute_crs`.
 * They agree today, and the sidecar is the authority for the raster while the
 * summary is the authority for the receiver table (`docs/result-containers-v1
 * .md`, "Raster sidecar — where the cells are"). Reading one for the other is
 * the kind of shortcut that survives until they stop agreeing.
 *
 * Four corners and not one point per cell: MapLibre maps an image onto the
 * quad affinely in Mercator, while a UTM grid's true outline in WGS84 is not a
 * Mercator-affine quad. The error is the non-linear part of Mercator's
 * 1/cos(phi) scale, which over a 1 km grid at 50°N is a taper of about
 * 1.9e-4 — 19 cm across the full width, and the interior deviates by a
 * fraction of that, against a cell of 5–20 m. Meridian convergence contributes
 * nothing at all, being linear in Mercator coordinates and therefore
 * reproduced exactly by the quad. The term grows with the square of the
 * north–south extent, so at 20 km it approaches one cell; the dimension cap
 * above bounds the ground extent with it. Projecting per cell, or adding
 * proj4, is not the answer — the frontend has exactly one projection.
 */
function usePlacedCorners(
  metadata: RasterMetadata | undefined,
  metadataPending: boolean,
): PlacedCorners {
  const canReproject = backend.capabilities.canReprojectForDisplay;
  const [state, setState] = useState<PlacedCorners>(CORNERS_IDLE);

  // Monotonic, and compared on arrival: a run switching under a slow transform
  // must not be overwritten by the answer to the question before it, which
  // would place this run's pixels on the previous run's ground.
  const requestRef = useRef(0);

  const georeference = metadata?.georeference;
  const width = metadata?.width;
  const height = metadata?.height;
  const crs = metadata?.crs ?? "";

  useEffect(() => {
    if (
      georeference === undefined ||
      width === undefined ||
      height === undefined
    ) {
      requestRef.current += 1;
      setState(CORNERS_IDLE);
      return;
    }

    let corners: RasterCorners;
    try {
      corners = cornerCoordinates(
        rasterCornerExtent(georeference, width, height),
      );
    } catch (error) {
      // A row order this build does not recognise, or a pixel size that is not
      // a size. Refused rather than guessed, as `Georeference.Validate` refuses
      // it: a grid placed upside down looks entirely plausible.
      console.error("useResultRaster: could not place the raster", error);
      requestRef.current += 1;
      setState({ status: "failed" });
      return;
    }

    if (crs === "") {
      requestRef.current += 1;
      setState(metadataPending ? CORNERS_IDLE : { status: "unknown-crs" });
      return;
    }

    // Already what MapLibre draws in. The identity short-circuit is not an
    // optimisation: `wasmkernel.resolveTarget` transforms unconditionally on an
    // explicit target, so a 4326 → 4326 request would build a projection
    // pipeline for nothing.
    if (crs === DISPLAY_CRS) {
      requestRef.current += 1;
      setState({ status: "ready", coordinates: corners });
      return;
    }

    if (!canReproject) {
      requestRef.current += 1;
      setState({ status: "unsupported", crs });
      return;
    }

    const request = (requestRef.current += 1);
    setState({ status: "projecting" });

    void backend
      .transformCoordinates({
        source_crs: crs,
        target_crs: DISPLAY_CRS,
        coordinates: corners.flat(),
      })
      .then(
        (response) => {
          if (requestRef.current !== request) return;
          setState(toCorners(response.coordinates));
        },
        () => {
          if (requestRef.current !== request) return;
          setState({ status: "failed" });
        },
      );
  }, [georeference, width, height, crs, metadataPending, canReproject]);

  return state;
}

/**
 * Pairs the flat response back into four corners, and refuses anything else.
 *
 * A short batch would place the image on a quad of the wrong shape rather than
 * report a failure, and MapLibre takes any four pairs it is given.
 */
function toCorners(coordinates: number[]): PlacedCorners {
  if (coordinates.length !== 8) return { status: "failed" };

  const pairs: [number, number][] = [];
  for (let i = 0; i < 4; i += 1) {
    const x = coordinates[i * 2];
    const y = coordinates[i * 2 + 1];
    if (x === undefined || y === undefined) return { status: "failed" };
    pairs.push([x, y]);
  }

  const [tl, tr, br, bl] = pairs;
  if (
    tl === undefined ||
    tr === undefined ||
    br === undefined ||
    bl === undefined
  ) {
    return { status: "failed" };
  }

  return { status: "ready", coordinates: [tl, tr, br, bl] };
}

/**
 * The band the picker is on, by **name**.
 *
 * Never a fallback to band 0. The picker's names come from the receiver
 * table's `indicator_order` while the bands come from the sidecar's
 * `band_names`; the two agree today and nothing enforces that they must. A
 * default would paint Lr,Nacht under the label Lr,Tag — a map that is wrong
 * and looks right, which is the failure this whole file is careful about.
 */
function bandIndex(metadata: RasterMetadata, indicator: string): number {
  return metadata.band_names?.indexOf(indicator) ?? -1;
}

/**
 * The decision, as a function of what was fetched — no hooks, so the whole
 * table of outcomes is one `expect` away and the hook below is only plumbing.
 */
function resolveRaster(input: {
  run: RunSummary | null;
  indicator: string;
  hasArtifacts: boolean;
  metadata: RasterMetadata | undefined;
  bytes: ArrayBuffer | undefined;
  loading: boolean;
  failed: boolean;
  corners: PlacedCorners;
  image: string | null | undefined;
}): ResultRaster {
  const { run, indicator, metadata, bytes, corners, image } = input;

  // Ordered as a reader needs them: no raster at all, then the ones that are
  // about the data, then the ones that are about drawing it.
  if (run === null || !input.hasArtifacts) return NONE;
  if (input.failed) return { status: "failed" };
  if (metadata === undefined) return input.loading ? LOADING : NONE;

  // Every refusal decidable from the sidecar alone is answered here, before
  // the bytes are looked at — because in two of these cases the hook
  // deliberately never asked for them, so an absent payload is the answer
  // rather than a wait.
  if (!declaresLevels(metadata.unit)) {
    return { status: "not-levels", unit: metadata.unit };
  }
  if (metadata.georeference === undefined) return { status: "not-a-grid" };
  if (exceedsOneImage(metadata)) {
    return {
      status: "too-large",
      width: metadata.width,
      height: metadata.height,
    };
  }
  if (bandIndex(metadata, indicator) < 0) {
    return { status: "no-such-band", indicator };
  }

  if (bytes === undefined) return input.loading ? LOADING : NONE;

  switch (corners.status) {
    case "idle":
    case "projecting":
      return LOADING;
    case "unsupported":
      return { status: "unsupported", crs: corners.crs };
    case "unknown-crs":
      return { status: "unknown-crs" };
    case "failed":
      return { status: "failed" };
    case "ready":
      break;
  }

  if (image === undefined) return LOADING;
  if (image === null) return { status: "failed" };

  return { status: "ready", url: image, coordinates: corners.coordinates };
}

export function useResultRaster(
  run: RunSummary | null,
  indicator: string,
): ResultRaster {
  const metadataArtifact = run?.artifacts.find(
    (artifact) => artifact.kind === "run.result.raster_metadata",
  );
  const binaryArtifact = run?.artifacts.find(
    (artifact) => artifact.kind === "run.result.raster_binary",
  );

  const {
    data: metadata,
    isLoading: metadataLoading,
    error: metadataError,
  } = useRasterMetadata(metadataArtifact?.id ?? null);
  const {
    data: bytes,
    isLoading: bytesLoading,
    error: bytesError,
  } = useArtifactBytes(binaryArtifact?.id ?? null);

  const corners = usePlacedCorners(
    metadata,
    metadataArtifact !== undefined && metadataLoading,
  );

  const image = useMemo(() => {
    if (metadata === undefined || bytes === undefined) return undefined;
    if (exceedsOneImage(metadata)) return null;

    const band = bandIndex(metadata, indicator);
    if (band < 0) return null;

    try {
      const values = readRasterBand(bytes, metadata, band);
      const rgba = rasterToRGBA({
        values,
        width: metadata.width,
        height: metadata.height,
        nodata: metadata.nodata,
        ramp: NOISE_LEVEL_RAMP,
      });
      return encodeRasterPNG(rgba, metadata.width, metadata.height);
    } catch (error) {
      // A buffer that does not match the sidecar it came with. Reported rather
      // than drawn: the bytes and the shape disagreeing means one of them is
      // from a different run.
      console.error("useResultRaster: could not colour the raster", error);
      return null;
    }
  }, [metadata, bytes, indicator]);

  const hasArtifacts =
    metadataArtifact !== undefined && binaryArtifact !== undefined;
  const failed = metadataError != null || bytesError != null;
  const loading = metadataLoading || bytesLoading;

  // Memoised because the caller feeds this straight into an effect's
  // dependency list. A fresh object literal per render would re-run that
  // effect on every render and re-upload the texture with it, which is a GL
  // buffer upload per keystroke anywhere else on the page.
  return useMemo(
    () =>
      resolveRaster({
        run,
        indicator,
        hasArtifacts,
        metadata,
        bytes,
        loading,
        failed,
        corners,
        image,
      }),
    [
      run,
      indicator,
      hasArtifacts,
      metadata,
      bytes,
      loading,
      failed,
      corners,
      image,
    ],
  );
}
