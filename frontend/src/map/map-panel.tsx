import type * as React from "react";
import { cn } from "@/ui/lib/utils";

export type MapPanelPosition =
  | "top-left"
  | "top-right"
  | "bottom-left"
  | "bottom-right"
  /** Centred on the bottom edge — for a readout that belongs to no corner. */
  | "bottom-center";

export interface MapPanelProps extends React.HTMLAttributes<HTMLDivElement> {
  /** The corner of the map the panel is anchored to. */
  position: MapPanelPosition;
  /**
   * Tailwind inset classes that move the panel off the default corner
   * offset — `"right-12 top-2"` to sit beside the navigation control,
   * `"bottom-14"` to stack above another panel. Later classes win.
   */
  inset?: string;
  /** Tailwind width class, e.g. `"w-72"`. */
  width?: string;
  /** See-through over the map: for read-only overlays like a coordinate readout. */
  translucent?: boolean;
  children: React.ReactNode;
}

const positionClass: Record<MapPanelPosition, string> = {
  "top-left": "left-3 top-3",
  "top-right": "right-3 top-3",
  "bottom-left": "bottom-3 left-3",
  "bottom-right": "bottom-3 right-3",
  // The four corners are taken on the workspace route — toolbar, layer control
  // and editor, validation, undo — so the coordinate readout sits between the
  // two bottom ones instead of under the undo bar, where it used to overlap.
  "bottom-center": "bottom-3 left-1/2 -translate-x-1/2",
};

/**
 * A floating panel over the map: toolbars, editors, readouts. Anchored to a
 * corner, above the map canvas, on the page background so the controls
 * inside stay legible over any basemap. `role` and `aria-label` pass through
 * so a toolbar or a dialog-like editor can name itself.
 */
export function MapPanel({
  position,
  inset,
  width,
  translucent = false,
  className,
  children,
  ...props
}: MapPanelProps) {
  return (
    <div
      data-slot="map-panel"
      data-position={position}
      className={cn(
        "absolute z-10 rounded-md border p-2",
        translucent
          ? "bg-background/90 shadow-sm backdrop-blur-sm"
          : "bg-background shadow-md",
        positionClass[position],
        inset,
        width,
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
