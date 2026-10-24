import {
  useId,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from "react";
import { m } from "@/i18n/messages";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import type { PhotoRecord } from "./photo";

/** The corners in the order they are clicked, as `facadeCorners` lists them. */
const CORNER_LABELS = [
  () => m.label_corner_bottom_left(),
  () => m.label_corner_bottom_right(),
  () => m.label_corner_top_right(),
  () => m.label_corner_top_left(),
] as const;

type Corner = [number | null, number | null];

const NO_CORNERS: Corner[] = [
  [null, null],
  [null, null],
  [null, null],
  [null, null],
];

export interface PhotoCornersProps {
  /** The photo's object URL; it is shown here and nowhere else. */
  url: string;
  /** Whether the rectification is being solved. */
  busy: boolean;
  /** Why the last attempt was refused, in the Go code's words. */
  error: string | null;
  onSubmit: (points: [number, number][], record: PhotoRecord) => void;
  onCancel: () => void;
}

/**
 * Where a facade's four corners are in a photo of it (P18 D1): clicked in
 * order — bottom-left, bottom-right, top-right, top-left, seen from outside —
 * then dragged, nudged with the arrow keys or typed, in the photo's own
 * pixels as the browser shows it (EXIF orientation applied, so the overlay
 * uses the same grid). With them the person records where the photo is kept
 * and who took it. The pixels never leave this view: only the corners do.
 */
export function PhotoCorners({
  url,
  busy,
  error,
  onSubmit,
  onCancel,
}: PhotoCornersProps) {
  const id = useId();
  const svgRef = useRef<SVGSVGElement>(null);
  const [size, setSize] = useState<{ w: number; h: number } | null>(null);
  const [corners, setCorners] = useState<Corner[]>(NO_CORNERS);
  const [dragging, setDragging] = useState<number | null>(null);
  const [sourceId, setSourceId] = useState("");
  const [attribution, setAttribution] = useState("");
  const [capturedAt, setCapturedAt] = useState("");

  const points = corners.flatMap(([x, y]) =>
    x === null || y === null ? [] : [[x, y] as [number, number]],
  );
  const next = corners.findIndex(([x, y]) => x === null || y === null);
  const complete = next === -1;
  const marker = size === null ? 1 : Math.max(size.w, size.h) / 120;

  function setCorner(i: number, corner: Corner) {
    setCorners((cs) => cs.map((c, j) => (j === i ? corner : c)));
  }

  function clamp(v: number, max: number | undefined) {
    return max === undefined ? v : Math.min(Math.max(v, 0), max);
  }

  function pixelOf(event: ReactPointerEvent<SVGSVGElement>) {
    const svg = svgRef.current;
    if (svg === null || size === null) return null;
    const rect = svg.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;
    return [
      clamp(((event.clientX - rect.left) * size.w) / rect.width, size.w),
      clamp(((event.clientY - rect.top) * size.h) / rect.height, size.h),
    ] as [number, number];
  }

  function handlePointerDown(event: ReactPointerEvent<SVGSVGElement>) {
    if (event.button !== 0) return;
    const p = pixelOf(event);
    if (p === null) return;
    const hit = (event.target as Element).getAttribute("data-corner");
    if (hit !== null) {
      setDragging(Number(hit));
    } else if (!complete) {
      setCorner(next, p);
      return;
    } else {
      return;
    }
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // Not every environment captures pointers; dragging works without.
    }
  }

  function handlePointerMove(event: ReactPointerEvent<SVGSVGElement>) {
    if (dragging === null) return;
    const p = pixelOf(event);
    if (p !== null) setCorner(dragging, p);
  }

  function handleMarkerKeyDown(
    i: number,
    event: ReactKeyboardEvent<SVGCircleElement>,
  ) {
    const step = event.shiftKey ? 10 : 1;
    const delta: Record<string, [number, number]> = {
      ArrowLeft: [-step, 0],
      ArrowRight: [step, 0],
      ArrowUp: [0, -step],
      ArrowDown: [0, step],
    };
    const d = delta[event.key];
    const [x, y] = corners[i] ?? [null, null];
    if (d === undefined || x === null || y === null) return;
    event.preventDefault();
    setCorner(i, [clamp(x + d[0], size?.w), clamp(y + d[1], size?.h)]);
  }

  const nextLabel = complete ? "" : (CORNER_LABELS[next]?.() ?? "");

  return (
    <section
      aria-label={m.heading_photo_corners()}
      className="grid gap-3 rounded-md border p-3"
    >
      <h3 className="text-sm font-medium">{m.heading_photo_corners()}</h3>
      <p aria-live="polite" className="text-xs text-muted-foreground">
        {complete
          ? m.msg_photo_corners_done()
          : m.msg_photo_corner_prompt({ corner: nextLabel })}
      </p>
      <div className="relative">
        <img
          src={url}
          alt={m.label_photo_file()}
          draggable={false}
          className="block h-auto w-full select-none"
          onLoad={(e) => {
            const img = e.currentTarget;
            setSize({ w: img.naturalWidth, h: img.naturalHeight });
          }}
        />
        {size !== null && (
          <svg
            ref={svgRef}
            viewBox={`0 0 ${String(size.w)} ${String(size.h)}`}
            preserveAspectRatio="none"
            className="absolute inset-0 h-full w-full cursor-crosshair touch-none"
            role="group"
            aria-label={m.heading_photo_corners()}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={() => {
              setDragging(null);
            }}
            onPointerCancel={() => {
              setDragging(null);
            }}
          >
            {points.length > 1 && (
              <polygon
                points={points.map((p) => p.join(",")).join(" ")}
                className="pointer-events-none fill-primary/10 stroke-primary"
                strokeWidth={marker / 3}
              />
            )}
            {corners.map(([x, y], i) =>
              x === null || y === null ? null : (
                <circle
                  key={CORNER_LABELS[i]?.() ?? i}
                  data-corner={i}
                  role="button"
                  tabIndex={0}
                  aria-label={m.label_photo_corner({
                    corner: CORNER_LABELS[i]?.() ?? "",
                  })}
                  cx={x}
                  cy={y}
                  r={marker}
                  className="cursor-move fill-primary/60 stroke-background outline-none focus-visible:stroke-ring"
                  strokeWidth={marker / 3}
                  onKeyDown={(e) => {
                    handleMarkerKeyDown(i, e);
                  }}
                />
              ),
            )}
          </svg>
        )}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {corners.map(([x, y], i) => {
          const corner = CORNER_LABELS[i]?.() ?? "";
          return [
            [0, x, m.label_photo_corner_x({ corner })],
            [1, y, m.label_photo_corner_y({ corner })],
          ].map(([axis, value, label]) => (
            <div key={`${String(i)}-${String(axis)}`} className="grid gap-1">
              <Label
                htmlFor={`${id}-${String(i)}-${String(axis)}`}
                className="text-xs"
              >
                {label}
              </Label>
              <Input
                id={`${id}-${String(i)}-${String(axis)}`}
                type="number"
                step="1"
                className="h-8 text-xs"
                value={value === null ? "" : String(value)}
                onChange={(e) => {
                  const parsed =
                    e.target.value === ""
                      ? null
                      : Number.parseFloat(e.target.value);
                  const v =
                    parsed === null || Number.isFinite(parsed) ? parsed : null;
                  setCorner(i, axis === 0 ? [v, y] : [x, v]);
                }}
              />
            </div>
          ));
        })}
      </div>
      <div className="grid gap-2 sm:grid-cols-3">
        <div className="grid gap-1 sm:col-span-3">
          <Label htmlFor={`${id}-ref`} className="text-xs">
            {m.label_photo_reference()}
          </Label>
          <Input
            id={`${id}-ref`}
            required
            aria-describedby={`${id}-ref-hint`}
            className="h-8 text-xs"
            value={sourceId}
            onChange={(e) => {
              setSourceId(e.target.value);
            }}
          />
          <p id={`${id}-ref-hint`} className="text-xs text-muted-foreground">
            {m.msg_photo_reference_hint()}
          </p>
        </div>
        <div className="grid gap-1">
          <Label htmlFor={`${id}-by`} className="text-xs">
            {m.label_photo_photographer()}
          </Label>
          <Input
            id={`${id}-by`}
            className="h-8 text-xs"
            value={attribution}
            onChange={(e) => {
              setAttribution(e.target.value);
            }}
          />
        </div>
        <div className="grid gap-1">
          <Label htmlFor={`${id}-at`} className="text-xs">
            {m.label_photo_captured()}
          </Label>
          <Input
            id={`${id}-at`}
            type="date"
            className="h-8 text-xs"
            value={capturedAt}
            onChange={(e) => {
              setCapturedAt(e.target.value);
            }}
          />
        </div>
      </div>
      {error !== null && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          size="sm"
          className="h-8 text-xs"
          disabled={!complete || sourceId.trim() === "" || busy}
          onClick={() => {
            onSubmit(points, {
              sourceId: sourceId.trim(),
              attribution: attribution.trim(),
              capturedAt,
            });
          }}
        >
          {m.action_rectify_photo()}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 text-xs"
          onClick={() => {
            setCorners(NO_CORNERS);
          }}
        >
          {m.action_reset_corners()}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="ml-auto h-8 text-xs"
          onClick={onCancel}
        >
          {m.action_cancel()}
        </Button>
      </div>
    </section>
  );
}
