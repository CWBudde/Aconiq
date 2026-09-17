// Schall 03 (Anlage 2) property names and vocabularies, for the map editor.
//
// This mirrors backend/internal/app/cli/run_extract_schall03_normative.go for
// the property names and backend/internal/standards/schall03/vocabulary.go for
// the enumerations; `schall03.test.ts` pins both against those files so the
// editor cannot offer a name the extractor does not know.
//
// The `rail_*` properties are deliberately absent: they feed the non-normative
// preview data pack, and an editor that offered both would let a model declare
// two disagreeing track descriptions. See docs/geojson-schema-v1.md.

/** On a `source` feature with `source_type: line`. */
export const PROP_SCHALL03_OPERATIONS = "schall03_operations";
export const PROP_SCHALL03_STRECKE_MAX_KPH = "schall03_strecke_max_kph";
export const PROP_SCHALL03_FAHRBAHN = "schall03_fahrbahn";
export const PROP_SCHALL03_S_FAHRBAHN = "schall03_s_fahrbahn";
export const PROP_SCHALL03_SURFACE = "schall03_surface";
export const PROP_SCHALL03_BRIDGE_TYPE = "schall03_bridge_type";
export const PROP_SCHALL03_BRIDGE_MITIGATION = "schall03_bridge_mitigation";
export const PROP_SCHALL03_CURVE_RADIUS_M = "schall03_curve_radius_m";
export const PROP_SCHALL03_IS_STATION = "schall03_is_station";
export const PROP_SCHALL03_PERMANENTLY_SLOW = "schall03_permanently_slow";
export const PROP_SCHALL03_TRACK_FEATURES = "schall03_track_features";
export const PROP_SCHALL03_WATER_BODY_FRACTION = "schall03_water_body_fraction";

/** On a `barrier` feature. */
export const PROP_SCHALL03_REFLECTIVE = "schall03_reflective";
export const PROP_SCHALL03_BASE_HEIGHT_M = "schall03_base_height_m";
export const PROP_SCHALL03_THICKNESS_M = "schall03_thickness_m";
export const PROP_SCHALL03_PARALLEL_EDGES = "schall03_parallel_edges";

/** On a `building` feature. */
export const PROP_SCHALL03_REFLECTING_WALL = "schall03_reflecting_wall";

/** On either, once the obstacle reflects. */
export const PROP_SCHALL03_WALL_SURFACE = "schall03_wall_surface";

/**
 * The elevation channel, shared with RLS-19 and with the preview rail path.
 * Not namespaced, because it describes the geometry rather than a standard.
 */
export const PROP_ELEVATION_M = "elevation_m";

// The vocabularies are carried as names rather than as table ordinals: an
// ordinal moves whenever the reference row moves, and a wire format must not
// carry one. The first entry of each list is the reference row — the one that
// carries no correction — which is why an omitted property is not a missing
// value but that row.

/** Tabelle 7, Fahrbahnart Eisenbahn. */
export const SCHALL03_FAHRBAHN_TYPES = [
  "schwellengleis",
  "feste-fahrbahn",
  "feste-fahrbahn-mit-absorber",
  "bahnuebergang",
] as const;

/** Tabelle 15, Fahrbahnart Straßenbahn. */
export const SCHALL03_S_FAHRBAHN_TYPES = [
  "schwellengleis",
  "strassenbuendig",
  "begruent-tief",
  "begruent-hoch",
] as const;

/** Tabelle 8, Maßnahme am Fahrweg. */
export const SCHALL03_SURFACE_TYPES = [
  "none",
  "bug",
  "schienenstegdaempfer",
  "schienenstegabschirmung",
] as const;

/** Tabelle 18, Wandoberfläche. */
export const SCHALL03_WALL_SURFACES = [
  "hard",
  "building",
  "absorbing",
  "highly-absorbing",
] as const;

/**
 * How many entries an array-valued property holds, or null when the property is
 * absent.
 *
 * `schall03_operations` and `schall03_track_features` are arrays of objects —
 * one entry per Zugart, one per Weiche or Haltestelle — and this editor does
 * not edit them: a form for them would be a second model editor, and getting it
 * half right is how a Fz composition silently loses a vehicle. The count is
 * reported instead, so the panel says what the feature carries rather than
 * pretending the feature carries nothing.
 */
export function arrayPropertyLength(
  properties: Record<string, unknown> | undefined,
  key: string,
): number | null {
  const value = properties?.[key];
  return Array.isArray(value) ? value.length : null;
}
