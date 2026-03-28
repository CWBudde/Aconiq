import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";

export type ViewState = {
  center: [number, number];
  zoom: number;
};

type Bounds = {
  west: number;
  south: number;
  east: number;
  north: number;
};

function visitBounds(coords: unknown, bounds: Bounds): void {
  if (!Array.isArray(coords)) return;
  if (
    coords.length >= 2 &&
    typeof coords[0] === "number" &&
    typeof coords[1] === "number"
  ) {
    bounds.west = Math.min(bounds.west, coords[0]);
    bounds.south = Math.min(bounds.south, coords[1]);
    bounds.east = Math.max(bounds.east, coords[0]);
    bounds.north = Math.max(bounds.north, coords[1]);
    return;
  }
  for (const value of coords) {
    visitBounds(value, bounds);
  }
}

function collectBounds(
  features: ModelFeature[],
  receivers: ModelReceiver[],
  calcArea: CalcArea | null,
): Bounds | null {
  const bounds: Bounds = {
    west: Number.POSITIVE_INFINITY,
    south: Number.POSITIVE_INFINITY,
    east: Number.NEGATIVE_INFINITY,
    north: Number.NEGATIVE_INFINITY,
  };

  for (const feature of features) {
    visitBounds(feature.geometry.coordinates, bounds);
  }

  if (calcArea) {
    visitBounds(calcArea.geometry.coordinates, bounds);
  }

  for (const receiver of receivers) {
    visitBounds(receiver.geometry.coordinates, bounds);
  }

  if (!Number.isFinite(bounds.west)) return null;
  return bounds;
}

export function fitViewToWorkspace(
  features: ModelFeature[],
  receivers: ModelReceiver[],
  calcArea: CalcArea | null,
  fallbackCenter: [number, number],
): ViewState {
  const bounds = collectBounds(features, receivers, calcArea);
  if (!bounds) {
    return { center: fallbackCenter, zoom: 6 };
  }

  const centerLng = (bounds.west + bounds.east) / 2;
  const centerLat = (bounds.south + bounds.north) / 2;

  const lonSpan = Math.max(0.01, bounds.east - bounds.west);
  const latSpan = Math.max(0.01, bounds.north - bounds.south);
  const span = Math.max(lonSpan, latSpan);

  let zoom = 14;
  if (span > 20) zoom = 4;
  else if (span > 8) zoom = 6;
  else if (span > 2) zoom = 8;
  else if (span > 0.5) zoom = 10;
  else if (span > 0.1) zoom = 12;

  return {
    center: [centerLng, centerLat],
    zoom,
  };
}
