import { useState, useMemo } from "react";
import { useModelStore } from "@/model/model-store";
import { useProjectSync } from "@/model/use-project-sync";
import {
  Play,
  Settings2,
  Grid2x2,
  AlertCircle,
  Loader2,
  CheckCircle2,
  RefreshCw,
  StopCircle,
  Terminal,
  Info,
} from "lucide-react";
import { Badge } from "@/ui/components/badge";
import { Button } from "@/ui/components/button";
import { Checkbox } from "@/ui/components/checkbox";
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
import { Switch } from "@/ui/components/switch";
import { Callout } from "@/ui/callout";
import { CopyButton } from "@/ui/copy-field";
import { EmptyState } from "@/ui/empty-state";
import { formatDurationBetween, formatTime } from "@/ui/format";
import { ItemList, ListItem, MasterDetail } from "@/ui/master-detail";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { StatusBadge, type RunStatus } from "@/ui/status-badge";
import { statusLabel } from "@/ui/run-status";
import { useCreateRun, useStandards, useRuns, useRunLog } from "@/api/hooks";
import { backend } from "@/api/backend";
import type {
  ArtifactRef,
  ParameterDefinition,
  ProfileInfo,
  RunSummary,
} from "@/api/client";
import {
  EvidenceTierBadge,
  EvidenceTierWarning,
} from "@/ui/evidence-tier-badge";
import {
  asAPIRequestError,
  ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
} from "@/api/api-error";
import { isScaffoldTier } from "@/api/evidence-tier";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const STANDARD_LABELS: Record<string, () => string> = {
  upstream_mapping_standard: m.standard_upstream_mapping_standard,
};

const STANDARD_DESCRIPTIONS: Record<string, () => string> = {
  upstream_mapping_standard: m.standard_upstream_mapping_standard_description,
};

function getStandardLabel(standardId: string): string {
  return STANDARD_LABELS[standardId]?.() ?? standardId;
}

function getStandardDescription(standardId: string, fallback: string): string {
  return STANDARD_DESCRIPTIONS[standardId]?.() ?? fallback;
}

function isFinished(run: RunSummary): boolean {
  return run.status !== "running" && run.status !== "pending";
}

