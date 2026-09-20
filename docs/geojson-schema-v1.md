# GeoJSON Schema v1 (Phase 4)

Status date: 2026-03-06

This schema is the minimal common input for `aconiq import` / `aconiq validate`.

## Container

- GeoJSON `FeatureCollection`.
- Each item must be a GeoJSON `Feature` with `properties` and `geometry`.

### The `crs` Member

A collection may declare the CRS its coordinates are in, as the OGC named-CRS
member:

```json
{ "type": "name", "properties": { "name": "EPSG:25832" } }
```

`aconiq export` and `GET /api/v1/model` write it, and the browser import reads
it; the URN spelling `urn:ogc:def:crs:EPSG::25832` is accepted on the way in.
A collection that declares nothing is read in the project's own CRS — in the
browser, `EPSG:4326`.

Declaring it matters for a projected file. Its coordinates are metric eastings
and northings, and read as `EPSG:4326` they are handed to the transform as
longitude and latitude, which refuses them as out of range. RFC 7946 deprecates
this member, and nothing here depends on a consumer honouring it: it is written
so a file Aconiq produced says what it is, and read so one can come back.

## Required Properties

- `id` (string or numeric, normalized to string)
- `kind` (string): one of
  - `source`
  - `building`
  - `barrier`
  - `receiver`
  - `calc-area`
  - `ground-zone`

### `source` Features

- `source_type` required: `point` | `line` | `area`
- Geometry compatibility:
  - `point` -> `Point` or `MultiPoint`
  - `line` -> `LineString` or `MultiLineString`
  - `area` -> `Polygon` or `MultiPolygon`

### `building` Features

- `height_m` required and `> 0`. Missing is `building.height.required`, zero or
  negative is `building.height.invalid`. It is required rather than defaulted
  because it is a screening input: ISO 9613-2 reads it for the line-of-sight
  test, so a number chosen here would change computed levels without saying so.
- `height_source` optional, and written by an importer rather than by a reader.
  Its only value is `assumed`, and it says that `height_m` on this feature is
  the importer's assumption and not a measurement — `aconiq import --from-osm`
  sets it on a building whose way carries neither `height` nor `building:levels`
  (and on a wall or fence with no `height`, which has always been assumed to be
  2 m). Its **absence records no provenance**, not "read from the source": an
  ordinary GeoJSON import carries the property only if its own source did.
- Geometry must be `Polygon` or `MultiPolygon`

### `barrier` Features

- `height_m` required and `> 0`
- Geometry must be `LineString` or `MultiLineString`

### `receiver` Features

- `height_m` required and `> 0`
- Geometry must be `Point`
- Optional for 16. BImSchV traffic-noise assessment on explicit receivers:
  - `bimschv16_area_category`
  - accepted examples:
    - `allgemeines Wohngebiet`
    - `Mischgebiet`
    - `Gewerbegebiet`
    - `Krankenhaus`
  - the current assessment/export slice uses explicit receiver IDs plus this
    property to compare `LrDay`/`LrNight` against the legal threshold table

#### Receivers imported from SoundPLAN

A SoundPLAN Immissionsort is a facade position **plus the floor geometry of the
building behind it**, and SoundPLAN evaluates it once per floor. `aconiq import
--from-soundplan` therefore emits one `receiver` feature per (immission point,
floor), at `soundplan-receiver-<obj_id>-f<floor>`, each at that floor's own
height above ground. Those features carry:

| property                             | meaning                                                                             |
| ------------------------------------ | ----------------------------------------------------------------------------------- |
| `soundplan_obj_id`                   | The SoundPLAN object id. It is the `ObjID` column of `RREC*.abs`.                   |
| `soundplan_floor`                    | The floor, counting from 1. `0` means the column could not be expanded — see below. |
| `soundplan_floor_count`              | How many floors the immission point declares.                                       |
| `soundplan_receiver_name`            | The Immissionsort's name, which in practice is the building's address.              |
| `soundplan_z_m`                      | The floor's absolute elevation.                                                     |
| `soundplan_ground_height_m`          | The ground elevation SoundPLAN cached for the point; `height_m` is the difference.  |
| `soundplan_limit_day_db`             | The day threshold the SoundPLAN project assesses this point against.                |
| `soundplan_limit_night_db`           | The night threshold.                                                                |
| `soundplan_floor_attributes_missing` | Present and `true` only on an unexpanded point.                                     |

