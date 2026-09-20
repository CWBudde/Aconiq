import type {
  ModelFeature,
  ModelReceiver,
  ValidationCode,
  ValidationIssue,
  ValidationIssueParams,
  ValidationReport,
} from "./types";
import { isGeometryCompatible } from "./types";
import type { Point2D } from "./geometry";
import {
  hasSelfIntersection,
  SELF_INTERSECTION_POINT_LIMIT,
} from "./self-intersection";
import {
  polygonParts,
  PROP_PARKING_FACILITY_TYPE,
  PROP_PARKING_MOVEMENTS_DAY,
  PROP_PARKING_MOVEMENTS_NIGHT,
  PROP_PARKING_NUM_SPACES,
  PROP_PARKING_TYPE,
  RLS19_PARKING_FACILITY_TYPES,
  RLS19_PARKING_LOT_TYPES,
} from "./rls19-parking";
import {
  getFeatureNumber,
  getFeatureString,
  getRLS19ReviewRequired,
  RLS19_JUNCTION_TYPES,
  RLS19_SPEED_KEYS,
  RLS19_SURFACE_TYPES,
  RLS19_TRAFFIC_KEYS,
} from "./source-acoustics";

// The RLS-19 gradient band, as numbers rather than as prose.
//
// They used to be spelled twice — once in the comparison and once inside the
// English sentence beside it — which is precisely the drift a parameterised
// message removes: the bounds the check enforces are now the bounds the
// reader is shown, in whichever language they are shown in.
const RLS19_GRADIENT_MIN_PERCENT = -12;
const RLS19_GRADIENT_MAX_PERCENT = 12;

/**
 * The model's findings: the features, the receivers, and the ids they share.
 *
 * There is no features-only variant any more. A receiver-blind report is a
 * *valid-looking* report — a model whose only defect is a duplicate or
 * malformed receiver came back clean — and the import wizard, the last caller
 * that had a reason for one, now reads receivers too. Everything reading the
 * store goes through `useModelValidation`.
 */
export function validateProjectModel(
  features: ModelFeature[],
  receivers: ModelReceiver[],
): ValidationReport {
  const errors: ValidationIssue[] = [];
  const warnings: ValidationIssue[] = [];

  // An empty model is an error, not merely "nothing to say": without it the
  // run gate would accept a model with nothing in it. `useModelValidation`
  // answers "empty" before calling this rather than filtering the code out
  // afterwards, which would leave the count disagreeing with `valid`. The code
  // still carries a message of its own — a finding the renderer cannot render
  // would be a worse exception than one nobody shows.
  if (features.length === 0 && receivers.length === 0) {
    errors.push({
      level: "error",
      code: "model.empty",
      featureId: "",
      params: {},
    });
    return {
      valid: false,
      errors,
      warnings,
      checkedAt: new Date().toISOString(),
    };
  }

  const ids = new Set<string>();

  for (const feature of features) {
    validateUniqueID(feature.id, "feature", ids, errors);
    ids.add(feature.id);

    validateFeature(feature, errors, warnings);
    validateGeometry(feature, errors, warnings);
  }

  for (const receiver of receivers) {
    validateUniqueID(receiver.id, "receiver", ids, errors);
    ids.add(receiver.id);
    validateReceiver(receiver, errors);
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings,
    checkedAt: new Date().toISOString(),
  };
}

