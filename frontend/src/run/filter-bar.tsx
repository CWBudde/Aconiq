import { useMemo } from "react";
import { Button } from "@/ui/components/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import type { RunSummary } from "@/api/client";
import { statusLabel } from "@/ui/run-status";
import { useStandardLabel } from "@/run/use-standard-label";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Filter bar
// ---------------------------------------------------------------------------

export interface RunFilters {
  status: string;
  standardId: string;
  scenarioId: string;
}

export function RunFilterBar({
  runs,
  filters,
  onChange,
}: {
  runs: RunSummary[];
  filters: RunFilters;
  onChange: (f: RunFilters) => void;
}) {
  const standardLabel = useStandardLabel();
  const statuses = useMemo(
    () => Array.from(new Set(runs.map((r) => r.status))).sort(),
    [runs],
  );
  // Sorted by label, not by id: the list reads as it is sorted, and
  // "CNOSSOS-EU Straße" under `c` next to "BUB Straße" under `b` is an order
  // nobody can see.
  const standards = useMemo(
    () =>
      Array.from(new Set(runs.map((r) => r.standard_id))).sort((a, b) =>
        standardLabel(a).localeCompare(standardLabel(b)),
      ),
    [runs, standardLabel],
  );
  const scenarios = useMemo(
    () => Array.from(new Set(runs.map((r) => r.scenario_id))).sort(),
    [runs],
  );

  const hasFilter =
    filters.status !== "" ||
    filters.standardId !== "" ||
    filters.scenarioId !== "";

  return (
    <div className="flex flex-wrap items-center gap-2 border-b px-4 py-2">
      <Select
        value={filters.status || "_all"}
        onValueChange={(v) => {
          onChange({ ...filters, status: v === "_all" ? "" : v });
        }}
      >
        <SelectTrigger
          className="h-7 w-32 text-xs"
          aria-label={m.label_status_field()}
        >
          <SelectValue placeholder={m.label_status_field()} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="_all">{m.label_status_filter()}</SelectItem>
          {statuses.map((s) => (
            <SelectItem key={s} value={s}>
              {statusLabel(s)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        value={filters.standardId || "_all"}
        onValueChange={(v) => {
          onChange({ ...filters, standardId: v === "_all" ? "" : v });
        }}
      >
        <SelectTrigger
          className="h-7 w-36 text-xs"
          aria-label={m.label_standard_select()}
        >
          <SelectValue placeholder={m.label_standard_select()} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="_all">{m.label_standard_filter()}</SelectItem>
          {standards.map((s) => (
            <SelectItem key={s} value={s}>
              {standardLabel(s)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {scenarios.length > 1 ? (
        <Select
          value={filters.scenarioId || "_all"}
          onValueChange={(v) => {
            onChange({ ...filters, scenarioId: v === "_all" ? "" : v });
          }}
        >
          <SelectTrigger
            className="h-7 w-32 text-xs"
            aria-label={m.label_scenarios_field()}
          >
            <SelectValue placeholder={m.label_scenarios_field()} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="_all">{m.label_scenario_filter()}</SelectItem>
            {scenarios.map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}

      {hasFilter ? (
        <Button
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs"
          onClick={() => {
            onChange({ status: "", standardId: "", scenarioId: "" });
          }}
        >
          {m.action_clear_filters()}
        </Button>
      ) : null}
    </div>
  );
}
