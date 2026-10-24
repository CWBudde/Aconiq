import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from "react";
import { backend } from "@/api/backend";
import { m } from "@/i18n/messages";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import { Switch } from "@/ui/components/switch";
import { ConfirmDialog } from "@/ui/confirm-dialog";
import type {
  ElevationWindow,
  FacadeElevation,
  FacadeWindowFrame,
  WindowPlacement,
} from "@/wasm/types";
import { reviewLabel } from "./elevation-labels";
import {
  ELEVATION_VIEW_HEIGHT,
  ELEVATION_VIEW_WIDTH,
  elevationScale,
} from "./elevation-scale";
import { ElevationSvg } from "./elevation-svg";
import {
  defaultFrame,
  drawnFrame,
  movedFrame,
  nudgedFrame,
  resizedFrame,
  sameFrame,
} from "./frame-edit";
import { nextWindowId, placeWindowFeature } from "./place-window";

const REVIEW_STATUSES = ["proposed", "confirmed", "rejected"] as const;
type ReviewStatus = (typeof REVIEW_STATUSES)[number];

/** A pointer gesture in progress, in the facade's frame. */
type Gesture =
  | {
      kind: "move" | "resize";
      id: string;
      startU: number;
      startZ: number;
      origin: FacadeWindowFrame;
      frame: FacadeWindowFrame;
    }
  | {
      kind: "draw";
      startU: number;
      startZ: number;
      frame: FacadeWindowFrame | null;
    };

export interface ElevationEditorProps {
  buildingId: string;
  facade: FacadeElevation;
  /** The facade's number in the picker, 1-based. */
  number: number;
  heightM: number;
  slabHeightsM: number[];
}

/**
 * The facade drawing, editable (P18 C2): drag on the wall to draw a window,
 * drag a window to move it and its corner to resize it, or select one and use
 * the arrow keys and the form beside it.
 *
 * Every change that moves a window goes through `placeWindowFeature`, so its
 * key and centre are the Go placer's, and lands in the store as one feature
 * replaced — one undo step per drop, never one per pointer move. A refusal
 * (an edge with no room for the window) is shown and leaves the model as it
 * was. A new window starts `proposed` and not sensitive, as an imported one
 * does: marking it sensitive and confirming it are a person's explicit acts.
 */
