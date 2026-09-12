import { Badge } from "@/ui/components/badge";
import { cn } from "@/ui/lib/utils";
import { statusConfig, type RunStatus } from "@/ui/run-status";

export type { RunStatus } from "@/ui/run-status";

/**
 * The state of a run as a soft badge: an icon plus the word, so the state
 * survives greyscale and colour-blind vision. The running icon spins.
 */
export function StatusBadge({
  status,
  className,
}: {
  status: RunStatus;
  className?: string;
}) {
  const cfg = statusConfig[status];
  const Icon = cfg.icon;
  return (
    <Badge
      variant={cfg.variant}
      data-status={status}
      className={cn("shrink-0", className)}
    >
      <Icon
        aria-hidden="true"
        className={status === "running" ? "animate-spin" : undefined}
      />
      {cfg.label()}
    </Badge>
  );
}
