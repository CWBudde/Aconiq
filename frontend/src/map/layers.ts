import type {
  ExpressionSpecification,
  DataDrivenPropertyValueSpecification,
  LayerSpecification,
} from "maplibre-gl";
import { NOISE_LEVEL_RAMP, rampToExpression } from "./color-ramp";
import { m } from "@/i18n/messages";
import { PROP_ACOUSTICS_REVIEWED } from "@/model/source-acoustics";

/**
 * Layer definitions for noise model features.
 *
 * Each model layer group has:
 * - A source ID (matching the GeoJSON source added to the map)
 * - One or more MapLibre layer specs for rendering
 *
 * Layer ordering (bottom to top):
 *   basemap → result raster → result contours → calc area → buildings →
 *   barriers → review flags → sources → receivers → result receivers →
 *   focus flash
 *
 * The review flags sit directly under the sources they belong to, so a flagged
 * road reads as a red line on a violet casing rather than as a violet line. The
 * focus flash is on top of everything and is added lazily by `feature-focus.tsx`
 * rather than in that one pass — see {@link FLASH_LAYERS}.
 *
 * Everything but the result raster gets that order from the order it is added
 * in — `ModelLayers` adds its own in one pass and `pages/map.tsx` renders
 * `ResultLayers` after it. The raster cannot: it is added several commits
 * later, once a run's bytes have been fetched and its corners projected, so
 * appending would draw it over the buildings it is a result for. It is the one
 * layer here inserted with a `beforeId` — see {@link BOTTOM_MODEL_LAYER_ID}.
 */

// --- Source IDs ---

export const SOURCE_IDS = {
  buildings: "model-buildings",
  barriers: "model-barriers",
  groundZones: "model-ground-zones",
  sources: "model-sources",
  receivers: "model-receivers",
  calcArea: "calc-area",
  resultReceivers: "result-receivers",
  resultRaster: "result-raster",
  resultContours: "result-contours",
  /** Holds the one feature a focus request is announcing, or nothing. */
  flash: "map-flash",
} as const;

// --- Layer IDs ---

export const LAYER_IDS = {
  buildingsFill: "buildings-fill",
  buildingsOutline: "buildings-outline",
  barrierLine: "barrier-line",
  groundZoneFill: "ground-zone-fill",
  groundZoneOutline: "ground-zone-outline",
  sourcesPoint: "sources-point",
  sourcesLine: "sources-line",
  sourcesArea: "sources-area-fill",
  sourcesReviewLine: "sources-review-line",
  sourcesReviewPoint: "sources-review-point",
  receiversPoint: "receivers-point",
  flashLine: "map-flash-line",
  flashPoint: "map-flash-point",
  calcAreaFill: "calc-area-fill",
  calcAreaOutline: "calc-area-outline",
  resultReceiverLevel: "result-receiver-level",
  resultRaster: "result-raster-fill",
  resultContoursHalo: "result-contour-halo",
  resultContours: "result-contour-line",
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

// --- Focus flash ---

/** How long the halo sits at full strength once the camera has arrived. */
export const FLASH_HOLD_MS = 900;
/** How long it takes to fade out afterwards. */
export const FLASH_FADE_MS = 600;
/**
 * The hold a reader who asked for reduced motion gets instead. Longer, because
 * a ring that appears and disappears with no fade needs the extra dwell to be
 * noticed at all.
 */
export const FLASH_HOLD_REDUCED_MS = 1500;
export const FLASH_OPACITY = 0.75;

/**
 * The halo that announces where the camera just took the reader.
 *
 * Drawn in {@link SELECTION_COLOR} rather than a colour of its own: it is the
 * selection announcing itself, not a third meaning, and it is told apart from
 * the thin selected stroke by being wide and blurred rather than by hue. A
 * second colour would compete with the one it is pointing at.
 *
 * Two layers, not three — a `line` layer draws polygon rings, so buildings,
 * ground zones and area sources need no fill. Both start invisible; the opacity
 * and its transition are written per phase by `feature-focus.tsx`, which is
 * also what adds these layers, lazily and last, so the halo is on top of
 * everything and a basemap rebuild simply re-adds it on the next request.
 */
export const FLASH_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.flashLine,
    type: "line",
    source: SOURCE_IDS.flash,
    layout: { "line-join": "round", "line-cap": "round" },
    paint: {
      "line-color": SELECTION_COLOR,
      "line-width": 14,
      "line-blur": 4,
      "line-opacity": 0,
    },
  },
  {
    id: LAYER_IDS.flashPoint,
    type: "circle",
    source: SOURCE_IDS.flash,
    filter: ["==", ["geometry-type"], "Point"],
    paint: {
      "circle-radius": 22,
      "circle-blur": 0.4,
      "circle-color": SELECTION_COLOR,
      "circle-opacity": 0,
    },
  },
];

/** The opacity property each flash layer fades, paired with its layer id. */
export const FLASH_OPACITY_PROPERTIES = [
  [LAYER_IDS.flashLine, "line-opacity"],
  [LAYER_IDS.flashPoint, "circle-opacity"],
] as const;

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

