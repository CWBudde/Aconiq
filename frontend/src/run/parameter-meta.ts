import type { ParameterDefinition } from "@/api/client";
import { m } from "@/i18n/messages";

/**
 * How a standard's parameters are named and arranged in the run dialog.
 *
 * The backend publishes 98 parameters across thirteen modules, each with a
 * `name`, an optional `unit` and an English `description`. Until now the dialog
 * printed the name — `traffic_day_lkw1` — as the form label.
 *
 * **Nothing here is a mirror of the backend.** The unit is read from
 * `ParameterDefinition.unit` and rendered on the control by `UnitInput`, never
 * as label text; the grouping is derived from the name; and the label is
 * composed rather than tabulated.
 *
 * **And nothing here invents German for a normative standard.** Where the
 * catalogue already carries reviewed terminology — the RLS-19 vehicle classes,
 * which `map/feature-editor.tsx` has used since it was written — a parameter
 * gets it. Everywhere else the label is a humanised form of the backend's own
 * name, which reads as English. Composing German token by token would produce
 * `Richtwirkungskorrektur` and `Gleisrauheitsklasse` out of nowhere: invented
 * terminology for published norms, which is exactly what this project does not
 * do. Phase E's i18n pass is where the rest gets German, from someone who can
 * defend the words.
 *
 * The raw name is rendered under the control rather than replaced by it. It is
 * the CLI and API identity — it is what `--param` takes — so a user reading the
 * dialog and a user writing a command are looking at the same string.
 *
 * The *description* is the one thing here that is tabulated, and only for the
 * three normative modules — see `parameterDescription` at the foot of the file.
 */

export type ParameterGroupKey =
  | "grid"
  | "speeds"
  | "traffic_day"
  | "traffic_evening"
  | "traffic_night"
  | "other";

/** Render order. `other` is last because it is the catch-all, not a topic. */
export const PARAMETER_GROUP_ORDER: ParameterGroupKey[] = [
  "grid",
  "speeds",
  "traffic_day",
  "traffic_evening",
  "traffic_night",
  "other",
];

const GROUP_LABELS: Record<ParameterGroupKey, () => string> = {
  grid: m.param_group_grid,
  speeds: m.param_group_speeds,
  traffic_day: m.param_group_traffic_day,
  traffic_evening: m.param_group_traffic_evening,
  traffic_night: m.param_group_traffic_night,
  other: m.param_group_other,
};

/** The heading for a group. Called during render, never at module scope. */
export function parameterGroupLabel(key: ParameterGroupKey): string {
  return GROUP_LABELS[key]();
}

/**
 * Which group a parameter belongs to, by name prefix.
 *
 * Prefixes rather than a per-parameter table, so a standard added later lands
 * in a sensible group without being enumerated — and in `other` if it does not
 * match, which is a fine place to be.
 */
export function parameterGroup(name: string): ParameterGroupKey {
  if (name.startsWith("traffic_day_")) return "traffic_day";
  if (name.startsWith("traffic_evening_")) return "traffic_evening";
  if (name.startsWith("traffic_night_")) return "traffic_night";
  if (name.startsWith("speed_")) return "speeds";
  if (name.startsWith("grid_") || name.startsWith("receiver_")) return "grid";
  return "other";
}

/**
 * The vehicle classes RLS-19 counts in, as the catalogue already names them.
 *
 * Inside a "Tagesverkehr" group, `traffic_day_pkw` wants to read "Pkw" and
 * nothing more — the group has already said which period it is. The same four
 * classes carry the speed parameters.
 */
const VEHICLE_CLASS_LABELS: Record<string, () => string> = {
  pkw: m.label_vehicle_class_pkw,
  lkw1: m.label_vehicle_class_lkw1,
  lkw2: m.label_vehicle_class_lkw2,
  krad: m.label_vehicle_class_krad,
};

/** Tokens whose conventional spelling is not their capitalised form. */
const TOKEN_SPELLING: Record<string, string> = {
  db: "dB",
  crs: "CRS",
  epsg: "EPSG",
  id: "ID",
  iso9613: "ISO 9613-2",
  rls19: "RLS-19",
  osm: "OSM",
  ptw: "PTW",
  utm: "UTM",
};

