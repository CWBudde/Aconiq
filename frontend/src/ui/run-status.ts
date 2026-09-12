import {
  CheckCircle2,
  Clock,
  Loader2,
  XCircle,
  type LucideIcon,
} from "lucide-react";
import type { RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";
import type { BadgeProps } from "@/ui/components/badge";

/** The lifecycle state of a run, as the API reports it. */
export type RunStatus = RunSummary["status"];

// Labels are held as functions, not resolved strings: calling a message at
// module scope freezes it to the locale active at import time.
export const statusConfig: Record<
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

/** The localised word for a run state, for filters and lists without a badge. */
export function statusLabel(status: RunStatus): string {
  return statusConfig[status].label();
}
