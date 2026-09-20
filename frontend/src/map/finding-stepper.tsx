import { Check, ChevronLeft, ChevronRight, X } from "lucide-react";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";
import { MapPanel } from "./map-panel";

export interface FindingStepperProps {
  /** 1-based, for the reader. */
  position: number;
  total: number;
  onStep: (direction: 1 | -1) => void;
  onClose: () => void;
  /**
   * Signs the feature the reader is standing on off and steps to the next one,
   * or `null` when the current finding is not one a sign-off can retire.
   *
   * `null` rather than a disabled button: most findings are defects to be
   * fixed, and offering a greyed-out "accept this" against a self-intersecting
   * line would suggest the app can wave it through.
   */
  onSignOff: (() => void) | null;
}

/**
 * Walks the reader through the model's findings one feature at a time.
 *
 * It is its own panel rather than a strip inside the docked editor, because the
 * queue is a property of the workspace and not of the feature that happens to
 * be open: a stepper in the editor would disappear on close, which is exactly
 * when a reader working through 608 imported sources wants to keep going.
 *
 * `bottom-center` is the slot that was free — the four corners are taken by the
 * toolbar, the layer control, the editor and the undo bar — and `bottom-14`
 * stacks it above the coordinate readout.
 *
 * Presentational: the cursor lives in `pages/map.tsx`, next to the selection it
 * moves.
 */
export function FindingStepper({
  position,
  total,
  onStep,
  onClose,
  onSignOff,
}: FindingStepperProps) {
  return (
    <MapPanel
      position="bottom-center"
      inset="bottom-14"
      className="flex items-center gap-1 py-1 pl-1 pr-1"
      role="status"
      aria-label={m.label_findings()}
    >
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label={m.action_previous_finding()}
        aria-keyshortcuts="Alt+ArrowUp"
        onClick={() => {
          onStep(-1);
        }}
      >
        <ChevronLeft aria-hidden="true" />
      </Button>
      <span className="px-1 text-xs tabular-nums">
        {m.label_finding_position({ position, total })}
      </span>
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label={m.action_next_finding()}
        aria-keyshortcuts="Alt+ArrowDown"
        onClick={() => {
          onStep(1);
        }}
      >
        <ChevronRight aria-hidden="true" />
      </Button>
      {onSignOff !== null ? (
        // The whole point of the walk over an OSM import: look at the road the
        // camera just flew to, accept its guessed acoustics, and move on in one
        // press rather than three.
        <Button
          variant="outline"
          size="sm"
          className="ml-1 h-6 px-2 text-2xs"
          onClick={onSignOff}
        >
          <Check aria-hidden="true" className="mr-1 size-3" />
          {m.action_mark_reviewed_and_next()}
        </Button>
      ) : null}
      <Button
        variant="ghost"
        size="icon"
        className="size-6"
        aria-label={m.action_stop_stepping()}
        onClick={onClose}
      >
        <X aria-hidden="true" />
      </Button>
    </MapPanel>
  );
}
