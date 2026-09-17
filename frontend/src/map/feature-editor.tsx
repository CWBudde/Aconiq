import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";
import { AlertTriangle, Trash2, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { ConfirmDialog } from "@/ui/confirm-dialog";
import { focusMainContent } from "@/ui/main-content";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Switch } from "@/ui/components/switch";
import { useModelStore } from "@/model/model-store";
import { useModelValidation } from "@/model/use-model-validation";
import type {
  GeometryType,
  ModelFeature,
  SourceType,
  ValidationIssue,
} from "@/model/types";
import {
  getFeatureBoolean,
  getFeatureNumber,
  getFeatureString,
  getInferredFlag,
  getRLS19ReviewRequired,
  RLS19_JUNCTION_TYPES,
  RLS19_SURFACE_TYPES,
  setFeatureProperty,
} from "@/model/source-acoustics";
import { useGlobalShortcut } from "@/ui/hooks/use-global-shortcut";
import {
  PROP_PARKING_FACILITY_TYPE,
  PROP_PARKING_MOVEMENTS_DAY,
  PROP_PARKING_MOVEMENTS_NIGHT,
  PROP_PARKING_NUM_SPACES,
  PROP_PARKING_TYPE,
  RLS19_PARKING_FACILITY_TYPES,
  RLS19_PARKING_LOT_TYPES,
} from "@/model/rls19-parking";
import {
  arrayPropertyLength,
  PROP_ELEVATION_M,
  PROP_SCHALL03_BASE_HEIGHT_M,
  PROP_SCHALL03_BRIDGE_MITIGATION,
  PROP_SCHALL03_BRIDGE_TYPE,
  PROP_SCHALL03_CURVE_RADIUS_M,
  PROP_SCHALL03_FAHRBAHN,
  PROP_SCHALL03_IS_STATION,
  PROP_SCHALL03_OPERATIONS,
  PROP_SCHALL03_PARALLEL_EDGES,
  PROP_SCHALL03_PERMANENTLY_SLOW,
  PROP_SCHALL03_REFLECTING_WALL,
  PROP_SCHALL03_REFLECTIVE,
  PROP_SCHALL03_S_FAHRBAHN,
  PROP_SCHALL03_STRECKE_MAX_KPH,
  PROP_SCHALL03_SURFACE,
  PROP_SCHALL03_THICKNESS_M,
  PROP_SCHALL03_TRACK_FEATURES,
  PROP_SCHALL03_WALL_SURFACE,
  PROP_SCHALL03_WATER_BODY_FRACTION,
  SCHALL03_FAHRBAHN_TYPES,
  SCHALL03_S_FAHRBAHN_TYPES,
  SCHALL03_SURFACE_TYPES,
  SCHALL03_WALL_SURFACES,
} from "@/model/schall03";
import { MapPanel } from "./map-panel";
import { m } from "@/i18n/messages";

// Message functions are resolved against the *current* locale, so they must be
// called during render. Calling them here at module scope froze both label sets
// to whatever locale was active at import time: switching to German relabelled
// the rest of the app and left this editor in English until a full reload.
// `draw-toolbar.tsx` has the pattern these follow, and it is also why every
// field table below holds a label *function* rather than a string.
function featureKindLabel(kind: "source" | "building" | "barrier"): string {
  switch (kind) {
    case "source":
      return m.option_source();
    case "building":
      return m.option_building();
    case "barrier":
      return m.option_barrier();
  }
}

function vehicleClassLabel(
  vehicleClass: "pkw" | "lkw1" | "lkw2" | "krad",
): string {
  switch (vehicleClass) {
    case "pkw":
      return m.label_vehicle_class_pkw();
    case "lkw1":
      return m.label_vehicle_class_lkw1();
    case "lkw2":
      return m.label_vehicle_class_lkw2();
    case "krad":
      return m.label_vehicle_class_krad();
  }
}

function sourceTypeLabel(sourceType: SourceType): string {
  switch (sourceType) {
    case "point":
      return m.option_source_type_point();
    case "line":
      return m.option_source_type_line();
    case "area":
      return m.option_source_type_area();
  }
}

interface FeatureEditorProps {
  featureId: string | null;
  onClose: () => void;
}

export function FeatureEditor({ featureId, onClose }: FeatureEditorProps) {
  const feature = useModelStore((s) =>
    featureId ? s.getFeatureById(featureId) : undefined,
  );
  const receiver = useModelStore((s) =>
    featureId && !feature ? s.getReceiverById(featureId) : undefined,
  );
  const removeFeature = useModelStore((s) => s.removeFeature);

  const handleDelete = useCallback(() => {
    if (!feature) return;
    removeFeature(feature.id);
    onClose();
  }, [feature, removeFeature, onClose]);

  if (receiver) {
    return <ReceiverEditor receiverId={receiver.id} onClose={onClose} />;
  }

  if (!feature) return null;

  return (
    <EditorPanel title={featureKindLabel(feature.kind)} onClose={onClose}>
      <IdentityFields id={feature.id} geometry={feature.geometry.type} />
      <FeatureIssues featureId={feature.id} />
      <FeatureFields feature={feature} />
      <DeleteButton
        title={m.confirm_delete_feature_title()}
        description={m.confirm_delete_feature_desc({
          kind: featureKindLabel(feature.kind),
          id: feature.id,
        })}
        onDelete={handleDelete}
      />
    </EditorPanel>
  );
}