`aconiq compare` matches against the SoundPLAN reference table on
`(soundplan_obj_id, soundplan_floor)`, which is the key `RREC*.abs` is itself
indexed by, so the correspondence is exact rather than geometric.

An immission point whose floor attributes could not be decoded, or whose
geometry puts a floor at or below ground level, is **not** expanded: it becomes
a single receiver at the project's default receiver height carrying
`soundplan_floor: 0`, and the import warns naming the point. Such a receiver has
no usable key and will not match a reference row.

### `calc-area` Features

- Geometry must be `Polygon` — **not** `MultiPolygon`. A disjoint multi-part
  area would be reduced to one bounding box spanning the gap between its parts,
  which would compute receivers over ground the drawing explicitly excluded.
- No `height_m`: a calculation area is a footprint on the ground, not an object
  sound travels around.
- **At most one per model.** Two areas have no honest resolution — the union
  bounding box covers ground the user drew around, and picking the first one
  makes a run's extent depend on feature order in the file. A second one is
  rejected with `model.calc_area.duplicate`, naming the second feature.
- Like every other feature, it requires an `id`.

The calculation area is the extent the **automatic receiver grid** is built
over, replacing the extent of the sources. `grid_padding_m` is still applied to
it, so set `grid_padding_m=0` to use the drawn area exactly. `aconiq run` and
`POST /api/v1/runs` honour it identically, because it travels in the model file
rather than in the run request.

It has no effect in `custom` receiver mode: explicit `receiver` features are
used as they are, and no grid is built.

`aconiq compare` reads the same feature when it compares a run against imported
SoundPLAN grid maps, so that the comparison covers the ground the run actually
computed. A SoundPLAN bundle also records its own calculation area in
`.noise/model/soundplan-import-report.json`, which is used only when the model
carries no `calc-area` feature; `calc_area_source` in the comparison artifact
says which of the two was used.

`calc_area_role` says what that area then did. It is `receiver_placement` when
the raster receivers are synthesized over the area scanline by scanline, and
`row_direction_only` when the grid map's own origin and spacing were usable: the
decoded values exist at those cells and nowhere else, so the receivers go there
and the calculation area only decides which way the rows run.

### `ground-zone` Features

- `ground_factor` required, a finite number within `[0,1]`: 0 for acoustically
  hard ground, 1 for porous ground. It is **required rather than defaulted** —
  a zone whose factor is missing is indistinguishable from ground the model says
  nothing about, which the run already answers with its global `ground_factor`
  parameter, so defaulting it here would let a typo in the property name read as
  hard ground. Missing is `groundzone.factor.required`, out of range is
  `groundzone.factor.invalid`.
- Geometry must be `Polygon` — **not** `MultiPolygon`, for `calc-area`'s reason:
  a multi-part zone is a shape the UI cannot draw, and refusing one now settles
  how its parts combine before anything needs them to.
- No `height_m`: a ground zone is a footprint on the ground, not an object sound
  travels around.

A ground zone is where ISO 9613-2 Abschnitt 7.3.1's three regions get their
ground factors. The standard splits each propagation path into a source region
reaching 30·h_s from the source, a receiver region reaching 30·h_r back from the
receiver, and the middle region between them; G for a region is the porous
fraction of its ground, so a region crossing several zones resolves to the
**length-weighted mean** of their factors over its own span. Ground no zone
covers takes the run's global `ground_factor`, which is why a model with no
zones computes exactly what it always did.

Zones may overlap; the **first one in file order wins** over the stretch they
share, so the answer never depends on iteration order.

Only `iso9613` reads them today. Other standards ignore the kind rather than
refusing it, the way they ignore the vocabularies that are not theirs.

## Standard-Specific Geometry Conventions

The normalized model stays standard-agnostic, but some standards consume extra
geometry conventions from feature properties.

### RLS-19 Road

- Line sources may use `LineString` / `MultiLineString` coordinates in either
  `2D` (`[x, y]`) or `3D` (`[x, y, z]`) form.
- A source feature may optionally provide `lane_count` (or imported OSM `lanes`)
  to derive the normative source line automatically from the reference line.
  The current implementation applies the Bild 6 lane-count placement rules with
  right-hand traffic and a default 3.5 m lane width.
- When `3D` coordinates are present, the vertex `z` values are mapped to
  per-vertex road elevations for rising / descending roads.