export function ElevationEditor({
  buildingId,
  facade,
  number,
  heightM,
  slabHeightsM,
}: ElevationEditorProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [gesture, setGestureState] = useState<Gesture | null>(null);
  // The gesture as of the last pointer event, not the last render: a release
  // right after a move must commit where the move put the window.
  const gestureRef = useRef<Gesture | null>(null);
  function setGesture(next: Gesture | null) {
    gestureRef.current = next;
    setGestureState(next);
  }
  // One placement at a time. A second gesture while one is in flight is
  // ignored: two new windows would otherwise both take the same `win-<n>`,
  // and two moves would each overwrite the other's answer.
  const placing = useRef(false);
  // A dropped frame stays drawn until the model's new elevation arrives, so a
  // window does not jump back to where it was for the length of a round trip.
  const [pending, setPending] = useState<{
    id: string;
    frame: FacadeWindowFrame;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirmingDelete, setConfirmingDelete] = useState<string | null>(null);

  const getFeatureById = useModelStore((s) => s.getFeatureById);
  const addFeature = useModelStore((s) => s.addFeature);
  const updateFeature = useModelStore((s) => s.updateFeature);
  const removeFeature = useModelStore((s) => s.removeFeature);

  useEffect(() => {
    setPending(null);
  }, [facade]);

  const scale = elevationScale(facade.length_m, heightM);

  const windows = facade.windows.map((w): ElevationWindow => {
    if (gesture !== null && gesture.kind !== "draw" && gesture.id === w.id) {
      return { ...w, window: gesture.frame };
    }
    if (pending?.id === w.id) return { ...w, window: pending.frame };
    return w;
  });
  const selected = windows.find((w) => w.id === selectedId) ?? null;

  function placementOf(
    id: string,
    frame: FacadeWindowFrame,
    feature: ModelFeature | undefined,
  ): WindowPlacement {
    const props = feature?.properties ?? {};
    const status = props.review_status;
    return {
      id,
      building_ref: buildingId,
      part: facade.part,
      ring: facade.ring,
      edge: facade.edge,
      window: frame,
      sensitive: props.sensitive === true,
      ...(isReviewStatus(status) ? { review_status: status } : {}),
      ...(Array.isArray(props.evidence) ? { evidence: props.evidence } : {}),
    };
  }

  /** Places one window; resolves whether the model took it. */
  async function place(
    id: string,
    frame: FacadeWindowFrame,
    isNew: boolean,
  ): Promise<boolean> {
    if (placing.current) return false;
    const before = isNew ? undefined : getFeatureById(id);
    if (!isNew && before === undefined) return false;
    placing.current = true;
    setPending({ id, frame });
    try {
      const placed = await placeWindowFeature(
        backend,
        workspaceNow(),
        placementOf(id, frame, before),
      );
      setError(null);
      if (isNew) {
        addFeature(placed);
        setSelectedId(id);
        return true;
      }
      // Read again: a review field changed, or an undo, while the placement
      // was in flight must not be written back over.
      const current = getFeatureById(id);
      if (current === undefined) {
        setPending(null);
        return false;
      }
      // A move keeps everything a person or an import said about the
      // window; only where it is changes.
      updateFeature({
        ...current,
        properties: {
          ...current.properties,
          facade: placed.properties?.facade,
          window: placed.properties?.window,
        },
        geometry: placed.geometry,
      });
      return true;
    } catch (err) {
      setPending(null);
      setError(
        m.msg_window_place_failed({
          error: err instanceof Error ? err.message : String(err),
        }),
      );
      return false;
    } finally {
      placing.current = false;
    }
  }

  function setReviewField(id: string, patch: Record<string, unknown>) {
    const feature = getFeatureById(id);
    if (feature === undefined) return;
    updateFeature({
      ...feature,
      properties: { ...feature.properties, ...patch },
    });
  }

  // The drawing is laid out at its viewBox's aspect ratio (full width, auto
  // height), so its box scales both axes alike and no letterbox offsets it.
  function facadePointOf(event: ReactPointerEvent<SVGSVGElement>) {
    const svg = svgRef.current;
    if (svg === null) return null;
    const rect = svg.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;
    const x = ((event.clientX - rect.left) * ELEVATION_VIEW_WIDTH) / rect.width;
    const y =
      ((event.clientY - rect.top) * ELEVATION_VIEW_HEIGHT) / rect.height;
    return { u: scale.u(x), z: scale.z(y) };
  }

  function handlePointerDown(event: ReactPointerEvent<SVGSVGElement>) {
    if (event.button !== 0 || placing.current) return;
    const p = facadePointOf(event);
    if (p === null) return;
    const target = event.target as Element;
    const id = target.getAttribute("data-window-id");
    const window = windows.find((w) => w.id === id);

    if (window !== undefined) {
      setSelectedId(window.id);
      setGesture({
        kind:
          target.getAttribute("data-handle") === "resize" ? "resize" : "move",
        id: window.id,
        startU: p.u,
        startZ: p.z,
        origin: window.window,
        frame: window.window,
      });
    } else if (target.hasAttribute("data-wall")) {
      setSelectedId(null);
      setGesture({ kind: "draw", startU: p.u, startZ: p.z, frame: null });
    } else {
      return;
    }

    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // Not every environment captures pointers; the gesture works without.
    }
  }

  function handlePointerMove(event: ReactPointerEvent<SVGSVGElement>) {
    const gesture = gestureRef.current;
    if (gesture === null) return;
    const p = facadePointOf(event);
    if (p === null) return;
    const du = p.u - gesture.startU;
    const dz = p.z - gesture.startZ;
    switch (gesture.kind) {
      case "move":
        setGesture({
          ...gesture,
          frame: movedFrame(gesture.origin, du, dz, facade.length_m),
        });
        break;
      case "resize":
        setGesture({
          ...gesture,
          frame: resizedFrame(gesture.origin, du, dz, facade.length_m),
        });
        break;
      case "draw":
        setGesture({
          ...gesture,
          frame: drawnFrame(
            gesture.startU,
            gesture.startZ,
            p.u,
            p.z,
            facade.length_m,
            heightM,
          ),
        });
        break;
    }
  }

  function handlePointerUp() {
    const gesture = gestureRef.current;
    if (gesture === null) return;
    setGesture(null);
    if (gesture.kind === "draw") {
      if (gesture.frame !== null) {
        void place(nextWindowId(useModelStore.getState()), gesture.frame, true);
      }
    } else if (!sameFrame(gesture.frame, gesture.origin)) {
      void place(gesture.id, gesture.frame, false);
    }
  }

  /** A cancelled pointer (a scroll taking over, a lost capture) aborts. */
  function handlePointerCancel() {
    setGesture(null);
  }

  function handleWindowKeyDown(
    id: string,
    event: ReactKeyboardEvent<SVGRectElement>,
  ) {
    const window = windows.find((w) => w.id === id);
    if (window === undefined) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      setSelectedId(id);
      return;
    }
    if (event.key === "Delete" || event.key === "Backspace") {
      event.preventDefault();
      setConfirmingDelete(id);
      return;
    }
    const frame = nudgedFrame(
      window.window,
      event.key,
      event.shiftKey,
      facade.length_m,
    );
    if (frame === null) return;
    event.preventDefault();
    if (!sameFrame(frame, window.window)) void place(id, frame, false);
  }

  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          {m.msg_elevation_edit_help()}
        </p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 text-xs"
          onClick={() => {
            void place(
              nextWindowId(useModelStore.getState()),
              defaultFrame(facade.length_m, heightM),
              true,
            );
          }}
        >
          {m.action_add_window()}
        </Button>
      </div>
      <ElevationSvg
        facade={facade}
        number={number}
        heightM={heightM}
        slabHeightsM={slabHeightsM}
        windows={windows}
        selectedId={selectedId}
        draft={gesture?.kind === "draw" ? gesture.frame : null}
        svgRef={svgRef}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerCancel}
        onSelect={setSelectedId}
        onWindowKeyDown={handleWindowKeyDown}
      />
      {error !== null && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
      {selected !== null && (
        <WindowForm
          key={selected.id}
          window={selected}
          onFrame={(frame) =>
            sameFrame(frame, selected.window)
              ? Promise.resolve(true)
              : place(selected.id, frame, false)
          }
          onSensitive={(sensitive) => {
            setReviewField(selected.id, { sensitive });
          }}
          onReviewStatus={(status) => {
            setReviewField(selected.id, { review_status: status });
          }}
          onDelete={() => {
            setConfirmingDelete(selected.id);
          }}
        />
      )}
      <ConfirmDialog
        open={confirmingDelete !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmingDelete(null);
        }}
        tone="destructive"
        title={m.confirm_delete_window_title({ id: confirmingDelete ?? "" })}
        description={m.confirm_delete_window_desc()}
        confirmLabel={m.action_delete_window()}
        onConfirm={() => {
          if (confirmingDelete !== null) {
            removeFeature(confirmingDelete);
            if (selectedId === confirmingDelete) setSelectedId(null);
          }
          setConfirmingDelete(null);
        }}
      />
    </div>
  );
}