function ReceiverEditor({
  receiverId,
  onClose,
}: {
  receiverId: string;
  onClose: () => void;
}) {
  const receiver = useModelStore((s) => s.getReceiverById(receiverId));
  const updateReceiver = useModelStore((s) => s.updateReceiver);
  const removeReceiver = useModelStore((s) => s.removeReceiver);

  const [heightValue, setHeightValue] = useState(
    String(receiver?.heightM ?? "4"),
  );

  useEffect(() => {
    if (receiver) {
      setHeightValue(String(receiver.heightM));
    }
  }, [receiver, receiver?.heightM]);

  const handleHeightBlur = useCallback(() => {
    if (!receiver) return;
    const num = parseFloat(heightValue);
    if (Number.isFinite(num) && num > 0) {
      updateReceiver({ ...receiver, heightM: num });
    }
  }, [receiver, heightValue, updateReceiver]);

  const handleDelete = useCallback(() => {
    removeReceiver(receiverId);
    onClose();
  }, [receiverId, removeReceiver, onClose]);

  if (!receiver) return null;

  return (
    <EditorPanel title={m.label_receiver()} onClose={onClose}>
      <IdentityFields id={receiver.id} geometry={receiver.geometry.type} />
      <FeatureIssues featureId={receiver.id} />
      <div className="grid gap-1.5">
        <Label htmlFor="receiver-height" className="text-xs">
          {m.label_height_m()}
        </Label>
        <Input
          id="receiver-height"
          type="number"
          step="0.1"
          min="0.1"
          className="h-8 text-xs"
          value={heightValue}
          onChange={(e) => {
            setHeightValue(e.target.value);
          }}
          onBlur={handleHeightBlur}
        />
      </div>
      <DeleteButton
        title={m.confirm_delete_receiver_title()}
        description={m.confirm_delete_receiver_desc({ id: receiver.id })}
        onDelete={handleDelete}
      />
    </EditorPanel>
  );
}

/**
 * The docked editor shell: one `MapPanel` for the feature and the receiver
 * branch alike.
 *
 * **Docked, not floating over the controls.** It used to be anchored to the
 * top-right corner, which is where MapLibre's navigation control and the layer
 * control already are — opening the editor buried both. It now takes a column
 * down the left of the canvas, clear of the draw toolbar above it and the
 * validation panel below, and scrolls on its own: the source field groups are
 * long, and a panel that grows past the viewport cannot reach its own delete
 * button.
 *
 * **`role="dialog"` with a focus trap and Escape**, because it behaves as one:
 * it opens on a selection, it is the only thing that can edit that selection,
 * and it closes. The trap is a React `onKeyDown` on the panel rather than a
 * document listener, which is what keeps the delete confirmation working — the
 * `AlertDialog` portals its content outside this subtree, so the `contains`
 * guard below skips a key pressed inside it and Radix keeps its own trap and
 * its own Escape.
 *
 * It is deliberately *not* `aria-modal`: the map behind it stays pannable and
 * the toolbar reachable, and claiming modality would tell a screen reader the
 * rest of the page had gone away when it has not.
 */
function EditorPanel({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const headingId = useId();

  // Focus lands on the panel itself rather than on the first field: the panel
  // opens from a click on the map, and dropping the caret into a number input
  // would make the next keystroke an edit.
  useEffect(() => {
    panelRef.current?.focus();
  }, []);

  const handleKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLDivElement>) => {
      const panel = panelRef.current;
      if (!panel) return;
      // React bubbles synthetic events through portals, so without this a key
      // pressed in the delete confirmation would also reach the trap below.
      if (!panel.contains(event.target as Node)) return;

      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
        focusMainContent();
        return;
      }

      if (event.key !== "Tab") return;

      const focusable = focusableWithin(panel);
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) {
        event.preventDefault();
        panel.focus();
        return;
      }

      const active = document.activeElement;
      if (event.shiftKey && (active === first || active === panel)) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    },
    [onClose],
  );

  return (
    <MapPanel
      ref={panelRef}
      position="top-left"
      inset="bottom-16 left-3 top-16"
      width="w-80"
      className="flex flex-col overflow-y-auto p-4"
      role="dialog"
      aria-labelledby={headingId}
      tabIndex={-1}
      onKeyDown={handleKeyDown}
    >
      <div className="mb-3 flex items-center justify-between">
        <h3 id={headingId} className="text-sm font-semibold capitalize">
          {title}
        </h3>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          aria-label={m.tooltip_close_editor()}
        >
          &times;
        </Button>
      </div>
      <div className="space-y-3">{children}</div>
    </MapPanel>
  );
}

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(",");

/** Tab stops inside the panel, in document order. */
function focusableWithin(panel: HTMLElement): HTMLElement[] {
  return [...panel.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)].filter(
    (element) =>
      element.getAttribute("aria-hidden") !== "true" && !element.hidden,
  );
}