/**
 * The colour a source still awaiting acoustic review is cased in.
 *
 * Violet is the one hue nothing else on this map claims: sources are red,
 * receivers and the calc area blue, barriers brown, buildings grey, ground
 * zones green, and the level ramp runs green→red. It must especially not be
 * {@link SELECTION_COLOR} — a whole imported district is flagged at once, and
 * in amber all 608 of them would read as selected.
 *
 * Colour is not the only signal here either: the dash pattern and the ring are
 * what carry the meaning on a monochrome display.
 */
export const REVIEW_COLOR = "#a855f7";

/**
 * The flag an import leaves on a source whose acoustics it had to guess.
 *
 * This is the same question `needsAcousticsReview` asks of the model, and it
 * gives the same answer by construction: `getFeatureBoolean` accepts a real
 * boolean and nothing else, and MapLibre's `==` against a boolean literal is
 * just as strict — so a source carrying the *string* `"true"` raises no finding
 * and gets no flag, rather than the map and the validator disagreeing.
 *
 * Both halves are read, for that same reason: the sign-off retires the finding,
 * so a flag that ignored it would leave the map insisting on work the panel
 * considers done. A source with no `source_acoustics_reviewed` property answers
 * `null` here, and `!= true` holds for it, which is how an untouched import is
 * still flagged.
 *
 * It reads model properties rather than the validation report on purpose. The
 * report is a derived opinion computed over the store, while these layers draw
 * the projected display model; feeding one into the other would make the map a
 * source for the model instead of a projection of it.
 */
const REVIEW_REQUIRED_FILTER: ExpressionSpecification = [
  "all",
  ["==", ["get", "source_acoustics_review_required"], true],
  ["!=", ["get", PROP_ACOUSTICS_REVIEWED], true],
];

