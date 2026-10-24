import type {
  KeyboardEvent as ReactKeyboardEvent,
  PointerEvent as ReactPointerEvent,
  Ref,
} from "react";
import { m } from "@/i18n/messages";
import { cn } from "@/ui/lib/utils";
import { formatNumber } from "@/ui/format";
import type {
  ElevationWindow,
  FacadeElevation,
  FacadeWindowFrame,
} from "@/wasm/types";
import { windowLabel } from "./elevation-labels";
import {
  ELEVATION_VIEW_HEIGHT,
  ELEVATION_VIEW_PAD as PAD,
  ELEVATION_VIEW_WIDTH,
  elevationScale,
  type ElevationScale,
} from "./elevation-scale";

/** The side of the square a selected window is resized by, in view units. */
const HANDLE = 8;

export interface ElevationSvgProps {
  facade: FacadeElevation;
  /** The facade's number in the picker, 1-based. */
  number: number;
  heightM: number;
  slabHeightsM: number[];
  /**
   * The windows to draw: the facade's own, with a window being moved shown
   * where the gesture has it rather than where the model still has it.
   */
  windows: ElevationWindow[];
  selectedId: string | null;
  /** A window being drawn, not yet in the model. */
  draft: FacadeWindowFrame | null;
  svgRef?: Ref<SVGSVGElement>;
  onPointerDown?: (event: ReactPointerEvent<SVGSVGElement>) => void;
  onPointerMove?: (event: ReactPointerEvent<SVGSVGElement>) => void;
  onPointerUp?: (event: ReactPointerEvent<SVGSVGElement>) => void;
  onPointerCancel?: (event: ReactPointerEvent<SVGSVGElement>) => void;
  onSelect?: (id: string) => void;
  onWindowKeyDown?: (
    id: string,
    event: ReactKeyboardEvent<SVGRectElement>,
  ) => void;
}

/**
 * One facade unfolded: the wall as a rectangle of its length by the
 * building's height, a line at each storey's ceiling slab — where facade mode
 * places that storey's points — and each window as the rectangle its frame
 * gives it, looking at the facade from outside.
 *
 * It draws and reports gestures; it decides nothing. A window is a focusable
 * button, so the drawing is usable from the keyboard as well as by pointer.
 */
export function ElevationSvg({
  facade,
  number,
  heightM,
  slabHeightsM,
  windows,
  selectedId,
  draft,
  svgRef,
  onPointerDown,
  onPointerMove,
  onPointerUp,
  onPointerCancel,
  onSelect,
  onWindowKeyDown,
}: ElevationSvgProps) {
  const s = elevationScale(facade.length_m, heightM);
  const left = s.x(0);
  const right = s.x(facade.length_m);
  const ground = s.y(0);
  const top = s.y(heightM);
  const storeyFloors = [0, ...slabHeightsM];

  return (
    <svg
      ref={svgRef}
      viewBox={`0 0 ${String(ELEVATION_VIEW_WIDTH)} ${String(ELEVATION_VIEW_HEIGHT)}`}
      className="h-auto w-full touch-none select-none"
      role="group"
      aria-label={m.label_facade_elevation_svg({
        n: number,
        length: formatNumber(facade.length_m),
        height: formatNumber(heightM),
        count: windows.length,
      })}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerCancel}
    >
      <rect
        data-wall=""
        x={left}
        y={top}
        width={right - left}
        height={ground - top}
        className="fill-muted stroke-border"
        strokeWidth={1}
      />
      {slabHeightsM.map((z) =>
        z < heightM ? (
          <line
            key={z}
            x1={left}
            x2={right}
            y1={s.y(z)}
            y2={s.y(z)}
            className="pointer-events-none stroke-muted-foreground/50"
            strokeDasharray="4 3"
          />
        ) : null,
      )}
      {storeyFloors.map((floor, storey) => {
        const ceiling = Math.min(slabHeightsM[storey] ?? heightM, heightM);
        if (floor >= heightM) return null;
        return (
          <text
            key={floor}
            x={left - 6}
            y={(s.y(floor) + s.y(ceiling)) / 2}
            textAnchor="end"
            dominantBaseline="middle"
            className="pointer-events-none fill-muted-foreground text-[10px]"
          >
            {storey}
          </text>
        );
      })}
      <line
        x1={left - PAD / 2}
        x2={right + PAD / 2}
        y1={ground}
        y2={ground}
        className="pointer-events-none stroke-foreground"
        strokeWidth={1.5}
      />
      {windows.map((w) => (
        <WindowRect
          key={w.id}
          window={w}
          scale={s}
          selected={w.id === selectedId}
          onSelect={onSelect}
          onKeyDown={onWindowKeyDown}
        />
      ))}
      {draft !== null && (
        <rect
          x={s.x(draft.u_m)}
          y={s.y(draft.sill_z_m + draft.height_m)}
          width={draft.width_m * s.scale}
          height={draft.height_m * s.scale}
          className="pointer-events-none fill-primary/20 stroke-primary"
          strokeDasharray="3 2"
        />
      )}
    </svg>
  );
}

function WindowRect({
  window: w,
  scale: s,
  selected,
  onSelect,
  onKeyDown,
}: {
  window: ElevationWindow;
  scale: ElevationScale;
  selected: boolean;
  onSelect?: ((id: string) => void) | undefined;
  onKeyDown?:
    | ((id: string, event: ReactKeyboardEvent<SVGRectElement>) => void)
    | undefined;
}) {
  const frame = w.window;
  const flagged = w.findings.length > 0;
  const x = s.x(frame.u_m);
  const y = s.y(frame.sill_z_m + frame.height_m);
  const width = frame.width_m * s.scale;
  return (
    <g>
      <rect
        data-window-id={w.id}
        role="button"
        tabIndex={0}
        aria-label={windowLabel(w)}
        aria-pressed={selected}
        x={x}
        y={y}
        width={width}
        height={frame.height_m * s.scale}
        strokeWidth={selected ? 3 : flagged ? 2 : 1.5}
        strokeDasharray={w.review_status === "proposed" ? "3 2" : undefined}
        className={cn(
          "cursor-move outline-none focus-visible:stroke-ring",
          w.sensitive ? "fill-primary/40" : "fill-background",
          w.review_status === "rejected" && "opacity-40",
          flagged ? "stroke-destructive" : "stroke-primary",
        )}
        onFocus={() => onSelect?.(w.id)}
        onKeyDown={(event) => onKeyDown?.(w.id, event)}
      >
        <title>{windowLabel(w)}</title>
      </rect>
      {selected && (
        <rect
          data-window-id={w.id}
          data-handle="resize"
          aria-hidden="true"
          x={x + width - HANDLE / 2}
          y={y - HANDLE / 2}
          width={HANDLE}
          height={HANDLE}
          className="cursor-nesw-resize fill-primary stroke-background"
        />
      )}
    </g>
  );
}