function IdentityFields({
  id,
  geometry,
}: {
  id: string;
  geometry: GeometryType | "Point";
}) {
  return (
    <>
      <div>
        <Label className="text-xs text-muted-foreground">{m.label_id()}</Label>
        <p className="font-mono text-xs">{id}</p>
      </div>
      <div>
        <Label className="text-xs text-muted-foreground">
          {m.label_geometry()}
        </Label>
        <p className="text-xs">{geometry}</p>
      </div>
    </>
  );
}

/**
 * What the validator says about *this* feature, on the panel that can fix it.
 *
 * The workspace's validation panel lists every finding in the model and offers
 * a "go to" that opens this editor — so the reader arrives here knowing there
 * is a problem and with nothing on the panel repeating what it was. A Parkplatz
 * missing its `rls19_parking_num_spaces` is the case that made this
 * unavoidable: the field is now here, and so is the sentence asking for it.
 *
 * The messages are `validate.ts`'s English strings; localising them is Phase E
 * (they become codes plus parameters) and is not worked around here with a
 * second set of strings that would then drift.
 */
function FeatureIssues({ featureId }: { featureId: string }) {
  const { report } = useModelValidation();
  if (report === null) return null;

  const issues: ValidationIssue[] = [...report.errors, ...report.warnings]
    .filter((issue) => issue.featureId === featureId)
    .sort((a, b) => a.code.localeCompare(b.code));
  if (issues.length === 0) return null;

  return (
    <section
      aria-label={m.label_validation()}
      className="space-y-2 rounded-md border bg-muted/30 p-2"
    >
      {issues.map((issue) => (
        <div key={issue.code} className="flex items-start gap-2">
          {issue.level === "error" ? (
            <XCircle
              aria-hidden="true"
              className="mt-0.5 size-3.5 shrink-0 text-destructive"
            />
          ) : (
            <AlertTriangle
              aria-hidden="true"
              className="mt-0.5 size-3.5 shrink-0 text-warning"
            />
          )}
          <div className="min-w-0">
            <p className="text-2xs leading-relaxed">{issue.message}</p>
            <p className="font-mono text-2xs text-muted-foreground">
              {issue.code}
            </p>
          </div>
        </div>
      ))}
    </section>
  );
}

function FeatureFields({ feature }: { feature: ModelFeature }) {
  switch (feature.kind) {
    case "source":
      return <SourceFields feature={feature} />;
    case "building":
      return (
        <>
          <HeightField feature={feature} />
          <Schall03BuildingFields feature={feature} />
        </>
      );
    case "barrier":
      return (
        <>
          <HeightField feature={feature} />
          <Schall03BarrierFields feature={feature} />
        </>
      );
  }
}

/**
 * The source type the geometry *is*, or null for a geometry no source type
 * accepts.
 *
 * `isGeometryCompatible` in `model/types.ts` is the same rule read the other
 * way round, and the backend validator enforces it: a `LineString` is a line
 * source and can be nothing else. So the type is derived here rather than
 * chosen. The select this replaces could be set to `area` on a `LineString`,
 * which produced a model the map drew happily and `aconiq run` refused with
 * `source.geometry.mismatch` — and the panel that wrote it said nothing.
 */
function geometrySourceType(geometry: GeometryType): SourceType | null {
  switch (geometry) {
    case "Point":
    case "MultiPoint":
      return "point";
    case "LineString":
    case "MultiLineString":
      return "line";
    case "Polygon":
    case "MultiPolygon":
      return "area";
  }
}

function SourceFields({ feature }: { feature: ModelFeature }) {
  const derived = geometrySourceType(feature.geometry.type);

  return (
    <div className="space-y-3">
      <DerivedSourceType feature={feature} derived={derived} />
      {derived === "line" ? (
        <>
          <RLS19RoadFields feature={feature} />
          <Schall03TrackFields feature={feature} />
        </>
      ) : null}
      {derived === "area" ? <RLS19ParkingFields feature={feature} /> : null}
    </div>
  );
}

/**
 * The derived source type, and the one repair the derivation makes possible.
 *
 * Reading it from the geometry is not enough on its own: a model imported with
 * `source_type: area` on a `LineString` still carries that property, and
 * without a way to write the derived value the panel would show the
 * contradiction and leave the reader unable to resolve it. So the mismatch
 * offers the correction explicitly rather than applying it on open — a silent
 * write would make the project dirty for a panel that was merely looked at.
 */
function DerivedSourceType({
  feature,
  derived,
}: {
  feature: ModelFeature;
  derived: SourceType | null;
}) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const declared = feature.sourceType;
  const mismatch = derived !== null && declared !== derived;

  const applyDerived = useCallback(() => {
    if (derived === null) return;
    updateFeature({ ...feature, sourceType: derived });
  }, [derived, feature, updateFeature]);

  // The value names itself after the label: a read-only value with a loose
  // `<Label>` beside it is announced as bare text, and "Point" on this panel is
  // also the geometry type one row above.
  const labelId = `${feature.id}-source-type-label`;

  return (
    <div className="grid gap-1">
      <Label id={labelId} className="text-xs">
        {m.label_source_type()}
      </Label>
      <p
        id={`${feature.id}-source-type`}
        aria-labelledby={labelId}
        className="text-xs"
      >
        {derived === null ? feature.geometry.type : sourceTypeLabel(derived)}
      </p>
      <p className="text-2xs text-muted-foreground">
        {m.msg_source_type_derived()}
      </p>
      {mismatch ? (
        <Button
          variant="outline"
          size="sm"
          className="mt-1 h-7 text-2xs"
          onClick={applyDerived}
        >
          {m.action_apply_derived_source_type()}
        </Button>
      ) : null}
    </div>
  );
}

