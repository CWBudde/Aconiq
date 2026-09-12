import type * as React from "react";
import { cn } from "@/ui/lib/utils";

export interface KeyValueItem {
  /** The term. Also the React key, so labels within one list must differ. */
  label: string;
  value: React.ReactNode;
  /** Render the value in the monospace face (ids, paths, CRS codes). */
  mono?: boolean;
}

export interface KeyValueListProps {
  items: readonly KeyValueItem[];
  /** Tighter rows at `text-xs` for a sidebar or a panel. */
  dense?: boolean;
  className?: string;
}

/**
 * A two-column definition list: muted terms on the left, values on the
 * right, the columns sized to the longest term.
 */
export function KeyValueList({
  items,
  dense = false,
  className,
}: KeyValueListProps) {
  return (
    <dl
      data-slot="key-value-list"
      className={cn(
        "grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm",
        dense && "gap-x-3 gap-y-0.5 text-xs",
        className,
      )}
    >
      {items.map((item) => (
        // A fragment keyed per item keeps `dt` and `dd` direct children of
        // the `dl`, which the grid and the list semantics both need.
        <KeyValueRow key={item.label} item={item} />
      ))}
    </dl>
  );
}

function KeyValueRow({ item }: { item: KeyValueItem }) {
  return (
    <>
      <dt className="text-muted-foreground">{item.label}</dt>
      <dd className={cn("min-w-0 break-words", item.mono && "font-mono")}>
        {item.value}
      </dd>
    </>
  );
}
