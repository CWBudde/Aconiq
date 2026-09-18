import { useMemo, useRef, useState } from "react";
import { AlertCircle, Loader2, Play } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Callout } from "@/ui/callout";
import { EmptyState } from "@/ui/empty-state";
import { ItemList, ListItem, MasterDetail } from "@/ui/master-detail";
import { PageHeader } from "@/ui/page-header";
import { StatusBadge } from "@/ui/status-badge";
import { runTiming } from "@/ui/run-status";
import { useRuns } from "@/api/hooks";
import { useStandardLabel } from "@/run/use-standard-label";
import { RunDetail } from "@/run/detail";
import { RunFilterBar, type RunFilters } from "@/run/filter-bar";
import { RunSetupDialog } from "@/run/setup-dialog";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Run page
// ---------------------------------------------------------------------------

const EMPTY_FILTERS: RunFilters = {
  status: "",
  standardId: "",
  scenarioId: "",
};

export default function RunPage() {
  const standardLabel = useStandardLabel();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [filters, setFilters] = useState<RunFilters>(EMPTY_FILTERS);
  const newRunRef = useRef<HTMLButtonElement>(null);

  // Fetch runs; `useRuns` polls by activity so CLI-launched runs show up too.
  const { data: runs = [], isLoading, error } = useRuns();

  const hasRunning = runs.some((r) => r.status === "running");

  // Client-side filtering.
  const filteredRuns = useMemo(() => {
    return runs.filter((r) => {
      if (filters.status && r.status !== filters.status) return false;
      if (filters.standardId && r.standard_id !== filters.standardId)
        return false;
      if (filters.scenarioId && r.scenario_id !== filters.scenarioId)
        return false;
      return true;
    });
  }, [runs, filters]);

  // Derived, not stored: when the stored id is filtered out, the first visible
  // run stands in. Doing that with a render-phase `setSelectedRunId` (as this
  // used to) made every filter change cost an extra render pass.
  const selectedRun = useMemo(
    () =>
      filteredRuns.find((r) => r.id === selectedRunId) ??
      filteredRuns[0] ??
      null,
    [filteredRuns, selectedRunId],
  );

  const hasRuns = runs.length > 0;

  // Confirming a delete unmounts the detail pane, including the button Radix
  // would have restored focus to — which drops a keyboard user on `<body>`,
  // at the top of the document. Route-level axe cannot see that.
  function handleRunDeleted() {
    setSelectedRunId(null);
    newRunRef.current?.focus();
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Toolbar */}
      <PageHeader
        className="border-b px-5 py-3"
        title={m.page_title_runs()}
        description={
          hasRunning ? (
            <span className="inline-flex items-center gap-1 text-xs text-info">
              <Loader2 aria-hidden="true" className="h-3 w-3 animate-spin" />
              {m.msg_run_in_progress()}
            </span>
          ) : (
            <span className="text-xs">
              {m.msg_run_count({ count: runs.length })}
            </span>
          )
        }
        actions={
          <Button
            ref={newRunRef}
            size="sm"
            onClick={() => {
              setDialogOpen(true);
            }}
          >
            <Play aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
            {m.action_new_run()}
          </Button>
        }
      />

      {/* Body */}
      {isLoading ? (
        <div className="flex flex-1 items-center justify-center">
          <Loader2
            aria-hidden="true"
            className="h-6 w-6 animate-spin text-muted-foreground"
          />
        </div>
      ) : error ? (
        <div className="flex flex-1 items-start justify-center p-8">
          <Callout variant="destructive" icon={AlertCircle}>
            {m.msg_api_error_run()}
          </Callout>
        </div>
      ) : !hasRuns ? (
        <EmptyState icon={Play} title={m.msg_no_runs_empty_state()} />
      ) : (
        <MasterDetail
          listLabel={m.page_title_runs()}
          header={
            <RunFilterBar runs={runs} filters={filters} onChange={setFilters} />
          }
          list={
            filteredRuns.length === 0 ? (
              <EmptyState compact title={m.msg_no_runs_match_filters()} />
            ) : (
              <ItemList>
                {filteredRuns.map((run) => (
                  <ListItem
                    key={run.id}
                    selected={run.id === selectedRun?.id}
                    onSelect={() => {
                      setSelectedRunId(run.id);
                    }}
                    badge={<StatusBadge status={run.status} />}
                    code={run.id}
                    title={`${standardLabel(run.standard_id)}${run.version ? ` / ${run.version}` : ""}`}
                    meta={runTiming(run)}
                  />
                ))}
              </ItemList>
            )
          }
        >
          {selectedRun ? (
            /* Keyed by run id so selecting another run gives the pane a fresh
               `useDeleteRun`. Without it the pane is reused, and a deletion
               that failed on run A keeps its error state and renders that
               failure under run B's heading. */
            <RunDetail
              key={selectedRun.id}
              run={selectedRun}
              onRetry={() => {
                setDialogOpen(true);
              }}
              onDeleted={handleRunDeleted}
            />
          ) : (
            <EmptyState title={m.msg_select_run_details()} />
          )}
        </MasterDetail>
      )}

      <RunSetupDialog
        open={dialogOpen}
        onClose={() => {
          setDialogOpen(false);
        }}
        onCreated={(runId) => {
          setSelectedRunId(runId);
        }}
      />
    </div>
  );
}
