import { useState, useMemo } from "react";
import { useModelStore } from "@/model/model-store";
import {
  Play,
  Settings2,
  Grid2x2,
  AlertCircle,
  Loader2,
  CheckCircle2,
  XCircle,
  Clock,
  RefreshCw,
  StopCircle,
  ChevronRight,
  Terminal,
  Info,
} from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/ui/components/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import { useCreateRun, useStandards, useRuns, useRunLog } from "@/api/hooks";
import { IS_WASM_MODE } from "@/api/mode";
import type {
  ArtifactRef,
  ParameterDefinition,
  ProfileInfo,
  RunSummary,
} from "@/api/client";

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
// Status badge
// ---------------------------------------------------------------------------

type RunStatus = RunSummary["status"];

const statusConfig: Record<
  RunStatus,
  {
    label: string;
    icon: React.ComponentType<{ className?: string }>;
    className: string;
  }
> = {
  pending: {
    label: "Pending",
    icon: Clock,
    className: "text-muted-foreground bg-muted",
  },
  running: {
    label: "Running",
    icon: Loader2,
    className: "text-blue-600 bg-blue-50 dark:bg-blue-950",
  },
  completed: {
    label: "Completed",
    icon: CheckCircle2,
    className: "text-green-600 bg-green-50 dark:bg-green-950",
  },
  failed: {
    label: "Failed",
    icon: XCircle,
    className: "text-destructive bg-destructive/10",
  },
};