// --- Field tables -----------------------------------------------------------
//
// The fields are data, not markup. Sixteen hand-written `PropertyNumberField`
// blocks is how `speed_krad_kph` ends up with the `min` of a traffic count and
// nobody sees it in the diff; a row in a table is read against its neighbours.
// Every label is a function so it resolves in the locale that is active when
// the panel renders.

interface NumberFieldSpec {
  propertyKey: string;
  /** Older spellings the same value arrives under; replaced on write. */
  aliases?: string[];
  label: () => string;
  min?: number;
  max?: number;
  step?: number;
  /**
   * What the field says below itself when the value is neither set nor
   * inferred. The default is "the run's default applies"; a property that has
   * no default states its own rule here.
   */
  helper?: () => string;
  /** False where an empty field is not "use the run default". */
  defaultable?: boolean;
}

interface SelectFieldSpec {
  propertyKey: string;
  aliases?: string[];
  label: () => string;
  options: readonly string[];
  /** The option that clears the property. */
  emptyLabel?: () => string;
  helper?: () => string;
}

interface BooleanFieldSpec {
  propertyKey: string;
  label: () => string;
  helper?: () => string;
}

const VEHICLE_CLASSES = ["pkw", "lkw1", "lkw2", "krad"] as const;

const SPEED_FIELDS: NumberFieldSpec[] = VEHICLE_CLASSES.map((vehicleClass) => ({
  propertyKey: `speed_${vehicleClass}_kph`,
  label: () => vehicleClassLabel(vehicleClass),
  min: 0.1,
  step: 1,
}));

const TRAFFIC_DAY_FIELDS: NumberFieldSpec[] = VEHICLE_CLASSES.map(
  (vehicleClass) => ({
    propertyKey: `traffic_day_${vehicleClass}`,
    label: () => vehicleClassLabel(vehicleClass),
    min: 0,
    step: 1,
  }),
);

const TRAFFIC_NIGHT_FIELDS: NumberFieldSpec[] = VEHICLE_CLASSES.map(
  (vehicleClass) => ({
    propertyKey: `traffic_night_${vehicleClass}`,
    label: () => vehicleClassLabel(vehicleClass),
    min: 0,
    step: 1,
  }),
);

const ROAD_UNIFORM_SPEED_FIELD: NumberFieldSpec = {
  propertyKey: "road_speed_kph",
  label: m.label_uniform_speed_kph,
  min: 0.1,
  step: 1,
};

// ±12 % is the range Tabelle 3 tabulates. Outside it RLS-19 has no correction
// to apply, so a larger value is a unit error rather than a steeper road.
const ROAD_GRADIENT_FIELD: NumberFieldSpec = {
  propertyKey: "gradient_percent",
  aliases: ["road_gradient_percent"],
  label: m.label_gradient_percent,
  min: -12,
  max: 12,
  step: 0.1,
};

const ROAD_JUNCTION_FIELDS: NumberFieldSpec[] = [
  {
    propertyKey: "junction_distance_m",
    aliases: ["road_junction_distance_m"],
    label: m.label_junction_distance_m,
    min: 0,
    step: 1,
  },
  {
    // A dB correction, which may go either way — hence no `min`, unlike every
    // count and speed above.
    propertyKey: "reflection_surcharge_db",
    label: m.label_reflection_surcharge_db,
    step: 0.1,
  },
];

const ROAD_SURFACE_FIELD: SelectFieldSpec = {
  propertyKey: "surface_type",
  aliases: ["road_surface_type"],
  label: m.label_surface_type,
  options: RLS19_SURFACE_TYPES,
};

const ROAD_JUNCTION_TYPE_FIELD: SelectFieldSpec = {
  propertyKey: "junction_type",
  aliases: ["road_junction_type"],
  label: m.label_junction_type,
  options: RLS19_JUNCTION_TYPES,
};