function validateFeature(
  feature: ModelFeature,
  errors: ValidationIssue[],
  warnings: ValidationIssue[],
): void {
  const { id, kind } = feature;

  switch (kind) {
    case "source": {
      if (!feature.sourceType) {
        errors.push({
          level: "error",
          code: "source.type.required",
          featureId: id,
          params: {},
        });
      } else if (
        !isGeometryCompatible(feature.geometry.type, feature.sourceType)
      ) {
        errors.push({
          level: "error",
          code: "source.geometry.mismatch",
          featureId: id,
          params: {
            geometry: feature.geometry.type,
            sourceType: feature.sourceType,
          },
        });
      }
      validateRLS19SourceAcoustics(feature, errors, warnings);
      break;
    }
    case "building": {
      if (feature.heightM == null) {
        errors.push({
          level: "error",
          code: "building.height.required",
          featureId: id,
          params: {},
        });
      } else if (feature.heightM <= 0) {
        errors.push({
          level: "error",
          code: "building.height.invalid",
          featureId: id,
          params: {},
        });
      }
      if (
        feature.geometry.type !== "Polygon" &&
        feature.geometry.type !== "MultiPolygon"
      ) {
        errors.push({
          level: "error",
          code: "building.geometry.invalid",
          featureId: id,
          params: {},
        });
      }
      break;
    }
    case "ground-zone": {
      validateGroundZone(feature, errors);
      break;
    }
    case "barrier": {
      if (feature.heightM == null) {
        errors.push({
          level: "error",
          code: "barrier.height.required",
          featureId: id,
          params: {},
        });
      } else if (feature.heightM <= 0) {
        errors.push({
          level: "error",
          code: "barrier.height.invalid",
          featureId: id,
          params: {},
        });
      }
      if (
        feature.geometry.type !== "LineString" &&
        feature.geometry.type !== "MultiLineString"
      ) {
        errors.push({
          level: "error",
          code: "barrier.geometry.invalid",
          featureId: id,
          params: {},
        });
      }
      break;
    }
  }
}

/**
 * Self-intersection, for every feature that carries a line or a ring.
 *
 * The browser had no geometry checks at all — not a laxer opinion, an absent
 * one — which is how an import could be shown as clean in the preview and then
 * refused by the save one screen later, with nothing earlier to explain it.
 *
 * The severities are the backend's rather than a second opinion: a ring that
 * crosses itself has no reliable inside, because point-in-polygon and the
 * screening crossing counts both read its winding, so the model is refused; a
 * line's winding is read by nothing and a road drawn as one way that touches
 * itself is a roundabout, so it is reported and kept. A Berlin OSM extract
 * carries 45 of the latter across 2397 ways and none of the former.
 *
 * Coordinate finiteness, ring closure and minimum vertex counts are still the
 * backend's alone. `normalize.ts` and the drawing tools do not produce them,
 * so they are a narrower gap than this one was.
 */
function validateGeometry(
  feature: ModelFeature,
  errors: ValidationIssue[],
  warnings: ValidationIssue[],
): void {
  const { id } = feature;

  const report = (
    code: ValidationCode,
    points: Point2D[],
    closed: boolean,
  ): void => {
    const intersects = hasSelfIntersection(points, closed);

    if (intersects === null) {
      warnings.push({
        level: "warning",
        code: `${code}.skipped` as ValidationCode,
        featureId: id,
        params: {
          points: points.length,
          limit: SELF_INTERSECTION_POINT_LIMIT,
        },
      } as ValidationIssue);
      return;
    }

    if (!intersects) return;

    // Closed decides the severity as well as the geometry; see the note above.
    (closed ? errors : warnings).push({
      level: closed ? "error" : "warning",
      code,
      featureId: id,
      params: {},
    } as ValidationIssue);
  };

  switch (feature.geometry.type) {
    case "LineString": {
      const line = parseLine(feature.geometry.coordinates);
      if (line) report("geometry.linestring.self_intersection", line, false);
      break;
    }
    case "MultiLineString": {
      for (const line of parseLines(feature.geometry.coordinates)) {
        report("geometry.multilinestring.self_intersection", line, false);
      }
      break;
    }
    case "Polygon":
    case "MultiPolygon": {
      const parts = polygonParts(feature);
      if (!parts) break;

      const code =
        feature.geometry.type === "Polygon"
          ? "geometry.polygon.self_intersection"
          : "geometry.multipolygon.self_intersection";

      for (const rings of parts) {
        for (const ring of rings) {
          report(code, ring, true);
        }
      }
      break;
    }
    default:
      break;
  }
}

/** One line's positions, or null when the coordinates are not a line. */
function parseLine(coordinates: unknown): Point2D[] | null {
  if (!Array.isArray(coordinates) || coordinates.length === 0) return null;

  const points: Point2D[] = [];
  for (const position of coordinates) {
    if (
      !Array.isArray(position) ||
      typeof position[0] !== "number" ||
      typeof position[1] !== "number"
    ) {
      return null;
    }
    points.push({ x: position[0], y: position[1] });
  }

  return points;
}

