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
 * `ParameterDefinition.unit`, never restated; the grouping is derived from the
 * name; and the label is composed rather than tabulated.
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
 * The raw name is rendered beside the label rather than replaced by it. It is
 * the CLI and API identity — it is what `--param` takes — so a user reading the
 * dialog and a user writing a command are looking at the same string.
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
 * A vehicle-class parameter inside a period or speed group reads as the class
 * alone; everything else is the backend's name, humanised.
 */
export function parameterLabel(param: ParameterDefinition): string {
  const tokens = significantTokens(param.name, param.unit);
  const group = parameterGroup(param.name);
  if (group !== "grid" && group !== "other") {
    // `speed_pkw_kph` and `traffic_day_pkw` both end in the class once the
    // unit suffix is off, which is why this runs after the trim and not on the
    // raw name.
    const classLabel = VEHICLE_CLASS_LABELS[tokens.at(-1) ?? ""];
    if (classLabel) return classLabel();
  }
  return tokens
    .map((token, i) => spell(token, i === 0))
    .join(" ")
    .trim();
}

/** " (km/h)", or "" when the parameter is dimensionless. */
export function parameterUnitSuffix(param: ParameterDefinition): string {
  return param.unit ? ` (${param.unit})` : "";
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
