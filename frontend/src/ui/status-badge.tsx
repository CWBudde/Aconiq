import {
  CheckCircle2,
  Clock,
  Loader2,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import type { RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";
import { Badge, type BadgeProps } from "@/ui/components/badge";
import { cn } from "@/ui/lib/utils";

/** The lifecycle state of a run, as the API reports it. */
export type RunStatus = RunSummary["status"];

// Labels are held as functions, not resolved strings: calling a message at
// module scope freezes it to the locale active at import time.
const statusConfig: Record<
  RunStatus,
  {
    label: () => string;
    icon: LucideIcon;
    variant: NonNullable<BadgeProps["variant"]>;
  }
> = {
  pending: { label: m.status_badge_pending, icon: Clock, variant: "secondary" },
  running: { label: m.status_badge_running, icon: Loader2, variant: "info" },
  completed: {
    label: m.status_badge_completed,
    icon: CheckCircle2,
    variant: "success",
  },
  failed: {
    label: m.status_badge_failed,
    icon: XCircle,
    variant: "destructive",
  },
};

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