/**
 * The selected window's rectangle and review fields: the keyboard path to
 * everything a pointer does in the drawing. A frame field commits on blur or
 * Enter, through the same placement a drag ends in.
 */
function WindowForm({
  window: w,
  onFrame,
  onSensitive,
  onReviewStatus,
  onDelete,
}: {
  window: ElevationWindow;
  /** Resolves whether the model took the frame. */
  onFrame: (frame: FacadeWindowFrame) => Promise<boolean>;
  onSensitive: (sensitive: boolean) => void;
  onReviewStatus: (status: ReviewStatus) => void;
  onDelete: () => void;
}) {
  const id = useId();
  const fields = [
    ["u_m", m.label_window_u()],
    ["sill_z_m", m.label_window_sill()],
    ["width_m", m.label_window_width()],
    ["height_m", m.label_window_height()],
  ] as const;

  return (
    <section
      aria-label={m.heading_window_edit({ id: w.id })}
      className="grid gap-3 rounded-md border p-3"
    >
      <h3 className="text-sm font-medium">
        {m.heading_window_edit({ id: w.id })}
      </h3>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {fields.map(([key, label]) => (
          <FrameField
            key={key}
            id={`${id}-${key}`}
            label={label}
            value={w.window[key]}
            onCommit={(value) => onFrame({ ...w.window, [key]: value })}
          />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-4">
        <div className="flex items-center gap-2">
          <Switch
            id={`${id}-sensitive`}
            checked={w.sensitive}
            onCheckedChange={onSensitive}
            className="h-5 w-9"
          />
          <Label htmlFor={`${id}-sensitive`} className="text-xs">
            {m.label_window_sensitive()}
          </Label>
        </div>
        <div className="flex items-center gap-2">
          <Label htmlFor={`${id}-review`} className="text-xs">
            {m.label_window_review()}
          </Label>
          <select
            id={`${id}-review`}
            className="h-8 rounded-md border border-input bg-background px-2 text-xs"
            value={w.review_status}
            onChange={(e) => {
              if (isReviewStatus(e.target.value))
                onReviewStatus(e.target.value);
            }}
          >
            {REVIEW_STATUSES.map((status) => (
              <option key={status} value={status}>
                {reviewLabel(status)}
              </option>
            ))}
          </select>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="ml-auto h-8 text-xs text-destructive"
          onClick={onDelete}
        >
          {m.action_delete_window()}
        </Button>
      </div>
    </section>
  );
}

function FrameField({
  id,
  label,
  value,
  onCommit,
}: {
  id: string;
  label: string;
  value: number;
  /** Resolves whether the model took the value. */
  onCommit: (value: number) => Promise<boolean>;
}) {
  const [text, setText] = useState(String(value));
  useEffect(() => {
    setText(String(value));
  }, [value]);

  // Commits on blur only; Enter blurs, so one edit is one placement and one
  // undo step. A refusal puts back the value the model still has.
  function commit() {
    const parsed = Number.parseFloat(text);
    if (!Number.isFinite(parsed) || parsed === value) {
      setText(String(value));
      return;
    }
    void onCommit(parsed).then((taken) => {
      if (!taken) setText(String(value));
    });
  }

  return (
    <div className="grid gap-1">
      <Label htmlFor={id} className="text-xs">
        {label}
      </Label>
      <Input
        id={id}
        type="number"
        step="0.05"
        className="h-8 text-xs"
        value={text}
        onChange={(e) => {
          setText(e.target.value);
        }}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
        }}
      />
    </div>
  );
}

function isReviewStatus(value: unknown): value is ReviewStatus {
  return (
    typeof value === "string" &&
    (REVIEW_STATUSES as readonly string[]).includes(value)
  );
}

/** The store's workspace as it is now, for a placement that ends a gesture. */
function workspaceNow() {
  const s = useModelStore.getState();
  return {
    features: s.features,
    receivers: s.receivers,
    calcArea: s.calcArea,
    crs: s.crs,
  };
}