function RLS19RoadFields({ feature }: { feature: ModelFeature }) {
  return (
    <FieldSection
      title={m.label_section_source_acoustics()}
      note={m.msg_source_acoustics_defaults()}
      warning={
        getRLS19ReviewRequired(feature)
          ? m.msg_source_acoustics_review_required()
          : undefined
      }
    >
      <PropertySelectField feature={feature} spec={ROAD_SURFACE_FIELD} />
      <PropertyNumberField feature={feature} spec={ROAD_UNIFORM_SPEED_FIELD} />
      <NumberFieldGrid feature={feature} fields={SPEED_FIELDS} columns={2} />
      <PropertyNumberField feature={feature} spec={ROAD_GRADIENT_FIELD} />
      <PropertySelectField feature={feature} spec={ROAD_JUNCTION_TYPE_FIELD} />
      <NumberFieldGrid
        feature={feature}
        fields={ROAD_JUNCTION_FIELDS}
        columns={2}
      />
      <FieldGroup label={m.label_traffic_day()}>
        <NumberFieldGrid
          feature={feature}
          fields={TRAFFIC_DAY_FIELDS}
          columns={2}
        />
      </FieldGroup>
      <FieldGroup label={m.label_traffic_night()}>
        <NumberFieldGrid
          feature={feature}
          fields={TRAFFIC_NIGHT_FIELDS}
          columns={2}
        />
      </FieldGroup>
    </FieldSection>
  );
}

// --- RLS-19 Parkplatz (Nr. 3.4) ---------------------------------------------
//
// An area source under rls19-road is a Parkplatz. Every property here is one
// `validate.ts` already refuses a half-filled lot over — which until now no
// control on this panel could set, so the finding named a field that did not
// exist. There is deliberately no `rls19_parking_area_m2`: Eq. 10's
// −10·lg[P/1 m²] cancels when the lot is propagated as a total-power point
// source, so it would be a required value that provably changes no output. The
// Stellplatzfläche and the centroid both come from the polygon.

const PARKING_NUMBER_FIELDS: NumberFieldSpec[] = [
  {
    propertyKey: PROP_PARKING_NUM_SPACES,
    label: m.label_parking_num_spaces,
    min: 1,
    step: 1,
    defaultable: false,
    helper: m.msg_field_required_no_default,
  },
];

const PARKING_MOVEMENT_FIELDS: NumberFieldSpec[] = [
  {
    propertyKey: PROP_PARKING_MOVEMENTS_DAY,
    label: m.label_parking_movements_day,
    min: 0,
    step: 0.01,
    defaultable: false,
    helper: m.msg_parking_movements_required,
  },
  {
    propertyKey: PROP_PARKING_MOVEMENTS_NIGHT,
    label: m.label_parking_movements_night,
    min: 0,
    step: 0.01,
    defaultable: false,
    helper: m.msg_parking_movements_required,
  },
];

const PARKING_ELEVATION_FIELD: NumberFieldSpec = {
  propertyKey: PROP_ELEVATION_M,
  label: m.label_elevation_m,
  step: 0.1,
  defaultable: false,
  helper: m.msg_field_default_zero,
};

const PARKING_TYPE_FIELD: SelectFieldSpec = {
  propertyKey: PROP_PARKING_TYPE,
  label: m.label_parking_type,
  options: RLS19_PARKING_LOT_TYPES,
  emptyLabel: m.option_not_set,
  helper: m.msg_field_required_no_default,
};

const PARKING_FACILITY_FIELD: SelectFieldSpec = {
  propertyKey: PROP_PARKING_FACILITY_TYPE,
  label: m.label_parking_facility_type,
  options: RLS19_PARKING_FACILITY_TYPES,
  emptyLabel: m.option_not_set,
  helper: m.msg_parking_facility_seeds_rates,
};

function RLS19ParkingFields({ feature }: { feature: ModelFeature }) {
  return (
    <FieldSection
      title={m.label_section_parking()}
      note={m.msg_parking_section_note()}
    >
      <NumberFieldGrid feature={feature} fields={PARKING_NUMBER_FIELDS} />
      <PropertySelectField feature={feature} spec={PARKING_TYPE_FIELD} />
      <PropertySelectField feature={feature} spec={PARKING_FACILITY_FIELD} />
      <NumberFieldGrid feature={feature} fields={PARKING_MOVEMENT_FIELDS} />
      <PropertyNumberField feature={feature} spec={PARKING_ELEVATION_FIELD} />
    </FieldSection>
  );
}

// --- Schall 03 (Anlage 2) ---------------------------------------------------

const RAIL_TRACK_NUMBER_FIELDS: NumberFieldSpec[] = [
  {
    propertyKey: PROP_SCHALL03_STRECKE_MAX_KPH,
    label: m.label_rail_strecke_max_kph,
    min: 0.1,
    step: 1,
    defaultable: false,
    helper: m.msg_field_required_no_default,
  },
  {
    propertyKey: PROP_SCHALL03_BRIDGE_TYPE,
    label: m.label_rail_bridge_type,
    min: 0,
    max: 4,
    step: 1,
    defaultable: false,
    helper: m.msg_field_default_zero,
  },
  {
    propertyKey: PROP_SCHALL03_CURVE_RADIUS_M,
    label: m.label_rail_curve_radius_m,
    min: 0,
    step: 1,
    defaultable: false,
    helper: m.msg_rail_curve_radius_straight,
  },
  {
    propertyKey: PROP_SCHALL03_WATER_BODY_FRACTION,
    label: m.label_rail_water_body_fraction,
    min: 0,
    max: 1,
    step: 0.01,
    defaultable: false,
    helper: m.msg_field_default_zero,
  },
  {
    propertyKey: PROP_ELEVATION_M,
    label: m.label_elevation_m,
    step: 0.1,
    defaultable: false,
    helper: m.msg_field_default_zero,
  },
];

