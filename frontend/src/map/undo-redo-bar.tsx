import { useEffect } from "react";
import { Undo2, Redo2 } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/ui/components/tooltip";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";

export function UndoRedoBar() {
  const canUndo = useModelStore((s) => s.canUndo);
  const canRedo = useModelStore((s) => s.canRedo);
  const undo = useModelStore((s) => s.undo);
  const redo = useModelStore((s) => s.redo);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "z") {
        e.preventDefault();
        if (e.shiftKey) {
          redo();
        } else {
          undo();
        }
      }
      if ((e.ctrlKey || e.metaKey) && e.key === "y") {
        e.preventDefault();
        redo();
      }
    };
    window.addEventListener("keydown", handler);
    return () => {
      window.removeEventListener("keydown", handler);
    };
  }, [undo, redo]);

  return (
    <div className="absolute bottom-3 right-3 z-10 flex gap-1 rounded-md border bg-background p-1 shadow-md">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={!canUndo}
            onClick={undo}
            aria-label={m.tooltip_undo()}
          >
            <Undo2 className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>{m.tooltip_undo()}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={!canRedo}
            onClick={redo}
            aria-label={m.tooltip_redo()}
          >
            <Redo2 className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>{m.tooltip_redo()}</TooltipContent>
      </Tooltip>
    </div>
  );
}
