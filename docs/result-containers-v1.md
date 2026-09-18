# Result Containers v1 (Phase 6)

Status date: 2026-03-06

## Decisions

1. Raster persistence: **Option A selected**

- Custom binary payload (`float64` little-endian) + JSON metadata sidecar.
- GeoTIFF is no longer deferred: `backend/internal/report/export` writes GeoTIFF,
  COG, GeoPackage and contours, and reads its georeferencing out of the sidecar
  described below.

2. Receiver tables: **CSV + JSON selected for v1**

- CSV for interoperability.
- JSON for structured debugging and machine-readable exchange.
- Parquet deferred.

## Raster Container API

Implemented in `backend/internal/report/results`:

- Metadata: width, height, bands, nodata, unit, band names, CRS, georeference
- Indexing: `At(x,y,band)`, `Set(x,y,band,value)`
- Utilities: `Fill`, `Values`, validation

Persistence files:

- `<base>.json` metadata
- `<base>.bin` binary values

### Raster binary — byte contract

`<base>.bin` is a headerless little-endian `float64` array, tagged `float64-le-v1`
in the sidecar's `encoding`. Values are in the raster's own index order:
**band-major, then row, then column** — `(band*height + y)*width + x`. Every
value is finite; `results.SaveRaster` refuses a NaN or an infinity rather than
writing one, and `LoadRaster` checks the file is exactly `cell_count * 8` bytes.

Row 0 is the **southernmost** row, because `geo.GridReceiverSet.Generate` walks
Y ascending and the raster is laid out in the order its receivers were
generated. The sidecar says so rather than leaving it to be known; see below.

### Raster sidecar — where the cells are

`<base>.json` carries the metadata plus bookkeeping (`data_file`, `encoding`,
`created_at`, `cell_count`, `data_bytes`, `schema_name: "aconiq.raster.v1"`).
Two of its fields answer "where on the ground is this?", and they are the only
place that question is answered:

- **`crs`** — the CRS the values are in. This is the run's _compute_ CRS, not
  necessarily the project's: a run over a geographic project CRS is projected
  before it computes, and the results are in the projected one.
- **`georeference`** — `origin_x`, `origin_y`, `pixel_size_m`, `row_order`.
  **`origin_x`/`origin_y` is the centre of cell (0,0)**, not a corner, because
  a grid receiver is a point in the middle of the cell it stands for. It is
  therefore exactly the coordinate of the first row of `receivers.csv`.
  `row_order` is `"south-up"`; nothing writes anything else today, and a value
  that is not recognised is refused rather than guessed.

`georeference` is **absent** when the receivers are not a grid — explicit
receiver mode places points, and no cell size describes them. Absence means
"not a grid", never "a grid at the origin".

The conversion to GDAL's corner-based affine transform happens in exactly one
place, `exportfmt.GeoTransformFromGeoreference`: half a pixel west, and
`(height-1)` rows plus half a pixel north, because row 0 is the southernmost.

### Raster binary — the browser mirror

`frontend/src/model/raster-bin.ts` is the TypeScript half, as
`receiver-csv.ts` is for the CSV. Go is canonical; the mirror is pinned against
it by `backend/internal/report/results/testdata/raster-parity/`, written by
`raster_parity_test.go` and read by `raster-bin.parity.test.ts`.

The part a mirror gets wrong is the index order, not the encoding: a run
produces one value per receiver in receiver order, and writing them in arrival
order gives a file of exactly the right length, full of finite values, with
every cell in the wrong place. The parity fixture is therefore 3x2 over two
bands, so that a width/height swap and a band/row swap both change the bytes.

Browser mode stores the bytes in their own IndexedDB record rather than inside
the state document (`browser-storage.saveArtifactBytes`). The document is
written whole on every change, so a raster inside it would be structured-cloned,
every stored run included, on every save. `getArtifactContent` reads the record
on demand; `getArtifactURL` is synchronous and refuses binary content rather
than minting a blob from a placeholder.

`exportfmt.InferGeoTransformFromReceivers` reconstructs the same transform from
the receiver table's coordinates. It is the **fallback**, for sidecars written
before `georeference` existed, and it is never preferred where a declaration
exists: it cannot tell a grid from a scatter of receivers that happens to have
the right count. With neither, `aconiq export` refuses a georeferenced format
rather than writing one at the origin — which is what it used to do, silently.

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
