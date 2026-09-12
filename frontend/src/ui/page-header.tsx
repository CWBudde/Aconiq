import type * as React from "react";
import { cn } from "@/ui/lib/utils";

type HeadingTag = "h1" | "h2" | "h3" | "h4";

export interface PageHeaderProps {
  title: React.ReactNode;
  /** Muted line under the title. */
  description?: React.ReactNode;
  /** Right-aligned slot for the page's primary actions. */
  actions?: React.ReactNode;
  /** The heading level. The shell owns the `<h1>`, so pages default to `h2`. */
  as?: HeadingTag;
  className?: string;
}

/**
 * The title block at the top of a page: a heading, an optional description
 * and an optional actions slot on the right.
 */
export function PageHeader({
  title,
  description,
  actions,
  as: Tag = "h2",
  className,
}: PageHeaderProps) {
  return (
    <div
      data-slot="page-header"
      className={cn("flex items-start justify-between gap-4", className)}
    >
      <div className="min-w-0 space-y-1">
        <Tag className="text-lg font-semibold leading-tight">{title}</Tag>
        {description != null ? (
          <div className="text-sm text-muted-foreground">{description}</div>
        ) : null}
      </div>
      {actions != null ? (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      ) : null}
    </div>
  );
}

export interface SectionHeadingProps {
  children: React.ReactNode;
  /** The heading level; defaults to `h3` under a page's `h2`. */
  as?: HeadingTag;
  /** Muted line under the heading — a count, a hint, a summary. */
  description?: React.ReactNode;
  /** Right-aligned slot for a section-level action. */
  actions?: React.ReactNode;
  /**
   * `default` is the sidebar and section heading (`text-sm font-semibold`);
   * `eyebrow` is the small upper-case muted label that introduces a group.
   */
  variant?: "default" | "eyebrow";
  className?: string;
}

const headingVariant = {
  default: "text-sm font-semibold",
  eyebrow:
    "text-xs font-semibold uppercase tracking-wider text-muted-foreground",
} as const;

/**
 * A heading inside a page: a list column header, a form section, a group
 * label. Same slots as `PageHeader`, one size down.
 */
export function SectionHeading({
  children,
  as: Tag = "h3",
  description,
  actions,
  variant = "default",
  className,
}: SectionHeadingProps) {
  return (
    <div
      data-slot="section-heading"
      data-variant={variant}
      className={cn("flex items-start justify-between gap-3", className)}
    >
      <div className="min-w-0">
        <Tag className={headingVariant[variant]}>{children}</Tag>
        {description != null ? (
          <div className="text-xs text-muted-foreground">{description}</div>
        ) : null}
      </div>
      {actions != null ? (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      ) : null}
    </div>
  );
}
