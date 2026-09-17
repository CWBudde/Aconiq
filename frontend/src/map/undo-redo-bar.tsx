import type * as React from "react";
import { Undo2, Redo2 } from "lucide-react";
import { Button } from "@/ui/components/button";
import { cn } from "@/ui/lib/utils";
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

  // `aria-disabled`, not `disabled`, for the reason `ui/mode-gate.tsx` argues
  // in full: `ui/components/button.tsx` puts `disabled:pointer-events-none` on
  // every button, and a disabled DOM button takes no focus either — so a
  // tooltip whose trigger is one never opens, by mouse or by keyboard, in
  // exactly the state it describes.
  //
  // This one cost little while it stood: both tooltips render text
  // byte-identical to the button's own `aria-label`, so nothing was lost to a
  // screen reader, and a sighted mouse user lost the "(Ctrl+Z)" hint in a
  // state where the shortcut would do nothing anyway. The shape is the
  // problem — the day anyone puts a real reason in one of these tooltips it is
  // unreachable, which is what had already happened in `map/draw-toolbar.tsx`.
  //
  // A click handler swallows activation instead, which covers Enter and Space
  // too because both produce a click.
  const refuse = (event: React.MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
  };

  const refusedClass = (available: boolean) =>
    cn("size-8", !available && "opacity-50 cursor-not-allowed");

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
            className={refusedClass(canUndo)}
            aria-disabled={!canUndo || undefined}
            onClick={(event) => {
              if (!canUndo) {
                refuse(event);
                return;
              }
              undo();
            }}
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
            className={refusedClass(canRedo)}
            aria-disabled={!canRedo || undefined}
            onClick={(event) => {
              if (!canRedo) {
                refuse(event);
                return;
              }
              redo();
            }}
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