- Alternatively, a source feature may provide:
  - `elevation_m`: one uniform elevation for the whole line, or
  - `centerline_elevations`: one elevation value per line vertex.
- For explicit per-direction modeling inside one source feature, RLS-19 also
  accepts `properties.rls19_directional_sources`, an array of objects where
  each entry defines one directional line source with:
  - `centerline` or `coordinates`
  - optional `id` / `direction_id` / `direction`
  - optional `lane_count`
  - optional per-direction acoustic overrides such as `traffic_day_*`,
    `traffic_night_*`, `speed_*_kph`, `surface_type`, `gradient_percent`,
    `junction_type`, `junction_distance_m`, `reflection_surcharge_db`,
    `elevation_m`, and `centerline_elevations`
- If directional sources would resolve to different `surface_type` values, the
  input must already be harmonized to a single shared surface choice that
  reflects the larger per-direction correction.
- A `source` feature with `source_type: area` under `rls19-road` is a Parkplatz,
  not a road; see the next section.

A surface/speed pairing that Tabelle 4a crosses out is refused rather than
computed as `D_StrO = 0` — `OPA` at or below 60 km/h, for instance. The error
names the vehicle group, the speed property and the band the row is tabulated
in. The check skips Kräder, for which Nr. 3.3.3 prescribes zero, and any vehicle
group with no traffic in either period.

### RLS-19 Parkplatz (§3.4)

A Parkplatz is a `source` feature with `source_type: area` and a single
`Polygon`. The polygon supplies both the Stellplatzfläche `P` and the centroid
the lot is propagated from, so neither is asserted by hand — a mistyped centroid
is the input that would silently move the level.

A polygon hole is subtracted from both the Stellplatzfläche and the centroid, so
a lot with a building in the middle is propagated from the Flächenschwerpunkt of
what is left — Nr. 3.2 puts the substitute point source "im Flächenschwerpunkt
jeder Teilfläche". A single-part `MultiPolygon` is accepted as a Polygon by
another spelling.

There is deliberately **no** `rls19_parking_area_m2` property: Eq. 10's
`−10·lg[P/1m²]` cancels when the lot is propagated as a total-power point
source, so such a property would be a required value that provably changes no
output.

A `MultiPolygon` is refused. The number of Stellplätze `n` can be neither split
across parts nor duplicated once per part, and §3.4 asks for a Parkplatz to be
divided into Teilflächen (Bild 10) — which is a modelling decision, so each
Teilfläche is its own feature with its own `n`.

Like the `schall03_*` vocabulary, the enumerations are carried as names rather
than ordinals: both are row identifiers of a published table, and a row ordinal
moves when the table does.

| Property                                  | Required                                      | Meaning                                                                                             |
| ----------------------------------------- | --------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `rls19_parking_num_spaces`                | yes                                           | Number of Stellplätze `n`, an integer ≥ 1                                                           |
| `rls19_parking_type`                      | yes                                           | Tabelle 6 Parkplatztyp: `pkw` (0 dB), `motorrad` (+5 dB), `lkw-omnibus` (+10 dB)                    |
| `rls19_parking_facility_type`             | no                                            | Tabelle 7 Parkplatztyp: `park-and-ride`, `tank-rastanlage`. Seeds both movement rates               |
| `rls19_parking_movements_per_space_day`   | unless `rls19_parking_facility_type` is given | Movements `N` per space and hour, 06–22 Uhr. An arrival and a departure each count as one movement  |
| `rls19_parking_movements_per_space_night` | unless `rls19_parking_facility_type` is given | The same for 22–06 Uhr                                                                              |
| `elevation_m`                             | no (default `0`)                              | Absolute Z of the parking surface. Polygon `z` ordinates are ignored; this is the elevation channel |

Omissions are errors, not defaults. An omitted `rls19_parking_type` does not
mean Pkw, and an omitted movement rate does not mean zero — a rate of zero is
the silence sentinel, which would report an occupied Parkplatz as inaudible. An
**explicitly stated** `0` is legal and means exactly that: a period with no
movements.

A stated `rls19_parking_facility_type` seeds both rates from Tabelle 7, and an
explicit rate then overrides its own period. Nr. 3.4.1 admits the Tabelle 7
standard values only where no suitable project-specific survey exists, so
stating the rates is the primary case; and Tabelle 7 carries only those two
rows, so an ordinary public car park has no standard rate to fall back on.

