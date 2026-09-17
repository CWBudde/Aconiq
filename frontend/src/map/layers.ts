import type { LayerSpecification } from "maplibre-gl";
import { m } from "@/i18n/messages";

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

// --- Selection ---

/**
 * The key {@link SELECTION_STATE} is written under, and the one the paint
 * expressions below read. `model-layers.tsx` is the only writer.
 */
export const SELECTED_STATE_KEY = "selected";

/**
 * The colour a selected feature is drawn in.
 *
 * One colour for every kind, and a colour no kind already uses: the selection
 * has to be readable against the grey of a building, the red of a source, the
 * blue of a receiver and the brown of a barrier alike, over a light or a dark
 * basemap. It is not a theme token because nothing here is CSS — MapLibre
 * paints the canvas from the style, where a `var(--…)` never resolves.
 *
 * Colour is never the only signal: every selected layer also widens, so the
 * selection survives a monochrome display and a colour-vision deficiency.
 */
export const SELECTION_COLOR = "#f59e0b";

// --- Model layer styles ---

export const BUILDING_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.buildingsFill,
    type: "fill",
    source: SOURCE_IDS.buildings,
    paint: {
      "fill-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#b0b0b0",
      ],
      "fill-opacity": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        0.35,
        0.4,
      ],
    },
  },
  {
    id: LAYER_IDS.buildingsOutline,
    type: "line",
    source: SOURCE_IDS.buildings,
    paint: {
      "line-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#666666",
      ],
      "line-width": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        3,
        1,
      ],
    },
  },
];

export const BARRIER_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.barrierLine,
    type: "line",
    source: SOURCE_IDS.barriers,
    paint: {
      "line-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#8B4513",
      ],
      "line-width": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        5,
        2.5,
      ],
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
      "fill-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#e63946",
      ],
      "fill-opacity": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        0.4,
        0.2,
      ],
    },
  },
  {
    id: LAYER_IDS.sourcesLine,
    type: "line",
    source: SOURCE_IDS.sources,
    filter: ["==", ["geometry-type"], "LineString"],
    paint: {
      "line-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#e63946",
      ],
      "line-width": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        5.5,
        3,
      ],
    },
  },
  {
    id: LAYER_IDS.sourcesPoint,
    type: "circle",
    source: SOURCE_IDS.sources,
    filter: ["==", ["geometry-type"], "Point"],
    paint: {
      "circle-radius": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        8,
        5,
      ],
      "circle-color": "#e63946",
      "circle-stroke-width": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        3,
        1.5,
      ],
      "circle-stroke-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#ffffff",
      ],
    },
  },
];

export const RECEIVER_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.receiversPoint,
    type: "circle",
    source: SOURCE_IDS.receivers,
    paint: {
      "circle-radius": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        6,
        3,
      ],
      "circle-color": "#2196F3",
      "circle-stroke-width": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        2.5,
        1,
      ],
      "circle-stroke-color": [
        "case",
        ["boolean", ["feature-state", SELECTED_STATE_KEY], false],
        SELECTION_COLOR,
        "#ffffff",
      ],
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

// --- Layer group metadata (for UI controls) ---

export interface LayerGroup {
  id: string;
  /**
   * Localized label, held as a *function*. Storing the resolved string here
   * called the message at module scope and froze it to the locale that was
   * active when this module was first imported, so the layer control kept
   * English names after a language switch.
   */
  label: () => string;
  layerIds: string[];
  defaultVisible: boolean;
}

export const MODEL_LAYER_GROUPS: LayerGroup[] = [
  {
    id: "calc-area",
    label: m.label_calc_area,
    layerIds: [LAYER_IDS.calcAreaFill, LAYER_IDS.calcAreaOutline],
    defaultVisible: true,
  },
  {
    id: "buildings",
    label: m.label_buildings,
    layerIds: [LAYER_IDS.buildingsFill, LAYER_IDS.buildingsOutline],
    defaultVisible: true,
  },
  {
    id: "barriers",
    label: m.label_barriers,
    layerIds: [LAYER_IDS.barrierLine],
    defaultVisible: true,
  },
  {
    id: "sources",
    label: m.label_sources,
    layerIds: [
      LAYER_IDS.sourcesArea,
      LAYER_IDS.sourcesLine,
      LAYER_IDS.sourcesPoint,
    ],
    defaultVisible: true,
  },
  {
    id: "receivers",
    label: m.label_receivers,
    layerIds: [LAYER_IDS.receiversPoint],
    defaultVisible: true,
  },
];

export const RESULT_LAYER_GROUPS: LayerGroup[] = [
  {
    id: "raster",
    label: m.label_result_raster,
    layerIds: [LAYER_IDS.resultRaster],
    defaultVisible: true,
  },
  {
    id: "contours",
    label: m.label_result_contours,
    layerIds: [LAYER_IDS.contourLine, LAYER_IDS.contourLabel],
    defaultVisible: true,
  },
];
