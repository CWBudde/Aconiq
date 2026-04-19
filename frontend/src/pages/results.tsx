import { useState, useMemo } from "react";
import {
  BarChart3,
  Table2,
  GitCompare,
  Loader2,
  AlertCircle,
  Download,
  Info,
  ChevronRight,
  ChevronUp,
  ChevronDown,
  SlidersHorizontal,
  Crosshair,
  CheckCircle2,
  XCircle,
  Clock,
} from "lucide-react";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { useRuns, useReceiverTable, useRasterMetadata } from "@/api/hooks";
import type { ArtifactRef, RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatDuration(startedAt: string, finishedAt: string): string {
  const start = new Date(startedAt).getTime();
  const end = new Date(finishedAt).getTime();
  const ms = end - start;
  if (ms < 1000) return `${String(ms)}ms`;
  if (ms < 60_000) return `${String(Math.round(ms / 1000))}s`;
  return `${String(Math.floor(ms / 60_000))}m ${String(Math.round((ms % 60_000) / 1000))}s`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

// ---------------------------------------------------------------------------
// Status badge (re-implemented locally)
// ---------------------------------------------------------------------------

type RunStatus = RunSummary["status"];

const statusConfig: Record<
  RunStatus,
  {
    label: () => string;
    icon: React.ComponentType<{ className?: string }>;
    className: string;
  }
> = {
  pending: {
    label: m.status_badge_pending,
    icon: Clock,
    className: "text-muted-foreground bg-muted",
  },
  running: {
    label: m.status_badge_running,
    icon: Loader2,
    className: "text-blue-600 bg-blue-50 dark:bg-blue-950",
  },
  completed: {
    label: m.status_badge_completed,
    icon: CheckCircle2,
    className: "text-green-600 bg-green-50 dark:bg-green-950",
  },
  failed: {
    label: m.status_badge_failed,
    icon: XCircle,
    className: "text-destructive bg-destructive/10",
  },
};

function StatusBadge({ status }: { status: RunStatus }) {
  const cfg = statusConfig[status];
  const Icon = cfg.icon;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${cfg.className}`}
    >
      <Icon
        className={`h-3 w-3 ${status === "running" ? "animate-spin" : ""}`}
      />
      {cfg.label()}
    </span>
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
    if (sortCol !== col) return <ChevronUp className="h-3 w-3 opacity-30" />;
    return sortDir === "asc" ? (
      <ChevronUp className="h-3 w-3" />
    ) : (
      <ChevronDown className="h-3 w-3" />
    );
  }

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
      <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-4 text-sm text-muted-foreground">
        <Info className="mt-0.5 h-4 w-4 shrink-0" />
        {m.msg_no_raster_artifacts()}
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        {m.status_loading_receiver_table()}
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex items-center gap-2 text-sm text-destructive">
        <AlertCircle className="h-4 w-4" />
        {m.error_load_receiver_table()}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Indicator summary cards */}
      {summaryCards.length > 0 ? (
        <div className="flex flex-wrap gap-3">
          {summaryCards.map(({ ind, min, max, mean }) => (
            <div
              key={ind}
              className="min-w-[140px] rounded-lg border bg-card p-3 shadow-sm"
            >
              <p className="font-mono text-xs font-semibold text-muted-foreground">
                {ind}
              </p>
              <div className="mt-1 space-y-0.5 text-xs">
                <p>
                  <span className="text-muted-foreground">
                    {m.label_min()}:
                  </span>{" "}
                  <span className="font-medium">
                    {min.toFixed(1)} {unit}
                  </span>
                </p>
                <p>
                  <span className="text-muted-foreground">
                    {m.label_max()}:
                  </span>{" "}
                  <span className="font-medium">
                    {max.toFixed(1)} {unit}
                  </span>
                </p>
                <p>
                  <span className="text-muted-foreground">
                    {m.label_mean()}:
                  </span>{" "}
                  <span className="font-medium">
                    {mean.toFixed(1)} {unit}
                  </span>
                </p>
              </div>
            </div>
          ))}
        </div>
      ) : null}

      {/* Filter + Download */}
      <div className="flex items-center gap-3">
        <Input
          className="h-8 w-64 text-xs"
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
            <Download className="mr-1.5 h-3.5 w-3.5" />
            {m.action_download_csv()}
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-auto rounded-md border">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b bg-muted/50">
              {["id", "x", "y", "height_m", ...indicators].map((col) => (
                <th
                  key={col}
                  className="cursor-pointer whitespace-nowrap px-3 py-2 text-left font-semibold text-muted-foreground hover:text-foreground"
                  onClick={() => {
                    toggleSort(col);
                  }}
                >
                  <span className="inline-flex items-center gap-1">
                    {col === "height_m" ? m.table_header_height_m() : col}
                    <SortIcon col={col} />
                  </span>
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
                <td className="px-3 py-1.5">{r.x.toFixed(2)}</td>
                <td className="px-3 py-1.5">{r.y.toFixed(2)}</td>
                <td className="px-3 py-1.5">{r.height_m.toFixed(1)}</td>
                {indicators.map((ind) => (
                  <td key={ind} className="px-3 py-1.5">
                    {(r.values[ind] ?? 0).toFixed(1)}
                  </td>
                ))}
              </tr>
            ))}
            {sortedRecords.length === 0 ? (
              <tr>
                <td
                  colSpan={4 + indicators.length}
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
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        {m.status_loading_raster_metadata()}
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex items-center gap-2 text-sm text-destructive">
        <AlertCircle className="h-4 w-4" />
        {m.error_load_raster_metadata()}
      </div>
    );
  }

  return (
    <div className="rounded-lg border bg-card p-4 shadow-sm">
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
          <SlidersHorizontal className="h-3.5 w-3.5" />
          {m.section_rendering_controls()}
        </div>
        <div className="grid grid-cols-2 gap-3 opacity-50">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">
              {m.label_color_ramp()}
            </p>
            <Select disabled>
              <SelectTrigger className="h-7 text-xs">
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
                placeholder={m.label_min()}
              />
              <Input
                disabled
                className="h-7 text-xs"
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
      <div className="mt-3 flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
        <Crosshair className="h-3.5 w-3.5 shrink-0" />
        {m.label_receiver_probe_tool()}
      </div>
    </div>
  );
}

function RasterTab({ run }: { run: RunSummary }) {
  const rasterArtifacts = run.artifacts.filter(
    (a) => a.kind === "run.result.raster_metadata",
  );

  if (rasterArtifacts.length === 0) {
    return (
      <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-4 text-sm text-muted-foreground">
        <Info className="mt-0.5 h-4 w-4 shrink-0" />
        {m.msg_no_raster_artifacts()}
      </div>
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

  function RunColumn({ r, label }: { r: RunSummary; label: string }) {
    return (
      <div className="flex-1 rounded-lg border bg-card p-4 shadow-sm">
        <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {label}
        </p>
        <p className="font-mono text-xs">{r.id}</p>
        <div className="mt-2 space-y-0.5 text-xs">
          <p>
            <span className="text-muted-foreground">{m.label_standard()}:</span>{" "}
            <span className="font-mono">{r.standard_id}</span>
          </p>
          <p>
            <span className="text-muted-foreground">{m.label_version()}:</span>{" "}
            <span className="font-mono">{r.version}</span>
          </p>
          {r.profile ? (
            <p>
              <span className="text-muted-foreground">
                {m.label_profile()}:
              </span>{" "}
              <span className="font-mono">{r.profile}</span>
            </p>
          ) : null}
          <p>
            <span className="text-muted-foreground">{m.label_started()}:</span>{" "}
            {formatTime(r.started_at)}
          </p>
          <p>
            <span className="text-muted-foreground">{m.label_duration()}:</span>{" "}
            {formatDuration(r.started_at, r.finished_at)}
          </p>
          <p>
            <span className="text-muted-foreground">
              {m.label_artifacts()}:
            </span>{" "}
            {String(r.artifacts.length)}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {/* Run B selector */}
      <div className="flex items-center gap-3">
        <p className="text-sm text-muted-foreground">
          {m.label_compare_with()}:
        </p>
        <Select
          value={compareRunId || "_none"}
          onValueChange={(v) => {
            setCompareRunId(v === "_none" ? "" : v);
          }}
        >
          <SelectTrigger className="h-8 w-64 text-xs">
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
            <RunColumn r={run} label={m.msg_run_column_selected()} />
            <RunColumn r={compareRun} label={m.msg_run_column_compare()} />
          </div>
          <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
            <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            {m.msg_run_to_run_diff_deferred()}
          </div>
        </>
      ) : (
        <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-4 text-sm text-muted-foreground">
          <GitCompare className="mt-0.5 h-4 w-4 shrink-0" />
          {m.msg_select_run_compare()}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run detail (right panel)
// ---------------------------------------------------------------------------

type ResultTab = "receivers" | "raster" | "compare";

function RunResultDetail({
  run,
  allCompletedRuns,
}: {
  run: RunSummary;
  allCompletedRuns: RunSummary[];
}) {
  const [tab, setTab] = useState<ResultTab>("receivers");

  const tabs: { id: ResultTab; label: string; icon: React.ReactNode }[] = [
    {
      id: "receivers",
      label: m.tab_receivers(),
      icon: <Table2 className="h-3.5 w-3.5" />,
    },
    {
      id: "raster",
      label: m.tab_raster(),
      icon: <BarChart3 className="h-3.5 w-3.5" />,
    },
    {
      id: "compare",
      label: m.tab_compare(),
      icon: <GitCompare className="h-3.5 w-3.5" />,
    },
  ];

  return (
    <div className="flex flex-col overflow-hidden">
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
          Started {formatTime(run.started_at)} ·{" "}
          {formatDuration(run.started_at, run.finished_at)}
        </p>
      </div>

      {/* Tab bar */}
      <div className="flex gap-1 border-b px-4 py-1.5">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => {
              setTab(t.id);
            }}
            className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
              tab === t.id
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            {t.icon}
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className="flex-1 overflow-y-auto p-5">
        {tab === "receivers" ? <ReceiversTab run={run} /> : null}
        {tab === "raster" ? <RasterTab run={run} /> : null}
        {tab === "compare" ? (
          <CompareTab run={run} allCompletedRuns={allCompletedRuns} />
        ) : null}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run list item (left panel)
// ---------------------------------------------------------------------------

function ResultRunListItem({
  run,
  selected,
  onClick,
}: {
  run: RunSummary;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex w-full items-center gap-3 border-b px-4 py-3 text-left transition-colors hover:bg-muted/50 ${
        selected ? "bg-muted/60" : ""
      }`}
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <StatusBadge status={run.status} />
          <span className="truncate font-mono text-xs text-muted-foreground">
            {run.id}
          </span>
        </div>
        <p className="mt-0.5 truncate text-sm">
          {run.standard_id}
          {run.version ? ` / ${run.version}` : ""}
        </p>
        <p className="text-xs text-muted-foreground">
          {formatTime(run.started_at)} ·{" "}
          {formatDuration(run.started_at, run.finished_at)}
        </p>
      </div>
      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// Results page
// ---------------------------------------------------------------------------

export default function ResultsPage() {
  const { data: runs = [], isLoading, error } = useRuns(5_000);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);

  const completedRuns = useMemo(
    () => runs.filter((r) => r.status === "completed"),
    [runs],
  );

  const selectedRun = useMemo(
    () => completedRuns.find((r) => r.id === selectedRunId) ?? null,
    [completedRuns, selectedRunId],
  );

  // Auto-select first completed run if selection is invalid.
  if (!selectedRun && completedRuns.length > 0 && !isLoading) {
    setSelectedRunId(completedRuns[0].id);
  }

  if (isLoading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 items-center justify-center p-8">
        <div className="flex items-center gap-2 text-sm text-destructive">
          <AlertCircle className="h-4 w-4" />
          Could not load runs. Is the API server running?
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 overflow-hidden">
      {/* Left panel */}
      <div className="flex w-72 shrink-0 flex-col overflow-hidden border-r">
        <div className="border-b px-4 py-3">
          <h2 className="text-sm font-semibold">Results</h2>
          <p className="text-xs text-muted-foreground">
            {String(completedRuns.length)} completed run
            {completedRuns.length !== 1 ? "s" : ""}
          </p>
        </div>
        <div className="flex-1 overflow-y-auto">
          {completedRuns.length === 0 ? (
            <div className="px-4 py-6 text-center">
              <BarChart3 className="mx-auto h-8 w-8 text-muted-foreground" />
              <p className="mt-2 text-xs text-muted-foreground">
                No completed runs yet. Use{" "}
                <code className="rounded bg-muted px-1">aconiq run</code> from
                the CLI to execute a run.
              </p>
            </div>
          ) : (
            completedRuns.map((run) => (
              <ResultRunListItem
                key={run.id}
                run={run}
                selected={run.id === selectedRunId}
                onClick={() => {
                  setSelectedRunId(run.id);
                }}
              />
            ))
          )}
        </div>
      </div>

      {/* Right panel */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {selectedRun ? (
          <RunResultDetail run={selectedRun} allCompletedRuns={completedRuns} />
        ) : (
          <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
            Select a completed run to view its results.
          </div>
        )}
      </div>
    </div>
  );
}