/** "13:05:07 · 12 sec" for a finished run, the start time alone otherwise. */
function runTiming(run: RunSummary): string {
  const started = formatTime(run.started_at);
  return isFinished(run)
    ? `${started} · ${formatDurationBetween(run.started_at, run.finished_at)}`
    : started;
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

const LOG_STAGES: Array<{ key: string; pattern: RegExp; label: () => string }> =
  [
    { key: "started", pattern: /run started/, label: m.timeline_run_started },
    { key: "model", pattern: /model=/, label: m.timeline_loading_model },
    {
      key: "sources",
      pattern: /(?:sources|road_sources)=\d+/,
      label: m.timeline_extracting_sources,
    },
    {
      key: "receivers",
      pattern: /receivers=\d+/,
      label: m.timeline_building_receivers,
    },
    {
      key: "compute",
      pattern: /stage=compute/,
      label: m.timeline_computing,
    },
    {
      key: "persist",
      pattern: /(?:output_hash=|persisted=)/,
      label: m.timeline_persisting_outputs,
    },
    {
      key: "done",
      pattern: /run (?:completed|failed)/,
      label: m.timeline_finalised,
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
    const prevStage = LOG_STAGES[i - 1];
    const nextStage = LOG_STAGES[i + 1];
    const active =
      !done &&
      status === "running" &&
      (!prevStage || matched.has(prevStage.key)) &&
      (!nextStage || !matched.has(nextStage.key));
    return {
      label: stage.label(),
      ...(ts !== undefined && { timestamp: ts }),
      done,
      active,
    };
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
                  ? "border-success bg-success text-success-foreground"
                  : step.active
                    ? "border-info bg-info text-info-foreground"
                    : "border-border bg-muted text-muted-foreground"
              }`}
            >
              {step.done ? (
                <CheckCircle2 aria-hidden="true" className="h-3 w-3" />
              ) : step.active ? (
                <Loader2 aria-hidden="true" className="h-3 w-3 animate-spin" />
              ) : (
                <span className="h-1.5 w-1.5 rounded-full bg-current" />
              )}
            </div>
            {i < steps.length - 1 ? (
              <div
                className={`mt-0.5 w-px flex-1 ${step.done ? "bg-success/40" : "bg-border"}`}
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
          <Terminal aria-hidden="true" className="h-3.5 w-3.5" />
          {m.label_log()} ({m.msg_log_line_count({ count: lines.length })})
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
            {expanded ? m.action_show_less() : m.action_show_all()}
          </Button>
        ) : null}
      </div>
      <div className="max-h-48 overflow-y-auto p-3 font-mono text-xs leading-relaxed">
        {lines.length === 0 ? (
          <span className="text-muted-foreground">{m.msg_no_log_lines()}</span>
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

// Message functions must be *called* during render, never at module scope, or
// the labels freeze to the locale that was active at import time.
const ARTIFACT_KIND_LABELS: Record<string, () => string> = {
  "run.result.receiver_table_json": m.artifact_kind_receivers_json,
  "run.result.receiver_table_csv": m.artifact_kind_receivers_csv,
  "run.result.raster_metadata": m.artifact_kind_raster_metadata,
  "run.result.raster_binary": m.artifact_kind_raster_binary,
  "run.result.summary": m.artifact_kind_summary,
};

function ArtifactLinks({ artifacts }: { artifacts: ArtifactRef[] }) {
  if (artifacts.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {m.msg_no_artifacts_yet()}
      </p>
    );
  }

  return (
    <ul className="m-0 list-none space-y-1 p-0">
      {artifacts.map((a) => {
        const label = ARTIFACT_KIND_LABELS[a.kind]?.() ?? a.kind;
        const filename = a.path.split("/").pop() ?? a.path;
        return (
          <li
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
            <CopyButton
              text={a.path}
              label={m.action_copy_path()}
              variant="ghost"
              className="h-6 shrink-0"
            />
          </li>
        );
      })}
    </ul>
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
              {getStandardLabel(s)}
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

// ---------------------------------------------------------------------------
// Run detail panel
// ---------------------------------------------------------------------------

function LoadingLine() {
  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
      {m.status_loading()}
    </div>
  );
}

function RunDetail({ run, onRetry }: { run: RunSummary; onRetry: () => void }) {
  const { data: log, isLoading: logLoading } = useRunLog(
    run.id,
    run.status === "running" || run.status === "pending",
  );
  const lines = log?.lines ?? [];

  return (
    <div className="flex flex-col gap-5 p-5">
      {/* Header */}
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <StatusBadge status={run.status} />
          <span className="font-mono text-xs text-muted-foreground">
            {run.id}
          </span>
        </div>
        <p className="text-sm">
          <span className="font-mono">{getStandardLabel(run.standard_id)}</span>
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

      {/* Determinism hint for completed runs */}
      {run.status === "completed" ? (
        <Callout variant="neutral" icon={Info}>
          {m.msg_determinism_hint()}
        </Callout>
      ) : null}

      {/* Progress timeline */}
      <section>
        <SectionHeading variant="eyebrow" className="mb-2">
          {m.section_progress()}
        </SectionHeading>
        {logLoading ? (
          <LoadingLine />
        ) : (
          <ProgressTimeline lines={lines} status={run.status} />
        )}
      </section>

      {/* Log viewer */}
      <section>
        <SectionHeading variant="eyebrow" className="mb-2">
          {m.section_logs()}
        </SectionHeading>
        {logLoading ? <LoadingLine /> : <LogViewer lines={lines} />}
      </section>

      {/* Artifacts */}
      <section>
        <SectionHeading variant="eyebrow" className="mb-2">
          {m.section_artifacts()}
        </SectionHeading>
        <ArtifactLinks artifacts={run.artifacts} />
      </section>

      {/* Actions. Neither backend can cancel a run (the API has no endpoint
          and the kernel completes inside `startRun`), so the control is
          disabled rather than offered and then refused on click. */}
      <section className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled
          title={m.alert_cancel_not_supported()}
        >
          <StopCircle aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
          {m.action_cancel()}
        </Button>
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
          {m.action_retry()}
        </Button>
      </section>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Parameter editor (shared with setup dialog)
// ---------------------------------------------------------------------------

function ParameterLabel({
  id,
  param,
}: {
  id: string;
  param: ParameterDefinition;
}) {
  return (
    <Label htmlFor={id}>
      {param.name}
      {param.required ? <span className="ml-1 text-destructive">*</span> : null}
    </Label>
  );
}

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
  const description = param.description ? (
    <p className="text-xs text-muted-foreground">{param.description}</p>
  ) : null;

  if (param.enum && param.enum.length > 0) {
    return (
      <div className="space-y-1">
        <ParameterLabel id={id} param={param} />
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
        {description}
      </div>
    );
  }

  if (param.kind === "bool") {
    return (
      <div className="space-y-1">
        <div className="flex h-10 items-center gap-3">
          <Switch
            id={id}
            checked={value === "true"}
            onCheckedChange={(checked) => {
              onChange(checked ? "true" : "false");
            }}
          />
          <ParameterLabel id={id} param={param} />
        </div>
        {description}
      </div>
    );
  }

  const inputType =
    param.kind === "float" || param.kind === "int" ? "number" : "text";
  const step = param.kind === "float" ? "any" : undefined;

  return (
    <div className="space-y-1">
      <ParameterLabel id={id} param={param} />
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
      {description}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run creation failure
// ---------------------------------------------------------------------------

/**
 * The API refuses a run against a scaffold-tier standard that the request did
 * not acknowledge. The UI gates that ahead of the request, so reaching this
 * means the two disagree — a tier that changed server-side, or a standards
 * list this dialog cached before it did. The server's own words are shown
 * rather than a generic failure, because they name the standard and the tier.
 */
function RunCreateError({ error }: { error: Error }) {
  const apiError = asAPIRequestError(error);
  const optInRequired =
    apiError?.code === ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED;

  return (
    <Callout
      variant="destructive"
      icon={AlertCircle}
      data-testid="run-create-error"
      data-error-code={apiError?.code}
      title={
        optInRequired ? m.msg_experimental_opt_in_required_error() : undefined
      }
    >
      <p>{error.message}</p>
      {apiError?.hint !== undefined ? <p>{apiError.hint}</p> : null}
    </Callout>
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

function ReceiverModeButton({
  mode,
  current,
  onSelect,
  icon: Icon,
  title,
  description,
}: {
  mode: ReceiverMode;
  current: ReceiverMode;
  onSelect: (mode: ReceiverMode) => void;
  icon: typeof Grid2x2;
  title: string;
  description: string;
}) {
  const selected = mode === current;
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={() => {
        onSelect(mode);
      }}
      className={`flex items-start gap-3 rounded-lg border p-4 text-left transition-colors ${
        selected
          ? "border-primary bg-primary/5"
          : "border-border hover:bg-muted/50"
      }`}
    >
      <Icon
        aria-hidden="true"
        className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground"
      />
      <div>
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
    </button>
  );
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
  const calcArea = useModelStore((s) => s.calcArea);
  // Where a run reads the saved model, starting one with unsaved edits would
  // silently compute the project's older copy. The same save the header
  // offers is offered here, so the user need not leave the dialog.
  const projectSync = useProjectSync();
  const unsavedChanges = projectSync.enabled && projectSync.dirty;

  const firstStandard = standards?.[0];

  const [standardId, setStandardId] = useState<string>("");
  const [version, setVersion] = useState<string>("");
  const [profile, setProfile] = useState<string>("");
  const [params, setParams] = useState<Record<string, string>>({});
  const [receiverMode, setReceiverMode] = useState<ReceiverMode>("auto-grid");

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

  // A scaffold module carries no normative coefficients, so the API refuses to
  // run one until the request says so. The tier of the standard actually
  // selected decides that — never the checkbox, which only records that the
  // user was told.
  const requiresExperimentalOptIn = isScaffoldTier(
    selectedStandard?.evidence_tier,
  );

  // The acknowledgement is stored as the standard it was given for, not as a
  // flag: it is an acknowledgement of one choice, not a preference. Changing
  // the standard clears it outright, so switching away and back asks again,
  // and the identity check keeps a stale tick from surviving a standards list
  // that reloads under it.
  const [acknowledgedStandardId, setAcknowledgedStandardId] = useState<
    string | null
  >(null);
  const experimentalAcknowledged =
    acknowledgedStandardId !== null &&
    acknowledgedStandardId === effectiveStandardId;
  const experimentalOptInMissing =
    requiresExperimentalOptIn && !experimentalAcknowledged;

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
    setAcknowledgedStandardId(null);
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
    if (experimentalOptInMissing || unsavedChanges) return;

    createRun.mutate(
      {
        standardId: effectiveStandardId,
        version: effectiveVersion,
        profile: effectiveProfile,
        params,
        receiverMode,
        // Derived from the tier of the standard being run, so a normative one
        // can never inherit an opt-in from an earlier selection.
        ...(requiresExperimentalOptIn ? { experimental: true } : {}),
      },
      {
        onSuccess: (run) => {
          onCreated(run.id);
          handleClose();
        },
      },
    );
  }

  function handleClose() {
    // A reopened dialog is a fresh decision, not a resumed one.
    setAcknowledgedStandardId(null);
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
          <DialogDescription>{m.dialog_desc_new_run()}</DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2
              aria-hidden="true"
              className="h-6 w-6 animate-spin text-muted-foreground"
            />
          </div>
        ) : error ? (
          <Callout variant="destructive" icon={AlertCircle}>
            {m.msg_api_error_standards()}
          </Callout>
        ) : createRun.isError ? (
          <RunCreateError error={createRun.error} />
        ) : (
          <div className="space-y-6">
            {/* Standard / Version / Profile */}
            <section className="space-y-4">
              <SectionHeading variant="eyebrow">
                {m.label_standard()}
              </SectionHeading>
              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1">
                  <Label htmlFor="standard">{m.label_standard()}</Label>
                  <Select
                    value={effectiveStandardId}
                    onValueChange={handleStandardChange}
                  >
                    <SelectTrigger id="standard">
                      <SelectValue
                        placeholder={m.placeholder_select_standard()}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {standards?.map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          <span className="flex items-center gap-2">
                            <span>{getStandardLabel(s.id)}</span>
                            <EvidenceTierBadge tier={s.evidence_tier} />
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <Label htmlFor="version">{m.label_version()}</Label>
                  <Select
                    value={effectiveVersion}
                    onValueChange={handleVersionChange}
                    disabled={!selectedStandard}
                  >
                    <SelectTrigger id="version">
                      <SelectValue
                        placeholder={m.placeholder_select_version()}
                      />
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
                  <Label htmlFor="profile">{m.label_profile()}</Label>
                  <Select
                    value={effectiveProfile}
                    onValueChange={handleProfileChange}
                    disabled={!selectedVersion}
                  >
                    <SelectTrigger id="profile">
                      <SelectValue
                        placeholder={m.placeholder_select_profile()}
                      />
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
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-medium">
                      {getStandardLabel(selectedStandard.id)}
                    </span>
                    <EvidenceTierBadge tier={selectedStandard.evidence_tier} />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {getStandardDescription(
                      selectedStandard.id,
                      selectedStandard.description,
                    )}
                  </p>
                </div>
              ) : null}

              {selectedProfile ? (
                <div className="flex flex-wrap gap-2">
                  {selectedProfile.supported_indicators.map((ind) => (
                    <Badge key={ind} variant="secondary">
                      {ind}
                    </Badge>
                  ))}
                </div>
              ) : null}
            </section>

            {/* Parameters */}
            {selectedProfile && selectedProfile.parameters.length > 0 ? (
              <section className="space-y-4">
                <SectionHeading variant="eyebrow">
                  {m.label_section_parameters()}
                </SectionHeading>
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
              <SectionHeading variant="eyebrow">
                {m.label_receivers()}
              </SectionHeading>
              <div className="grid grid-cols-2 gap-3">
                <ReceiverModeButton
                  mode="auto-grid"
                  current={receiverMode}
                  onSelect={setReceiverMode}
                  icon={Grid2x2}
                  title={m.label_receiver_auto_grid()}
                  description={m.msg_receiver_auto_grid_desc()}
                />
                <ReceiverModeButton
                  mode="custom"
                  current={receiverMode}
                  onSelect={setReceiverMode}
                  icon={Settings2}
                  title={m.label_receiver_custom_set()}
                  description={m.msg_receiver_custom_set_desc()}
                />
              </div>
              {/* The calculation area is not part of the saved model
                  (`modelToGeoJSON` leaves it out), so a backend auto-grid
                  cannot honour it: say so instead of claiming it is active. */}
              {receiverMode === "auto-grid" && calcArea ? (
                backend.capabilities.runsAgainstSavedModel ? (
                  <Callout
                    variant="warning"
                    icon={AlertCircle}
                    data-testid="calc-area-not-in-project"
                  >
                    {m.msg_calc_area_not_in_project()}
                  </Callout>
                ) : (
                  <p className="text-xs text-info">
                    {m.msg_calc_area_active()}
                  </p>
                )
              ) : null}
              {receiverMode === "custom" &&
              receiverCount === 0 &&
              !backend.capabilities.runsAgainstSavedModel ? (
                <Callout variant="warning" icon={AlertCircle}>
                  {m.msg_no_explicit_receivers()}
                </Callout>
              ) : null}
              {receiverMode === "custom" && receiverCount > 0 ? (
                <p className="text-xs text-muted-foreground">
                  {receiverCount === 1
                    ? m.msg_receivers_placed_one({ count: receiverCount })
                    : m.msg_receivers_placed_other({ count: receiverCount })}
                </p>
              ) : null}
              {receiverMode === "custom" &&
              backend.capabilities.runsAgainstSavedModel ? (
                <p className="text-xs text-muted-foreground">
                  {m.msg_api_mode_reads_explicit_receivers()}
                </p>
              ) : null}
            </section>

            {/* Determinism hint */}
            {selectedProfile ? (
              <Callout variant="neutral" icon={Info}>
                {m.msg_determinism_hint_dialog()}
              </Callout>
            ) : null}

            {/* Sits immediately above the run action: a scaffold module has no
                normative coefficients, so nothing it produces may be read as
                an assessment result. */}
            <EvidenceTierWarning tier={selectedStandard?.evidence_tier} />

            {/* The deliberate acknowledgement the API demands before a
                scaffold-tier standard may emit levels. It sits with the
                warning it acknowledges, and gates the run action. The warning
                above is the live region; this box is a plain group, so a
                reader is not interrupted twice for one fact. */}
            {requiresExperimentalOptIn ? (
              <Callout variant="warning" role="group">
                <div className="flex items-start gap-2.5">
                  <Checkbox
                    id="experimental-opt-in"
                    className="mt-0.5"
                    checked={experimentalAcknowledged}
                    onCheckedChange={(checked) => {
                      setAcknowledgedStandardId(
                        checked === true ? effectiveStandardId : null,
                      );
                    }}
                  />
                  <div className="space-y-1">
                    <Label
                      htmlFor="experimental-opt-in"
                      className="text-xs font-medium"
                    >
                      {m.label_experimental_opt_in()}
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      {m.msg_experimental_opt_in_help()}
                    </p>
                  </div>
                </div>
              </Callout>
            ) : null}

            {/* The run reads the project's copy of the model, so unsaved
                edits would not be in it. Gates the run action. */}
            {unsavedChanges ? (
              <Callout
                variant="warning"
                icon={AlertCircle}
                data-testid="unsaved-changes-callout"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="flex-1">
                    {m.msg_unsaved_changes_before_run()}
                    {projectSync.status === "error"
                      ? ` ${m.msg_save_failed()}.`
                      : null}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={projectSync.status === "saving"}
                    onClick={() => {
                      void projectSync.save();
                    }}
                  >
                    {projectSync.status === "saving"
                      ? m.status_saving()
                      : m.action_save_to_project()}
                  </Button>
                </div>
              </Callout>
            ) : null}
          </div>
        )}

        {!isLoading && !error ? (
          <DialogFooter>
            <Button variant="ghost" onClick={handleClose}>
              {m.action_cancel()}
            </Button>
            <Button
              onClick={handleSubmit}
              title={
                experimentalOptInMissing
                  ? m.tooltip_experimental_opt_in_required()
                  : undefined
              }
              disabled={
                !selectedProfile ||
                createRun.isPending ||
                experimentalOptInMissing ||
                unsavedChanges ||
                (!backend.capabilities.runsAgainstSavedModel &&
                  receiverMode === "custom" &&
                  receiverCount === 0)
              }
            >
              <Play aria-hidden="true" className="mr-2 h-4 w-4" />
              {createRun.isPending
                ? m.status_starting_run()
                : m.action_start_run()}
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
              {runs.length === 1
                ? m.msg_run_count_one({ count: runs.length })
                : m.msg_run_count_other({ count: runs.length })}
            </span>
          )
        }
        actions={
          <Button
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
                    title={`${run.standard_id}${run.version ? ` / ${run.version}` : ""}`}
                    meta={runTiming(run)}
                  />
                ))}
              </ItemList>
            )
          }
        >
          {selectedRun ? (
            <RunDetail
              run={selectedRun}
              onRetry={() => {
                setDialogOpen(true);
              }}
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
