/**
 * The receiver-table CSV builder — the browser half of a byte contract.
 *
 * Go's `encoding/csv.Writer` (default `Comma`, `UseCRLF = false`) is canonical.
 * Everything here mirrors it: the CLI writes a receivers CSV through
 * `WriteReceiverTableCSV` in
 * `backend/internal/results/receiver_table_io.go`, and a browser-mode run
 * must produce the same bytes for the same table. The contract is written out in
 * `docs/result-containers-v1.md`, section "Receiver table CSV — byte contract",
 * and pinned from both sides by
 * `backend/internal/results/testdata/csv-parity/`.
 *
 * Deliberately import-free. `model/` must not grow a dependency on `api/client`
 * for the sake of two field names, so the input types below are declared
 * structurally: `ReceiverTable` from `api/client` is assignable to them, and so
 * is a filtered-and-sorted view assembled by the results page.
 */

/**
 * Structural mirror of `ReceiverFacade` in `backend/internal/results`: the
 * footprint edge a facade receiver belongs to, with `from` / `to` as `[x, y]`.
 */
export interface ReceiverCSVFacade {
  building_ref: string;
  part: number;
  ring: number;
  edge: number;
  from: [number, number];
  to: [number, number];
}

/**
 * Structural mirror of `ReceiverRecord` in `api/client`.
 *
 * `placement_rule`, `facade` and `storey` are the optional facade attributes
 * of `results.ReceiverRecord`. Go requires all three or none; this type does
 * not, and the builder below copes with either.
 */
export interface ReceiverCSVRecord {
  id: string;
  x: number;
  y: number;
  height_m: number;
  values: Record<string, number>;
  placement_rule?: string;
  facade?: ReceiverCSVFacade;
  storey?: number;
}

/**
 * Structural mirror of the part of `ReceiverTable` the CSV represents. `unit`
 * is deliberately absent: the Go writer does not put it in the file either.
 */
export interface ReceiverCSVTable {
  indicator_order: string[];
  records: ReceiverCSVRecord[];
}

/**
 * The runes Go's `unicode.IsSpace` accepts, as an explicit character class.
 *
 * Do NOT simplify this to `/^\s/`. The two sets are not the same, and they
 * differ in both directions:
 *
 *   - Go includes U+0085 NEL; JavaScript's `\s` does not.
 *   - JavaScript's `\s` includes U+FEFF; Go's `unicode.IsSpace` does not.
 *
 * Either difference silently changes whether a field is quoted, which is a
 * byte-level divergence from the CLI that no type checker will catch. The set
 * is `unicode.White_Space` as `$(go env GOROOT)/src/unicode/tables.go` spells
 * it, plus U+0085 and U+00A0 from the Latin-1 fast path in `graphic.go`.
 */
const GO_LEADING_SPACE =
  /^[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]/u;

/**
 * Mirrors `csv.Writer.fieldNeedsQuotes` in
 * `$(go env GOROOT)/src/encoding/csv/writer.go`.
 *
 * The clauses, in Go's order: the empty field is never quoted (Go dropped that
 * in 1.4, so an empty field is zero bytes and not `""`); the exact string `\.`
 * always is, because Postgres COPY reads it as an end-of-data marker; a field
 * containing the delimiter, a quote, a CR or an LF must be; and finally a field
 * whose *first* code point is a Go space must be. Only the first — trailing and
 * interior whitespace never force quoting.
 */