Parkplatz contributions are shielded by `barrier` and `building` features, and
reflect off `building` features and explicit reflectors, on the same §3.5 and
§3.6 chain road sources use. Eq. 3 gives the level of a Parkplatzteilfläche as
`L_W'' + 10·lg[P] − D_A − D_RV1 − D_RV2` and Eq. 1 sums Teilstücke and
Teilflächen "jeweils einschließlich etwaiger Spiegelschallquellen", so the two
source shapes differ only in how their sound power is composed. The remaining
deviations are listed in `docs/conformance/rls19-konformitaetserklaerung.md`.

### Schall 03 Rail

Schall 03 has two computation chains, and the properties are namespaced so they
cannot be confused. The `rail_*` properties feed the non-normative preview data
pack only; the `schall03_*` properties below feed the normative Anlage-2 chain.
A run resolves to the normative chain when at least one `source` feature carries
`schall03_operations`, and fails otherwise unless the preview chain is opted
into with `--param schall03_engine=preview`. See
`docs/conformance/schall03-konformitaetserklaerung.md`.

On a `source` feature with `source_type: line`:

- `schall03_operations` (**required** for the normative chain): a non-empty
  array, one entry per train type operating on the segment. Each entry gives
  either
  - `zugart`: a name from Beiblatt 1 (Eisenbahn, Tabelle 4) or Beiblatt 2
    (Straßenbahn), which supplies both the Fz composition and a default speed —
    e.g. `ICE-3-Vollzug`, `Nahverkehrszug-ET`, `Gueterzug-E-Lok`,
    `Niederflur-ET`; or
  - `fz_composition`: an array of `{ "fz": <Fz-Kategorie>, "count": <n> }`,
    where `fz` is 1–10 for Eisenbahn and 21–23 for Straßenbahn vehicles;

  plus `trains_per_hour_day` and `trains_per_hour_night`, an optional
  `speed_kph` (required when no `zugart` supplies one; it overrides the Zugart
  default when both are present), and an optional `train_type` label.

- `schall03_strecke_max_kph` (**required**): Streckenhöchstgeschwindigkeit in
  km/h. The effective speed per Nr. 4.3 / Nr. 5.3.2 is derived from this and the
  operation speed.
- `schall03_fahrbahn`: Eisenbahn track type (Tabelle 7) — `schwellengleis`
  (default, the reference type carrying no correction), `feste-fahrbahn`,
  `feste-fahrbahn-mit-absorber`, `bahnuebergang`.
- `schall03_s_fahrbahn`: Straßenbahn track type (Tabelle 15) —
  `schwellengleis` (default), `strassenbuendig`, `begruent-tief`,
  `begruent-hoch`.
- `schall03_surface`: active surface measure (Tabelle 8) — `none` (default),
  `bug`, `schienenstegdaempfer`, `schienenstegabschirmung`.
- `schall03_bridge_type`: 0 (default, no bridge) to 4, per Tabelle 9
  (Eisenbahn) or Tabelle 16 (Straßenbahn).
- `schall03_bridge_mitigation`: boolean, K_LM noise reduction on the bridge.
- `schall03_curve_radius_m`: curve radius in metres; 0 (default) means straight.
- `schall03_is_station`: boolean, Nr. 4.3 Personenbahnhof/Haltepunkt — raises
  the effective Eisenbahn speed to at least 70 km/h.
- `schall03_permanently_slow`: boolean, Nr. 5.3.2 exception for Straßenbahn
  sections permanently at v ≤ 30 km/h.
- `schall03_track_features`: array of the Nr. 5.3.2 track features the 50 km/h
  substitute speed is scoped to. Each entry is an object with `kind`
  (`weiche`, `kreuzung` or `haltestelle`) and the coordinates `x` and `y`. Each
  feature is projected onto the track centerline and substitutes the speed over
  25 m on either side; a track that declares none keeps the substitution over
  its whole length. A feature more than 25 m from the centerline is rejected,
  and the array cannot be combined with `schall03_permanently_slow`, whose
  exception presupposes a section carrying none of these features.
- `schall03_water_body_fraction`: 0–1, the share of the source–receiver path
  crossing water (Gl. 16).
