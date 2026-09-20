import * as React from "react";

import { Input } from "@/ui/components/input";
import { cn } from "@/ui/lib/utils";

/**
 * A number field that carries its unit inside its own border.
 *
 * The unit used to be appended to the label — `Pkw (1/h)` — which reads as part
 * of what the field is *called* rather than as the dimension of what it
 * *holds*. Two fields named `Pkw`, one counting vehicles and one holding a
 * speed, are told apart by the affix beside the value, not by a parenthetical
 * in front of it.
 *
 * Three details are load-bearing:
 *
 * **The trailing padding is computed from the unit's length.** The backend's
 * vocabulary runs from `m` to `dB/km` (`framework.go`), so one fixed utility
 * class either clips the long symbol or strands the short one. `ch` is the
 * right unit because the affix is text rendered in the field's own font.
 *
 * **`end-6` puts the affix before the native number spinner**, which is where
 * the eye reads it as belonging to the value. Appending it in flex order would
 * land it to the right of the spinner, where it reads as a third control. The
 * reserve is 1.5rem and not more: the spinner is about 15px wide and Chrome
 * only paints it on hover, and every pixel reserved here comes off a field that
 * is 118px wide in the map's side panel.
 *
 * **The affix is named, not decorative.** It carries `unitId` and the caller
 * lists that id in the input's `aria-describedby`, so a screen reader says the
 * unit once. `aria-hidden` plus an `sr-only` twin would be the alternative and
 * costs a second copy of the same string.
 */
export interface UnitInputProps extends React.ComponentProps<"input"> {
  /** The unit symbol as the descriptor publishes it; absent when dimensionless. */
  unit?: string | undefined;
  /**
   * The id given to the affix. Required with `unit`, because the caller owns
   * `aria-describedby` — it usually has a description or an error note to name
   * alongside this one.
   */
  unitId?: string | undefined;
}

const UnitInput = React.forwardRef<HTMLInputElement, UnitInputProps>(
  ({ unit, unitId, className, style, ...props }, ref) => {
    if (unit === undefined || unit === "") {
      return <Input ref={ref} className={className} style={style} {...props} />;
    }

    return (
      <div className="relative">
        <Input
          ref={ref}
          className={className}
          style={{
            ...style,
            paddingInlineEnd: `calc(1.5rem + ${String(unit.length)}ch)`,
          }}
          {...props}
        />
        <span
          id={unitId}
          className={cn(
            "pointer-events-none absolute inset-y-0 end-6 flex items-center",
            "text-xs text-muted-foreground",
          )}
        >
          {unit}
        </span>
      </div>
    );
  },
);
UnitInput.displayName = "UnitInput";

export { UnitInput };
