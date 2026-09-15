import type {
  ModelFeature,
  ModelReceiver,
  ValidationIssue,
  ValidationReport,
} from "./types";
import { isGeometryCompatible } from "./types";
import {
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

/**
 * Validates features alone, as if no receiver were placed.
 *
 * **Nothing calls this any more.** A receiver-blind report is a *valid-looking*
 * report: a model whose only defect is a duplicate or malformed receiver comes
 * back clean, and nothing fails. The import wizard was the last caller and the
 * exception this docblock used to grant it — "a candidate list that has no
 * receivers by construction" — stopped being true when the wizard started
 * reading them. Everything reading the store goes through
 * `useModelValidation`; everything else calls `validateProjectModel` directly.
 */
export function validateModel(features: ModelFeature[]): ValidationReport {
  return validateProjectModel(features, []);
}

export function validateProjectModel(
  features: ModelFeature[],
  receivers: ModelReceiver[],
): ValidationReport {
  const errors: ValidationIssue[] = [];
  const warnings: ValidationIssue[] = [];

  // An empty model is an error, not merely "nothing to say": without it the
  // run gate would accept a model with nothing in it. `useModelValidation`
  // answers "empty" before calling this rather than filtering the code out
  // afterwards, which would leave the count disagreeing with `valid` — and
  // this message is hardcoded English, so it must never reach the UI.
  if (features.length === 0 && receivers.length === 0) {
    errors.push({
      level: "error",
      code: "model.empty",
      featureId: "",
      message: "Model contains no features or receivers",
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
          message: "Source requires source_type (point|line|area)",
        });
      } else if (
        !isGeometryCompatible(feature.geometry.type, feature.sourceType)
      ) {
        errors.push({
          level: "error",
          code: "source.geometry.mismatch",
          featureId: id,
          message: `Geometry ${feature.geometry.type} incompatible with source_type ${feature.sourceType}`,
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
          message: "Building requires height_m",
        });
      } else if (feature.heightM <= 0) {
        errors.push({
          level: "error",
          code: "building.height.invalid",
          featureId: id,
          message: "Building height_m must be > 0",
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
          message: "Building geometry must be Polygon or MultiPolygon",
        });
      }
      break;
    }
    case "barrier": {
      if (feature.heightM == null) {
        errors.push({
          level: "error",
          code: "barrier.height.required",
          featureId: id,
          message: "Barrier requires height_m",
        });
      } else if (feature.heightM <= 0) {
        errors.push({
          level: "error",
          code: "barrier.height.invalid",
          featureId: id,
          message: "Barrier height_m must be > 0",
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
          message: "Barrier geometry must be LineString or MultiLineString",
        });
      }
      break;
    }
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
      message: `RLS-19 surface_type "${surfaceType}" is not supported`,
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
      message: `RLS-19 junction_type "${junctionType}" is not supported`,
    });
  }

  const gradient = getFeatureNumber(
    feature,
    "gradient_percent",
    "road_gradient_percent",
  );
  if (
    gradient != null &&
    (!Number.isFinite(gradient) || gradient < -12 || gradient > 12)
  ) {
    errors.push({
      level: "error",
      code: "source.rls19.gradient.invalid",
      featureId: feature.id,
      message: "RLS-19 gradient_percent must be between -12 and 12",
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
      message: "RLS-19 junction_distance_m must be >= 0",
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
      message: "RLS-19 reflection_surcharge_db must be finite",
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
      message: "RLS-19 road_speed_kph must be > 0",
    });
  }

  for (const key of RLS19_SPEED_KEYS) {
    const value = getFeatureNumber(feature, key);
    if (value != null && (!Number.isFinite(value) || value <= 0)) {
      errors.push({
        level: "error",
        code: "source.rls19.speed.invalid",
        featureId: feature.id,
        message: `RLS-19 ${key} must be > 0`,
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
        message: `RLS-19 ${key} must be >= 0`,
      });
    }
  }

  if (getRLS19ReviewRequired(feature)) {
    warnings.push({
      level: "warning",
      code: "source.rls19.review_required",
      featureId: feature.id,
      message: "Review imported source acoustics before running RLS-19",
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

  const push = (code: string, message: string): void => {
    errors.push({ level: "error", code, featureId: feature.id, message });
  };

  const numSpaces = getFeatureNumber(feature, PROP_PARKING_NUM_SPACES);
  if (numSpaces === undefined) {
    push(
      "source.rls19.parking.num_spaces.missing",
      `RLS-19 ${PROP_PARKING_NUM_SPACES} is required: the number of Stellplätze n has no default`,
    );
  } else if (numSpaces < 1 || Math.trunc(numSpaces) !== numSpaces) {
    push(
      "source.rls19.parking.num_spaces.invalid",
      `RLS-19 ${PROP_PARKING_NUM_SPACES} must be an integer >= 1`,
    );
  }

  const lotType = getFeatureString(feature, PROP_PARKING_TYPE)?.trim();
  if (!lotType) {
    push(
      "source.rls19.parking.parking_type.missing",
      `RLS-19 ${PROP_PARKING_TYPE} is required, expected one of ${RLS19_PARKING_LOT_TYPES.join(", ")}; it selects the Tabelle 6 row and has no default`,
    );
  } else if (
    !RLS19_PARKING_LOT_TYPES.includes(
      lotType.toLowerCase() as (typeof RLS19_PARKING_LOT_TYPES)[number],
    )
  ) {
    push(
      "source.rls19.parking.parking_type.invalid",
      `RLS-19 parking_type "${lotType}" is not supported`,
    );
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
    push(
      "source.rls19.parking.facility_type.invalid",
      `RLS-19 ${PROP_PARKING_FACILITY_TYPE} "${facility}" is not supported, expected one of ${RLS19_PARKING_FACILITY_TYPES.join(", ")}`,
    );
  }

  for (const key of [
    PROP_PARKING_MOVEMENTS_DAY,
    PROP_PARKING_MOVEMENTS_NIGHT,
  ]) {
    const rate = getFeatureNumber(feature, key);
    if (rate === undefined && !seeded) {
      push(
        "source.rls19.parking.movements.missing",
        `RLS-19 ${key} is required unless ${PROP_PARKING_FACILITY_TYPE} states a Tabelle 7 Parkplatztyp; state 0 explicitly for a period with no movements`,
      );
    } else if (rate !== undefined && (!Number.isFinite(rate) || rate < 0)) {
      push(
        "source.rls19.parking.movements.invalid",
        `RLS-19 ${key} must be >= 0`,
      );
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
      message: "Receiver coordinates must be finite",
    });
  }

  if (!Number.isFinite(receiver.heightM) || receiver.heightM <= 0) {
    errors.push({
      level: "error",
      code: "receiver.height.invalid",
      featureId: receiver.id,
      message: "Receiver height_m must be > 0",
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
    errors.push({
      level: "error",
      code: `${kind}.id.duplicate`,
      featureId: id,
      message: `Duplicate ${kind} ID`,
    });
  }
}
