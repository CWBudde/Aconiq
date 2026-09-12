import * as React from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "@/ui/lib/utils";

export interface MasterDetailProps extends Omit<
  React.HTMLAttributes<HTMLDivElement>,
  "children"
> {
  /** The list column's body; usually an `ItemList` or an `EmptyState`. */
  list: React.ReactNode;
  /** Sticky top of the list column: a heading, a filter bar. */
  header?: React.ReactNode;
  /** Tailwind width class of the list column. */
  listWidth?: string;
  /** Accessible name of the list column. */
  listLabel?: string;
  /** The detail pane. */
  children: React.ReactNode;
}

/**
 * The two-pane layout of the runs, results and exports pages: a fixed-width
 * list column with its own scroll beside a detail pane that scrolls on its
 * own. Both panes stay inside the shell's height; nothing scrolls the page.
 */
export function MasterDetail({
  list,
  header,
  listWidth = "w-72",
  listLabel,
  children,
  className,
  ...props
}: MasterDetailProps) {
  return (
    <div
      data-slot="master-detail"
      className={cn("flex min-h-0 flex-1 overflow-hidden", className)}
      {...props}
    >
      <aside
        aria-label={listLabel}
        data-slot="master-detail-list"
        className={cn(
          "flex shrink-0 flex-col overflow-hidden border-r",
          listWidth,
        )}
      >
        {header}
        <div className="min-h-0 flex-1 overflow-y-auto">{list}</div>
      </aside>
      <div
        data-slot="master-detail-detail"
        className="flex min-w-0 flex-1 flex-col overflow-y-auto"
      >
        {children}
      </div>
    </div>
  );
}

/**
 * The `<ul>` that holds `ListItem`s. `list-none` strips the list semantics
 * in some browsers, so the role is restated.
 */
export function ItemList({
  className,
  ...props
}: React.HTMLAttributes<HTMLUListElement>) {
  return (
    <ul
      role="list"
      data-slot="item-list"
      className={cn("m-0 list-none p-0", className)}
      {...props}
    />
  );
}

export interface ListItemProps {
  selected: boolean;
  onSelect: () => void;
  /** Main line, truncated. */
  title: React.ReactNode;
  /** Small muted line under the title: a time, a duration, a count. */
  meta?: React.ReactNode;
  /** Leading slot of the top line: a `StatusBadge` or an icon. */
  badge?: React.ReactNode;
  /** Trailing part of the top line, in the monospace face: an id. */
  code?: React.ReactNode;
  /** Show the trailing chevron. */
  chevron?: boolean;
  className?: string;
}

/**
 * One row of an `ItemList`: a full-width button that selects the item. The
 * selected row is marked with `aria-current`, so the state is exposed and
 * not just painted.
 */
export function ListItem({
  selected,
  onSelect,
  title,
  meta,
  badge,
  code,
  chevron = true,
  className,
}: ListItemProps) {
  const hasTopLine = badge != null || code != null;
  return (
    <li data-slot="list-item">
      <button
        type="button"
        onClick={onSelect}
        aria-current={selected ? "true" : undefined}
        data-selected={selected ? "true" : undefined}
        className={cn(
          "flex w-full items-center gap-3 border-b px-4 py-3 text-left transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
          selected && "bg-accent text-accent-foreground",
          className,
        )}
      >
        <div className="min-w-0 flex-1">
          {hasTopLine ? (
            <div className="flex items-center gap-2">
              {badge}
              {code != null ? (
                <span className="truncate font-mono text-xs text-muted-foreground">
                  {code}
                </span>
              ) : null}
            </div>
          ) : null}
          <p className={cn("truncate text-sm", hasTopLine && "mt-0.5")}>
            {title}
          </p>
          {meta != null ? (
            <p className="truncate text-xs text-muted-foreground">{meta}</p>
          ) : null}
        </div>
        {chevron ? (
          <ChevronRight
            aria-hidden="true"
            className="size-4 shrink-0 text-muted-foreground"
          />
        ) : null}
      </button>
    </li>
  );
}
