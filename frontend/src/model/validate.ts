import type {
  ModelFeature,
  ModelReceiver,
  ValidationIssue,
  ValidationReport,
} from "./types";
import { isGeometryCompatible } from "./types";
import {
  getFeatureNumber,
  getFeatureString,
  getRLS19ReviewRequired,
  RLS19_JUNCTION_TYPES,
  RLS19_SPEED_KEYS,
  RLS19_SURFACE_TYPES,
  RLS19_TRAFFIC_KEYS,
} from "./source-acoustics";

export function validateModel(features: ModelFeature[]): ValidationReport {
  return validateProjectModel(features, []);
}

export function validateProjectModel(
  features: ModelFeature[],
  receivers: ModelReceiver[],
): ValidationReport {
  const errors: ValidationIssue[] = [];
  const warnings: ValidationIssue[] = [];

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
