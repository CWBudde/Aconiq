import { useEffect, useMemo, useRef, useState } from "react";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver } from "@/model/types";

interface FallbackMapProps {
  center: [number, number];
}

type ViewState = {
  center: [number, number];
  zoom: number;
};

type Point = {
  x: number;
  y: number;
};

const TILE_SIZE = 256;
const MIN_ZOOM = 2;
const MAX_ZOOM = 19;
const WHEEL_ZOOM_STEP = 0.25;

export function FallbackMap({ center }: FallbackMapProps) {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const containerRef = useRef<HTMLDivElement>(null);
  const dragStateRef = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    startCenterWorld: Point;
  } | null>(null);

  const [size, setSize] = useState({ width: 0, height: 0 });
  const initialView = useMemo(
    () => fitViewToFeatures(features, center),
    [features, center],
  );
  const [view, setView] = useState<ViewState>(initialView);

  useEffect(() => {
    setView(initialView);
  }, [initialView]);

  useEffect(() => {
    const node = containerRef.current;
    if (!node) return;

    const updateSize = () => {
      setSize({
        width: node.clientWidth,
        height: node.clientHeight,
      });
    };

    updateSize();

    const observer = new ResizeObserver(updateSize);
    observer.observe(node);
    return () => {
      observer.disconnect();
    };
  }, []);

  const worldCenter = useMemo(
    () => lngLatToWorld(view.center, view.zoom),
    [view.center, view.zoom],
  );

  const tiles = useMemo(() => {
    if (size.width <= 0 || size.height <= 0) return [];
    return computeVisibleTiles(worldCenter, view.zoom, size.width, size.height);
  }, [worldCenter, view.zoom, size.width, size.height]);

  function handlePointerDown(event: React.PointerEvent<HTMLDivElement>) {
    if (size.width <= 0 || size.height <= 0) return;
    dragStateRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      startCenterWorld: worldCenter,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function handlePointerMove(event: React.PointerEvent<HTMLDivElement>) {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId) return;

    const dx = event.clientX - dragState.startX;
    const dy = event.clientY - dragState.startY;
    const newCenterWorld = {
      x: dragState.startCenterWorld.x - dx,
      y: dragState.startCenterWorld.y - dy,
    };

    setView((current) => ({
      ...current,
      center: worldToLngLat(newCenterWorld, current.zoom),
    }));
  }

  function handlePointerUp(event: React.PointerEvent<HTMLDivElement>) {
    if (dragStateRef.current?.pointerId === event.pointerId) {
      dragStateRef.current = null;
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  function handleWheel(event: React.WheelEvent<HTMLDivElement>) {
    event.preventDefault();
    const direction = event.deltaY > 0 ? -WHEEL_ZOOM_STEP : WHEEL_ZOOM_STEP;
    setView((current) => ({
      ...current,
      zoom: clamp(current.zoom + direction, MIN_ZOOM, MAX_ZOOM),
    }));
  }

  function zoomBy(delta: number) {
    setView((current) => ({
      ...current,
      zoom: clamp(current.zoom + delta, MIN_ZOOM, MAX_ZOOM),
    }));
  }

  return (
    <div
      ref={containerRef}
      className="absolute inset-0 overflow-hidden bg-slate-100"
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      onWheel={handleWheel}
    >
      {tiles.map((tile) => (
        <img
          key={`${String(view.zoom)}/${String(tile.x)}/${String(tile.y)}`}
          src={`https://tile.openstreetmap.org/${String(view.zoom)}/${String(tile.x)}/${String(tile.y)}.png`}
          alt=""
          draggable={false}
          className="pointer-events-none absolute max-w-none select-none"
          style={{
            left: tile.left,
            top: tile.top,
            width: TILE_SIZE,
            height: TILE_SIZE,
          }}
        />
      ))}

      <svg
        className="pointer-events-none absolute inset-0 h-full w-full"
        viewBox={`0 0 ${String(size.width || 1)} ${String(size.height || 1)}`}
        preserveAspectRatio="none"
      >
        {features.map((feature) => (
          <FeatureOverlay
            key={feature.id}
            feature={feature}
            centerWorld={worldCenter}
            zoom={view.zoom}
            viewportWidth={size.width}
            viewportHeight={size.height}
          />
        ))}
        {receivers.map((receiver) => (
          <ReceiverOverlay
            key={receiver.id}
            receiver={receiver}
            centerWorld={worldCenter}
            zoom={view.zoom}
            viewportWidth={size.width}
            viewportHeight={size.height}
          />
        ))}
      </svg>

      <div className="absolute left-3 top-3 rounded-md border border-amber-300 bg-amber-50/95 px-3 py-2 text-xs text-amber-950 shadow-sm">
        WebGL unavailable. Showing raster fallback map.
      </div>

      <div className="absolute right-3 top-3 flex flex-col overflow-hidden rounded-md border bg-background/95 shadow-sm">
        <button
          type="button"
          className="h-9 w-9 border-b text-lg"
          onClick={() => {
            zoomBy(1);
          }}
        >
          +
        </button>
        <button
          type="button"
          className="h-9 w-9 text-lg"
          onClick={() => {
            zoomBy(-1);
          }}
        >
          -
        </button>
      </div>
    </div>
  );
}

function FeatureOverlay({
  feature,
  centerWorld,
  zoom,
  viewportWidth,
  viewportHeight,
}: {
  feature: ModelFeature;
  centerWorld: Point;
  zoom: number;
  viewportWidth: number;
  viewportHeight: number;
}) {
  const color =
    feature.kind === "source"
      ? "#dc2626"
      : feature.kind === "building"
        ? "#475569"
        : "#92400e";

  const project = (point: [number, number]) =>
    projectToViewport(point, centerWorld, zoom, viewportWidth, viewportHeight);

  const polygons = collectPolygons(feature.geometry.coordinates);
  if (polygons.length > 0) {
    return (
      <>
        {polygons.map((polygon, index) => (
          <polygon
            key={`${feature.id}-poly-${String(index)}`}
            points={polygon
              .map(project)
              .map(({ x, y }) => `${String(x)},${String(y)}`)
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
              .map(project)
              .map(({ x, y }) => `${String(x)},${String(y)}`)
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
        const projected = project(point);
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

function ReceiverOverlay({
  receiver,
  centerWorld,
  zoom,
  viewportWidth,
  viewportHeight,
}: {
  receiver: ModelReceiver;
  centerWorld: Point;
  zoom: number;
  viewportWidth: number;
  viewportHeight: number;
}) {
  const projected = projectToViewport(
    receiver.geometry.coordinates,
    centerWorld,
    zoom,
    viewportWidth,
    viewportHeight,
  );
  return (
    <circle
      cx={projected.x}
      cy={projected.y}
      r="4"
      fill="#2196F3"
      stroke="#ffffff"
      strokeWidth="1.5"
    />
  );
}

function fitViewToFeatures(
  features: ModelFeature[],
  center: [number, number],
): ViewState {
  const bounds = collectBounds(features);
  if (!bounds) {
    return { center, zoom: 6 };
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

function collectBounds(
  features: ModelFeature[],
): { west: number; south: number; east: number; north: number } | null {
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

  if (!Number.isFinite(west)) return null;
  return { west, south, east, north };
}

function lngLatToWorld([lng, lat]: [number, number], zoom: number): Point {
  const scale = TILE_SIZE * 2 ** zoom;
  const x = ((lng + 180) / 360) * scale;
  const sinLat = Math.sin((lat * Math.PI) / 180);
  const y =
    (0.5 - Math.log((1 + sinLat) / (1 - sinLat)) / (4 * Math.PI)) * scale;
  return { x, y };
}

function worldToLngLat(point: Point, zoom: number): [number, number] {
  const scale = TILE_SIZE * 2 ** zoom;
  const lng = (point.x / scale) * 360 - 180;
  const mercatorY = Math.PI * (1 - (2 * point.y) / scale);
  const lat = (Math.atan(Math.sinh(mercatorY)) * 180) / Math.PI;
  return [lng, lat];
}

function projectToViewport(
  point: [number, number],
  centerWorld: Point,
  zoom: number,
  viewportWidth: number,
  viewportHeight: number,
): Point {
  const world = lngLatToWorld(point, zoom);
  return {
    x: world.x - centerWorld.x + viewportWidth / 2,
    y: world.y - centerWorld.y + viewportHeight / 2,
  };
}

function computeVisibleTiles(
  centerWorld: Point,
  zoom: number,
  width: number,
  height: number,
): Array<{ x: number; y: number; left: number; top: number }> {
  const worldSize = 2 ** zoom;
  const topLeft = {
    x: centerWorld.x - width / 2,
    y: centerWorld.y - height / 2,
  };
  const startX = Math.floor(topLeft.x / TILE_SIZE);
  const endX = Math.floor((topLeft.x + width) / TILE_SIZE);
  const startY = Math.floor(topLeft.y / TILE_SIZE);
  const endY = Math.floor((topLeft.y + height) / TILE_SIZE);

  const tiles: Array<{ x: number; y: number; left: number; top: number }> = [];
  for (let x = startX; x <= endX; x++) {
    for (let y = startY; y <= endY; y++) {
      if (y < 0 || y >= worldSize) continue;
      const wrappedX = ((x % worldSize) + worldSize) % worldSize;
      tiles.push({
        x: wrappedX,
        y,
        left: x * TILE_SIZE - topLeft.x,
        top: y * TILE_SIZE - topLeft.y,
      });
    }
  }
  return tiles;
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
    !Array.isArray(coords[0][0])
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
    !Array.isArray(coords[0][0][0])
  ) {
    const outer = collectPoints(coords[0]);
    return outer.length >= 3 ? [outer] : [];
  }
  return coords.flatMap((value) => collectPolygons(value));
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}
