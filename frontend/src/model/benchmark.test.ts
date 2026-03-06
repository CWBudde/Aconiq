/**
 * Large-model synthetic performance benchmarks.
 * These tests verify that core model operations remain fast under load.
 * They do not check acoustic correctness — only throughput.
 */
import { beforeEach, describe, expect, it } from "vitest";
import { useModelStore } from "./model-store";
import { normalizeGeoJSON } from "./normalize";
import { validateModel } from "./validate";
import type { GeoJSONFeatureCollection } from "./types";

const LARGE_N = 5_000;

function makeSourceCollection(n: number): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: Array.from({ length: n }, (_, i) => ({
      type: "Feature",
      properties: { kind: "source", source_type: "point" },
      geometry: {
        type: "Point",
        coordinates: [(i % 360) - 180, (i % 180) - 90],
      },
    })),
  };
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("Model benchmarks (large-scene synthetic)", () => {
  it(`normalizes ${String(LARGE_N)} features within 500 ms`, () => {
    const collection = makeSourceCollection(LARGE_N);
    const start = performance.now();
    const result = normalizeGeoJSON(collection);
    const elapsed = performance.now() - start;
    expect(result.features).toHaveLength(LARGE_N);
    expect(elapsed).toBeLessThan(500);
  });

  it(`validates ${String(LARGE_N)} features within 500 ms`, () => {
    const collection = makeSourceCollection(LARGE_N);
    const { features } = normalizeGeoJSON(collection);
    const start = performance.now();
    const report = validateModel(features);
    const elapsed = performance.now() - start;
    expect(report.errors).toHaveLength(0);
    expect(elapsed).toBeLessThan(500);
  });

  it(`loads ${String(LARGE_N)} features into the store within 200 ms`, () => {
    const collection = makeSourceCollection(LARGE_N);
    const { features } = normalizeGeoJSON(collection);
    const store = useModelStore.getState();
    const start = performance.now();
    store.loadFeatures(features);
    const elapsed = performance.now() - start;
    expect(useModelStore.getState().features).toHaveLength(LARGE_N);
    expect(elapsed).toBeLessThan(200);
  });

  it(`featuresByKind filter over ${String(LARGE_N)} features within 50 ms`, () => {
    const collection = makeSourceCollection(LARGE_N);
    const { features } = normalizeGeoJSON(collection);
    useModelStore.getState().loadFeatures(features);
    const store = useModelStore.getState();
    const start = performance.now();
    const sources = store.featuresByKind("source");
    const elapsed = performance.now() - start;
    expect(sources).toHaveLength(LARGE_N);
    expect(elapsed).toBeLessThan(50);
  });
});