function StatusBadge({ status }: { status: RunStatus }) {
  const cfg = statusConfig[status] ?? statusConfig.pending;
  const Icon = cfg.icon;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${cfg.className}`}
    >
      <Icon
        className={`h-3 w-3 ${status === "running" ? "animate-spin" : ""}`}
      />
      {cfg.label}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Progress timeline (parsed from log lines)
// ---------------------------------------------------------------------------

interface TimelineStep {
  label: string;
  timestamp?: string;
  done: boolean;
  active: boolean;
}

const LOG_STAGES: Array<{ key: string; pattern: RegExp; label: string }> = [
  { key: "started", pattern: /run started/, label: "Run started" },
  { key: "model", pattern: /model=/, label: "Loading model" },
  {
    key: "sources",
    pattern: /(?:sources|road_sources)=\d+/,
    label: "Extracting sources",
  },
  {
    key: "receivers",
    pattern: /receivers=\d+/,
    label: "Building receivers",
  },
  {
    key: "compute",
    pattern: /stage=compute/,
    label: "Computing",
  },
  {
    key: "persist",
    pattern: /(?:output_hash=|persisted=)/,
    label: "Persisting outputs",
  },
  {
    key: "done",
    pattern: /run (?:completed|failed)/,
    label: "Finalised",
  },
];

function parseTimeline(lines: string[], status: RunStatus): TimelineStep[] {
  const matched = new Map<string, string>();

  for (const line of lines) {
    const ts = line.slice(0, 20);
    for (const stage of LOG_STAGES) {
      if (!matched.has(stage.key) && stage.pattern.test(line)) {
        matched.set(stage.key, ts);
      }
    }
  }

  const steps: TimelineStep[] = LOG_STAGES.map((stage, i) => {
    const ts = matched.get(stage.key);
    const done = matched.has(stage.key);
    const nextStage = LOG_STAGES[i + 1];
    const active =
      !done &&
      status === "running" &&
      (i === 0 || matched.has(LOG_STAGES[i - 1].key)) &&
      (!nextStage || !matched.has(nextStage.key));
    return { label: stage.label, timestamp: ts, done, active };
  });

  return steps;
}

function ProgressTimeline({
  lines,
  status,
}: {
  lines: string[];
  status: RunStatus;
}) {
  const steps = parseTimeline(lines, status);

  return (
    <div className="space-y-1">
      {steps.map((step, i) => (
        <div key={i} className="flex items-start gap-2.5">
          <div className="flex flex-col items-center">
            <div
              className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-xs ${
                step.done
                  ? "border-green-500 bg-green-500 text-white"
                  : step.active
                    ? "border-blue-500 bg-blue-500 text-white"
                    : "border-border bg-muted text-muted-foreground"
              }`}
            >
              {step.done ? (
                <CheckCircle2 className="h-3 w-3" />
              ) : step.active ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <span className="h-1.5 w-1.5 rounded-full bg-current" />
              )}
            </div>
            {i < steps.length - 1 ? (
              <div
                className={`mt-0.5 w-px flex-1 ${step.done ? "bg-green-300 dark:bg-green-800" : "bg-border"}`}
                style={{ minHeight: "12px" }}
              />
            ) : null}
          </div>
          <div className="pb-2 pt-0.5">
            <p
              className={`text-sm ${step.done || step.active ? "font-medium" : "text-muted-foreground"}`}
            >
              {step.label}
            </p>
            {step.timestamp ? (
              <p className="text-xs text-muted-foreground">{step.timestamp}</p>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Log viewer
// ---------------------------------------------------------------------------

function LogViewer({ lines }: { lines: string[] }) {
  const [expanded, setExpanded] = useState(false);
  const visible = expanded ? lines : lines.slice(-20);

  return (
    <div className="rounded-md border bg-muted/30">
      <div className="flex items-center justify-between border-b px-3 py-2">
        <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
          <Terminal className="h-3.5 w-3.5" />
          Log ({String(lines.length)} lines)
        </div>
        {lines.length > 20 ? (
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-xs"
            onClick={() => {
              setExpanded((e) => !e);
            }}
          >
            {expanded ? "Show less" : "Show all"}
          </Button>
        ) : null}
      </div>
      <div className="max-h-48 overflow-y-auto p-3 font-mono text-xs leading-relaxed">
        {lines.length === 0 ? (
          <span className="text-muted-foreground">No log lines yet.</span>
        ) : (
          visible.map((line, i) => (
            <div key={i} className="whitespace-pre-wrap break-all">
              {line}
            </div>
          ))
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Artifact links
// ---------------------------------------------------------------------------

const ARTIFACT_KIND_LABELS: Record<string, string> = {
  "run.result.receiver_table_json": "Receivers (JSON)",
  "run.result.receiver_table_csv": "Receivers (CSV)",
  "run.result.raster_metadata": "Raster metadata",
  "run.result.raster_binary": "Raster data",
  "run.result.summary": "Run summary",
};

function ArtifactLinks({ artifacts }: { artifacts: ArtifactRef[] }) {
  const [copied, setCopied] = useState<string | null>(null);

  function copyPath(path: string) {
    void navigator.clipboard.writeText(path).then(() => {
      setCopied(path);
      setTimeout(() => {
        setCopied(null);
      }, 1500);
    });
  }

  if (artifacts.length === 0) {
    return <p className="text-xs text-muted-foreground">No artifacts yet.</p>;
  }

  return (
    <div className="space-y-1">
      {artifacts.map((a) => {
        const label = ARTIFACT_KIND_LABELS[a.kind] ?? a.kind;
        const filename = a.path.split("/").pop() ?? a.path;
        return (
          <div
            key={a.id}
            className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 px-3 py-2"
          >
            <div className="min-w-0">
              <p className="text-xs font-medium">{label}</p>
              <p
                className="truncate font-mono text-xs text-muted-foreground"
                title={a.path}
              >
                {filename}
              </p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 shrink-0 px-2 text-xs"
              onClick={() => {
                copyPath(a.path);
              }}
            >
              {copied === a.path ? "Copied!" : "Copy path"}
            </Button>
          </div>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Filter bar
// ---------------------------------------------------------------------------

interface RunFilters {
  status: string;
  standardId: string;
  scenarioId: string;
}

function RunFilterBar({
  runs,
  filters,
  onChange,
}: {
  runs: RunSummary[];
  filters: RunFilters;
  onChange: (f: RunFilters) => void;
}) {
  const statuses = useMemo(
    () => Array.from(new Set(runs.map((r) => r.status))).sort(),
    [runs],
  );
  const standards = useMemo(
    () => Array.from(new Set(runs.map((r) => r.standard_id))).sort(),
    [runs],
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
        <SelectTrigger className="h-7 w-32 text-xs">
          <SelectValue placeholder="Status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="_all">All statuses</SelectItem>
          {statuses.map((s) => (
            <SelectItem key={s} value={s}>
              {s.charAt(0).toUpperCase() + s.slice(1)}
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
        <SelectTrigger className="h-7 w-36 text-xs">
          <SelectValue placeholder="Standard" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="_all">All standards</SelectItem>
          {standards.map((s) => (
            <SelectItem key={s} value={s}>
              {s}
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
          <SelectTrigger className="h-7 w-32 text-xs">
            <SelectValue placeholder="Scenario" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="_all">All scenarios</SelectItem>
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
          Clear
        </Button>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run detail panel
// ---------------------------------------------------------------------------

function RunDetail({ run, onRetry }: { run: RunSummary; onRetry: () => void }) {
  const { data: log, isLoading: logLoading } = useRunLog(run.id);
  const lines = log?.lines ?? [];
  const isRunning = run.status === "running";

  return (
    <div className="flex flex-col gap-5 overflow-y-auto p-5">
      {/* Header */}
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <StatusBadge status={run.status} />
          <span className="font-mono text-xs text-muted-foreground">
            {run.id}
          </span>
        </div>
        <p className="text-sm">
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
          Started {formatTime(run.started_at)}
          {run.status !== "running" && run.status !== "pending"
            ? ` · ${formatDuration(run.started_at, run.finished_at)}`
            : null}
        </p>
      </div>

      {/* Determinism hint for completed runs */}
      {run.status === "completed" ? (
        <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>
            Re-running the same inputs with this standard, version, and profile
            will produce identical results.
          </span>
        </div>
      ) : null}

      {/* Progress timeline */}
      <section>
        <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Progress
        </h4>
        {logLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading…
          </div>
        ) : (
          <ProgressTimeline lines={lines} status={run.status} />
        )}
      </section>

      {/* Log viewer */}
      <section>
        <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Logs
        </h4>
        {logLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading…
          </div>
        ) : (
          <LogViewer lines={lines} />
        )}
      </section>

      {/* Artifacts */}
      <section>
        <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Artifacts
        </h4>
        <ArtifactLinks artifacts={run.artifacts} />
      </section>

      {/* Actions */}
      <section className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={!isRunning}
          title={
            isRunning
              ? "Runs are started from the CLI — kill the noise process to cancel."
              : "Only running jobs can be cancelled"
          }
          onClick={() => {
            alert(
              "Cancel is not supported from the UI. Kill the `noise run` CLI process to abort.",
            );
          }}
        >
          <StopCircle className="mr-1.5 h-3.5 w-3.5" />
          Cancel
        </Button>
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
          Retry
        </Button>
      </section>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run list item
// ---------------------------------------------------------------------------

function RunListItem({
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
          {formatTime(run.started_at)}
          {run.status !== "running" && run.status !== "pending"
            ? ` · ${formatDuration(run.started_at, run.finished_at)}`
            : null}
        </p>
      </div>
      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// Parameter editor (shared with setup dialog)
// ---------------------------------------------------------------------------

function ParameterField({
  param,
  value,
  onChange,
}: {
  param: ParameterDefinition;
  value: string;
  onChange: (v: string) => void;
}) {
  const id = `param-${param.name}`;

  if (param.enum && param.enum.length > 0) {
    return (
      <div className="space-y-1">
        <Label htmlFor={id}>
          {param.name}
          {param.required ? (
            <span className="ml-1 text-destructive">*</span>
          ) : null}
        </Label>
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {param.enum.map((opt) => (
              <SelectItem key={opt} value={opt}>
                {opt}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {param.description ? (
          <p className="text-xs text-muted-foreground">{param.description}</p>
        ) : null}
      </div>
    );
  }

  if (param.kind === "bool") {
    return (
      <div className="space-y-1">
        <Label htmlFor={id}>
          {param.name}
          {param.required ? (
            <span className="ml-1 text-destructive">*</span>
          ) : null}
        </Label>
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="true">true</SelectItem>
            <SelectItem value="false">false</SelectItem>
          </SelectContent>
        </Select>
        {param.description ? (
          <p className="text-xs text-muted-foreground">{param.description}</p>
        ) : null}
      </div>
    );
  }

  const inputType =
    param.kind === "float" || param.kind === "int" ? "number" : "text";
  const step = param.kind === "float" ? "any" : undefined;

  return (
    <div className="space-y-1">
      <Label htmlFor={id}>
        {param.name}
        {param.required ? (
          <span className="ml-1 text-destructive">*</span>
        ) : null}
      </Label>
      <Input
        id={id}
        type={inputType}
        step={step}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        min={param.min}
        max={param.max}
      />
      {param.description ? (
        <p className="text-xs text-muted-foreground">{param.description}</p>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run setup dialog
// ---------------------------------------------------------------------------

type ReceiverMode = "auto-grid" | "custom";

function defaultParams(profile: ProfileInfo): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of profile.parameters) {
    out[p.name] = p.default_value ?? "";
  }
  return out;
}

function RunSetupDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (runId: string) => void;
}) {
  const { data: standards, isLoading, error } = useStandards();
  const createRun = useCreateRun();
  const receiverCount = useModelStore((s) => s.receivers.length);

  const firstStandard = standards?.[0];

  const [standardId, setStandardId] = useState<string>("");
  const [version, setVersion] = useState<string>("");
  const [profile, setProfile] = useState<string>("");
  const [params, setParams] = useState<Record<string, string>>({});
  const [receiverMode, setReceiverMode] = useState<ReceiverMode>("auto-grid");
  const [submitted, setSubmitted] = useState(false);
  const [submittedConfig, setSubmittedConfig] = useState<{
    standardId: string;
    version: string;
    profile: string;
    params: Record<string, string>;
    receiverMode: ReceiverMode;
  } | null>(null);

  const effectiveStandardId = standardId || firstStandard?.id || "";
  const selectedStandard = useMemo(
    () => standards?.find((s) => s.id === effectiveStandardId),
    [standards, effectiveStandardId],
  );

  const effectiveVersion = version || selectedStandard?.default_version || "";
  const selectedVersion = useMemo(
    () => selectedStandard?.versions.find((v) => v.name === effectiveVersion),
    [selectedStandard, effectiveVersion],
  );

  const effectiveProfile = profile || selectedVersion?.default_profile || "";
  const selectedProfile = useMemo(
    () => selectedVersion?.profiles.find((p) => p.name === effectiveProfile),
    [selectedVersion, effectiveProfile],
  );

  const profileKey = `${effectiveStandardId}/${effectiveVersion}/${effectiveProfile}`;
  const [lastProfileKey, setLastProfileKey] = useState<string>("");

  if (profileKey !== lastProfileKey && selectedProfile) {
    setLastProfileKey(profileKey);
    setParams(defaultParams(selectedProfile));
  }

  function handleStandardChange(id: string) {
    setStandardId(id);
    setVersion("");
    setProfile("");
    setParams({});
    setLastProfileKey("");
  }

  function handleVersionChange(v: string) {
    setVersion(v);
    setProfile("");
    setParams({});
    setLastProfileKey("");
  }

  function handleProfileChange(p: string) {
    setProfile(p);
    setParams({});
    setLastProfileKey("");
  }

  function handleSubmit() {
    if (IS_WASM_MODE) {
      createRun.mutate(
        {
          standardId: effectiveStandardId,
          version: effectiveVersion,
          profile: effectiveProfile,
          params,
          receiverMode,
        },
        {
          onSuccess: (run) => {
            onCreated(run.id);
            handleClose();
          },
        },
      );
      return;
    }
    setSubmittedConfig({
      standardId: effectiveStandardId,
      version: effectiveVersion,
      profile: effectiveProfile,
      params,
      receiverMode,
    });
    setSubmitted(true);
  }

  function handleClose() {
    setSubmitted(false);
    setSubmittedConfig(null);
    onClose();
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{m.dialog_title_new_run()}</DialogTitle>
          <DialogDescription>
            {m.dialog_desc_new_run()}
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <div className="flex items-center gap-2 rounded-md border border-destructive/50 p-4 text-sm text-destructive">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{m.msg_api_error_standards()}</span>
          </div>
        ) : createRun.isError ? (
          <div className="flex items-center gap-2 rounded-md border border-destructive/50 p-4 text-sm text-destructive">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{createRun.error.message}</span>
          </div>
        ) : submitted && submittedConfig ? (
          <div className="space-y-4">
            <div className="rounded-md border bg-muted/30 p-4 text-sm">
              <p className="mb-2 font-medium">{m.status_run_queued()}</p>
              <p className="text-muted-foreground">
                <span className="font-mono">{submittedConfig.standardId}</span>{" "}
                / <span className="font-mono">{submittedConfig.version}</span> /{" "}
                <span className="font-mono">{submittedConfig.profile}</span>
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                Receivers: {submittedConfig.receiverMode}
              </p>
              <pre className="mt-3 max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                {JSON.stringify(submittedConfig.params, null, 2)}
              </pre>
            </div>
            <p className="text-xs text-muted-foreground">
              Run execution via the HTTP API is not yet implemented. Use{" "}
              <code className="rounded bg-muted px-1">noise run</code> from the
              CLI to execute this configuration.
            </p>
            <DialogFooter>
              <Button onClick={handleClose}>Close</Button>
            </DialogFooter>
          </div>
        ) : (
          <div className="space-y-6">
            {/* Standard / Version / Profile */}
            <section className="space-y-4">
              <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                Standard
              </h3>
              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1">
                  <Label htmlFor="standard">Standard</Label>
                  <Select
                    value={effectiveStandardId}
                    onValueChange={handleStandardChange}
                  >
                    <SelectTrigger id="standard">
                      <SelectValue placeholder="Select…" />
                    </SelectTrigger>
                    <SelectContent>
                      {standards?.map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          {s.id}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="version">Version</Label>
                  <Select
                    value={effectiveVersion}
                    onValueChange={handleVersionChange}
                    disabled={!selectedStandard}
                  >
                    <SelectTrigger id="version">
                      <SelectValue placeholder="Select…" />
                    </SelectTrigger>
                    <SelectContent>
                      {selectedStandard?.versions.map((v) => (
                        <SelectItem key={v.name} value={v.name}>
                          {v.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="profile">Profile</Label>
                  <Select
                    value={effectiveProfile}
                    onValueChange={handleProfileChange}
                    disabled={!selectedVersion}
                  >
                    <SelectTrigger id="profile">
                      <SelectValue placeholder="Select…" />
                    </SelectTrigger>
                    <SelectContent>
                      {selectedVersion?.profiles.map((p) => (
                        <SelectItem key={p.name} value={p.name}>
                          {p.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              {selectedStandard ? (
                <p className="text-xs text-muted-foreground">
                  {selectedStandard.description}
                </p>
              ) : null}

              {selectedProfile ? (
                <div className="flex flex-wrap gap-2">
                  {selectedProfile.supported_indicators.map((ind) => (
                    <span
                      key={ind}
                      className="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium"
                    >
                      {ind}
                    </span>
                  ))}
                </div>
              ) : null}
            </section>

            {/* Parameters */}
            {selectedProfile && selectedProfile.parameters.length > 0 ? (
              <section className="space-y-4">
                <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                  Parameters
                </h3>
                <div className="grid grid-cols-2 gap-x-4 gap-y-3">
                  {selectedProfile.parameters.map((param) => (
                    <ParameterField
                      key={param.name}
                      param={param}
                      value={params[param.name] ?? ""}
                      onChange={(v) => {
                        setParams((prev) => ({ ...prev, [param.name]: v }));
                      }}
                    />
                  ))}
                </div>
              </section>
            ) : null}

            {/* Receiver set */}
            <section className="space-y-3">
              <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
                Receivers
              </h3>
              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => {
                    setReceiverMode("auto-grid");
                  }}
                  className={`flex items-start gap-3 rounded-lg border p-4 text-left transition-colors ${
                    receiverMode === "auto-grid"
                      ? "border-primary bg-primary/5"
                      : "border-border hover:bg-muted/50"
                  }`}
                >
                  <Grid2x2 className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">Auto-grid</p>
                    <p className="text-xs text-muted-foreground">
                      Regular grid derived from source extent and
                      grid_resolution_m / grid_padding_m parameters.
                    </p>
                  </div>
                </button>

                <button
                  type="button"
                  onClick={() => {
                    setReceiverMode("custom");
                  }}
                  className={`flex items-start gap-3 rounded-lg border p-4 text-left transition-colors ${
                    receiverMode === "custom"
                      ? "border-primary bg-primary/5"
                      : "border-border hover:bg-muted/50"
                  }`}
                >
                  <Settings2 className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">Custom receiver set</p>
                    <p className="text-xs text-muted-foreground">
                      Explicit receivers placed in the map workspace.
                    </p>
                  </div>
                </button>
              </div>
              {receiverMode === "custom" && receiverCount === 0 ? (
                <div className="flex items-start gap-2 rounded-md border border-amber-300 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-200">
                  <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  <span>
                    No explicit receivers have been placed. Use the map workspace
                    to add receiver points before running.
                  </span>
                </div>
              ) : null}
              {receiverMode === "custom" && receiverCount > 0 ? (
                <p className="text-xs text-muted-foreground">
                  {String(receiverCount)} receiver
                  {receiverCount !== 1 ? "s" : ""} placed.
                </p>
              ) : null}
            </section>

            {/* Determinism hint */}
            {selectedProfile ? (
              <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
                <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                <span>
                  <span className="font-medium text-foreground">
                    Deterministic by design.
                  </span>{" "}
                  The same inputs, standard, version, and profile always produce
                  identical outputs — regardless of worker count or execution
                  order.
                </span>
              </div>
            ) : null}
          </div>
        )}

        {!submitted && !isLoading && !error ? (
          <DialogFooter>
            <Button variant="ghost" onClick={handleClose}>
              Cancel
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={
                !selectedProfile ||
                createRun.isPending ||
                (receiverMode === "custom" && receiverCount === 0)
              }
            >
              <Play className="mr-2 h-4 w-4" />
              {createRun.isPending ? "Running…" : "Start Run"}
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Run page
// ---------------------------------------------------------------------------

const EMPTY_FILTERS: RunFilters = {
  status: "",
  standardId: "",
  scenarioId: "",
};

export default function RunPage() {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [filters, setFilters] = useState<RunFilters>(EMPTY_FILTERS);

  // Fetch runs; poll every 3 s to pick up CLI-launched runs quickly.
  const { data: runs = [], isLoading, error } = useRuns(3_000);

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

  const selectedRun = useMemo(
    () => filteredRuns.find((r) => r.id === selectedRunId) ?? null,
    [filteredRuns, selectedRunId],
  );

  // Auto-select first visible run if current selection isn't visible.
  if (!selectedRun && filteredRuns.length > 0 && !isLoading) {
    setSelectedRunId(filteredRuns[0].id);
  }

  const hasRuns = runs.length > 0;

  return (
    <div className="flex flex-1 flex-col">
      {/* Toolbar */}
      <div className="flex items-center justify-between border-b px-5 py-3">
        <div>
          <h2 className="text-sm font-semibold">Runs</h2>
          {hasRunning ? (
            <p className="text-xs text-blue-600">
              <Loader2 className="mr-1 inline h-3 w-3 animate-spin" />
              Run in progress…
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">
              {String(runs.length)} run{runs.length !== 1 ? "s" : ""}
            </p>
          )}
        </div>
        <Button
          size="sm"
          onClick={() => {
            setDialogOpen(true);
          }}
        >
          <Play className="mr-1.5 h-3.5 w-3.5" />
          {m.action_new_run()}
        </Button>
      </div>

      {/* Body */}
      {isLoading ? (
        <div className="flex flex-1 items-center justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <div className="flex flex-1 items-center justify-center p-8">
          <div className="flex items-center gap-2 text-sm text-destructive">
            <AlertCircle className="h-4 w-4" />
            {m.msg_api_error_run()}
          </div>
        </div>
      ) : !hasRuns ? (
        <div className="flex flex-1 items-center justify-center">
          <div className="text-center">
            <Play className="mx-auto h-8 w-8 text-muted-foreground" />
            <p className="mt-2 text-sm text-muted-foreground">
              {m.msg_no_runs_empty_state()}
            </p>
          </div>
        </div>
      ) : (
        <div className="flex flex-1 overflow-hidden">
          {/* Run list */}
          <div className="flex w-72 shrink-0 flex-col overflow-hidden border-r">
            <RunFilterBar runs={runs} filters={filters} onChange={setFilters} />
            <div className="overflow-y-auto">
              {filteredRuns.length === 0 ? (
                <p className="px-4 py-6 text-center text-xs text-muted-foreground">
                  {m.msg_no_runs_match_filters()}
                </p>
              ) : null}
              {filteredRuns.map((run) => (
                <RunListItem
                  key={run.id}
                  run={run}
                  selected={run.id === selectedRunId}
                  onClick={() => {
                    setSelectedRunId(run.id);
                  }}
                />
              ))}
            </div>
          </div>

          {/* Detail panel */}
          <div className="flex-1 overflow-hidden">
            {selectedRun ? (
              <RunDetail
                run={selectedRun}
                onRetry={() => {
                  setDialogOpen(true);
                }}
              />
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                Select a run to view details
              </div>
            )}
          </div>
        </div>
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
