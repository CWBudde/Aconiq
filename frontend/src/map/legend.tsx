import type { ColorStop } from "./color-ramp";
import { NOISE_LEVEL_RAMP } from "./color-ramp";
import { m } from "@/i18n/messages";

interface LegendProps {
  title?: string;
  ramp?: ColorStop[];
  unit?: string;
}

export function Legend({ title, ramp = NOISE_LEVEL_RAMP, unit }: LegendProps) {
  const resolvedTitle = title ?? m.legend_title_noise_level();
  const resolvedUnit = unit ?? m.legend_unit_db();
  return (
    <div className="absolute bottom-8 right-2 z-10 rounded-md border bg-background/90 p-2 shadow-sm backdrop-blur-sm">
      <div className="mb-1 text-xs font-medium text-muted-foreground">
        {resolvedTitle} ({resolvedUnit})
      </div>
      <div className="grid gap-px">
        {ramp.map((stop) => (
          <div key={stop.value} className="flex items-center gap-1.5">
            <div
              className="h-3 w-5 rounded-sm"
              style={{ backgroundColor: stop.color }}
            />
            <span className="text-[10px] font-mono tabular-nums">
              {stop.label}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