/**
 * Trailing tokens that only restate the unit the descriptor already publishes.
 *
 * `grid_resolution_m` carries `unit: "m"` and `speed_pkw_kph` carries
 * `unit: "km/h"`, so the suffix would be printed twice. Dropped only when the
 * parameter actually declares a unit — a name is never shortened on the guess
 * that its last token is one.
 */
const UNIT_SUFFIXES = new Set([
  "m",
  "km",
  "kph",
  "db",
  "deg",
  "percent",
  "vph",
  "c",
]);

/**
 * The visible label for a parameter, without its unit.
 *
 * Three rules, in order. A vehicle-class parameter inside a period or speed
 * group reads as the class alone. A normative module's parameter reads as the
 * catalogue names it. Everything else is the backend's name, humanised, which
 * reads as English — and is why the gate below is on the standard: a module
 * whose sentences are German must not keep English labels above them, which is
 * the same half-and-half in the other direction.
 */
export function parameterLabel(
  standardId: string,
  param: ParameterDefinition,
): string {
  const tokens = significantTokens(param.name, param.unit);
  const group = parameterGroup(param.name);
  if (group !== "grid" && group !== "other") {
    // `speed_pkw_kph` and `traffic_day_pkw` both end in the class once the
    // unit suffix is off, which is why this runs after the trim and not on the
    // raw name.
    const classLabel = VEHICLE_CLASS_LABELS[tokens.at(-1) ?? ""];
    if (classLabel) return classLabel();
  }
  if (CATALOGUED_STANDARDS.has(standardId)) {
    const own = PARAMETER_LABELS[param.name];
    if (own !== undefined) return own();
  }
  return tokens
    .map((token, i) => spell(token, i === 0))
    .join(" ")
    .trim();
}

/** The name's tokens, less a trailing one that only restates the unit. */
function significantTokens(name: string, unit: string | undefined): string[] {
  const tokens = name.split("_").filter((t) => t !== "");
  if (unit && tokens.length > 1 && UNIT_SUFFIXES.has(tokens.at(-1) ?? "")) {
    tokens.pop();
  }
  return tokens;
}

function spell(token: string, first: boolean): string {
  const known = TOKEN_SPELLING[token];
  if (known !== undefined) return known;
  if (!first) return token;
  return token.charAt(0).toUpperCase() + token.slice(1);
}

/**
 * The modules whose parameter descriptions the catalogue owns.
 *
 * The backend writes every description in English — all 285 of them, across
 * thirteen modules — because that is the project's language for technical
 * content and because the CLI and the reports read the same string. Rendering
 * it verbatim under a German label is what put "Night Pkw per hour" beneath
 * "Nachtverkehr › Pkw", so the sentence is the frontend's to write, keyed on
 * the parameter name the backend already publishes. That is the shape Phase E
 * proposes for the backend's validation findings, arrived at early here because
 * a parameter needs no new identity: its name *is* the key.
 *
 * Only these three, and the gate is on the **standard**, not the parameter. A
 * per-parameter fallback would leave `cnossos-road` half German and half
 * English, which reads worse than the English it replaced. And German for the
 * scaffolds would mean coining `Gleisrauheitsklasse` for a module that is not
 * an implementation of the norm it names — the thing this project does not do.
 * `parameter-description.test.ts` holds these three to full coverage.
 */
const CATALOGUED_STANDARDS = new Set(["rls19-road", "schall03", "iso9613"]);

/**
 * Parameters whose whole meaning is "the value a source without one of its own
 * is given".
 *
 * Twenty-five of the forty-five across the three modules are exactly that, and
 * one sentence says it better than the backend's did: "Night Pkw per hour" only
 * restated the group, the label and the unit, all three of which are already on
 * screen beside it.
 *
 * An explicit set rather than a `traffic_`/`speed_` prefix rule: a prefix would
 * silently claim a later parameter that happens to start the same way and is
 * not a per-source default, and the sentence would then be wrong rather than
 * missing — which the coverage test could not see.
 */
