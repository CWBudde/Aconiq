import type {
  DataDrivenPropertyValueSpecification,
  LayerSpecification,
} from "maplibre-gl";
import { NOISE_LEVEL_RAMP, rampToExpression } from "./color-ramp";
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
  resultReceivers: "result-receivers",
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
  resultReceiverLevel: "result-receiver-level",
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

// --- Result layer styles ---

/**
 * The feature property {@link RESULT_RECEIVER_LAYERS} colours by, and the one
 * `result-layers.tsx` writes the selected indicator's level under.
 *
 * A fixed name rather than the indicator's own: the paint expression is part
 * of the style and changing it would mean re-adding the layer every time the
 * picker moves, while the source can simply be re-fed.
 */
export const RESULT_LEVEL_PROPERTY = "value";

/**
 * One computed receiver, filled with its level.
 *
 * A circle and not a symbol: none of the styles in `basemap.ts` declares
 * `glyphs`, so a `text-field` layer would render nothing and log a font error
 * per tile. Labelling the receivers needs a glyph source first.
 *
 * The white stroke is what keeps a green circle readable over the light
 * basemap's green and a dark one's grey — the ramp's ends are the two colours
 * the basemap itself is most likely to supply underneath.
 */
export const RESULT_RECEIVER_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.resultReceiverLevel,
    type: "circle",
    source: SOURCE_IDS.resultReceivers,
    paint: {
      "circle-radius": 5,
      "circle-color": rampToExpression(
        NOISE_LEVEL_RAMP,
        RESULT_LEVEL_PROPERTY,
      ) as DataDrivenPropertyValueSpecification<string>,
      "circle-stroke-width": 1,
      "circle-stroke-color": "#ffffff",
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

/**
 * The id of the group {@link RESULT_RECEIVER_LAYERS} belongs to, named here so
 * `result-layers.tsx` can read the visibility the user chose before the layer
 * existed. A group toggled off on an empty map would otherwise come back on
 * its own the moment a run was drawn.
 */
export const RESULT_RECEIVERS_GROUP_ID = "receiver-levels";

/**
 * The result groups the layer control offers.
 *
 * It held two more — `raster` and `contours` — for layers nothing ever added:
 * the raster bytes reach neither mode (browser mode stores the run hash where
 * the binary belongs, and `StoredArtifactContent.encoding` has no case for a
 * binary payload), and the GeoTIFF/COG/contour exports get no `ArtifactRef`,
 * so no URL reaches the map. Both toggles were therefore permanently dead
 * controls, and are gone until the artifact side of that exists.
 */
export const RESULT_LAYER_GROUPS: LayerGroup[] = [
  {
    id: RESULT_RECEIVERS_GROUP_ID,
    label: m.label_result_receiver_levels,
    layerIds: [LAYER_IDS.resultReceiverLevel],
    defaultVisible: true,
  },
];
