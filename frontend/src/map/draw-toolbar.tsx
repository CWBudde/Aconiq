import { MousePointer, Circle, Minus, Pentagon, X } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/ui/components/tooltip";
import type { DrawMode } from "./use-draw";
import { cn } from "@/ui/lib/utils";

interface DrawToolbarProps {
  activeMode: DrawMode;
  onModeChange: (mode: DrawMode) => void;
  onCancel: () => void;
}

const tools: { mode: DrawMode; icon: typeof Circle; label: string }[] = [
  { mode: "select", icon: MousePointer, label: "Select / Edit" },
  { mode: "point", icon: Circle, label: "Draw Point" },
  { mode: "linestring", icon: Minus, label: "Draw Line" },
  { mode: "polygon", icon: Pentagon, label: "Draw Polygon" },
];

export function DrawToolbar({
  activeMode,
  onModeChange,
  onCancel,
}: DrawToolbarProps) {
  const isDrawing = activeMode !== "static";

  return (
    <div className="absolute left-3 top-3 z-10 flex flex-col gap-1 rounded-md border bg-background p-1 shadow-md">
      {tools.map(({ mode, icon: Icon, label }) => (
        <Tooltip key={mode}>
          <TooltipTrigger asChild>
            <Button
              variant={activeMode === mode ? "default" : "ghost"}
              size="icon"
              className={cn("h-8 w-8")}
              onClick={() => {
                onModeChange(mode);
              }}
              aria-label={label}
            >
              <Icon className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">{label}</TooltipContent>
        </Tooltip>
      ))}
      {isDrawing ? (
        <>
          <div className="my-1 border-t" />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive"
                onClick={onCancel}
                aria-label="Cancel drawing"
              >
                <X className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">Cancel</TooltipContent>
          </Tooltip>
        </>
      ) : null}
    </div>
  );
}