const SOURCE_DEFAULT_PARAMETERS = new Set([
  // rls19-road
  "speed_pkw_kph",
  "speed_lkw1_kph",
  "speed_lkw2_kph",
  "speed_krad_kph",
  "gradient_percent",
  "traffic_day_pkw",
  "traffic_day_lkw1",
  "traffic_day_lkw2",
  "traffic_day_krad",
  "traffic_night_pkw",
  "traffic_night_lkw1",
  "traffic_night_lkw2",
  "traffic_night_krad",
  // schall03
  "rail_train_class",
  "rail_traction_type",
  "rail_track_type",
  "rail_track_form",
  "rail_track_roughness_class",
  "rail_average_train_speed_kph",
  "rail_curve_radius_m",
  "rail_on_bridge",
  "traffic_day_trains_per_hour",
  "traffic_night_trains_per_hour",
  // iso9613
  "iso9613_source_height_m",
  "iso9613_sound_power_level_db",
]);

/**
 * What a normative module's parameter is called.
 *
 * Absent for the twelve RLS-19 vehicle-class parameters: the rule above already
 * names those from the terminology the catalogue has carried since
 * `map/feature-editor.tsx` was written, and a second entry saying "Pkw" would be
 * one more place for it to drift.
 *
 * These are terms for published norms, and they are hand-written and reviewed,
 * not composed. Composing them token by token is what this file used to refuse
 * to do — and rightly: it would have produced them "out of nowhere", with
 * nobody able to defend the words. Writing them down is the other half of that
 * refusal, not a reversal of it.
 */
const PARAMETER_LABELS: Record<string, () => string> = {
  // Shared across the three modules.
  grid_resolution_m: m.param_label_grid_resolution_m,
  grid_padding_m: m.param_label_grid_padding_m,
  receiver_height_m: m.param_label_receiver_height_m,
  min_distance_m: m.param_label_min_distance_m,
  // rls19-road
  surface_type: m.param_label_surface_type,
  gradient_percent: m.param_label_gradient_percent,
  segment_length_m: m.param_label_segment_length_m,
  segment_length_mode: m.param_label_segment_length_mode,
  // schall03
  schall03_engine: m.param_label_schall03_engine,
  rail_train_class: m.param_label_rail_train_class,
  rail_traction_type: m.param_label_rail_traction_type,
  rail_track_type: m.param_label_rail_track_type,
  rail_track_form: m.param_label_rail_track_form,
  rail_track_roughness_class: m.param_label_rail_track_roughness_class,
  rail_average_train_speed_kph: m.param_label_rail_average_train_speed_kph,
  rail_curve_radius_m: m.param_label_rail_curve_radius_m,
  rail_on_bridge: m.param_label_rail_on_bridge,
  // The group heading already says which period, as it does for Pkw and Krad.
  traffic_day_trains_per_hour: m.param_label_traffic_day_trains_per_hour,
  traffic_night_trains_per_hour: m.param_label_traffic_night_trains_per_hour,
  air_absorption_db_per_km: m.param_label_air_absorption_db_per_km,
  ground_attenuation_db: m.param_label_ground_attenuation_db,
  slab_track_correction_db: m.param_label_slab_track_correction_db,
  bridge_correction_db: m.param_label_bridge_correction_db,
  curve_correction_db: m.param_label_curve_correction_db,
  // iso9613
  iso9613_source_height_m: m.param_label_iso9613_source_height_m,
  iso9613_sound_power_level_db: m.param_label_iso9613_sound_power_level_db,
  iso9613_directivity_correction_db:
    m.param_label_iso9613_directivity_correction_db,
  iso9613_tonality_correction_db: m.param_label_iso9613_tonality_correction_db,
  iso9613_impulsivity_correction_db:
    m.param_label_iso9613_impulsivity_correction_db,
  ground_factor: m.param_label_ground_factor,
  air_temperature_c: m.param_label_air_temperature_c,
  relative_humidity_percent: m.param_label_relative_humidity_percent,
  meteorology_assumption: m.param_label_meteorology_assumption,
  c0_met: m.param_label_c0_met,
};

