# Coordinate reference-vector fixture

`proj-reference-vectors.json` holds external truth for the coordinate transforms in
`internal/geo`. It was generated once, locally, and checked in.

**PROJ is not a build or a CI dependency.** The vectors are the fixture; PROJ is only how
they were made. Nothing in `just go-ci` calls `cs2cs`.

| | |
| --- | --- |
| Generator | PROJ 9.8.1 (`/usr/bin/cs2cs`, Rel. 9.8.1, April 10th 2026) |
| Generated | 2026-09-16 |
| Script | [`generate-reference-vectors.sh`](generate-reference-vectors.sh) |
| Consumed by | `crs_reference_vectors_test.go` |

To regenerate, from this directory, with `cs2cs` and `jq` on `PATH`:

```sh
./generate-reference-vectors.sh > proj-reference-vectors.json
```

## Why the fixture exists

`github.com/wroge/wgs84` v1.1.7 computed the radius of curvature in the meridian in its
inverse transverse Mercator as `math.Pow(1-e²sin²φ₁, 3/2)`. `3/2` is an untyped integer
constant expression in Go, so the exponent was `1`. The forward direction was correct. The
test suite ran the two against each other and saw almost nothing, because the round trip it
ran started in degrees — inverse∘forward — and the error largely cancels on that ordering.

So the rule this fixture exists to enforce: **assert each direction on its own, against an
external reference.** A round trip proves that two errors cancel, never that either is
absent, and never which of the two is at fault.

## What is in the file

### `projection_cases` — the projection alone

52 rows. Each carries the full `+proj=tmerc` definition it was generated from, geodetic
coordinates, and plane coordinates on the **same ellipsoid**. No datum shift is involved on
either side, so a mismatch is the projection series and nothing else. The definitions
restate exactly what `wgs84.UTM`, `wgs84.ETRS89UTM` and `wgs84.DHDN2001GK` pass to
`Datum.TransverseMercator` (`reference.go:47`, `:62`, `:132`).

Coverage per CRS: central meridian, both zone edges, and the southern (47.27° N) and
northern (55.09° N) extremes of the German range, for EPSG:25831-25834, 31466-31469, 32632
and 32633; plus two points 5° off the central meridian of zone 32, because cross-zone
transforms are legal here and a truncated series degrades first where nobody looks.

One row reproduced by hand:

```sh
echo "9 51" | cs2cs -f '%.9f' \
  "+proj=longlat +ellps=GRS80 +no_defs" +to \
  +proj=tmerc +lat_0=0 +lon_0=9 +k=0.9996 +x_0=500000 +y_0=0 +ellps=GRS80 +units=m +no_defs
# 500000.000000001	5649824.888131557 0.000000000
```

Measured deviation of `geo.TransverseMercator` from these rows:

| direction | worst case | worst deviation | asserted at |
| --- | --- | --- | --- |
| forward (lon/lat → E/N) | EPSG:25831 western zone edge | 3.7e-9 m | 1e-6 m |
| inverse (E/N → lon/lat) | EPSG:32633 western zone edge | 2.1e-14° | 1e-11° |

### `crs_cases` — the whole pipeline

13 rows, one per supported EPSG code, generated as `EPSG:4326 → EPSG:<code>` through PROJ's
own datum handling. These cover EPSG:4326, 4258 and 3857, which have no projection case,
and they exercise what `geo.BuildTransformPipeline` actually builds.

`cs2cs` honours the authority axis order: EPSG:4326 and EPSG:4258 are latitude-then-longitude,
and **EPSG:31466-31469 are northing (X) then easting (Y)** — see `projinfo EPSG:31467`.
Aconiq is easting/longitude-first throughout, so the generator swaps those codes on the way
into the fixture.

```sh
echo "52.3759 9.7320" | cs2cs -f '%.9f' EPSG:4326 EPSG:25832
# 549829.857904924	5803100.337559809 0.000000000
```

The tolerances here are looser than the projection ones, and the slack is the **datum**
halves disagreeing, not the projection:

| codes | worst deviation, projected | worst deviation, geographic | cause |
| --- | --- | --- | --- |
| 25831-25834 | 1.0e-4 m (northing) | 9.2e-10° | `wgs84` models ETRS89 → WGS84 as an ellipsoid change; PROJ treats it as a null shift |
| 32632, 32633, 3857 | 2e-9 m | 2e-14° | same datum on both sides |
| 31466-31469 | 1.84 m (Aachen) | 2.6e-5° | `wgs84` carries one Helmert set for DHDN; PROJ 9.8.1 prefers the BETA2007 grid |

The DHDN row is the library's datum, and closing it would mean shipping a grid file. It
does not excuse the projection: the same Bessel ellipsoid is pinned to a micrometre by the
`projection_cases` rows for 31466-31469.
