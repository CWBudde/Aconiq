import { CheckCircle2, Loader2 } from "lucide-react";
import type { RunStatus } from "@/ui/status-badge";
import { parseTimeline, stepStateLabel } from "@/run/parse-timeline";

export function ProgressTimeline({
  lines,
  status,
}: {
  lines: string[];
  status: RunStatus;
}) {
  const steps = parseTimeline(lines, status);

  // An ordered list, because the steps are a fixed sequence and a reader
  // arriving mid-run needs to know how many there are and where they are in
  // them. The marker is the only thing that distinguishes a finished step from
  // a pending one, so it carries the state as its own accessible name rather
  // than as a colour and a shape: the tick, the spinner and the dot stay
  // decoration behind it. `aria-current="step"` marks where the run is now.
  return (
    <ol className="space-y-1">
      {steps.map((step, i) => (
        <li
          key={i}
          className="flex items-start gap-2.5"
          {...(step.active && { "aria-current": "step" as const })}
        >
          <div className="flex flex-col items-center">
            <div
              role="img"
              aria-label={stepStateLabel(step)}
              className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-xs ${
                step.done
                  ? "border-success bg-success text-success-foreground"
                  : step.active
                    ? "border-info bg-info text-info-foreground"
                    : "border-border bg-muted text-muted-foreground"
              }`}
            >
              {step.done ? (
                <CheckCircle2 aria-hidden="true" className="h-3 w-3" />
              ) : step.active ? (
                <Loader2 aria-hidden="true" className="h-3 w-3 animate-spin" />
              ) : (
                <span className="h-1.5 w-1.5 rounded-full bg-current" />
              )}
            </div>
            {i < steps.length - 1 ? (
              <div
                aria-hidden="true"
                className={`mt-0.5 w-px flex-1 ${step.done ? "bg-success/40" : "bg-border"}`}
                style={{ minHeight: "12px" }}
              />
            ) : null}
          </div>
          <div className="pb-2 pt-0.5">
            <p
              className={`text-sm ${step.done || step.active ? "font-medium" : "text-muted-foreground"}`}
            >
              {step.label}
            </p>
            {step.timestamp ? (
              <p className="text-xs text-muted-foreground">{step.timestamp}</p>
            ) : null}
          </div>
        </li>
      ))}
    </ol>
  );
}
