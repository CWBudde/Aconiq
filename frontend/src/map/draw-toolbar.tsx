import { MousePointer, Circle, Minus, Pentagon, Crop, X } from "lucide-react";
import { Button } from "@/ui/components/button";
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
}: DrawToolbarProps) {
  const isDrawing = activeMode !== "static";
  // The reason replaces the label rather than joining it: the tooltip is the
  // only place this can be said, and "Draw point" beside a dead button says
  // nothing about why it is dead.
  const reason = (label: string) =>
    disabled && disabledReason !== undefined ? disabledReason : label;

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
              className="size-8"
              aria-pressed={activeMode === mode}
              disabled={disabled}
              onClick={() => {
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
            className="size-8"
            aria-pressed={activeMode === "calc-area"}
            disabled={disabled}
            onClick={() => {
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
