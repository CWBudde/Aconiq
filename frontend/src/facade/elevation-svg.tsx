import { m } from "@/i18n/messages";
import { cn } from "@/ui/lib/utils";
import { formatNumber } from "@/ui/format";
import type { ElevationWindow, FacadeElevation } from "@/wasm/types";
import { windowLabel } from "./elevation-labels";
import {
  ELEVATION_VIEW_HEIGHT,
  ELEVATION_VIEW_PAD as PAD,
  ELEVATION_VIEW_WIDTH,
  elevationScale,
  type ElevationScale,
} from "./elevation-scale";

export interface ElevationSvgProps {
  facade: FacadeElevation;
  /** The facade's number in the picker, 1-based. */
  number: number;
  heightM: number;
  slabHeightsM: number[];
}

/**
 * One facade unfolded: the wall as a rectangle of its length by the
 * building's height, a line at each storey's ceiling slab — where facade mode
 * places that storey's points — and each window as the rectangle its frame
 * gives it, looking at the facade from outside.
 */
export function ElevationSvg({
  facade,
  number,
  heightM,
  slabHeightsM,
}: ElevationSvgProps) {
  const s = elevationScale(facade.length_m, heightM);
  const left = s.x(0);
  const right = s.x(facade.length_m);
  const ground = s.y(0);
  const top = s.y(heightM);
  const storeyFloors = [0, ...slabHeightsM];

  return (
    <svg
      viewBox={`0 0 ${String(ELEVATION_VIEW_WIDTH)} ${String(ELEVATION_VIEW_HEIGHT)}`}
      className="h-auto w-full"
      role="img"
      aria-label={m.label_facade_elevation_svg({
        n: number,
        length: formatNumber(facade.length_m),
        height: formatNumber(heightM),
        count: facade.windows.length,
      })}
    >
      <rect
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
            className="stroke-muted-foreground/50"
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
            className="fill-muted-foreground text-[10px]"
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
        className="stroke-foreground"
        strokeWidth={1.5}
      />
      {facade.windows.map((w) => (
        <WindowRect key={w.id} window={w} scale={s} />
      ))}
    </svg>
  );
}

function WindowRect({
  window: w,
  scale: s,
}: {
  window: ElevationWindow;
  scale: ElevationScale;
}) {
  const frame = w.window;
  const flagged = w.findings.length > 0;
  return (
    <rect
      data-window-id={w.id}
      x={s.x(frame.u_m)}
      y={s.y(frame.sill_z_m + frame.height_m)}
      width={frame.width_m * s.scale}
      height={frame.height_m * s.scale}
      strokeWidth={flagged ? 2 : 1.5}
      strokeDasharray={w.review_status === "proposed" ? "3 2" : undefined}
      className={cn(
        w.sensitive ? "fill-primary/40" : "fill-background",
        w.review_status === "rejected" && "opacity-40",
        flagged ? "stroke-destructive" : "stroke-primary",
      )}
    >
      <title>{windowLabel(w)}</title>
    </rect>
  );
}
