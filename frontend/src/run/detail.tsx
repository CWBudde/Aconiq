import { useState } from "react";
import { Info, RefreshCw, Terminal } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Callout } from "@/ui/callout";
import { CopyButton } from "@/ui/copy-field";
import { LoadingLine } from "@/ui/loading-line";
import { SectionHeading } from "@/ui/page-header";
import { StatusBadge } from "@/ui/status-badge";
import { runTiming } from "@/ui/run-status";
import { useRunLog } from "@/api/hooks";
import type { ArtifactRef, RunSummary } from "@/api/client";
import { ProgressTimeline } from "@/run/timeline";
import { getStandardLabel } from "@/run/standards-meta";
import { m } from "@/i18n/messages";

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
//
// This is the run-artifact half of the vocabulary; `export.tsx` holds the
// export half in `EXPORT_KIND_LABELS`/`kindMeta`. Two tables for one `kind`
// namespace, and moving this one here did not fix that — unifying them belongs
// with the export page's own rework.
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
// Run detail panel
// ---------------------------------------------------------------------------

export function RunDetail({
  run,
  onRetry,
}: {
  run: RunSummary;
  onRetry: () => void;
}) {
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
          <span>{getStandardLabel(run.standard_id)}</span>
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

      {/* Actions. There is no Cancel: the API has no cancel endpoint and the
          WASM kernel completes inside `startRun`, so no capability flag would
          ever enable one. */}
      <section className="flex gap-2">
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
          {m.action_retry()}
        </Button>
      </section>
    </div>
  );
}
