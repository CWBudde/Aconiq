import type { ModelFeature, ValidationIssue, ValidationReport } from "./types";
import { isGeometryCompatible } from "./types";

export function validateModel(features: ModelFeature[]): ValidationReport {
  const errors: ValidationIssue[] = [];
  const warnings: ValidationIssue[] = [];

  if (features.length === 0) {
    errors.push({
      level: "error",
      code: "model.empty",
      featureId: "",
      message: "Model contains no features",
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
    if (ids.has(feature.id)) {
      errors.push({
        level: "error",
        code: "feature.id.duplicate",
        featureId: feature.id,
        message: "Duplicate feature ID",
      });
    }
    ids.add(feature.id);

    validateFeature(feature, errors);
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
