# Result Containers v1 (Phase 6)

Status date: 2026-03-06

## Decisions

1. Raster persistence: **Option A selected**

- Custom binary payload (`float64` little-endian) + JSON metadata sidecar.
- GeoTIFF is deferred until dependency strategy is finalized.

2. Receiver tables: **CSV + JSON selected for v1**

- CSV for interoperability.
- JSON for structured debugging and machine-readable exchange.
- Parquet deferred.

## Raster Container API

Implemented in `backend/internal/report/results`:

- Metadata: width, height, bands, nodata, unit, band names
- Indexing: `At(x,y,band)`, `Set(x,y,band,value)`
- Utilities: `Fill`, `Values`, validation

Persistence files:

- `<base>.json` metadata
- `<base>.bin` binary values

## Receiver Table API

Implemented in `backend/internal/report/results`:

- `ReceiverTable` with ordered indicators and unit
- `ReceiverRecord` with coordinates, height, and per-indicator values
- Validation for duplicate IDs, required indicators, and finite numeric values
- Writers for JSON and CSV outputs

## Receiver table CSV — byte contract

The receivers CSV is written by two independent implementations — the CLI and
the browser build — and a run must produce the same file either way. **Go's
`encoding/csv.Writer`, with the default `Comma` and `UseCRLF = false`, is
canonical.** The TypeScript side mirrors it; where they disagree, Go is right.

- `backend/internal/report/results/receiver_table_io.go` —
  `WriteReceiverTableCSV`, a thin shell around `csv.Writer`.
- `frontend/src/model/receiver-csv.ts` — `buildReceiverTableCSV`, the mirror.
- `backend/internal/report/results/testdata/csv-parity/` — the fixtures. Written
  by `receiver_table_csv_test.go`, read by
  `frontend/src/model/receiver-csv.parity.test.ts`.

The contract, in full:

- Comma separator, no space after. **LF line endings, never CRLF.** Every record
  is terminated by `\n`, including the last one and including a header with no
  records — so an empty table is exactly `id,x,y,height_m,Lden\n`. No BOM.
- Header: `id`, `x`, `y`, `height_m`, then every entry of `indicator_order` in
  order. Data fields follow the same order. `unit` is not represented in the CSV.
- A field is written verbatim unless one of these holds:
  - it is empty — then it is zero bytes, **not** `""`;
  - it is exactly the two characters `\.`, the Postgres-COPY end-of-data
    sentinel that `encoding/csv` special-cases;
  - it contains `,`, `"`, CR or LF;
  - its **first code point** is a Go `unicode.IsSpace` character. Trailing or
    interior whitespace never forces quoting.
- Go's `unicode.IsSpace` is **not** JavaScript's `/\s/`. Go includes U+0085
  (NEL), which `\s` excludes; `\s` includes U+FEFF, which Go excludes. The
  TypeScript side therefore spells the set out as an explicit character class.
  Do not "simplify" it to `/^\s/`.
- Inside a quoted field: `"` becomes `""`, and everything else — CR and LF
  included — is copied verbatim. CSV has no backslash escaping.
- Numbers are `strconv.FormatFloat(v, 'f', -1, 64)`: the shortest digits that
  round-trip, rendered positionally, **never with an exponent**. JavaScript's
  `String(v)` picks the same digits but presents them differently for
  `|v| >= 1e21`, for `0 < |v| < 1e-6`, and for `-0`, `NaN` and `±Infinity`, so
  the mirror expands the exponential form and special-cases those four.
  Non-finite values spell as Go does: `NaN`, `+Inf`, `-Inf`.
- A missing indicator value writes the empty field. Go cannot reach that state —
  `ReceiverTable.Validate` rejects a record missing an ordered indicator, and
  `SaveReceiverTableCSV` validates before it opens the file — but the browser
  has no such gate, and an empty field keeps "not measured" distinguishable from
  a measured zero.

Browser runs already stored in IndexedDB keep whatever bytes they were written
with; only new runs get the canonical spelling. `PERSISTED_STATE_VERSION` is
deliberately **not** bumped for this: that guard exists to refuse a document the
code cannot read, and both spellings parse. Bumping it would discard the user's
model and every stored run to re-spell one downloadable.

## `aconiq export` Skeleton

`aconiq export` now:

- Selects a run (latest by default or explicit `--run-id`)
- Creates export bundle directory under `.noise/exports/`
- Copies available run log + provenance files
- Writes `export-summary.json`
- Optionally emits sample raster/table files (`--emit-sample-results`)
