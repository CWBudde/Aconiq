import { Undo2, Redo2 } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/ui/components/tooltip";
import { useModelStore } from "@/model/model-store";
import { useGlobalShortcut } from "@/ui/hooks/use-global-shortcut";
import { m } from "@/i18n/messages";

export function UndoRedoBar() {
  const canUndo = useModelStore((s) => s.canUndo);
  const canRedo = useModelStore((s) => s.canRedo);
  const undo = useModelStore((s) => s.undo);
  const redo = useModelStore((s) => s.redo);

  // Global shortcuts bow out of text fields by default, and here that is the
  // point: this handler once hijacked Ctrl+Z from every focused input, so
  // undoing a typo in the feature editor reverted a map edit instead — with
  // no way to get the text back.
  useGlobalShortcut({ key: "z", ctrl: true }, undo);
  useGlobalShortcut({ key: "z", ctrl: true, shift: true }, redo);
  useGlobalShortcut({ key: "y", ctrl: true }, redo);

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