/**
 * The parameters that say something the label and the unit cannot.
 *
 * Keyed on the name alone, so `grid_resolution_m` carries one sentence and not
 * three: the concept does not change between RLS-19, Schall 03 and ISO 9613-2,
 * and the backend's three variants of it differ only in prose.
 */
const PARAMETER_DESCRIPTIONS: Record<string, () => string> = {
  // Shared across the three modules.
  grid_resolution_m: m.param_desc_grid_resolution_m,
  grid_padding_m: m.param_desc_grid_padding_m,
  receiver_height_m: m.param_desc_receiver_height_m,
  min_distance_m: m.param_desc_min_distance_m,
  // rls19-road
  surface_type: m.param_desc_surface_type,
  segment_length_m: m.param_desc_segment_length_m,
  segment_length_mode: m.param_desc_segment_length_mode,
  // schall03
  schall03_engine: m.param_desc_schall03_engine,
  air_absorption_db_per_km: m.param_desc_air_absorption_db_per_km,
  ground_attenuation_db: m.param_desc_ground_attenuation_db,
  slab_track_correction_db: m.param_desc_slab_track_correction_db,
  bridge_correction_db: m.param_desc_bridge_correction_db,
  curve_correction_db: m.param_desc_curve_correction_db,
  // iso9613
  iso9613_directivity_correction_db:
    m.param_desc_iso9613_directivity_correction_db,
  iso9613_tonality_correction_db: m.param_desc_iso9613_tonality_correction_db,
  iso9613_impulsivity_correction_db:
    m.param_desc_iso9613_impulsivity_correction_db,
  ground_factor: m.param_desc_ground_factor,
  air_temperature_c: m.param_desc_air_temperature_c,
  relative_humidity_percent: m.param_desc_relative_humidity_percent,
  meteorology_assumption: m.param_desc_meteorology_assumption,
  c0_met: m.param_desc_c0_met,
};

/**
 * The sentence under a parameter's control, in the reader's language.
 *
 * `null` where there is nothing to say — never an empty paragraph. Called
 * during render, never at module scope, so it resolves in the active locale.
 */
export function parameterDescription(
  standardId: string,
  param: ParameterDefinition,
): string | null {
  if (CATALOGUED_STANDARDS.has(standardId)) {
    const own = PARAMETER_DESCRIPTIONS[param.name];
    if (own !== undefined) return own();
    if (SOURCE_DEFAULT_PARAMETERS.has(param.name)) {
      return m.param_desc_source_default();
    }
  }
  return param.description === undefined || param.description === ""
    ? null
    : param.description;
}

/** Whether the catalogue, rather than the backend, wrote this sentence. */
export function hasCataloguedDescription(
  standardId: string,
  name: string,
): boolean {
  return (
    CATALOGUED_STANDARDS.has(standardId) &&
    (PARAMETER_DESCRIPTIONS[name] !== undefined ||
      SOURCE_DEFAULT_PARAMETERS.has(name))
  );
}

/**
 * Whether the catalogue, rather than `spell()`, named this parameter.
 *
 * True for the vehicle classes as well: they are catalogue terminology, just
 * reached by a rule instead of a table.
 */
export function hasCataloguedLabel(
  standardId: string,
  param: ParameterDefinition,
): boolean {
  if (!CATALOGUED_STANDARDS.has(standardId)) return false;
  if (PARAMETER_LABELS[param.name] !== undefined) return true;
  const group = parameterGroup(param.name);
  if (group === "grid" || group === "other") return false;
  // The same tokens `parameterLabel` matches on, from the same call: asking
  // whether the vehicle rule fires must not be a second, looser version of it.
  const tokens = significantTokens(param.name, param.unit);
  return VEHICLE_CLASS_LABELS[tokens.at(-1) ?? ""] !== undefined;
}