/** Every member line of a MultiLineString that parses. */
function parseLines(coordinates: unknown): Point2D[][] {
  if (!Array.isArray(coordinates)) return [];

  const lines: Point2D[][] = [];
  for (const member of coordinates) {
    const line = parseLine(member);
    if (line) lines.push(line);
  }

  return lines;
}

/**
 * A ground zone is a polygon carrying a `ground_factor` in [0,1], from which
 * ISO 9613-2 resolves G per region instead of reading one global number three
 * times.
 *
 * The factor is required rather than defaulted, matching the backend: a zone
 * without one is indistinguishable from ground the model says nothing about,
 * which the run already answers with its global fallback.
 */
function validateGroundZone(
  feature: ModelFeature,
  errors: ValidationIssue[],
): void {
  const { id } = feature;
  const factor = feature.properties?.["ground_factor"];

  if (factor == null) {
    errors.push({
      level: "error",
      code: "groundzone.factor.required",
      featureId: id,
      params: {},
    });
  } else if (
    typeof factor !== "number" ||
    !Number.isFinite(factor) ||
    factor < 0 ||
    factor > 1
  ) {
    errors.push({
      level: "error",
      code: "groundzone.factor.invalid",
      featureId: id,
      params: {},
    });
  }

  if (feature.geometry.type !== "Polygon") {
    errors.push({
      level: "error",
      code: "groundzone.geometry.invalid",
      featureId: id,
      params: {},
    });
  }
}

function validateRLS19SourceAcoustics(
  feature: ModelFeature,
  errors: ValidationIssue[],
  warnings: ValidationIssue[],
): void {
  if (feature.sourceType === "area") {
    validateRLS19Parking(feature, errors);

    return;
  }

  if (feature.sourceType !== "line") {
    return;
  }

  const surfaceType = getFeatureString(
    feature,
    "surface_type",
    "road_surface_type",
  );
  if (
    surfaceType &&
    !RLS19_SURFACE_TYPES.includes(
      surfaceType as (typeof RLS19_SURFACE_TYPES)[number],
    )
  ) {
    errors.push({
      level: "error",
      code: "source.rls19.surface_type.invalid",
      featureId: feature.id,
      params: { value: surfaceType },
    });
  }

  const junctionType = getFeatureString(
    feature,
    "junction_type",
    "road_junction_type",
  );
  if (
    junctionType &&
    !RLS19_JUNCTION_TYPES.includes(
      junctionType as (typeof RLS19_JUNCTION_TYPES)[number],
    )
  ) {
    errors.push({
      level: "error",
      code: "source.rls19.junction_type.invalid",
      featureId: feature.id,
      params: { value: junctionType },
    });
  }

  const gradient = getFeatureNumber(
    feature,
    "gradient_percent",
    "road_gradient_percent",
  );
  if (
    gradient != null &&
    (!Number.isFinite(gradient) ||
      gradient < RLS19_GRADIENT_MIN_PERCENT ||
      gradient > RLS19_GRADIENT_MAX_PERCENT)
  ) {
    errors.push({
      level: "error",
      code: "source.rls19.gradient.invalid",
      featureId: feature.id,
      params: {
        min: RLS19_GRADIENT_MIN_PERCENT,
        max: RLS19_GRADIENT_MAX_PERCENT,
      },
    });
  }

  const junctionDistance = getFeatureNumber(
    feature,
    "junction_distance_m",
    "road_junction_distance_m",
  );
  if (
    junctionDistance != null &&
    (!Number.isFinite(junctionDistance) || junctionDistance < 0)
  ) {
    errors.push({
      level: "error",
      code: "source.rls19.junction_distance.invalid",
      featureId: feature.id,
      params: {},
    });
  }

  const reflectionSurcharge = getFeatureNumber(
    feature,
    "reflection_surcharge_db",
  );
  if (reflectionSurcharge != null && !Number.isFinite(reflectionSurcharge)) {
    errors.push({
      level: "error",
      code: "source.rls19.reflection_surcharge.invalid",
      featureId: feature.id,
      params: {},
    });
  }

  const uniformSpeed = getFeatureNumber(feature, "road_speed_kph");
  if (
    uniformSpeed != null &&
    (!Number.isFinite(uniformSpeed) || uniformSpeed <= 0)
  ) {
    errors.push({
      level: "error",
      code: "source.rls19.road_speed.invalid",
      featureId: feature.id,
      params: {},
    });
  }

  for (const key of RLS19_SPEED_KEYS) {
    const value = getFeatureNumber(feature, key);
    if (value != null && (!Number.isFinite(value) || value <= 0)) {
      errors.push({
        level: "error",
        code: "source.rls19.speed.invalid",
        featureId: feature.id,
        params: { field: key },
      });
    }
  }

  for (const key of RLS19_TRAFFIC_KEYS) {
    const value = getFeatureNumber(feature, key);
    if (value != null && (!Number.isFinite(value) || value < 0)) {
      errors.push({
        level: "error",
        code: "source.rls19.traffic.invalid",
        featureId: feature.id,
        params: { field: key },
      });
    }
  }

  if (getRLS19ReviewRequired(feature)) {
    warnings.push({
      level: "warning",
      code: "source.rls19.review_required",
      featureId: feature.id,
      params: {},
    });
  }
}