const RAIL_TRACK_SELECT_FIELDS: SelectFieldSpec[] = [
  {
    propertyKey: PROP_SCHALL03_FAHRBAHN,
    label: m.label_rail_fahrbahn,
    options: SCHALL03_FAHRBAHN_TYPES,
    emptyLabel: m.option_not_set,
    helper: m.msg_rail_reference_row,
  },
  {
    propertyKey: PROP_SCHALL03_S_FAHRBAHN,
    label: m.label_rail_s_fahrbahn,
    options: SCHALL03_S_FAHRBAHN_TYPES,
    emptyLabel: m.option_not_set,
    helper: m.msg_rail_reference_row,
  },
  {
    propertyKey: PROP_SCHALL03_SURFACE,
    label: m.label_rail_surface,
    options: SCHALL03_SURFACE_TYPES,
    emptyLabel: m.option_not_set,
    helper: m.msg_rail_reference_row,
  },
];

const RAIL_TRACK_BOOLEAN_FIELDS: BooleanFieldSpec[] = [
  {
    propertyKey: PROP_SCHALL03_BRIDGE_MITIGATION,
    label: m.label_rail_bridge_mitigation,
  },
  { propertyKey: PROP_SCHALL03_IS_STATION, label: m.label_rail_is_station },
  {
    propertyKey: PROP_SCHALL03_PERMANENTLY_SLOW,
    label: m.label_rail_permanently_slow,
  },
];

const RAIL_BARRIER_NUMBER_FIELDS: NumberFieldSpec[] = [
  {
    propertyKey: PROP_SCHALL03_BASE_HEIGHT_M,
    label: m.label_rail_base_height_m,
    min: 0,
    step: 0.1,
    defaultable: false,
    helper: m.msg_field_default_zero,
  },
  {
    propertyKey: PROP_SCHALL03_THICKNESS_M,
    label: m.label_rail_thickness_m,
    min: 0,
    step: 0.1,
    defaultable: false,
    helper: m.msg_field_default_zero,
  },
];

const RAIL_BARRIER_BOOLEAN_FIELDS: BooleanFieldSpec[] = [
  { propertyKey: PROP_SCHALL03_REFLECTIVE, label: m.label_rail_reflective },
  {
    propertyKey: PROP_SCHALL03_PARALLEL_EDGES,
    label: m.label_rail_parallel_edges,
  },
];

const RAIL_WALL_SURFACE_FIELD: SelectFieldSpec = {
  propertyKey: PROP_SCHALL03_WALL_SURFACE,
  label: m.label_rail_wall_surface,
  options: SCHALL03_WALL_SURFACES,
  emptyLabel: m.option_not_set,
  helper: m.msg_rail_reference_row,
};

const RAIL_BUILDING_BOOLEAN_FIELDS: BooleanFieldSpec[] = [
  {
    propertyKey: PROP_SCHALL03_REFLECTING_WALL,
    label: m.label_rail_reflecting_wall,
    helper: m.msg_rail_reflecting_wall_opt_in,
  },
];

function Schall03TrackFields({ feature }: { feature: ModelFeature }) {
  const operations = arrayPropertyLength(
    feature.properties,
    PROP_SCHALL03_OPERATIONS,
  );
  const trackFeatures = arrayPropertyLength(
    feature.properties,
    PROP_SCHALL03_TRACK_FEATURES,
  );

  return (
    <FieldSection
      title={m.label_section_rail()}
      note={m.msg_rail_section_note()}
    >
      <div className="grid gap-1">
        <Label className="text-2xs">{m.label_rail_operations()}</Label>
        <p className="text-xs tabular-nums">{String(operations ?? 0)}</p>
        <p className="text-2xs text-muted-foreground">
          {m.msg_rail_arrays_read_only()}
        </p>
      </div>
      {trackFeatures !== null ? (
        <div className="grid gap-1">
          <Label className="text-2xs">{m.label_rail_track_features()}</Label>
          <p className="text-xs tabular-nums">{String(trackFeatures)}</p>
        </div>
      ) : null}
      <NumberFieldGrid feature={feature} fields={RAIL_TRACK_NUMBER_FIELDS} />
      {RAIL_TRACK_SELECT_FIELDS.map((spec) => (
        <PropertySelectField
          key={spec.propertyKey}
          feature={feature}
          spec={spec}
        />
      ))}
      <BooleanFieldList feature={feature} fields={RAIL_TRACK_BOOLEAN_FIELDS} />
    </FieldSection>
  );
}

function Schall03BarrierFields({ feature }: { feature: ModelFeature }) {
  return (
    <FieldSection
      title={m.label_section_rail()}
      note={m.msg_rail_barrier_note()}
    >
      <BooleanFieldList
        feature={feature}
        fields={RAIL_BARRIER_BOOLEAN_FIELDS}
      />
      <NumberFieldGrid feature={feature} fields={RAIL_BARRIER_NUMBER_FIELDS} />
      <PropertySelectField feature={feature} spec={RAIL_WALL_SURFACE_FIELD} />
    </FieldSection>
  );
}

