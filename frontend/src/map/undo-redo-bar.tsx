import { Undo2, Redo2 } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/ui/components/tooltip";
import { useModelStore } from "@/model/model-store";
import { useGlobalShortcut } from "@/ui/hooks/use-global-shortcut";
import { MapPanel } from "./map-panel";
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
    <MapPanel
      position="bottom-right"
      role="toolbar"
      aria-label={m.label_edit_history()}
      className="flex gap-1 p-1"
    >
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            disabled={!canUndo}
            onClick={undo}
            aria-label={m.tooltip_undo()}
          >
            <Undo2 aria-hidden="true" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>{m.tooltip_undo()}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            disabled={!canRedo}
            onClick={redo}
            aria-label={m.tooltip_redo()}
          >
            <Redo2 aria-hidden="true" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>{m.tooltip_redo()}</TooltipContent>
      </Tooltip>
    </MapPanel>
  );
}
