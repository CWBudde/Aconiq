import type * as React from "react";
import {
  MousePointer,
  Circle,
  Minus,
  Pentagon,
  Crop,
  Keyboard,
  X,
} from "lucide-react";
import { Button } from "@/ui/components/button";
import { cn } from "@/ui/lib/utils";
import { Separator } from "@/ui/components/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/ui/components/tooltip";
import type { DrawMode } from "./use-draw";
import { MapPanel } from "./map-panel";
import { m } from "@/i18n/messages";

interface DrawToolbarProps {
  activeMode: DrawMode;
  onModeChange: (mode: DrawMode) => void;
  onCancel: () => void;
  /**
   * Every tool is refused. Set when the model is stored in a CRS the map does
   * not draw in: terra-draw emits WGS84, and a shape drawn over a metric model
   * would enter it in the wrong coordinate system.
   */
  disabled?: boolean;
  /** Why, shown in each tool's own tooltip in place of its label. */
  disabledReason?: string;
  /**
   * Opens the new-feature dialog with no drawn geometry, so the coordinates
   * are typed instead of pointed at.
   *
   * Deliberately **not** covered by `disabled`. Every reason the pointer tools
   * are refused is a reason terra-draw's WGS 84 output could not be moved into
   * the model's CRS; typed numbers are already in it, so nothing has to be
   * projected and nothing can go wrong in the way `disabled` guards against.
   * On a model in a CRS the kernel cannot project — where the map draws
   * nothing and every tool here is dead — this is the only way to add a
   * feature at all.
   */
  onCoordinateEntry: () => void;
}

const modelTools: {
  mode: DrawMode;
  icon: typeof Circle;
  label: () => string;
}[] = [
  { mode: "select", icon: MousePointer, label: m.tool_select_edit },
  { mode: "point", icon: Circle, label: m.tool_draw_point },
  { mode: "linestring", icon: Minus, label: m.tool_draw_line },
  { mode: "polygon", icon: Pentagon, label: m.tool_draw_polygon },
];

export function DrawToolbar({
  activeMode,
  onModeChange,
  onCancel,
  disabled = false,
  disabledReason,
  onCoordinateEntry,
}: DrawToolbarProps) {
  const isDrawing = activeMode !== "static";
  // The reason replaces the label rather than joining it: the tooltip is the
  // only place this can be said, and "Draw point" beside a dead button says
  // nothing about why it is dead.
  const reason = (label: string) =>
    disabled && disabledReason !== undefined ? disabledReason : label;

  // `aria-disabled`, not `disabled`, and the tooltip above is the whole reason.
  // `ui/components/button.tsx` puts `disabled:pointer-events-none` on every
  // button, and a disabled DOM button takes no focus either — so a tooltip
  // whose trigger is one never opens, by mouse or by keyboard, in exactly the
  // state it exists to explain. `ui/mode-gate.tsx` carries the full argument,
  // and `map/undo-redo-bar.tsx` carries the same fix — it was never the same
  // severity, its tooltip repeating the `aria-label` verbatim, but it was the
  // same trap.
  //
  // A click handler swallows activation instead, which covers Enter and Space
  // too because both produce a click.
  const refuse = (event: React.MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
  };

  const refusedClass = cn(
    "size-8",
    disabled && "opacity-50 cursor-not-allowed",
  );

  return (
    <MapPanel
      position="top-left"
      role="toolbar"
      aria-orientation="vertical"
      aria-label={m.label_draw_tools()}
      className="flex flex-col gap-1 p-1"
    >
      {modelTools.map(({ mode, icon: Icon, label }) => (
        <Tooltip key={mode}>
          <TooltipTrigger asChild>
            <Button
              variant={activeMode === mode ? "default" : "ghost"}
              size="icon"
              className={refusedClass}
              aria-pressed={activeMode === mode}
              aria-disabled={disabled || undefined}
              onClick={(event) => {
                if (disabled) {
                  refuse(event);
                  return;
                }
                onModeChange(mode);
              }}
              aria-label={label()}
            >
              <Icon aria-hidden="true" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">{reason(label())}</TooltipContent>
        </Tooltip>
      ))}
      <Separator className="my-1" />
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant={activeMode === "calc-area" ? "default" : "ghost"}
            size="icon"
            className={refusedClass}
            aria-pressed={activeMode === "calc-area"}
            aria-disabled={disabled || undefined}
            onClick={(event) => {
              if (disabled) {
                refuse(event);
                return;
              }
              onModeChange("calc-area");
            }}
            aria-label={m.tool_draw_calc_area()}
          >
            <Crop aria-hidden="true" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="right">
          {reason(m.tool_draw_calc_area())}
        </TooltipContent>
      </Tooltip>
      <Separator className="my-1" />
      {/* Below the separator and never greyed: this is not a drawing tool and
          it is not gated on the projection, for the reason `onCoordinateEntry`
          carries. */}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            onClick={onCoordinateEntry}
            aria-label={m.action_enter_coordinates()}
          >
            <Keyboard aria-hidden="true" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="right">
          {m.action_enter_coordinates()}
        </TooltipContent>
      </Tooltip>
      {isDrawing ? (
        <>
          <Separator className="my-1" />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-8 text-destructive"
                onClick={onCancel}
                aria-label={m.action_cancel_drawing()}
              >
                <X aria-hidden="true" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">{m.tooltip_cancel()}</TooltipContent>
          </Tooltip>
        </>
      ) : null}
    </MapPanel>
  );
}
