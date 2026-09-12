import { useState, useMemo } from "react";
import {
  BarChart3,
  Table2,
  GitCompare,
  Loader2,
  AlertCircle,
  Download,
  Info,
  ChevronUp,
  ChevronDown,
  SlidersHorizontal,
  Crosshair,
} from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Input } from "@/ui/components/input";
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
import { EmptyState } from "@/ui/empty-state";
import {
  formatCoordinate,
  formatDurationBetween,
  formatLevel,
  formatNumber,
  formatTime,
} from "@/ui/format";
import { ItemList, ListItem, MasterDetail } from "@/ui/master-detail";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { StatusBadge } from "@/ui/status-badge";
import { useRuns, useReceiverTable, useRasterMetadata } from "@/api/hooks";
import type { ArtifactRef, RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** "13:05:07 · 12 sec": when the run started and how long it took. */
function runTiming(run: RunSummary): string {
  return `${formatTime(run.started_at)} · ${formatDurationBetween(run.started_at, run.finished_at)}`;
}

function LoadingLine({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
      {text}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Receivers tab
// ---------------------------------------------------------------------------

type SortDir = "asc" | "desc";

function ReceiversTab({ run }: { run: RunSummary }) {
  const artifact = run.artifacts.find(
    (a) => a.kind === "run.result.receiver_table_json",
  );

  const { data, isLoading, error } = useReceiverTable(artifact?.id ?? null);

  const [filter, setFilter] = useState("");
  const [sortCol, setSortCol] = useState<string>("id");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const indicators = useMemo(() => data?.indicator_order ?? [], [data]);
  const unit = data?.unit ?? "";

  const summaryCards = useMemo(() => {
    if (!data) return [];
    return indicators.map((ind) => {
      const vals = data.records.map((r) => r.values[ind] ?? 0);
      if (vals.length === 0) return { ind, min: 0, max: 0, mean: 0 };
      const min = Math.min(...vals);
      const max = Math.max(...vals);
      const mean = vals.reduce((a, b) => a + b, 0) / vals.length;
      return { ind, min, max, mean };
    });
  }, [data, indicators]);

  const filteredRecords = useMemo(() => {
    if (!data) return [];
    const q = filter.toLowerCase();
    return data.records.filter((r) => r.id.toLowerCase().includes(q));
  }, [data, filter]);

  const sortedRecords = useMemo(() => {
    const copy = [...filteredRecords];
    copy.sort((a, b) => {
      let av: string | number;
      let bv: string | number;
      if (sortCol === "id") {
        av = a.id;
        bv = b.id;
      } else if (sortCol === "x") {
        av = a.x;
        bv = b.x;
      } else if (sortCol === "y") {
        av = a.y;
        bv = b.y;
      } else if (sortCol === "height_m") {
        av = a.height_m;
        bv = b.height_m;
      } else {
        av = a.values[sortCol] ?? 0;
        bv = b.values[sortCol] ?? 0;
      }
      if (typeof av === "string" && typeof bv === "string") {
        return sortDir === "asc" ? av.localeCompare(bv) : bv.localeCompare(av);
      }
      const an = av as number;
      const bn = bv as number;
      return sortDir === "asc" ? an - bn : bn - an;
    });
    return copy;
  }, [filteredRecords, sortCol, sortDir]);

  function toggleSort(col: string) {
    if (sortCol === col) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortCol(col);
      setSortDir("asc");
    }
  }

  function SortIcon({ col }: { col: string }) {
    if (sortCol !== col)
      return <ChevronUp aria-hidden="true" className="h-3 w-3 opacity-30" />;
    return sortDir === "asc" ? (
      <ChevronUp aria-hidden="true" className="h-3 w-3" />
    ) : (
      <ChevronDown aria-hidden="true" className="h-3 w-3" />
    );
  }

  // The level formatter wants a unit; a table that names none falls back to
  // the bare number rather than printing a dangling space.
  function level(value: number): string {
    return unit === "" ? formatNumber(value) : formatLevel(value, unit);
  }

  // Raw values, not the locale-formatted ones: the CSV is for other tools.
  function downloadCSV() {
    if (!data) return;
    const headers = ["id", "x", "y", "height_m", ...indicators];
    const rows = sortedRecords.map((r) => [
      r.id,
      String(r.x),
      String(r.y),
      String(r.height_m),
      ...indicators.map((ind) => String(r.values[ind] ?? "")),
    ]);
    const csv = [headers, ...rows]
      .map((row) => row.map((c) => `"${c}"`).join(","))
      .join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "receivers.csv";
    a.click();
    URL.revokeObjectURL(url);
  }

  if (!artifact) {
    return (
      <Callout variant="neutral" icon={Info}>
        {m.msg_no_raster_artifacts()}
      </Callout>
    );
  }

  if (isLoading) {
    return <LoadingLine text={m.status_loading_receiver_table()} />;
  }

  if (error || !data) {
    return (
      <Callout variant="destructive" icon={AlertCircle}>
        {m.error_load_receiver_table()}
      </Callout>
    );
  }

  const columns = ["id", "x", "y", "height_m", ...indicators];

  function columnLabel(col: string): string {
    if (col === "height_m") return m.table_header_height_m();
    if (indicators.includes(col) && unit !== "") return `${col} (${unit})`;
    return col;
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Indicator summary cards */}
      {summaryCards.length > 0 ? (
        <div className="flex flex-wrap gap-3">
          {summaryCards.map(({ ind, min, max, mean }) => {
            const stats: Array<[string, number]> = [
              [m.label_min(), min],
              [m.label_max(), max],
              [m.label_mean(), mean],
            ];
            return (
              <Card key={ind} className="min-w-36 p-3">
                <p className="font-mono text-xs font-semibold text-muted-foreground">
                  {ind}
                </p>
                <dl className="mt-1 space-y-0.5 text-xs">
                  {stats.map(([label, value]) => (
                    <div key={label} className="flex gap-1">
                      <dt className="text-muted-foreground">{label}:</dt>
                      <dd className="font-medium tabular-nums">
                        {level(value)}
                      </dd>
                    </div>
                  ))}
                </dl>
              </Card>
            );
          })}
        </div>
      ) : null}

      {/* Filter + Download */}
      <div className="flex items-center gap-3">
        <Input
          className="h-8 w-64 text-xs"
          aria-label={m.label_filter_receiver_id()}
          placeholder={m.label_filter_receiver_id()}
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
          }}
        />
        <span className="text-xs text-muted-foreground">
          {String(sortedRecords.length)} / {String(data.records.length)}{" "}
          {m.msg_records_count()}
        </span>
        <div className="ml-auto">
          <Button variant="outline" size="sm" onClick={downloadCSV}>
            <Download aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
            {m.action_download_csv()}
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-auto rounded-md border">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b bg-muted/50">
              {columns.map((col) => (
                <th
                  key={col}
                  scope="col"
                  aria-sort={
                    sortCol === col
                      ? sortDir === "asc"
                        ? "ascending"
                        : "descending"
                      : undefined
                  }
                  className="whitespace-nowrap px-3 py-2 text-left font-semibold text-muted-foreground"
                >
                  <button
                    type="button"
                    onClick={() => {
                      toggleSort(col);
                    }}
                    className="inline-flex items-center gap-1 rounded-sm hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    {columnLabel(col)}
                    <SortIcon col={col} />
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sortedRecords.map((r) => (
              <tr
                key={r.id}
                className="border-b last:border-0 hover:bg-muted/30"
              >
                <td className="px-3 py-1.5 font-mono">{r.id}</td>
                <td className="px-3 py-1.5 tabular-nums">
                  {formatCoordinate(r.x)}
                </td>
                <td className="px-3 py-1.5 tabular-nums">
                  {formatCoordinate(r.y)}
                </td>
                <td className="px-3 py-1.5 tabular-nums">
                  {formatNumber(r.height_m)}
                </td>
                {indicators.map((ind) => (
                  <td key={ind} className="px-3 py-1.5 tabular-nums">
                    {formatNumber(r.values[ind] ?? 0)}
                  </td>
                ))}
              </tr>
            ))}
            {sortedRecords.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-3 py-6 text-center text-muted-foreground"
                >
                  {m.msg_no_records_match_filter()}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Raster tab
// ---------------------------------------------------------------------------

function RasterArtifactCard({ artifact }: { artifact: ArtifactRef }) {
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
        <div>
          <span className="text-muted-foreground">{m.label_unit()}:</span>{" "}
          <span className="font-medium">{data.unit}</span>
        </div>
        {data.band_names && data.band_names.length > 0 ? (
          <div className="col-span-2">
            <span className="text-muted-foreground">{m.label_bands()}:</span>{" "}
            <span className="font-mono font-medium">
              {data.band_names.join(", ")}
            </span>
          </div>
        ) : null}
      </div>

      {/* Rendering controls (placeholder) */}
      <div className="space-y-3 rounded-md border bg-muted/30 p-3">
        <div className="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
          <SlidersHorizontal aria-hidden="true" className="h-3.5 w-3.5" />
          {m.section_rendering_controls()}
        </div>
        <div className="grid grid-cols-2 gap-3 opacity-50">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">
              {m.label_color_ramp()}
            </p>
            <Select disabled>
              <SelectTrigger
                className="h-7 text-xs"
                aria-label={m.label_color_ramp()}
              >
                <SelectValue placeholder="Viridis" />
              </SelectTrigger>
              <SelectContent>
                {["Viridis", "Blues", "Reds", "YlOrRd", "Greens"].map((c) => (
                  <SelectItem key={c} value={c}>
                    {c}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{m.label_min_max()}</p>
            <div className="flex gap-1">
              <Input
                disabled
                className="h-7 text-xs"
                aria-label={m.label_min()}
                placeholder={m.label_min()}
              />
              <Input
                disabled
                className="h-7 text-xs"
                aria-label={m.label_max()}
                placeholder={m.label_max()}
              />
            </div>
          </div>
        </div>
        <p className="text-xs text-muted-foreground">
          {m.msg_raster_not_implemented()}
        </p>
      </div>

      {/* Receiver probe placeholder */}
      <Callout variant="neutral" icon={Crosshair} className="mt-3">
        {m.label_receiver_probe_tool()}
      </Callout>
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
        <RasterArtifactCard key={a.id} artifact={a} />
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Compare tab
// ---------------------------------------------------------------------------

function RunColumn({ run, label }: { run: RunSummary; label: string }) {
  return (
    <Card className="flex-1 p-4">
      <SectionHeading variant="eyebrow" className="mb-1">
        {label}
      </SectionHeading>
      <p className="font-mono text-xs">{run.id}</p>
      <div className="mt-2 space-y-0.5 text-xs">
        <p>
          <span className="text-muted-foreground">{m.label_standard()}:</span>{" "}
          <span className="font-mono">{run.standard_id}</span>
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
          <span className="text-muted-foreground">{m.label_started()}</span>{" "}
          {formatTime(run.started_at)}
        </p>
        <p>
          <span className="text-muted-foreground">{m.label_duration()}</span>{" "}
          {formatDurationBetween(run.started_at, run.finished_at)}
        </p>
        <p>
          <span className="text-muted-foreground">{m.label_artifacts()}</span>{" "}
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
                  ({r.standard_id} / {r.version})
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {compareRun ? (
        <>
          <div className="flex gap-4">
            <RunColumn run={run} label={m.msg_run_column_selected()} />
            <RunColumn run={compareRun} label={m.msg_run_column_compare()} />
          </div>
          <Callout variant="neutral" icon={Info}>
            {m.msg_run_to_run_diff_deferred()}
          </Callout>
        </>
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
          <span className="font-mono">{run.standard_id}</span>
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
          {m.label_started()} {runTiming(run)}
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

export default function ResultsPage() {
  const { data: runs = [], isLoading, error } = useRuns();
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);

  const completedRuns = useMemo(
    () => runs.filter((r) => r.status === "completed"),
    [runs],
  );

  // Derived, not stored: falling back to the first completed run here avoids
  // the render-phase `setSelectedRunId` this used to do, which forced an extra
  // render pass on every load.
  const selectedRun = useMemo(
    () =>
      completedRuns.find((r) => r.id === selectedRunId) ??
      completedRuns[0] ??
      null,
    [completedRuns, selectedRunId],
  );

  if (isLoading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Loader2
          aria-hidden="true"
          className="h-6 w-6 animate-spin text-muted-foreground"
        />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 items-start justify-center p-8">
        <Callout variant="destructive" icon={AlertCircle}>
          {m.msg_api_error_results()}
        </Callout>
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
          />
        ) : (
          <ItemList>
            {completedRuns.map((run) => (
              <ListItem
                key={run.id}
                selected={run.id === selectedRun?.id}
                onSelect={() => {
                  setSelectedRunId(run.id);
                }}
                badge={<StatusBadge status={run.status} />}
                code={run.id}
                title={`${run.standard_id}${run.version ? ` / ${run.version}` : ""}`}
                meta={runTiming(run)}
              />
            ))}
          </ItemList>
        )
      }
    >
      {selectedRun ? (
        <RunResultDetail run={selectedRun} allCompletedRuns={completedRuns} />
      ) : (
        <EmptyState title={m.msg_select_completed_run_details()} />
      )}
    </MasterDetail>
  );
}
