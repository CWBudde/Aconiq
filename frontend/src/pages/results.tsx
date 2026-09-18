import { useState } from "react";
import { Link, useParams } from "react-router";
import {
  BarChart3,
  Table2,
  GitCompare,
  Loader2,
  AlertCircle,
  AlertTriangle,
  Info,
} from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/ui/components/tabs";
import { Callout } from "@/ui/callout";
import { CopyField } from "@/ui/copy-field";
import { EmptyState } from "@/ui/empty-state";
import { formatDurationBetween, formatTime } from "@/ui/format";
import { LoadingLine } from "@/ui/loading-line";
import { ItemList, ListItem, MasterDetail } from "@/ui/master-detail";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { StatusBadge } from "@/ui/status-badge";
import { runTiming } from "@/ui/run-status";
import { useRasterMetadata } from "@/api/hooks";
import { useStandardLabel } from "@/run/use-standard-label";
import { useRunFromRoute } from "@/run/use-run-from-route";
import { exportCommand } from "@/api/cli";
import type { ArtifactRef, RunSummary } from "@/api/client";
import { ReceiversTab } from "@/results/receiver-table";
import { m } from "@/i18n/messages";
import { unitFor } from "@/map/result-units";

// ---------------------------------------------------------------------------
// Raster tab
// ---------------------------------------------------------------------------

function RasterArtifactCard({
  artifact,
  runId,
}: {
  artifact: ArtifactRef;
  runId: string;
}) {
  const { data, isLoading, error } = useRasterMetadata(artifact.id);

  if (isLoading) {
    return <LoadingLine text={m.status_loading_raster_metadata()} />;
  }

  if (error || !data) {
    return (
      <Callout variant="destructive" icon={AlertCircle}>
        {m.error_load_raster_metadata()}
      </Callout>
    );
  }

  return (
    <Card className="p-4">
      <p className="mb-3 font-mono text-xs font-semibold text-muted-foreground">
        {artifact.path.split("/").pop()}
      </p>

      {/* Metadata */}
      <div className="mb-4 grid grid-cols-2 gap-x-6 gap-y-1 text-xs sm:grid-cols-3">
        <div>
          <span className="text-muted-foreground">{m.label_dimensions()}:</span>{" "}
          <span className="font-medium">
            {String(data.width)} × {String(data.height)}
          </span>
        </div>
        <div>
          <span className="text-muted-foreground">{m.label_bands()}:</span>{" "}
          <span className="font-medium">{String(data.bands)}</span>
        </div>
        <div>
          <span className="text-muted-foreground">{m.label_nodata()}:</span>{" "}
          <span className="font-medium">{String(data.nodata)}</span>
        </div>
        {/* No standalone Unit row: the unit is per band, so one value here
            would either repeat itself four times over or summarise away a
            disagreement. The bands below carry their own, as the generated
            report's map table does. */}
        {data.band_names && data.band_names.length > 0 ? (
          <div className="col-span-2">
            {/* Not `label_bands`: that one heads the band *count* two rows up,
                and one label over two different facts reads as a repeat. */}
            <span className="text-muted-foreground">
              {m.label_band_names()}:
            </span>{" "}
            <span className="font-mono font-medium">
              {data.band_names
                .map((band) => {
                  const unit = unitFor(data.units, band);
                  return unit === "" ? band : `${band} (${unit})`;
                })
                .join(", ")}
            </span>
          </div>
        ) : null}
      </div>

      {/* The bands themselves. `run.result.raster_metadata` is a sidecar;
          the grid it describes is a binary blob no API route serves, so the
          way to the pixels is an export bundle. */}
      <div className="space-y-1.5">
        <p className="text-xs text-muted-foreground">
          {m.msg_raster_export_hint()}
        </p>
        <CopyField
          label={m.label_command()}
          value={exportCommand(runId, { formats: ["geotiff"] })}
        />
      </div>
    </Card>
  );
}