function Schall03BuildingFields({ feature }: { feature: ModelFeature }) {
  return (
    <FieldSection
      title={m.label_section_rail()}
      note={m.msg_rail_building_note()}
    >
      <BooleanFieldList
        feature={feature}
        fields={RAIL_BUILDING_BOOLEAN_FIELDS}
      />
      <PropertySelectField feature={feature} spec={RAIL_WALL_SURFACE_FIELD} />
    </FieldSection>
  );
}

// --- Field renderers --------------------------------------------------------

function FieldSection({
  title,
  note,
  warning,
  children,
}: {
  title: string;
  note: string;
  warning?: string | undefined;
  children: React.ReactNode;
}) {
  return (
    <section
      aria-label={title}
      className="space-y-3 rounded-md border bg-muted/30 p-3"
    >
      <div className="space-y-1">
        <p className="text-xs font-medium">{title}</p>
        <p className="text-2xs leading-relaxed text-muted-foreground">{note}</p>
        {warning !== undefined ? (
          <p className="text-2xs leading-relaxed text-warning">{warning}</p>
        ) : null}
      </div>
      {children}
    </section>
  );
}

function FieldGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <p className="text-2xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </p>
      {children}
    </div>
  );
}

function NumberFieldGrid({
  feature,
  fields,
  columns = 1,
}: {
  feature: ModelFeature;
  fields: NumberFieldSpec[];
  columns?: 1 | 2;
}) {
  return (
    <div className={columns === 2 ? "grid grid-cols-2 gap-2" : "grid gap-2"}>
      {fields.map((spec) => (
        <PropertyNumberField
          key={spec.propertyKey}
          feature={feature}
          spec={spec}
        />
      ))}
    </div>
  );
}

function BooleanFieldList({
  feature,
  fields,
}: {
  feature: ModelFeature;
  fields: BooleanFieldSpec[];
}) {
  return (
    <div className="grid gap-2">
      {fields.map((spec) => (
        <PropertyBooleanField
          key={spec.propertyKey}
          feature={feature}
          spec={spec}
        />
      ))}
    </div>
  );
}

/**
 * The helper line under a field: what the value's absence means here.
 *
 * "The run's default applies" is the answer for the RLS-19 road properties and
 * the wrong one everywhere else — an omitted `rls19_parking_num_spaces` is a
 * refusal, an omitted `schall03_fahrbahn` is Schwellengleis. So a spec that has
 * no run default carries its own sentence, and an inferred value overrides both
 * because provenance is the more important thing to say.
 */
function fieldHelper(
  feature: ModelFeature,
  spec: { propertyKey: string; helper?: () => string },
): string {
  if (getInferredFlag(feature, spec.propertyKey)) {
    return m.msg_source_acoustics_inferred();
  }
  return spec.helper?.() ?? m.msg_source_acoustics_default_fallback();
}

function PropertyNumberField({
  feature,
  spec,
}: {
  feature: ModelFeature;
  spec: NumberFieldSpec;
}) {
  const { propertyKey, aliases = [], min, max, step } = spec;
  const updateFeature = useModelStore((s) => s.updateFeature);
  const current = getFeatureNumber(feature, propertyKey, ...aliases);
  const [value, setValue] = useState(current == null ? "" : String(current));

  useEffect(() => {
    setValue(current == null ? "" : String(current));
  }, [current]);

  const handleBlur = useCallback(() => {
    const trimmed = value.trim();
    if (trimmed === "") {
      updateFeature(
        setFeatureProperty(feature, propertyKey, undefined, ...aliases),
      );
      return;
    }

    const numeric = Number.parseFloat(trimmed);
    if (!Number.isFinite(numeric)) {
      return;
    }

    updateFeature(
      setFeatureProperty(feature, propertyKey, numeric, ...aliases),
    );
  }, [aliases, feature, propertyKey, updateFeature, value]);

  return (
    <div className="grid gap-1">
      <Label htmlFor={`${feature.id}-${propertyKey}`} className="text-2xs">
        {spec.label()}
      </Label>
      <Input
        id={`${feature.id}-${propertyKey}`}
        type="number"
        min={min != null ? String(min) : undefined}
        max={max != null ? String(max) : undefined}
        step={step != null ? String(step) : undefined}
        className="h-8 text-xs"
        placeholder={
          spec.defaultable === false
            ? undefined
            : m.placeholder_use_run_default()
        }
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
        }}
        onBlur={handleBlur}
      />
      <p className="text-2xs text-muted-foreground">
        {fieldHelper(feature, spec)}
      </p>
    </div>
  );
}

/** The sentinel the empty option carries; never written to the model. */
const UNSET_OPTION = "__default__";