// PARKING_PROPERTIES is what marks an area source as an RLS-19 Parkplatz.
//
// The check has to be property-driven, not `source_type: area` alone: this
// validator is standard-agnostic — it runs from the import and map pages with
// no standard selected — and an area source is also a legitimate input to
// cnossos-industry and bub-industry, which would otherwise collect RLS-19
// errors it has no business carrying. It mirrors how the road checks in this
// file behave: they fire on a property that is present and wrong, never on one
// that is absent.
//
// The consequence is deliberate and matches the CLI, where `aconiq validate` is
// likewise standard-agnostic and extraction is the enforcement point: an area
// source carrying no parking property at all passes here and is refused by the
// extractor when an rls19-road run actually reads it.
const PARKING_PROPERTIES = [
  PROP_PARKING_NUM_SPACES,
  PROP_PARKING_TYPE,
  PROP_PARKING_FACILITY_TYPE,
  PROP_PARKING_MOVEMENTS_DAY,
  PROP_PARKING_MOVEMENTS_NIGHT,
];

function isRLS19Parking(feature: ModelFeature): boolean {
  const properties = feature.properties ?? {};

  return PARKING_PROPERTIES.some((key) => properties[key] !== undefined);
}

/** The §3.4 codes, each paired with exactly the parameters its sentence needs. */
type ParkingIssueSpec = {
  [Code in Extract<ValidationCode, `source.rls19.parking.${string}`>]: {
    code: Code;
    params: ValidationIssueParams[Code];
  };
}[Extract<ValidationCode, `source.rls19.parking.${string}`>];

