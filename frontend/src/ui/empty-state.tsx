import type * as React from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/ui/lib/utils";

export interface EmptyStateProps {
  icon?: LucideIcon;
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Actions rendered in a row under the text. */
  children?: React.ReactNode;
  /** Smaller type and padding for a list column or a panel. */
  compact?: boolean;
  className?: string;
}

/**
 * The centred placeholder for a pane with nothing to show yet: an optional
 * icon, a title, an explanation and the actions that fill the pane.
 */
export function EmptyState({
  icon: Icon,
  title,
  description,
  children,
  compact = false,
  className,
}: EmptyStateProps) {
  return (
    <div
      data-slot="empty-state"
      className={cn(
        "flex flex-1 flex-col items-center justify-center text-center",
        compact ? "p-4" : "p-8",
        className,
      )}
    >
      {Icon ? (
        <Icon
          aria-hidden="true"
          className={cn(
            "mb-2 text-muted-foreground",
            compact ? "size-6" : "size-8",
          )}
        />
      ) : null}
      <p className={cn("font-medium", compact ? "text-sm" : "text-base")}>
        {title}
      </p>
      {description != null ? (
        <p
          className={cn(
            "mt-1 max-w-prose text-muted-foreground",
            compact ? "text-xs" : "text-sm",
          )}
        >
          {description}
        </p>
      ) : null}
      {children != null ? (
        <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
          {children}
        </div>
      ) : null}
    </div>
  );
}