- `elevation_m`: the **absolute Z** of the Schienenoberkante, shared with the preview path. The
  normative chain measures every height against the ground under the receiver, which it reads from
  an imported GeoTIFF DTM (`aconiq import --terrain`), so a project with no terrain places that
  ground at Z = 0 — and there `elevation_m` is read as a height above ground, which is what a model
  written without a DTM means by it.

On a `barrier` feature (`LineString`, `height_m` required): each consecutive
vertex pair becomes one barrier panel of that height.

- `schall03_reflective`: boolean, default false. A reflective barrier is
  additionally treated as a reflecting wall, and Gl. 20's D_refl applies.
- `schall03_base_height_m`: height of the absorbing Sockel, read only when the
  barrier is reflective (Gl. 20).
- `schall03_thickness_m` and `schall03_parallel_edges`: for wide barriers with
  double diffraction (Gl. 22, Gl. 25).
- `schall03_wall_surface`: Tabelle 18 category used when the barrier reflects —
  `hard` (default), `building`, `absorbing`, `highly-absorbing`.

On a `building` feature (`Polygon`, `height_m` required):

Every outer-ring edge becomes a shielding panel of the building's height, and
all panels of one ring share an obstacle identity, so a lateral path (Nr. 6.5)
may round the footprint's silhouette but never an interior wall vertex. Inner
rings are courtyards and are ignored. Shielding is unconditional: `height_m` is
already required on a `building` feature, so no model that validates today
starts failing.

- `schall03_reflecting_wall`: boolean, default false. When true, each outer-ring
  edge additionally becomes a reflecting wall; `schall03_wall_surface` then
  defaults to `building`. Reflection stays opt-in because it raises levels for
  every model that never asked for it, and because reflection paths are
  enumerated to third order over every facade. Gl. 20's D_refl is never applied
  to a building wall — it is scoped to reflektierende Schallschutzwände mit
  absorbierendem Sockel.

## Geometry Sanity Checks

- Coordinates must be finite numbers.
- `LineString` must have at least 2 points.
- Polygon rings must have at least 4 points and be closed.
- Self-intersection is checked on lines and on rings, and the two are **not**
  graded alike:

  | Code                                         | Level   |
  | -------------------------------------------- | ------- |
  | `geometry.polygon.self_intersection`         | error   |
  | `geometry.multipolygon.self_intersection`    | error   |
  | `geometry.linestring.self_intersection`      | warning |
  | `geometry.multilinestring.self_intersection` | warning |

  A ring's winding is read — point-in-polygon and the screening crossing counts
  both depend on it — so a footprint that crosses itself has no reliable inside
  and the model is refused. A line's winding is read by nothing, and a road
  drawn as one way that touches itself is a roundabout rather than a defect: 45
  of 2397 ways in a Berlin OSM extract are like that, which is not a proportion
  a reader can be asked to repair.

  The check compares every pair of segments, so its cost grows with the square
  of the vertex count. Above 10,000 points it is not attempted and the geometry
  is reported as unchecked, as a warning named `<code>.skipped`, rather than
  either walked or silently passed.

## CRS Plausibility Checks

Validation uses project CRS from `.noise/project.json`.

- Geographic CRS (for example EPSG:4326) enforces lon/lat bounds.
- Projected CRS with lon/lat-like bounds emits a mismatch warning.

## Debug Exports

`aconiq import` writes:

- `.noise/model/model.normalized.geojson`
- `.noise/model/model.dump.json`
- `.noise/model/validation-report.json`

## Changes to v1

`schema_version` stays at `1`. The field is written but never read back, and it
does not reach the file on disk — `ToFeatureCollection` does not emit it — so a
bump would be a stamp nobody consults. v1 has been widened in place before, for
`bimschv16_area_category` on `receiver` features.

- **2026-09-19 — `ground-zone` feature kind.** Ground-category areas, from which
  ISO 9613-2 resolves G per region instead of reading one global number three
  times. The same older-binary limitation the `calc-area` entry below records
  applies: a model carrying one fails an older `aconiq` with
  `feature.kind.invalid`.

- **2026-09-12 — `calc-area` feature kind.** The calculation area a user draws
  on the map, carried as a model feature so that it is reprojected with the rest
  of the model and reaches both the CLI and the local API through one path.

  Limitation: an older `aconiq` binary reading a model that contains a
  `calc-area` feature fails validation with `feature.kind.invalid` and no
  explanation of why the kind is unknown to it. Nothing versions this file, so
  it cannot do better than that here.