function fieldNeedsQuotes(field: string): boolean {
  if (field === "") return false;
  if (field === "\\.") return true;
  if (/["\r\n,]/.test(field)) return true;
  return GO_LEADING_SPACE.test(field);
}

/**
 * Encodes one field the way `csv.Writer.Write` does.
 *
 * Inside a quoted field Go doubles `"` and copies everything else verbatim —
 * CR and LF included, because `UseCRLF` is false. There is no backslash
 * escaping anywhere in CSV.
 */
export function encodeCSVField(field: string): string {
  if (!fieldNeedsQuotes(field)) return field;
  return `"${field.replaceAll('"', '""')}"`;
}

/**
 * Mirrors `strconv.FormatFloat(value, 'f', -1, 64)`.
 *
 * `'f'` means positional, never an exponent; `-1` means the shortest digit
 * string that round-trips back to the same double. `String(value)` in
 * JavaScript picks the same digits — both implement shortest round-tripping —
 * but presents them differently in four places, and each one is a real byte
 * difference against the CLI:
 *
 *   - `|value| >= 1e21` switches to `1e+21` notation;
 *   - `0 < |value| < 1e-6` switches to `1e-7` notation;
 *   - `-0` prints as `0`, where Go prints `-0`;
 *   - `NaN` and `±Infinity` print as `NaN`/`Infinity`/`-Infinity`, where Go
 *     prints `NaN`/`+Inf`/`-Inf`.
 *
 * So the exponential form is expanded by hand below. Do NOT reach for
 * `toFixed`: it caps out at 100 fraction digits where a subnormal double
 * needs 1074, and it rounds rather than reproducing the shortest digits.
 *
 * Go can never be handed a non-finite value here — `ReceiverTable.Validate`
 * rejects one — but the browser has no such gate, so the spellings are stated
 * rather than left to chance.
 */
export function formatCSVFloat(value: number): string {
  if (Number.isNaN(value)) return "NaN";
  if (value === Infinity) return "+Inf";
  if (value === -Infinity) return "-Inf";
  // `String(-0)` is "0"; Go's FormatFloat keeps the sign.
  if (Object.is(value, -0)) return "-0";

  const text = String(value);
  const exponentAt = text.indexOf("e");
  if (exponentAt < 0) return text;

  return expandExponential(text, exponentAt);
}

/**
 * Rewrites `String(value)`'s exponential form positionally: `1e+21` becomes
 * `1000000000000000000000`, `5e-324` becomes `0.000…005`. The digits are taken
 * verbatim — this only moves the decimal point, so the shortest-round-tripping
 * property of the input survives untouched.
 */
function expandExponential(text: string, exponentAt: number): string {
  const sign = text.startsWith("-") ? "-" : "";
  const mantissa = text.slice(sign.length, exponentAt);
  const exponent = Number(text.slice(exponentAt + 1));

  const pointAt = mantissa.indexOf(".");
  const integerPart = pointAt < 0 ? mantissa : mantissa.slice(0, pointAt);
  const fractionPart = pointAt < 0 ? "" : mantissa.slice(pointAt + 1);

  const digits = integerPart + fractionPart;
  // Where the decimal point lands within `digits` once the exponent is applied.
  const pointPosition = integerPart.length + exponent;

  if (pointPosition <= 0) {
    return `${sign}0.${"0".repeat(-pointPosition)}${digits}`;
  }
  if (pointPosition >= digits.length) {
    return `${sign}${digits}${"0".repeat(pointPosition - digits.length)}`;
  }
  return `${sign}${digits.slice(0, pointPosition)}.${digits.slice(pointPosition)}`;
}

/**
 * The columns `WriteReceiverTableCSV` appends after the indicators when any
 * record carries a `facade`, and only then — so a grid table, or a custom
 * table of plain receivers, keeps the bytes it had before they existed.
 */
const FACADE_CSV_COLUMNS = [
  "building_ref",
  "part",
  "ring",
  "edge",
  "storey",
  "placement_rule",
];

/**
 * The facade fields of one row. A record without a `facade` writes an empty
 * field in each, as Go does for a plain receiver in a mixed table.
 *
 * The integers go through `String`, which agrees with Go's `strconv.Itoa` on
 * every integer Go can hold here. A missing `storey` or `placement_rule` on a
 * facade row writes the empty field: Go's `Validate` refuses that state, the
 * browser has no such gate.
 */
function facadeCSVFields(record: ReceiverCSVRecord): string[] {
  const { facade } = record;
  if (facade === undefined) return FACADE_CSV_COLUMNS.map(() => "");

  return [
    encodeCSVField(facade.building_ref),
    String(facade.part),
    String(facade.ring),
    String(facade.edge),
    record.storey === undefined ? "" : String(record.storey),
    encodeCSVField(record.placement_rule ?? ""),
  ];
}

/**
 * Builds the receiver-table CSV exactly as the CLI writes it.
 *
 * Header: `id`, `x`, `y`, `height_m`, then `indicator_order` in order, then —
 * only when some record carries a `facade` — `building_ref`, `part`, `ring`,
 * `edge`, `storey`, `placement_rule`; data fields in the same order. Every record — the header included — is terminated
 * by a single LF, so an empty table is one header line plus a newline, and the
 * file never ends without one. No CRLF anywhere, and no BOM.
 *
 * A record that omits an ordered indicator writes the empty field. Go cannot
 * reach that state: `ReceiverTable.Validate` rejects a record missing an
 * ordered indicator and `SaveReceiverTableCSV` validates before it writes. The
 * browser has no such gate, and writing an empty field rather than a `0` keeps
 * a missing measurement distinguishable from a measured zero.
 */
export function buildReceiverTableCSV(table: ReceiverCSVTable): string {
  const withFacade = table.records.some(
    (record) => record.facade !== undefined,
  );
  const header = ["id", "x", "y", "height_m", ...table.indicator_order];
  if (withFacade) header.push(...FACADE_CSV_COLUMNS);
  const lines = [header.map(encodeCSVField).join(",")];

  for (const record of table.records) {
    const row = [
      encodeCSVField(record.id),
      formatCSVFloat(record.x),
      formatCSVFloat(record.y),
      formatCSVFloat(record.height_m),
    ];
    for (const indicator of table.indicator_order) {
      const value = record.values[indicator];
      row.push(value === undefined ? "" : formatCSVFloat(value));
    }
    if (withFacade) row.push(...facadeCSVFields(record));
    lines.push(row.join(","));
  }

  return `${lines.join("\n")}\n`;
}
