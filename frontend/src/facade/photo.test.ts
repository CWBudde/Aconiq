import { describe, expect, it } from "vitest";
import type { ModelFeature } from "@/model/types";
import { elevationScale } from "./elevation-scale";
import {
  facadeCorners,
  hashPhoto,
  photoEvidence,
  photoMatrix3d,
  recordedPhoto,
  withPhotoEvidence,
  type RectifiedPhoto,
} from "./photo";

const transform = {
  image_points: [
    [0, 800],
    [1000, 800],
    [1000, 0],
    [0, 0],
  ] as [number, number][],
  facade_points: facadeCorners(10, 8),
};

const photo: RectifiedPhoto = {
  sha256: "ab".repeat(32),
  record: { sourceId: "store/0042", attribution: "", capturedAt: "2026-10-20" },
  transform,
  rectification: { matrix: [0.01, 0, 0, 0, -0.01, 8, 0, 0, 1], inverse: [] },
};

describe("hashPhoto", () => {
  it("hashes the file's bytes as they are", async () => {
    const file = new Blob([new TextEncoder().encode("abc")]);
    expect(await hashPhoto(file)).toBe(
      "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
    );
  });
});

describe("photoMatrix3d", () => {
  it("composes the homography with the drawing's scale and size", () => {
    // A straight-on photo of a 10 × 8 m facade, 100 px to the metre: the
    // drawing puts that facade at 36 view units to the metre, 36 in from the
    // left and from the bottom, so a pixel lands at 72 + 0.72 x CSS px at
    // two CSS px per view unit.
    const css = photoMatrix3d(
      photo.rectification.matrix,
      elevationScale(10, 8),
      2,
    );
    const values = /matrix3d\((.*)\)/.exec(css)?.[1]?.split(",").map(Number);
    const want = [0.72, 0, 0, 0, 0, 0.72, 0, 0, 0, 0, 1, 0, 72, 72, 0, 1];
    expect(values).toHaveLength(16);
    want.forEach((w, i) => {
      expect(values?.[i]).toBeCloseTo(w, 9);
    });
  });
});

describe("photo evidence", () => {
  it("records the hash, the corners and the record, and no image", () => {
    const entry = photoEvidence(photo);
    expect(entry).toEqual({
      kind: "site-photo",
      source_id: "store/0042",
      content_sha256: "ab".repeat(32),
      licence: "own",
      captured_at: "2026-10-20",
      transform,
    });
  });

  it("replaces the entry for the same photo and keeps the others", () => {
    const other = { kind: "plan", source_id: "A-101" };
    const old = { kind: "site-photo", content_sha256: "ab".repeat(32) };
    const plan = { kind: "plan", content_sha256: "ab".repeat(32) };
    const entry = photoEvidence(photo);
    expect(withPhotoEvidence([other, old, plan], entry)).toEqual([
      other,
      plan,
      entry,
    ]);
    expect(withPhotoEvidence(undefined, entry)).toEqual([entry]);
  });

  it("finds the corners a window of this facade recorded for the photo", () => {
    const window = (edge: number, evidence: unknown[]): ModelFeature => ({
      id: `w${String(edge)}`,
      kind: "window",
      properties: {
        facade: { building_ref: "house", part: 0, ring: 0, edge },
        evidence,
      },
      geometry: { type: "Point", coordinates: [0, 0] },
    });
    const features = [
      window(1, [photoEvidence(photo)]),
      window(0, [{ kind: "plan" }, photoEvidence(photo)]),
    ];
    const facade = { buildingId: "house", part: 0, ring: 0, edge: 0 };

    expect(recordedPhoto(features, facade, photo.sha256)).toEqual({
      record: photo.record,
      transform,
    });
    expect(recordedPhoto(features, facade, "cd".repeat(32))).toBeNull();
    expect(
      recordedPhoto(features, { ...facade, edge: 2 }, photo.sha256),
    ).toBeNull();
  });
});