function RasterTab({ run }: { run: RunSummary }) {
  const rasterArtifacts = run.artifacts.filter(
    (a) => a.kind === "run.result.raster_metadata",
  );

  if (rasterArtifacts.length === 0) {
    return (
      <Callout variant="neutral" icon={Info}>
        {m.msg_no_raster_artifacts()}
      </Callout>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {rasterArtifacts.map((a) => (
        <RasterArtifactCard key={a.id} artifact={a} runId={run.id} />
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Compare tab
// ---------------------------------------------------------------------------

function RunColumn({ run, label }: { run: RunSummary; label: string }) {
  const standardLabel = useStandardLabel();

  return (
    <Card className="flex-1 p-4">
      <SectionHeading variant="eyebrow" className="mb-1">
        {label}
      </SectionHeading>
      <p className="font-mono text-xs">{run.id}</p>
      <div className="mt-2 space-y-0.5 text-xs">
        <p>
          <span className="text-muted-foreground">{m.label_standard()}:</span>{" "}
          <span>{standardLabel(run.standard_id)}</span>
        </p>
        <p>
          <span className="text-muted-foreground">{m.label_version()}:</span>{" "}
          <span className="font-mono">{run.version}</span>
        </p>
        {run.profile ? (
          <p>
            <span className="text-muted-foreground">{m.label_profile()}:</span>{" "}
            <span className="font-mono">{run.profile}</span>
          </p>
        ) : null}
        <p>
          <span className="text-muted-foreground">{m.label_started()}:</span>{" "}
          {formatTime(run.started_at)}
        </p>
        <p>
          <span className="text-muted-foreground">{m.label_duration()}:</span>{" "}
          {formatDurationBetween(run.started_at, run.finished_at)}
        </p>
        <p>
          <span className="text-muted-foreground">{m.label_artifacts()}:</span>{" "}
          {String(run.artifacts.length)}
        </p>
      </div>
    </Card>
  );
}

function CompareTab({
  run,
  allCompletedRuns,
}: {
  run: RunSummary;
  allCompletedRuns: RunSummary[];
}) {
  const standardLabel = useStandardLabel();
  const [compareRunId, setCompareRunId] = useState<string>("");
  const otherRuns = allCompletedRuns.filter((r) => r.id !== run.id);
  const compareRun = otherRuns.find((r) => r.id === compareRunId) ?? null;

  return (
    <div className="flex flex-col gap-4">
      {/* Run B selector */}
      <div className="flex items-center gap-3">
        <Label htmlFor="compare-run" className="text-sm text-muted-foreground">
          {m.label_compare_with()}:
        </Label>
        <Select
          value={compareRunId || "_none"}
          onValueChange={(v) => {
            setCompareRunId(v === "_none" ? "" : v);
          }}
        >
          <SelectTrigger id="compare-run" className="h-8 w-64 text-xs">
            <SelectValue placeholder={m.placeholder_compare_run()} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="_none">
              {m.option_select_compare_run()}
            </SelectItem>
            {otherRuns.map((r) => (
              <SelectItem key={r.id} value={r.id}>
                <span className="font-mono">{r.id}</span>{" "}
                <span className="text-muted-foreground">
                  ({standardLabel(r.standard_id)} / {r.version})
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {compareRun ? (
        <div className="flex gap-4">
          <RunColumn run={run} label={m.msg_run_column_selected()} />
          <RunColumn run={compareRun} label={m.msg_run_column_compare()} />
        </div>
      ) : (
        <Callout variant="neutral" icon={GitCompare}>
          {m.msg_select_run_compare()}
        </Callout>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run detail (right panel)
// ---------------------------------------------------------------------------

const RESULT_TABS = ["receivers", "raster", "compare"] as const;

type ResultTab = (typeof RESULT_TABS)[number];

function isResultTab(value: string): value is ResultTab {
  return (RESULT_TABS as readonly string[]).includes(value);
}

function RunResultDetail({
  run,
  allCompletedRuns,
}: {
  run: RunSummary;
  allCompletedRuns: RunSummary[];
}) {
  const standardLabel = useStandardLabel();
  const [tab, setTab] = useState<ResultTab>("receivers");

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Run header */}
      <div className="border-b px-5 py-3">
        <div className="flex items-center gap-2">
          <StatusBadge status={run.status} />
          <span className="font-mono text-xs text-muted-foreground">
            {run.id}
          </span>
        </div>
        <p className="mt-0.5 text-sm">
          <span>{standardLabel(run.standard_id)}</span>
          {run.version ? (
            <>
              {" / "}
              <span className="font-mono">{run.version}</span>
            </>
          ) : null}
          {run.profile ? (
            <>
              {" / "}
              <span className="font-mono">{run.profile}</span>
            </>
          ) : null}
        </p>
        <p className="text-xs text-muted-foreground">
          {m.label_started()}: {runTiming(run)}
        </p>
      </div>

      <Tabs
        value={tab}
        onValueChange={(value) => {
          if (isResultTab(value)) setTab(value);
        }}
        className="flex min-h-0 flex-1 flex-col"
      >
        <div className="border-b px-4 py-1.5">
          <TabsList className="h-8">
            <TabsTrigger value="receivers" className="text-xs">
              <Table2 aria-hidden="true" />
              {m.tab_receivers()}
            </TabsTrigger>
            <TabsTrigger value="raster" className="text-xs">
              <BarChart3 aria-hidden="true" />
              {m.tab_raster()}
            </TabsTrigger>
            <TabsTrigger value="compare" className="text-xs">
              <GitCompare aria-hidden="true" />
              {m.tab_compare()}
            </TabsTrigger>
          </TabsList>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          <TabsContent value="receivers" className="mt-0 p-5">
            <ReceiversTab run={run} />
          </TabsContent>
          <TabsContent value="raster" className="mt-0 p-5">
            <RasterTab run={run} />
          </TabsContent>
          <TabsContent value="compare" className="mt-0 p-5">
            <CompareTab run={run} allCompletedRuns={allCompletedRuns} />
          </TabsContent>
        </div>
      </Tabs>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Results page
// ---------------------------------------------------------------------------

// Module scope, not an inline arrow: `useRunFromRoute` memoises the filtered
// list on this identity, and a fresh closure each render would rebuild it
// every time.
function isCompleted(run: RunSummary): boolean {
  return run.status === "completed";
}

export default function ResultsPage() {
  const standardLabel = useStandardLabel();
  const { runId } = useParams();
  const {
    eligibleRuns: completedRuns,
    run: selectedRun,
    state,
    isLoading,
    error,
  } = useRunFromRoute(runId, isCompleted);

  // The heading stays above both transient states so every state of the page
  // keeps its landmark structure (`waitForPage` in e2e/app.ts needs it).
  if (isLoading || error) {
    return (
      <div className="flex flex-1 flex-col">
        <PageHeader
          className="border-b px-4 py-3"
          title={m.page_title_results()}
        />
        {error ? (
          <div className="flex flex-1 items-start justify-center p-8">
            <Callout variant="destructive" icon={AlertCircle}>
              {m.msg_api_error_results()}
            </Callout>
          </div>
        ) : (
          <div className="flex flex-1 items-center justify-center">
            <Loader2
              aria-hidden="true"
              className="h-6 w-6 animate-spin text-muted-foreground"
            />
          </div>
        )}
      </div>
    );
  }

  return (
    <MasterDetail
      listLabel={m.page_title_results()}
      header={
        <PageHeader
          className="border-b px-4 py-3"
          title={m.page_title_results()}
          description={
            <span className="text-xs">
              {String(completedRuns.length)}{" "}
              {completedRuns.length === 1
                ? m.msg_completed_runs()
                : m.msg_completed_runs_plural()}
            </span>
          }
        />
      }
      list={
        completedRuns.length === 0 ? (
          <EmptyState
            compact
            icon={BarChart3}
            title={m.msg_no_completed_runs()}
          >
            <Button asChild size="sm">
              <Link to="/run">{m.nav_run()}</Link>
            </Button>
          </EmptyState>
        ) : (
          <ItemList>
            {completedRuns.map((run) => (
              <ListItem
                key={run.id}
                selected={run.id === runId}
                to={`/results/${run.id}`}
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
        <RunResultDetail run={selectedRun} allCompletedRuns={completedRuns} />
      ) : runId != null && (state === "ineligible" || state === "unknown") ? (
        <div className="flex flex-1 items-start justify-center p-8">
          <Callout variant="warning" icon={AlertTriangle}>
            {state === "ineligible"
              ? m.msg_run_not_completed({ runId })
              : m.msg_unknown_run_id({ runId })}
          </Callout>
        </div>
      ) : (
        <EmptyState title={m.msg_select_completed_run_details()} />
      )}
    </MasterDetail>
  );
}
