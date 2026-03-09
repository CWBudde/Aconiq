import type { LayerSpecification } from "maplibre-gl";

/**
 * Layer definitions for noise model features.
 *
 * Each model layer group has:
 * - A source ID (matching the GeoJSON source added to the map)
 * - One or more MapLibre layer specs for rendering
 *
 * Layer ordering (bottom to top):
 *   basemap → buildings → barriers → sources → receivers → results
 */

// --- Source IDs ---

export const SOURCE_IDS = {
  buildings: "model-buildings",
  barriers: "model-barriers",
  sources: "model-sources",
  receivers: "model-receivers",
  calcArea: "calc-area",
  results: "result-raster",
  contours: "result-contours",
} as const;

// --- Layer IDs ---

export const LAYER_IDS = {
  buildingsFill: "buildings-fill",
  buildingsOutline: "buildings-outline",
  barrierLine: "barrier-line",
  sourcesPoint: "sources-point",
  sourcesLine: "sources-line",
  sourcesArea: "sources-area-fill",
  receiversPoint: "receivers-point",
  calcAreaFill: "calc-area-fill",
  calcAreaOutline: "calc-area-outline",
  resultRaster: "result-raster-layer",
  contourLine: "contour-line",
  contourLabel: "contour-label",
} as const;

// --- Model layer styles ---

export const BUILDING_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.buildingsFill,
    type: "fill",
    source: SOURCE_IDS.buildings,
    paint: {
      "fill-color": "#b0b0b0",
      "fill-opacity": 0.4,
    },
  },
  {
    id: LAYER_IDS.buildingsOutline,
    type: "line",
    source: SOURCE_IDS.buildings,
    paint: {
      "line-color": "#666666",
      "line-width": 1,
    },
  },
];

export const BARRIER_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.barrierLine,
    type: "line",
    source: SOURCE_IDS.barriers,
    paint: {
      "line-color": "#8B4513",
      "line-width": 2.5,
      "line-dasharray": [4, 2],
    },
  },
];

export const SOURCE_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.sourcesArea,
    type: "fill",
    source: SOURCE_IDS.sources,
    filter: ["==", ["geometry-type"], "Polygon"],
    paint: {
      "fill-color": "#e63946",
      "fill-opacity": 0.2,
    },
  },
  {
    id: LAYER_IDS.sourcesLine,
    type: "line",
    source: SOURCE_IDS.sources,
    filter: ["==", ["geometry-type"], "LineString"],
    paint: {
      "line-color": "#e63946",
      "line-width": 3,
    },
  },
  {
    id: LAYER_IDS.sourcesPoint,
    type: "circle",
    source: SOURCE_IDS.sources,
    filter: ["==", ["geometry-type"], "Point"],
    paint: {
      "circle-radius": 5,
      "circle-color": "#e63946",
      "circle-stroke-width": 1.5,
      "circle-stroke-color": "#ffffff",
    },
  },
];

export const RECEIVER_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.receiversPoint,
    type: "circle",
    source: SOURCE_IDS.receivers,
    paint: {
      "circle-radius": 3,
      "circle-color": "#2196F3",
      "circle-stroke-width": 1,
      "circle-stroke-color": "#ffffff",
    },
  },
];

export const CALC_AREA_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.calcAreaFill,
    type: "fill",
    source: SOURCE_IDS.calcArea,
    paint: {
      "fill-color": "#3b82f6",
      "fill-opacity": 0.06,
    },
  },
  {
    id: LAYER_IDS.calcAreaOutline,
    type: "line",
    source: SOURCE_IDS.calcArea,
    paint: {
      "line-color": "#3b82f6",
      "line-width": 2,
      "line-dasharray": [6, 3],
    },
  },
];

export const CONTOUR_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.contourLine,
    type: "line",
    source: SOURCE_IDS.contours,
    paint: {
      "line-color": "#333333",
      "line-width": 1,
    },
  },
  {
    id: LAYER_IDS.contourLabel,
    type: "symbol",
    source: SOURCE_IDS.contours,
    layout: {
      "symbol-placement": "line",
      "text-field": ["get", "level"],
      "text-size": 11,
      "text-font": ["Open Sans Regular"],
    },
    paint: {
      "text-color": "#333333",
      "text-halo-color": "#ffffff",
      "text-halo-width": 1.5,
    },
  },
];

// --- Layer group metadata (for UI controls) ---

export interface LayerGroup {
  id: string;
  label: string;
  layerIds: string[];
  defaultVisible: boolean;
}

export const MODEL_LAYER_GROUPS: LayerGroup[] = [
  {
    id: "calc-area",
    label: "Calculation Area",
    layerIds: [LAYER_IDS.calcAreaFill, LAYER_IDS.calcAreaOutline],
    defaultVisible: true,
  },
  {
    id: "buildings",
    label: "Buildings",
    layerIds: [LAYER_IDS.buildingsFill, LAYER_IDS.buildingsOutline],
    defaultVisible: true,
  },
  {
    id: "barriers",
    label: "Barriers",
    layerIds: [LAYER_IDS.barrierLine],
    defaultVisible: true,
  },
  {
    id: "sources",
    label: "Sources",
    layerIds: [
      LAYER_IDS.sourcesArea,
      LAYER_IDS.sourcesLine,
      LAYER_IDS.sourcesPoint,
    ],
    defaultVisible: true,
  },
  {
    id: "receivers",
    label: "Receivers",
    layerIds: [LAYER_IDS.receiversPoint],
    defaultVisible: true,
  },
];

export const RESULT_LAYER_GROUPS: LayerGroup[] = [
  {
    id: "raster",
    label: "Result Raster",
    layerIds: [LAYER_IDS.resultRaster],
    defaultVisible: true,
  },
  {
    id: "contours",
    label: "Contours",
    layerIds: [LAYER_IDS.contourLine, LAYER_IDS.contourLabel],
    defaultVisible: true,
  },
];
