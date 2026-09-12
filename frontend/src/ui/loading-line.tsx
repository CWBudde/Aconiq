import { Loader2 } from "lucide-react";
import { cn } from "@/ui/lib/utils";
import { m } from "@/i18n/messages";

export interface LoadingLineProps {
  /**
   * What is being loaded. Defaults to the generic "Loading…".
   *
   * A default argument rather than a module-level constant: paraglide message
   * functions read the active locale when called, and a default is evaluated
   * on every render, so the line still follows a language switch.
   */
  text?: string;
  className?: string;
}

/**
 * A spinner and a line of text, for a pane that is waiting on a query.
 *
 * `EmptyState` is the centred placeholder for a pane with nothing to show;
 * this is the inline one for a pane that expects something shortly. The
 * spinner is `aria-hidden` and the text carries the meaning, so a reader is
 * told what is happening rather than that an icon exists.
 */
export function LoadingLine({
  text = m.status_loading(),
  className,
}: LoadingLineProps) {
  return (
    <div
      data-slot="loading-line"
      className={cn(
        "flex items-center gap-2 text-sm text-muted-foreground",
        className,
      )}
    >
      <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
      {text}
    </div>
  );
}
