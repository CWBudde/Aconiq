import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useState,
  type MouseEvent as ReactMouseEvent,
} from "react";
import { Plus, Trash2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/ui/components/dialog";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { FormField } from "@/ui/form-field";
import { formatCoordinate } from "@/ui/format";
import { cn } from "@/ui/lib/utils";
import type {
  FeatureKind,
  Geometry,
  Position,
  SourceType,
} from "@/model/types";
import {
  createFeatureId,
  createReceiverId,
  DEFAULT_RECEIVER_HEIGHT_M,
} from "@/model/types";
import type { ModelReceiver } from "@/model/types";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";

interface NewFeatureDialogProps {
  open: boolean;
  /**
   * The shape terra-draw finished, already moved into the store's CRS by
   * `use-draw-projection.ts` — or `null`, which is the keyboard path: the
   * dialog then asks for the coordinates instead of being handed them.
   */
  geometry: Geometry | null;
  onClose: () => void;
}

/** One row of the vertex list, held as typed text until it is parsed. */
interface VertexInput {
  x: string;
  y: string;
}

/** How many vertices the typed geometry needs before it can be built. */
const MINIMUM_VERTICES: Record<TypedGeometryType, number> = {
  Point: 1,
  LineString: 2,
  Polygon: 3,
};

/** The geometry types coordinate entry can produce. */
type TypedGeometryType = "Point" | "LineString" | "Polygon";

function inferKind(geomType: string): FeatureKind {
  if (geomType === "Point" || geomType === "MultiPoint") return "source";
  if (geomType === "LineString" || geomType === "MultiLineString")
    return "barrier";
  return "building";
}

function inferSourceType(geomType: string): SourceType {
  if (geomType === "Point" || geomType === "MultiPoint") return "point";
  if (geomType === "LineString" || geomType === "MultiLineString")
    return "line";
  return "area";
}

/**
 * The geometry the typed coordinates will become, derived from the kind the
 * user picked rather than asked for separately: the schema already fixes it
 * (`docs/geojson-schema-v1.md`) — a building is a polygon, a barrier a line, a
 * receiver a point — and a source's is its `sourceType`. Offering the pair
 * independently is how a `source_type: "line"` on a `Point` gets written.
 */
function typedGeometryType(
  kind: FeatureKind | "receiver",
  sourceType: SourceType,
): TypedGeometryType {
  switch (kind) {
    case "receiver":
      return "Point";
    case "building":
      return "Polygon";
    case "barrier":
      return "LineString";
    case "source":
      if (sourceType === "point") return "Point";
      if (sourceType === "line") return "LineString";
      return "Polygon";
  }
}

function blankVertex(): VertexInput {
  return { x: "", y: "" };
}

function parseVertices(vertices: VertexInput[]): Position[] | null {
  const positions: Position[] = [];
  for (const vertex of vertices) {
    if (vertex.x.trim() === "" || vertex.y.trim() === "") return null;
    const x = Number(vertex.x);
    const y = Number(vertex.y);
    if (!Number.isFinite(x) || !Number.isFinite(y)) return null;
    positions.push([x, y]);
  }
  return positions;
}

/**
 * The geometry the typed rows describe, or `null` while they do not describe
 * one yet.
 *
 * **The coordinates are taken as they were typed. No transform happens here,
 * and none may be added.** The fields are labelled with `useModelStore`'s own
 * `crs` and the numbers go into the store in that CRS, so this path writes no
 * coordinate the model was not already measured in — it is not a second
 * coordinate writer beside `use-draw-projection.ts`, which stays the single
 * deliberate exception to "the map is a projection *of* the model, never a
 * source *for* it" (PLAN.md, Phase D). The drawn path needs that exception
 * because terra-draw emits WGS 84 whatever the model holds; typed numbers are
 * already in the model's frame, so there is nothing to invert.
 *
 * `CoordinateDisplay` is what makes this usable rather than blind: it reads the
 * pointer out in WGS 84 *and* in the store's CRS, so a target is read off the
 * map in the same numbers this dialog asks for.
 */
function typedGeometry(
  type: TypedGeometryType,
  vertices: VertexInput[],
): Geometry | null {
  const positions = parseVertices(vertices);
  if (positions === null || positions.length < MINIMUM_VERTICES[type]) {
    return null;
  }
  const first = positions[0];
  if (first === undefined) return null;

  if (type === "Point") return { type: "Point", coordinates: first };
  if (type === "LineString")
    return { type: "LineString", coordinates: positions };

  // A GeoJSON ring is closed. Appending the first position rather than asking
  // for it again: a typed ring whose last row repeats the first is what a user
  // copying coordinates out of a table produces, and closing it twice would
  // write a zero-length segment the validator then reports.
  const last = positions[positions.length - 1] ?? first;
  const closed =
    last[0] === first[0] && last[1] === first[1]
      ? positions
      : [...positions, first];
  return { type: "Polygon", coordinates: [closed] };
}

/** Every position in a geometry, whatever its nesting depth. */
function flattenPositions(geometry: Geometry): Position[] {
  const out: Position[] = [];
  const walk = (node: unknown): void => {
    if (!Array.isArray(node)) return;
    const nodes = node as unknown[];
    const x = nodes[0];
    const y = nodes[1];
    if (typeof x === "number" && typeof y === "number") {
      out.push([x, y]);
      return;
    }
    for (const child of nodes) walk(child);
  };
  walk(geometry.coordinates);
  return out;
}

/** How many positions the read-back prints before it summarises the rest. */
const READBACK_LIMIT = 4;

export function NewFeatureDialog({
  open,
  geometry,
  onClose,
}: NewFeatureDialogProps) {
  const addFeature = useModelStore((s) => s.addFeature);
  const addReceiver = useModelStore((s) => s.addReceiver);
  // The CRS the store holds, and therefore the CRS every number in this dialog
  // is in — read out loud in the field labels so a typed easting is never
  // mistaken for a longitude.
  const crs = useModelStore((s) => s.crs);
  const fieldId = useId();
  const reasonId = `${fieldId}-incomplete`;

  const defaultKind = geometry ? inferKind(geometry.type) : "source";
  const defaultSourceType = geometry ? inferSourceType(geometry.type) : "point";

  const [kind, setKind] = useState<FeatureKind | "receiver">(defaultKind);
  const [sourceType, setSourceType] = useState<SourceType>(defaultSourceType);
  const [height, setHeight] = useState("5");
  const [vertices, setVertices] = useState<VertexInput[]>([blankVertex()]);

  // Reset the form whenever the dialog is (re)opened, for either path.
  useEffect(() => {
    if (!open) return;
    if (geometry) {
      setKind(inferKind(geometry.type));
      setSourceType(inferSourceType(geometry.type));
      setHeight("5");
      return;
    }
    setKind("source");
    setSourceType("point");
    setHeight("5");
    setVertices([blankVertex()]);
  }, [open, geometry]);

  const typing = geometry === null;
  const targetType = typedGeometryType(kind, sourceType);

  // The row count follows the chosen kind: switching to a building with one
  // row typed would otherwise leave a polygon that can never be completed, and
  // switching back to a point would leave rows that are silently dropped.
  useEffect(() => {
    if (!typing) return;
    setVertices((current) => {
      const minimum = MINIMUM_VERTICES[targetType];
      if (targetType === "Point") {
        return current.length === 1 ? current : current.slice(0, 1);
      }
      if (current.length >= minimum) return current;
      return [
        ...current,
        ...Array.from({ length: minimum - current.length }, blankVertex),
      ];
    });
  }, [typing, targetType]);

  const drawnPositions = useMemo(
    () => (geometry ? flattenPositions(geometry) : []),
    [geometry],
  );

  const effectiveGeometry = geometry ?? typedGeometry(targetType, vertices);
  const canSave = effectiveGeometry !== null;

  // `aria-disabled`, not `disabled`, for the reason `ui/mode-gate.tsx` argues
  // in full and `map/draw-toolbar.tsx` implements: a disabled DOM button is out
  // of the focus order and carries `disabled:pointer-events-none`, so the very
  // explanation of why it is refused becomes unreachable in the state it
  // describes. A click handler swallows activation instead, which covers Enter
  // and Space too because both produce a click.
  const refuse = (event: ReactMouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
  };

  // Receivers are points, so the option only makes sense where the geometry is
  // one. On the typed path the kind *decides* the geometry, so it always does.
  const receiverAllowed =
    typing || geometry.type === "Point" || geometry.type === "MultiPoint";

  const handleSave = useCallback(() => {
    const saved = geometry ?? typedGeometry(targetType, vertices);
    if (!saved) return;

    if (kind === "receiver") {
      const receiver: ModelReceiver = {
        id: createReceiverId(),
        heightM: Math.max(0.1, parseFloat(height) || DEFAULT_RECEIVER_HEIGHT_M),
        geometry: saved as { type: "Point"; coordinates: Position },
      };
      addReceiver(receiver);
      onClose();
      return;
    }

    const feature = {
      id: createFeatureId(),
      kind,
      geometry: saved,
      ...(kind === "source" ? { sourceType } : {}),
      ...(kind === "building" || kind === "barrier"
        ? { heightM: Math.max(0.1, parseFloat(height) || 5) }
        : {}),
    };
    addFeature(feature);
    onClose();
  }, [
    geometry,
    targetType,
    vertices,
    kind,
    sourceType,
    height,
    addFeature,
    addReceiver,
    onClose,
  ]);

  const atMinimum = vertices.length <= MINIMUM_VERTICES[targetType];

  const setVertex = (index: number, axis: "x" | "y", value: string) => {
    setVertices((current) =>
      current.map((vertex, i) =>
        i === index ? { ...vertex, [axis]: value } : vertex,
      ),
    );
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>
            {typing
              ? m.dialog_title_coordinate_feature()
              : m.dialog_title_new_feature()}
          </DialogTitle>
        </DialogHeader>
        <div className="max-h-[60vh] space-y-3 overflow-y-auto py-2">
          <div className="grid gap-1.5">
            <Label className="text-xs">{m.label_kind()}</Label>
            <Select
              value={kind}
              onValueChange={(v) => {
                setKind(v as FeatureKind | "receiver");
                if (v === "receiver") {
                  setHeight("4");
                } else if (v === "building" || v === "barrier") {
                  setHeight("5");
                }
              }}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="source">{m.option_source()}</SelectItem>
                <SelectItem value="building">{m.option_building()}</SelectItem>
                <SelectItem value="barrier">{m.option_barrier()}</SelectItem>
                {receiverAllowed ? (
                  <SelectItem value="receiver">
                    {m.option_receiver()}
                  </SelectItem>
                ) : null}
              </SelectContent>
            </Select>
          </div>

          {kind === "source" ? (
            <div className="grid gap-1.5">
              <Label className="text-xs">{m.label_source_type()}</Label>
              <Select
                value={sourceType}
                onValueChange={(v) => {
                  setSourceType(v as SourceType);
                }}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="point">
                    {m.option_source_type_point()}
                  </SelectItem>
                  <SelectItem value="line">
                    {m.option_source_type_line()}
                  </SelectItem>
                  <SelectItem value="area">
                    {m.option_source_type_area()}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          ) : null}

          {typing ? (
            <VertexEditor
              crs={crs}
              fieldId={fieldId}
              vertices={vertices}
              atMinimum={atMinimum}
              minimum={MINIMUM_VERTICES[targetType]}
              onChange={setVertex}
              onAdd={() => {
                setVertices((current) => [...current, blankVertex()]);
              }}
              onRemove={(index) => {
                setVertices((current) => current.filter((_, i) => i !== index));
              }}
              refuse={refuse}
              allowAdd={targetType !== "Point"}
            />
          ) : (
            <CoordinateReadback crs={crs} positions={drawnPositions} />
          )}

          {kind === "building" || kind === "barrier" || kind === "receiver" ? (
            <div className="grid gap-1.5">
              <Label className="text-xs">{m.label_height_m()}</Label>
              <Input
                type="number"
                step="0.1"
                min="0.1"
                className="h-8 text-xs"
                value={height}
                onChange={(e) => {
                  setHeight(e.target.value);
                }}
              />
            </div>
          ) : null}

          {canSave ? null : (
            <p id={reasonId} className="text-xs text-muted-foreground">
              {m.msg_coordinate_entry_incomplete()}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" size="sm" onClick={onClose}>
            {m.action_cancel()}
          </Button>
          <Button
            size="sm"
            className={cn(!canSave && "opacity-50 cursor-not-allowed")}
            aria-disabled={canSave ? undefined : true}
            aria-describedby={canSave ? undefined : reasonId}
            onClick={(event) => {
              if (!canSave) {
                refuse(event);
                return;
              }
              handleSave();
            }}
          >
            {m.action_add_feature()}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The typed coordinate list: one row per vertex, x and y each a labelled
 * field, with the CRS named in every label.
 */
function VertexEditor({
  crs,
  fieldId,
  vertices,
  atMinimum,
  minimum,
  allowAdd,
  onChange,
  onAdd,
  onRemove,
  refuse,
}: {
  crs: string;
  fieldId: string;
  vertices: VertexInput[];
  atMinimum: boolean;
  minimum: number;
  allowAdd: boolean;
  onChange: (index: number, axis: "x" | "y", value: string) => void;
  onAdd: () => void;
  onRemove: (index: number) => void;
  refuse: (event: ReactMouseEvent) => void;
}) {
  const minimumId = `${fieldId}-minimum`;

  return (
    <div className="grid gap-2">
      <Label className="text-xs">{m.label_vertices()}</Label>
      <p className="text-2xs leading-relaxed text-muted-foreground">
        {m.msg_coordinate_entry_crs({ crs })}
      </p>
      <ul className="grid gap-2">
        {vertices.map((vertex, index) => (
          // Index keys, deliberately: the rows carry no identity of their own
          // and every value is controlled state, so a removal re-renders the
          // remaining rows with the right values whichever key they got.
          <li key={index} className="flex items-end gap-1.5">
            <FormField
              className="flex-1 gap-1"
              id={`${fieldId}-x-${String(index)}`}
              label={m.label_coordinate_x({ crs })}
              type="number"
              step="any"
              inputMode="decimal"
              value={vertex.x}
              onChange={(e) => {
                onChange(index, "x", e.target.value);
              }}
            />
            <FormField
              className="flex-1 gap-1"
              id={`${fieldId}-y-${String(index)}`}
              label={m.label_coordinate_y({ crs })}
              type="number"
              step="any"
              inputMode="decimal"
              value={vertex.y}
              onChange={(e) => {
                onChange(index, "y", e.target.value);
              }}
            />
            {allowAdd ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className={cn(
                  "size-8",
                  atMinimum && "opacity-50 cursor-not-allowed",
                )}
                aria-disabled={atMinimum ? true : undefined}
                aria-label={m.action_remove_vertex({ index: index + 1 })}
                aria-describedby={atMinimum ? minimumId : undefined}
                onClick={(event) => {
                  if (atMinimum) {
                    refuse(event);
                    return;
                  }
                  onRemove(index);
                }}
              >
                <Trash2 aria-hidden="true" />
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
      {allowAdd && atMinimum ? (
        <p id={minimumId} className="text-2xs text-muted-foreground">
          {m.msg_vertex_minimum({ count: minimum })}
        </p>
      ) : null}
      {allowAdd ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 justify-start text-xs"
          onClick={onAdd}
        >
          <Plus aria-hidden="true" className="size-3.5" />
          {m.action_add_vertex()}
        </Button>
      ) : null}
    </div>
  );
}

/**
 * Where the drawn shape ended up, in the CRS it was just moved into. Before
 * this the dialog took a geometry and never showed it, so the one visible
 * consequence of the projection in `use-draw-projection.ts` was the shape's
 * position on a basemap.
 */
function CoordinateReadback({
  crs,
  positions,
}: {
  crs: string;
  positions: Position[];
}) {
  if (positions.length === 0) return null;
  const shown = positions.slice(0, READBACK_LIMIT);
  const rest = positions.length - shown.length;

  return (
    <div className="grid gap-1">
      <Label className="text-xs">
        {m.label_coordinates()} ({crs})
      </Label>
      <ul className="font-mono text-2xs tabular-nums text-muted-foreground">
        {shown.map((position, index) => (
          <li key={index}>
            {formatCoordinate(position[0], 2)},{" "}
            {formatCoordinate(position[1], 2)}
          </li>
        ))}
        {rest > 0 ? <li>{m.msg_and_more({ count: rest })}</li> : null}
      </ul>
    </div>
  );
}
