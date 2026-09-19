import { m } from "@/i18n/messages";
import type { ValidationIssue } from "./types";

/**
 * The one place a validation finding becomes a sentence.
 *
 * `validate.ts` produces a code and its parameters and no prose at all, so
 * everything that shows a finding — the workspace panel, the docked feature
 * editor, both halves of the import wizard — reads its text from here and the
 * German user sees German. Before this, four call sites rendered
 * `issue.message`, which was English built by template literal inside the
 * validator, under a heading the catalogue had already translated.
 *
 * A `switch` rather than a lookup table keyed by code. TypeScript narrows
 * `issue.params` to exactly the shape the branch's code declares in
 * `ValidationIssueParams`, which an index into a `Record` cannot do without a
 * cast; and the `never` default makes an added code a compile error here
 * rather than a blank row on screen. The cost is thirty one-line branches,
 * which is the shape of the problem.
 *
 * Parameter values are passed through unformatted on purpose. They are
 * property keys (`traffic_day_lkw1`), enum members (`asphalt`) and the joined
 * vocabularies a reader has to type back — Phase C's rule that the raw name
 * stays on screen because it is what `--param` takes. `parts` is the one
 * number, and it is a ring count in a refusal rather than a measurement, so it
 * carries no unit and wants no locale grouping.
 */
export function validationIssueText(issue: ValidationIssue): string {
  switch (issue.code) {
    case "model.empty":
      return m.msg_validation_model_empty();
    case "feature.id.duplicate":
      return m.msg_validation_feature_id_duplicate();
    case "receiver.id.duplicate":
      return m.msg_validation_receiver_id_duplicate();

    case "source.type.required":
      return m.msg_validation_source_type_required();
    case "source.geometry.mismatch":
      return m.msg_validation_source_geometry_mismatch(issue.params);

    case "building.height.required":
      return m.msg_validation_building_height_required();
    case "building.height.invalid":
      return m.msg_validation_building_height_invalid();
    case "building.geometry.invalid":
      return m.msg_validation_building_geometry_invalid();

    case "barrier.height.required":
      return m.msg_validation_barrier_height_required();
    case "barrier.height.invalid":
      return m.msg_validation_barrier_height_invalid();
    case "barrier.geometry.invalid":
      return m.msg_validation_barrier_geometry_invalid();

    case "groundzone.factor.required":
      return m.msg_validation_groundzone_factor_required();
    case "groundzone.factor.invalid":
      return m.msg_validation_groundzone_factor_invalid();
    case "groundzone.geometry.invalid":
      return m.msg_validation_groundzone_geometry_invalid();

    case "receiver.coordinates.invalid":
      return m.msg_validation_receiver_coordinates_invalid();
    case "receiver.height.invalid":
      return m.msg_validation_receiver_height_invalid();

    case "source.rls19.surface_type.invalid":
      return m.msg_validation_source_rls19_surface_type_invalid(issue.params);
    case "source.rls19.junction_type.invalid":
      return m.msg_validation_source_rls19_junction_type_invalid(issue.params);
    case "source.rls19.gradient.invalid":
      return m.msg_validation_source_rls19_gradient_invalid(issue.params);
    case "source.rls19.junction_distance.invalid":
      return m.msg_validation_source_rls19_junction_distance_invalid();
    case "source.rls19.reflection_surcharge.invalid":
      return m.msg_validation_source_rls19_reflection_surcharge_invalid();
    case "source.rls19.road_speed.invalid":
      return m.msg_validation_source_rls19_road_speed_invalid();
    case "source.rls19.speed.invalid":
      return m.msg_validation_source_rls19_speed_invalid(issue.params);
    case "source.rls19.traffic.invalid":
      return m.msg_validation_source_rls19_traffic_invalid(issue.params);
    case "source.rls19.review_required":
      return m.msg_validation_source_rls19_review_required();

    case "source.rls19.parking.geometry.multipart":
      return m.msg_validation_source_rls19_parking_geometry_multipart(
        issue.params,
      );
    case "source.rls19.parking.num_spaces.missing":
      return m.msg_validation_source_rls19_parking_num_spaces_missing(
        issue.params,
      );
    case "source.rls19.parking.num_spaces.invalid":
      return m.msg_validation_source_rls19_parking_num_spaces_invalid(
        issue.params,
      );
    case "source.rls19.parking.parking_type.missing":
      return m.msg_validation_source_rls19_parking_parking_type_missing(
        issue.params,
      );
    case "source.rls19.parking.parking_type.invalid":
      return m.msg_validation_source_rls19_parking_parking_type_invalid(
        issue.params,
      );
    case "source.rls19.parking.facility_type.invalid":
      return m.msg_validation_source_rls19_parking_facility_type_invalid(
        issue.params,
      );
    case "source.rls19.parking.movements.missing":
      return m.msg_validation_source_rls19_parking_movements_missing(
        issue.params,
      );
    case "source.rls19.parking.movements.invalid":
      return m.msg_validation_source_rls19_parking_movements_invalid(
        issue.params,
      );
  }
}
