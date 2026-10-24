/**
 * A photo of a facade in the elevation view (P18 D1): drawn onto the facade
 * from four corners a person clicks in it, and recorded on the windows drawn
 * over it as a hash and those corners — never as the image.
 *
 * The homography itself is `facadeset.Rectify`'s, through the WASM kernel in
 * browser mode and `POST /api/v1/facades/rectify` in API mode; this module
 * only composes the matrix it answers with the drawing's scale, and keeps the
 * evidence record. The photo is personal data
 * (`docs/policies/data-handling.md`): it lives as an object URL in the open
 * view and nowhere else — not in the model, not in the store, not in a
 * request.
 */

import type { ModelFeature } from "@/model/types";
import type { PhotoTransform, Rectification } from "@/wasm/types";
import type { ElevationScale } from "./elevation-scale";

/** What a person records about a photo besides its corners. */
export interface PhotoRecord {
  /** An opaque reference into the engagement's photo store, never a path. */
  sourceId: string;
  /** The photographer, the attribution `site-photo` evidence carries. */
  attribution: string;
  /** When it was taken, as typed; empty when unknown. */
  capturedAt: string;
}

/** A photo drawn onto a facade, and what a window over it records. */
export interface RectifiedPhoto {
  sha256: string;
  record: PhotoRecord;
  transform: PhotoTransform;
  rectification: Rectification;
}

/**
 * The SHA-256 of the photo as chosen — the original file, before any EXIF
 * stripping or resizing — as 64 lowercase hex digits.
 */
export async function hashPhoto(file: Blob): Promise<string> {
  const bytes = await file.arrayBuffer();
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), (b) =>
    b.toString(16).padStart(2, "0"),
  ).join("");
}

/**
 * Where the four corners a person clicks are in the facade's frame, in the
 * order they are clicked: bottom-left, bottom-right, top-right, top-left,
 * seen from outside. The length and height are the Go code's measurements.
 */
export function facadeCorners(
  lengthM: number,
  heightM: number,
): [number, number][] {
  return [
    [0, 0],
    [lengthM, 0],
    [lengthM, heightM],
    [0, heightM],
  ];
}

/**
 * The evidence entry a window drawn, moved or confirmed over the photo
 * carries (`docs/research/facade-evidence-sources.md`, site-photo): the
 * photo's hash and the corners it was rectified from, so the same evidence
 * always draws the photo the same way under the window.
 */
export function photoEvidence(photo: RectifiedPhoto): Record<string, unknown> {
  return {
    kind: "site-photo",
    source_id: photo.record.sourceId,
    content_sha256: photo.sha256,
    licence: "own",
    ...(photo.record.attribution === ""
      ? {}
      : { attribution: photo.record.attribution }),
    ...(photo.record.capturedAt === ""
      ? {}
      : { captured_at: photo.record.capturedAt }),
    transform: photo.transform,
  };
}

/**
 * `evidence` with `entry` in it: replacing the entry for the same photo — a
 * `site-photo` entry with its hash — so a window moved twice over one photo
 * records it once, and evidence of another kind is kept whatever it hashes.
 */
export function withPhotoEvidence(
  evidence: unknown,
  entry: Record<string, unknown>,
): unknown[] {
  const kept: unknown[] = Array.isArray(evidence)
    ? (evidence as unknown[]).filter(
        (e) =>
          !(
            isRecord(e) &&
            e.kind === entry.kind &&
            e.content_sha256 === entry.content_sha256
          ),
      )
    : [];
  return [...kept, entry];
}

/**
 * The corners and record a window on this facade already holds for the
 * photo with this hash, so opening the same photo again draws it exactly as
 * before, without clicking its corners again. Null when no window has it.
 * Only a `site-photo` entry counts: another kind with the same hash is
 * someone else's material, and reusing it would relabel it as an own photo.
 */
export function recordedPhoto(
  features: readonly ModelFeature[],
  facade: { buildingId: string; part: number; ring: number; edge: number },
  sha256: string,
): { record: PhotoRecord; transform: PhotoTransform } | null {
  for (const f of features) {
    if (f.kind !== "window") continue;
    const key = f.properties?.facade;
    if (
      !isRecord(key) ||
      key.building_ref !== facade.buildingId ||
      key.part !== facade.part ||
      key.ring !== facade.ring ||
      key.edge !== facade.edge
    ) {
      continue;
    }
    const evidence = f.properties?.evidence;
    if (!Array.isArray(evidence)) continue;
    for (const e of evidence) {
      if (
        !isRecord(e) ||
        e.kind !== "site-photo" ||
        e.content_sha256 !== sha256
      ) {
        continue;
      }
      const transform = asTransform(e.transform);
      if (transform === null) continue;
      return {
        record: {
          sourceId: typeof e.source_id === "string" ? e.source_id : "",
          attribution: typeof e.attribution === "string" ? e.attribution : "",
          capturedAt: typeof e.captured_at === "string" ? e.captured_at : "",
        },
        transform,
      };
    }
  }
  return null;
}

/**
 * The CSS `matrix3d` that draws the photo, at its own pixel size with its
 * top-left corner at the drawing's, onto the facade: the Go homography
 * (pixel to facade metres), then the drawing's scale (metres to view units,
 * z up the page), then the view's size on screen (`cssPerUnit` CSS pixels
 * per view unit).
 */
export function photoMatrix3d(
  matrix: readonly number[],
  scale: ElevationScale,
  cssPerUnit: number,
): string {
  const k = cssPerUnit;
  const view = [
    k * scale.scale,
    0,
    k * scale.x(0),
    0,
    -k * scale.scale,
    k * scale.y(0),
    0,
    0,
    1,
  ];
  const m = times(view, matrix);
  const [m00, m01, m02, m10, m11, m12, m20, m21, m22] = m;
  return `matrix3d(${[m00, m10, 0, m20, m01, m11, 0, m21, 0, 0, 1, 0, m02, m12, 0, m22].map(String).join(", ")})`;
}

function times(a: readonly number[], b: readonly number[]): number[] {
  const out: number[] = [];
  for (let r = 0; r < 3; r++) {
    for (let c = 0; c < 3; c++) {
      out.push(
        (a[3 * r] ?? 0) * (b[c] ?? 0) +
          (a[3 * r + 1] ?? 0) * (b[3 + c] ?? 0) +
          (a[3 * r + 2] ?? 0) * (b[6 + c] ?? 0),
      );
    }
  }
  return out;
}

function asTransform(value: unknown): PhotoTransform | null {
  if (!isRecord(value)) return null;
  const image = asPairs(value.image_points);
  const facade = asPairs(value.facade_points);
  if (image === null || facade === null) return null;
  return { image_points: image, facade_points: facade };
}

function asPairs(value: unknown): [number, number][] | null {
  if (!Array.isArray(value) || value.length !== 4) return null;
  const pairs: [number, number][] = [];
  for (const p of value) {
    if (
      !Array.isArray(p) ||
      p.length !== 2 ||
      typeof p[0] !== "number" ||
      typeof p[1] !== "number"
    ) {
      return null;
    }
    pairs.push([p[0], p[1]]);
  }
  return pairs;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
