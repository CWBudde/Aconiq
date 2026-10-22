# SoundPLAN Export

Status: research note for Gate 4 Phase SL-7 (#117), subphase SL-7.1, plus the mapping
`aconiq export --format soundplan-shp` implements. The export is one-way, Aconiq → SoundPLAN.
It complements the importer in `internal/io/soundplanimport`, which this note does not cover
(see `soundplan-format.md`).

**Nothing in this note has been verified against a licensed SoundPLAN installation.** The format
support below comes from SoundPLAN's own published product material, not from an import that was
run. The open items at the end say what has to be checked before the pilot acceptance item
"Demonstrate a documented SoundPLAN import workflow" can be ticked.

## What SoundPLAN imports

SoundPLAN publishes two desktop editions, SoundPLANnoise and the reduced SoundPLANessential. Per
the vendor's module descriptions and the edition comparison:

| Route                 | SoundPLANnoise | SoundPLANessential | Notes                                                                           |
| --------------------- | -------------- | ------------------ | ------------------------------------------------------------------------------- |
| DXF                   | yes            | yes                | Geometry on layers; attributes only through layer names or extended data.       |
| ASCII                 | yes            | yes                | SoundPLAN's own column layouts; the column contract is not publicly documented. |
| Excel                 | yes            | not stated         | In the December 2025 module description, absent from the April 2024 one.        |
| ESRI Shapefile        | yes            | yes                | Object properties (building height, …) assigned from attributes during import.  |
| OpenStreetMap         | yes            | yes                | Not a target for an export.                                                     |
| QSI (DIN 45687)       | yes            | not stated         | The German exchange format for noise software; carries acoustic semantics.      |
| Georeferenced bitmaps | yes            | yes                | Background only.                                                                |

During import, SoundPLAN assigns file attributes to object properties "not only by direct
assignment but also using formulas". That assignment table is where a SoundPLAN user maps a
Shapefile column to building height, receiver name or source sound power.

Sources (retrieved 2026-10-22):

- _Modulbeschreibung SoundPLANnoise_, April 2024 edition,
  <https://www.soundplan.eu/fileadmin/user_upload/soundplan-dateien/software/Modulbeschreibung_SoundPLANnoise__4_.pdf>:
  - the base module imports from DXF, ASCII and QSI (DIN 45687);
  - the number of imported objects is not limited;
  - properties can be assigned by formula on import.

  The December 2025 edition, available only through an expiring download link, adds Excel to
  that list.

- _SoundPLANnoise vs. SoundPLANessential_,
  <https://www.soundplan.eu/en/software/soundplannoise/comparison-spnoise-vs-spessential>, and
  the PDF product comparison
  <https://www.soundplan.eu/fileadmin/user_upload/soundplan-dateien/software/EN_Produktvergleich_essential_-_noise.pdf>:
  "Interface to ASCII, DXF and Shape files for the model data import; Import of object
  properties (e.g. building height)".
- The SoundPLANessential brochure (EN), likewise only behind an expiring download link: existing
  model data is imported "via the DXF, ASCII, ESRI Shapefile or OpenStreet Map interfaces".

## Why ESRI Shapefile

- **Both editions read it**, and the vendor names it explicitly for model data with attributes.
- **It is an open, stable format.** The ESRI Shapefile Technical Description (1998) has not
  changed in decades, and every GIS reads it, so an export can be checked in QGIS or GDAL
  without SoundPLAN.
- **QSI was ruled out** although it is the purpose-built choice. DIN 45687 is a licensed
  standard that this project does not hold, so a mapping could be neither written nor checked
  against it. It is the obvious second target once the standard text is available, held as
  `docs/policies/data-handling.md` describes.
- **DXF** carries attributes only as layer names or extended data, which is too weak for a
  sound power level per source.
- **ASCII** has no public column contract to write against.

The issue's instruction to "prefer established exchange formats … over reverse engineering
proprietary project files" rules out writing SoundPLAN's own `.geo` files. The importer parses
those, but writing them would mean asserting a binary layout nobody has documented.

## What the export writes

`aconiq export --format soundplan-shp` writes into `formats/soundplan/` of the export bundle:

- one Shapefile per non-empty layer (`.shp`, `.shx`, `.dbf`, `.cpg` = `UTF-8`, `.prj`);
- `terrain.txt` when the project has a DTM;
- `soundplan-mapping.json`, which lists each layer, each column, the model property it holds,
  its unit and its decimals, and every property that could not be carried.

It exports **model input, not results**. SoundPLAN computes the levels.

### Layers

| Layer           | Shape    | Model features                       |
| --------------- | -------- | ------------------------------------ |
| `buildings`     | Polygon  | `kind: building`                     |
| `barriers`      | PolyLine | `kind: barrier`                      |
| `receivers`     | Point    | `kind: receiver`                     |
| `sources_point` | Point    | `kind: source`, `source_type: point` |
| `sources_line`  | PolyLine | `kind: source`, `source_type: line`  |
| `sources_area`  | Polygon  | `kind: source`, `source_type: area`  |
| `calc_area`     | Polygon  | `kind: calc-area`                    |
| `ground_zones`  | Polygon  | `kind: ground-zone`                  |

A Shapefile holds one geometry type and SoundPLAN imports one object type per assignment, so the
split is by kind and, for sources, by source type. A feature of any other kind is not exported
and is listed as lost.

Multi-geometries become multipart records. The exception is a MultiPoint, which becomes one
record per part with `ID` = `<id>-NN` and `PART` = the part number. Each part carries the
feature's full attributes, because ISO 9613-2 extraction (`extractISO9613Sources`) computes
every part as a source of its own at the feature's Lw. A MultiPoint therefore arrives in
SoundPLAN as the same set of sources Aconiq computed.

Polygon rings are reoriented: the Shapefile specification wants outer rings clockwise, the
reverse of RFC 7946.

### Columns

Fixed columns, with a declared unit and rounding:

| Field      | Model property                      | Unit | Decimals |
| ---------- | ----------------------------------- | ---- | -------- |
| `ID`       | `id`                                |      |          |
| `HEIGHT_M` | `height_m`                          | m    | 3        |
| `LW_DB`    | `iso9613_sound_power_level_db`      | dB   | 2        |
| `HS_M`     | `iso9613_source_height_m`           | m    | 3        |
| `K_DIR_DB` | `iso9613_directivity_correction_db` | dB   | 2        |
| `K_TON_DB` | `iso9613_tonality_correction_db`    | dB   | 2        |
| `K_IMP_DB` | `iso9613_impulsivity_correction_db` | dB   | 2        |
| `G`        | `ground_factor`                     | 1    | 3        |
| `AREA_CAT` | `bimschv16_area_category`           |      |          |

A fixed column is written on a layer only when one of its features carries the property.

**Every other scalar property passes through** under a derived name: upper case, `[A-Z0-9_]`,
at most ten characters, made unique with a `_1`, `_2` suffix. `soundplan-mapping.json` resolves
each name back to its property. That covers the RLS-19 and Schall 03 attributes, which have no
fixed SoundPLAN counterpart here: their meaning in SoundPLAN's road and rail emission models is
the user's assignment to make, not this export's to assert.

**Rounding.** The `.dbf` numeric fields are the only place the export rounds.

- A fixed column rounds at its declared decimals.
- A passthrough number gets the fewest decimals that write every value of that property exactly,
  capped at 6. A value that needs more is rounded there and reported.
- Rounding is `strconv`'s correctly rounded decimal conversion, so an exact binary tie rounds to
  even: 95.125 is written as 95.12.

Coordinates are written as IEEE doubles and are not rounded.

**Lost, and reported** as `feature_id`, `property` and `reason`:

- structured values (arrays, objects, e.g. `rls19_directional_sources`, `schall03_operations`);
- strings over 254 bytes, whose cell is left empty — the feature ID included, and a MultiPoint
  part's `<id>-NN`;
- a property whose values differ in type across one layer's features;
- vertex z coordinates;
- features of kinds without a layer;
- passthrough numbers rounded at 6 decimals.

### CRS

SoundPLAN measures distances in the plane of the coordinates it imports. The export therefore
writes the CRS the run computed in:

- a projected project is written in its own CRS;
- a geographic project is written in the ETRS89 / UTM zone the run projected it into;
- a bundle whose model `--target-crs` reprojected is projected back first.

`source_crs` in the report records any such projection.

Only conformal metric CRS get a `.prj`: ETRS89 / UTM 31–34, WGS84 / UTM 32–33 and DHDN /
Gauss-Krüger 2–5. Anything else is refused, Web Mercator included, whose plane distances are
about 1.6× the ground distance in Germany. The refusal is user input: `aconiq export` exits 2.

The `.prj` is written in ESRI WKT. Whether SoundPLAN reads it, or expects the project's
coordinate system to be set by hand, is unverified (see below).

### Heights and terrain

`HEIGHT_M` and `HS_M` are heights **above ground**, as the model holds them. Ground elevation
is not folded into them.

When the project has a DTM, `terrain.txt` carries it as elevation points:

- one `X;Y;Z` line per DTM cell centre, with a decimal comma and millimetre resolution;
- the stored cell values (`terrain.EachCell`), not interpolated samples, with nodata cells
  skipped;
- row-major order from the north-west corner.

This is the layout of the `Höhen.txt` files SoundPLAN projects carry, and
`soundplanimport.ParseHoehenTxtFile` reads it back in the tests. For a geographic project each
cell centre is projected, so the points are exact DTM samples but no longer lie on a grid.

## Manual import (to be confirmed in SoundPLAN)

The expected workflow, from the vendor material above:

1. Create a SoundPLAN project whose coordinate system matches `crs` in
   `soundplan-mapping.json`.
2. In the Geo-Database, import each Shapefile as its object type: buildings, walls, receivers,
   point/line/area sources, calculation area, ground absorption areas.
3. In each import's assignment table:
   - map `HEIGHT_M` to the object height and `ID` to the name;
   - for sources, map `LW_DB` to the sum sound power level and `HS_M` to the source height.
4. Import `terrain.txt` as ASCII elevation points and triangulate the DGM.
5. Compare the object counts with `features` and `records` per layer in the report, and
   review `lossy` before calculating.

## Open before the pilot acceptance item can be ticked

- Import a representative export into licensed SoundPLANnoise and SoundPLANessential, and record
  the versions, the import dialogs used and any deviation from the steps above.
- Confirm whether the `.prj` is read, and whether `.cpg` = UTF-8 is honoured for umlauts.
- Confirm that `terrain.txt` imports as elevation points without a column definition.
- Phase and scenario membership of construction sources, once SL-6 defines it.
- Route geometry, once SL-3 delivers it.
- AVV Baulärm attributes (operating periods, duty cycles), once SL-2 defines them.
- Octave-band spectra: the model carries none today; the export writes sum levels only.
