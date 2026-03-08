import { useMemo } from "react";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";

interface FallbackMapProps {
  center: [number, number];
}

type Bounds = {
  west: number;
  south: number;
  east: number;
  north: number;
};

export function FallbackMap({ center }: FallbackMapProps) {
  const features = useModelStore((s) => s.features);

  const bounds = useMemo(() => computeBounds(features, center), [features, center]);
  const iframeURL = useMemo(() => buildOSMEmbedURL(bounds), [bounds]);

  return (
    <div className="absolute inset-0 overflow-hidden bg-slate-100">
      <iframe
        title="OpenStreetMap fallback"
        src={iframeURL}
        className="absolute inset-0 h-full w-full border-0"
      />
      <svg
        className="absolute inset-0 h-full w-full"
        viewBox="0 0 1000 1000"
        preserveAspectRatio="none"
      >
        {features.map((feature) => (
          <FeatureOverlay
            key={feature.id}
            feature={feature}
            bounds={bounds}
          />
        ))}
      </svg>
      <div className="absolute left-3 top-3 rounded-md border border-amber-300 bg-amber-50/95 px-3 py-2 text-xs text-amber-950 shadow-sm">
        WebGL unavailable. Showing OSM fallback map.
      </div>
    </div>
  );
}

function FeatureOverlay({
  feature,
  bounds,
}: {
  feature: ModelFeature;
  bounds: Bounds;
}) {
  const color =
    feature.kind === "source"
      ? "#dc2626"
      : feature.kind === "building"
        ? "#475569"
        : "#92400e";

  const polygons = collectPolygons(feature.geometry.coordinates);
  if (polygons.length > 0) {
    return (
      <>
        {polygons.map((polygon, index) => (
          <polygon
            key={`${feature.id}-poly-${String(index)}`}
            points={polygon
              .map((point) => projectPoint(point, bounds))
              .map(({ x, y }) => `${x},${y}`)
              .join(" ")}
            fill={feature.kind === "source" ? `${color}33` : `${color}55`}
            stroke={color}
            strokeWidth="2"
          />
        ))}
      </>
    );
  }

  const lines = collectLines(feature.geometry.coordinates);
  if (lines.length > 0) {
    return (
      <>
        {lines.map((line, index) => (
          <polyline
            key={`${feature.id}-line-${String(index)}`}
            points={line
              .map((point) => projectPoint(point, bounds))
              .map(({ x, y }) => `${x},${y}`)
              .join(" ")}
            fill="none"
            stroke={color}
            strokeWidth={feature.kind === "barrier" ? 3 : 4}
            strokeDasharray={feature.kind === "barrier" ? "8 6" : undefined}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        ))}
      </>
    );
  }

  const points = collectPoints(feature.geometry.coordinates);
  return (
    <>
      {points.map((point, index) => {
        const projected = projectPoint(point, bounds);
        return (
          <circle
            key={`${feature.id}-point-${String(index)}`}
            cx={projected.x}
            cy={projected.y}
            r="6"
            fill={color}
            stroke="#ffffff"
            strokeWidth="2"
          />
        );
      })}
    </>
  );
}

function computeBounds(features: ModelFeature[], center: [number, number]): Bounds {
  let west = Number.POSITIVE_INFINITY;
  let south = Number.POSITIVE_INFINITY;
  let east = Number.NEGATIVE_INFINITY;
  let north = Number.NEGATIVE_INFINITY;

  function visit(coords: unknown): void {
    if (!Array.isArray(coords)) return;
    if (
      coords.length >= 2 &&
      typeof coords[0] === "number" &&
      typeof coords[1] === "number"
    ) {
      west = Math.min(west, coords[0]);
      south = Math.min(south, coords[1]);
      east = Math.max(east, coords[0]);
      north = Math.max(north, coords[1]);
      return;
    }
    for (const value of coords) visit(value);
  }

  for (const feature of features) {
    visit(feature.geometry.coordinates);
  }

  if (!Number.isFinite(west)) {
    const [lng, lat] = center;
    return {
      west: lng - 0.15,
      south: lat - 0.1,
      east: lng + 0.15,
      north: lat + 0.1,
    };
  }

  const dx = Math.max(0.01, (east - west) * 0.15);
  const dy = Math.max(0.01, (north - south) * 0.15);

  return {
    west: west - dx,
    south: south - dy,
    east: east + dx,
    north: north + dy,
  };
}

function buildOSMEmbedURL(bounds: Bounds): string {
  const query = new URLSearchParams({
    bbox: `${bounds.west},${bounds.south},${bounds.east},${bounds.north}`,
    layer: "mapnik",
  });
  return `https://www.openstreetmap.org/export/embed.html?${query.toString()}`;
}

function projectPoint(
  point: [number, number],
  bounds: Bounds,
): { x: number; y: number } {
  const width = Math.max(0.000001, bounds.east - bounds.west);
  const height = Math.max(0.000001, bounds.north - bounds.south);
  const x = ((point[0] - bounds.west) / width) * 1000;
  const y = (1 - (point[1] - bounds.south) / height) * 1000;
  return { x, y };
}

function collectPoints(coords: unknown): [number, number][] {
  if (!Array.isArray(coords)) return [];
  if (
    coords.length >= 2 &&
    typeof coords[0] === "number" &&
    typeof coords[1] === "number"
  ) {
    return [[coords[0], coords[1]]];
  }
  return coords.flatMap((value) => collectPoints(value));
}

function collectLines(coords: unknown): [number, number][][] {
  if (!Array.isArray(coords)) return [];
  if (
    coords.length > 0 &&
    Array.isArray(coords[0]) &&
    Array.isArray(coords[0][0]) === false
  ) {
    const line = collectPoints(coords);
    return line.length >= 2 ? [line] : [];
  }
  return coords.flatMap((value) => collectLines(value));
}

function collectPolygons(coords: unknown): [number, number][][] {
  if (!Array.isArray(coords)) return [];
  if (
    coords.length > 0 &&
    Array.isArray(coords[0]) &&
    Array.isArray(coords[0][0]) &&
    Array.isArray(coords[0][0][0]) === false
  ) {
    const outer = collectPoints(coords[0]);
    return outer.length >= 3 ? [outer] : [];
  }
  return coords.flatMap((value) => collectPolygons(value));
}
