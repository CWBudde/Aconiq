import type { ColorStop } from "./color-ramp";
import { NOISE_LEVEL_RAMP } from "./color-ramp";

interface LegendProps {
  title?: string;
  ramp?: ColorStop[];
  unit?: string;
}

export function Legend({
  title = "Noise Level",
  ramp = NOISE_LEVEL_RAMP,
  unit = "dB(A)",
}: LegendProps) {
  return (
    <div className="absolute bottom-8 right-2 z-10 rounded-md border bg-background/90 p-2 shadow-sm backdrop-blur-sm">
      <div className="mb-1 text-xs font-medium text-muted-foreground">
        {title} ({unit})
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
