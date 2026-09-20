/**
 * The unit symbols a value is rendered with, spelled once.
 *
 * A standard's parameters carry their unit in the descriptor, so the run dialog
 * never needs this table — it reads `ParameterDefinition.unit`. The map's
 * feature editor does: its fields are *model properties*, which no descriptor
 * describes, so the frontend has to name their units itself.
 *
 * That makes this file a copy of `framework.Unit*` in
 * `backend/internal/standards/framework/framework.go`, and the Go comment there
 * says why a copy is dangerous: two spellings of one unit reaching a reader is
 * a defect. A user editing `speed_pkw_kph` on the map and the same value in the
 * run dialog must not be shown `Fz/h` in one place and `1/h` in the other —
 * which is exactly what the catalogue used to do.
 *
 * `units.test.ts` reads the Go constants and fails on any symbol that is not
 * among them, so the copy cannot drift into a second vocabulary.
 */
export const UNIT_METER = "m";
export const UNIT_KILOMETERS_PER_HOUR = "km/h";
export const UNIT_DECIBEL = "dB";
export const UNIT_PER_HOUR = "1/h";
export const UNIT_PERCENT = "%";