function PropertySelectField({
  feature,
  spec,
}: {
  feature: ModelFeature;
  spec: SelectFieldSpec;
}) {
  const { propertyKey, aliases = [], options } = spec;
  const updateFeature = useModelStore((s) => s.updateFeature);
  const current = getFeatureString(feature, propertyKey, ...aliases);
  // The trigger takes the id the label points at. Without the pairing the only
  // accessible name a Radix trigger has is its own current value, so three
  // selects in a row read as "SMA", "none", "none" and a test can address them
  // by position alone.
  const fieldId = `${feature.id}-${propertyKey}`;

  const handleChange = useCallback(
    (value: string) => {
      updateFeature(
        setFeatureProperty(
          feature,
          propertyKey,
          value === UNSET_OPTION ? undefined : value,
          ...aliases,
        ),
      );
    },
    [aliases, feature, propertyKey, updateFeature],
  );

  return (
    <div className="grid gap-1">
      <Label htmlFor={fieldId} className="text-2xs">
        {spec.label()}
      </Label>
      <Select value={current ?? UNSET_OPTION} onValueChange={handleChange}>
        <SelectTrigger id={fieldId} className="h-8 text-xs">
          <SelectValue placeholder={m.placeholder_use_run_default()} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={UNSET_OPTION}>
            {spec.emptyLabel?.() ?? m.option_use_run_default()}
          </SelectItem>
          {options.map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-2xs text-muted-foreground">
        {fieldHelper(feature, spec)}
      </p>
    </div>
  );
}

/**
 * A boolean property, where "off" removes the key rather than writing `false`.
 *
 * Every boolean the schema defines defaults to false, so `false` and absent
 * mean the same thing to every reader — and absent is the spelling that keeps
 * a drawn feature's properties to what was actually chosen instead of a row of
 * `false`s in every model diff.
 */
function PropertyBooleanField({
  feature,
  spec,
}: {
  feature: ModelFeature;
  spec: BooleanFieldSpec;
}) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const checked = getFeatureBoolean(feature, spec.propertyKey) === true;
  const fieldId = `${feature.id}-${spec.propertyKey}`;

  const handleChange = useCallback(
    (next: boolean) => {
      updateFeature(
        setFeatureProperty(feature, spec.propertyKey, next ? true : undefined),
      );
    },
    [feature, spec.propertyKey, updateFeature],
  );

  return (
    <div className="grid gap-1">
      <div className="flex items-center justify-between gap-2">
        <Label htmlFor={fieldId} className="text-2xs">
          {spec.label()}
        </Label>
        <Switch
          id={fieldId}
          checked={checked}
          onCheckedChange={handleChange}
          className="h-5 w-9"
        />
      </div>
      <p className="text-2xs text-muted-foreground">
        {spec.helper?.() ?? m.msg_field_default_off()}
      </p>
    </div>
  );
}

function HeightField({ feature }: { feature: ModelFeature }) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const [value, setValue] = useState(String(feature.heightM ?? ""));

  useEffect(() => {
    setValue(String(feature.heightM ?? ""));
  }, [feature.heightM]);

  const handleBlur = useCallback(() => {
    const num = parseFloat(value);
    if (Number.isFinite(num) && num > 0) {
      updateFeature({ ...feature, heightM: num });
    }
  }, [feature, value, updateFeature]);

  return (
    <div className="grid gap-1.5">
      <Label htmlFor="height" className="text-xs">
        {m.label_height_m()}
      </Label>
      <Input
        id="height"
        type="number"
        step="0.1"
        min="0.1"
        className="h-8 text-xs"
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
        }}
        onBlur={handleBlur}
      />
    </div>
  );
}

/**
 * The one delete control in the editor, for a feature or a receiver.
 *
 * It asks first even though `removeFeature` and `removeReceiver` both go
 * through the command stack and are undoable. Nothing on this panel says so:
 * the undo bar is at the other end of the workspace, and its own tooltips do
 * not open in the state they describe. So the confirmation carries that
 * sentence, which is the cheapest way to make it true.
 *
 * Del reaches the same confirmation rather than deleting outright. This button
 * only exists while the editor has a feature or a receiver open, so the binding
 * is armed exactly when there is something for "the edited feature" to mean —
 * and routing the key through the dialog keeps one delete path instead of a
 * quiet second one that skips the sentence the button's own path insists on.
 * `useGlobalShortcut` bows out of text controls, so Del in the height field
 * still edits the number.
 */
function DeleteButton({
  title,
  description,
  onDelete,
}: {
  title: string;
  description: string;
  onDelete: () => void;
}) {
  const [confirming, setConfirming] = useState(false);

  useGlobalShortcut({ key: "Delete", enabled: !confirming }, () => {
    setConfirming(true);
  });
  // Read while the dialog closes, which is before the next render, so a ref
  // rather than state.
  const deleted = useRef(false);

  return (
    <>
      <Button
        variant="destructive"
        size="sm"
        className="mt-2 w-full"
        onClick={() => {
          setConfirming(true);
        }}
      >
        <Trash2 className="mr-1.5 h-3.5 w-3.5" />
        {m.action_delete_feature()}
      </Button>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        tone="destructive"
        title={title}
        description={description}
        confirmLabel={m.action_delete_feature()}
        onConfirm={() => {
          deleted.current = true;
          onDelete();
        }}
        // Both call sites close the panel on delete, taking this button with
        // it, so there is nothing for Radix to restore focus to. Cancelling
        // leaves the panel standing and keeps the ordinary restoration.
        onCloseAutoFocus={(event) => {
          if (!deleted.current) return;
          event.preventDefault();
          focusMainContent();
        }}
      />
    </>
  );
}