export const REVIEW_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.sourcesReviewLine,
    type: "line",
    source: SOURCE_IDS.sources,
    // A line layer draws a polygon's rings too, so one layer covers line
    // sources and area sources alike; only points need a shape of their own.
    filter: ["all", REVIEW_REQUIRED_FILTER, ["!=", ["geometry-type"], "Point"]],
    layout: { "line-join": "round", "line-cap": "round" },
    paint: {
      "line-color": REVIEW_COLOR,
      "line-width": 9,
      "line-opacity": 0.45,
      "line-dasharray": [2, 1.5],
    },
  },
  {
    id: LAYER_IDS.sourcesReviewPoint,
    type: "circle",
    source: SOURCE_IDS.sources,
    filter: ["all", REVIEW_REQUIRED_FILTER, ["==", ["geometry-type"], "Point"]],
    paint: {
      "circle-radius": 11,
      "circle-opacity": 0,
      "circle-stroke-width": 2.5,
      "circle-stroke-color": REVIEW_COLOR,
      "circle-stroke-opacity": 0.6,
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

/**
 * A ground-category area, drawn as the ground rather than as an object.
 *
 * Brown-green and unstroked-looking on purpose: a zone is not a thing sound
 * travels around, and giving it a building's weight would read as one. The
 * outline is solid where `calc-area`'s is dashed, because the two overlap
 * constantly — a yard with one calculation area over several ground zones is
 * the ordinary case — and the dash is what tells them apart at a glance.
 */
export const GROUND_ZONE_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.groundZoneFill,
    type: "fill",
    source: SOURCE_IDS.groundZones,
    paint: {
      "fill-color": "#84cc16",
      "fill-opacity": 0.14,
    },
  },
  {
    id: LAYER_IDS.groundZoneOutline,
    type: "line",
    source: SOURCE_IDS.groundZones,
    paint: {
      "line-color": "#65a30d",
      "line-width": 1.5,
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
    id: "ground-zones",
    label: m.label_ground_zones,
    layerIds: [LAYER_IDS.groundZoneFill, LAYER_IDS.groundZoneOutline],
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
    id: "review-required",
    label: m.label_review_required,
    layerIds: [LAYER_IDS.sourcesReviewLine, LAYER_IDS.sourcesReviewPoint],
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
 * The model layer the result raster is inserted *below*.
 *
 * `calc-area-fill` and not a building layer: `ModelLayers` adds
 * `CALC_AREA_LAYERS` first and unconditionally, whether or not a calculation
 * area exists, so it is the one model layer that is always there to anchor to.
 * A `beforeId` naming a layer the style does not hold makes MapLibre throw, so
 * the caller still has to check before passing it.
 */
export const BOTTOM_MODEL_LAYER_ID: string = LAYER_IDS.calcAreaFill;

/** The id of the group {@link RESULT_RASTER_LAYERS} belongs to. */
export const RESULT_RASTER_GROUP_ID = "result-raster";

/**
 * The computed grid, as one image under the model.
 *
 * MapLibre 5 has no `raster-color` and no `["raster-value"]` — both are Mapbox
 * GL JS v3 — so the values are coloured in `raster-image.ts` and this layer is
 * handed finished pixels through an `image` source. Three paint choices carry
 * an argument:
 *
 * `raster-resampling: "nearest"`, because a cell *is* a computed receiver.
 * Bilinear smoothing would invent levels between two grid points and blur the
 * 55/60 dB boundary, which is the line a reader is looking for, and would make
 * a cell disagree with the circle standing in the middle of it.
 *
 * `raster-opacity: 0.7`, so the basemap's streets and the model's buildings
 * read through the surface they are the cause of. The receiver circles stay
 * fully opaque on top.
 *
 * `raster-fade-duration: 0`, because the default cross-fade would blend the
 * previous indicator's image into the new one for 300 ms after a picker click
 * — showing a mixture of Lr,Tag and Lr,Nacht and labelling it neither.
 */
export const RESULT_RASTER_LAYERS: LayerSpecification[] = [
  {
    id: LAYER_IDS.resultRaster,
    type: "raster",
    source: SOURCE_IDS.resultRaster,
    paint: {
      "raster-opacity": 0.7,
      "raster-resampling": "nearest",
      "raster-fade-duration": 0,
    },
  },
];

/** The id of the group {@link RESULT_CONTOUR_LAYERS} belongs to. */
export const RESULT_CONTOURS_GROUP_ID = "result-contours";

/**
 * The computed grid as ISO-band lines, over the raster and under the model.
 *
 * Coloured by the same ramp the raster and the receiver circles use, off each
 * feature's own level under {@link RESULT_LEVEL_PROPERTY} — the ramp's stops
 * are 5 dB apart and `contour.DefaultInterval` is 5 dB, so a line lands on a
 * stop rather than between two.
 *
 * This is the tree's first data-driven `line-color`: every other one is a
 * constant or a `feature-state` case. It works because `rampToExpression` is
 * generic over the property name, and because each contour carries exactly one
 * level, so the interpolation resolves per feature rather than along the line.
 *
 * Unlabelled, and that is structural rather than an omission: no style in
 * `basemap.ts` declares `glyphs`, so a `symbol` layer with a `text-field`
 * renders nothing and logs a font error per tile. Adding a glyph source means
 * adding one to `OFFLINE_STYLE` too — a network dependency on the one path
 * that exists to survive without one. The legend names the levels instead.
 */
export const RESULT_CONTOUR_LAYERS: LayerSpecification[] = [
  {
    // The halo, under the line and wider. MapLibre has no `line-halo`, and a
    // contour needs one more than most lines do: it sits on the raster band
    // whose upper edge it *is*, so an unhaloed line is drawn in very nearly
    // the colour of the cells directly beneath it. The receiver circles solve
    // the same problem with `circle-stroke-color: #ffffff`.
    id: LAYER_IDS.resultContoursHalo,
    type: "line",
    source: SOURCE_IDS.resultContours,
    layout: { "line-join": "round", "line-cap": "round" },
    paint: { "line-color": "#ffffff", "line-width": 3.5, "line-opacity": 0.8 },
  },
  {
    id: LAYER_IDS.resultContours,
    type: "line",
    source: SOURCE_IDS.resultContours,
    layout: { "line-join": "round", "line-cap": "round" },
    paint: {
      "line-color": rampToExpression(
        NOISE_LEVEL_RAMP,
        RESULT_LEVEL_PROPERTY,
      ) as DataDrivenPropertyValueSpecification<string>,
      "line-width": 1.5,
    },
  },
];

/**
 * The result groups the layer control offers.
 *
 * Both of the two this list once lost are back, and the second took longer
 * for a reason worth keeping: `export.GenerateContours` had exactly one
 * caller, `aconiq export --format contour-geojson|contour-gpkg`, so a contour
 * artifact existed only after an explicit export in API mode and never at all
 * in browser mode. A toggle would have been a live control in one mode and a
 * dead one in the other. What fixed that was not a layer — it was moving Go's
 * marching squares behind the kernel boundary, the way `transform` and
 * `standards` already were, so both modes ask the same implementation. A
 * TypeScript tracer was never an option.
 *
 * The two groups overlap on screen and both default to visible, which is
 * deliberate: `layerVisibility` is keyed by group id alone and cannot express
 * "off, but only for runs that have a raster", so a default that depended on
 * the data would flip a choice the user had already made. The control is how
 * one of them goes away.
 */
export const RESULT_LAYER_GROUPS: LayerGroup[] = [
  {
    id: RESULT_RASTER_GROUP_ID,
    label: m.label_result_raster,
    layerIds: [LAYER_IDS.resultRaster],
    defaultVisible: true,
  },
  {
    id: RESULT_CONTOURS_GROUP_ID,
    label: m.label_result_contours,
    layerIds: [LAYER_IDS.resultContoursHalo, LAYER_IDS.resultContours],
    defaultVisible: true,
  },
  {
    id: RESULT_RECEIVERS_GROUP_ID,
    label: m.label_result_receiver_levels,
    layerIds: [LAYER_IDS.resultReceiverLevel],
    defaultVisible: true,
  },
];