// validateRLS19Parking surfaces the §3.4 refusals here rather than letting them
// arrive as a kernel error. An omitted Parkplatztyp is not Pkw and an omitted
// movement rate is not zero — zero is the silence sentinel, which would report
// an occupied Parkplatz as inaudible. An explicitly stated 0 is legal.
//
// A half-filled Parkplatz is the realistic mistake and is caught here; a feature
// carrying nothing is not assumed to be one at all.
function validateRLS19Parking(
  feature: ModelFeature,
  errors: ValidationIssue[],
): void {
  if (!isRLS19Parking(feature)) {
    return;
  }

  // A code and its parameters, with the level and the feature id filled in.
  //
  // The spec is a union over the codes, not `{ code: string; params: object }`,
  // so naming the wrong parameter for a code does not compile. That is the
  // whole reason the helper survived the move away from prose: it used to save
  // two repeated fields, and it now also carries the correlation.
  const push = (spec: ParkingIssueSpec): void => {
    errors.push({ level: "error", featureId: feature.id, ...spec });
  };

  // The one §3.4 refusal that is about the geometry rather than a property, and
  // the only one both extractors made and this file did not. `buildParkingSources`
  // and `rls19ParkingPolygon` each refuse a lot that is not a single polygon,
  // because n can be neither split across the parts nor repeated once per part.
  // Without the check here a multi-part lot can be filled in completely — on the
  // map or by an import — report clean, and be refused only when a run reads it.
  //
  // `null` is not a finding: it means the geometry is no polygon at all, which
  // `isGeometryCompatible` already refuses as a source-type mismatch.
  const parts = polygonParts(feature);
  if (parts !== null && parts.length !== 1) {
    push({
      code: "source.rls19.parking.geometry.multipart",
      params: { parts: parts.length, field: PROP_PARKING_NUM_SPACES },
    });
  }

  const numSpaces = getFeatureNumber(feature, PROP_PARKING_NUM_SPACES);
  if (numSpaces === undefined) {
    push({
      code: "source.rls19.parking.num_spaces.missing",
      params: { field: PROP_PARKING_NUM_SPACES },
    });
  } else if (numSpaces < 1 || Math.trunc(numSpaces) !== numSpaces) {
    push({
      code: "source.rls19.parking.num_spaces.invalid",
      params: { field: PROP_PARKING_NUM_SPACES },
    });
  }

  const lotType = getFeatureString(feature, PROP_PARKING_TYPE)?.trim();
  if (!lotType) {
    push({
      code: "source.rls19.parking.parking_type.missing",
      params: {
        field: PROP_PARKING_TYPE,
        expected: RLS19_PARKING_LOT_TYPES.join(", "),
      },
    });
  } else if (
    !RLS19_PARKING_LOT_TYPES.includes(
      lotType.toLowerCase() as (typeof RLS19_PARKING_LOT_TYPES)[number],
    )
  ) {
    push({
      code: "source.rls19.parking.parking_type.invalid",
      params: { value: lotType },
    });
  }

  const facility = getFeatureString(
    feature,
    PROP_PARKING_FACILITY_TYPE,
  )?.trim();
  const seeded =
    facility !== undefined &&
    facility !== "" &&
    RLS19_PARKING_FACILITY_TYPES.includes(
      facility.toLowerCase() as (typeof RLS19_PARKING_FACILITY_TYPES)[number],
    );

  if (facility && !seeded) {
    push({
      code: "source.rls19.parking.facility_type.invalid",
      params: {
        field: PROP_PARKING_FACILITY_TYPE,
        value: facility,
        expected: RLS19_PARKING_FACILITY_TYPES.join(", "),
      },
    });
  }

  for (const key of [
    PROP_PARKING_MOVEMENTS_DAY,
    PROP_PARKING_MOVEMENTS_NIGHT,
  ]) {
    const rate = getFeatureNumber(feature, key);
    if (rate === undefined && !seeded) {
      push({
        code: "source.rls19.parking.movements.missing",
        params: { field: key, facilityField: PROP_PARKING_FACILITY_TYPE },
      });
    } else if (rate !== undefined && (!Number.isFinite(rate) || rate < 0)) {
      push({
        code: "source.rls19.parking.movements.invalid",
        params: { field: key },
      });
    }
  }
}

function validateReceiver(
  receiver: ModelReceiver,
  errors: ValidationIssue[],
): void {
  const [x, y] = receiver.geometry.coordinates;
  if (!Number.isFinite(x) || !Number.isFinite(y)) {
    errors.push({
      level: "error",
      code: "receiver.coordinates.invalid",
      featureId: receiver.id,
      params: {},
    });
  }

  if (!Number.isFinite(receiver.heightM) || receiver.heightM <= 0) {
    errors.push({
      level: "error",
      code: "receiver.height.invalid",
      featureId: receiver.id,
      params: {},
    });
  }
}

function validateUniqueID(
  id: string,
  kind: "feature" | "receiver",
  ids: Set<string>,
  errors: ValidationIssue[],
): void {
  if (ids.has(id)) {
    // Spelled out rather than composed from `kind`, because the two codes buy
    // two separate messages: the kind is a noun inside the sentence, and a
    // language that declines it cannot take it as a parameter.
    errors.push({
      level: "error",
      code:
        kind === "feature" ? "feature.id.duplicate" : "receiver.id.duplicate",
      featureId: id,
      params: {},
    });
  }
}
