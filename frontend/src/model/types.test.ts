import { describe, expect, it } from "vitest";
import { createFeatureId, isGeometryCompatible } from "./types";

describe("createFeatureId", () => {
  it("returns a non-empty string", () => {
    const id = createFeatureId();
    expect(id).toBeTruthy();
    expect(typeof id).toBe("string");
  });

  it("returns unique ids", () => {
    const ids = new Set(Array.from({ length: 100 }, () => createFeatureId()));
    expect(ids.size).toBe(100);
  });
});

describe("isGeometryCompatible", () => {
  it("point source accepts Point geometry", () => {
    expect(isGeometryCompatible("Point", "point")).toBe(true);
  });

  it("point source accepts MultiPoint geometry", () => {
    expect(isGeometryCompatible("MultiPoint", "point")).toBe(true);
  });

  it("point source rejects Polygon geometry", () => {
    expect(isGeometryCompatible("Polygon", "point")).toBe(false);
  });

  it("line source accepts LineString geometry", () => {
    expect(isGeometryCompatible("LineString", "line")).toBe(true);
  });

  it("line source accepts MultiLineString geometry", () => {
    expect(isGeometryCompatible("MultiLineString", "line")).toBe(true);
  });

  it("line source rejects Point geometry", () => {
    expect(isGeometryCompatible("Point", "line")).toBe(false);
  });

  it("area source accepts Polygon geometry", () => {
    expect(isGeometryCompatible("Polygon", "area")).toBe(true);
  });

  it("area source accepts MultiPolygon geometry", () => {
    expect(isGeometryCompatible("MultiPolygon", "area")).toBe(true);
  });

  it("area source rejects Point geometry", () => {
    expect(isGeometryCompatible("Point", "area")).toBe(false);
  });
});
