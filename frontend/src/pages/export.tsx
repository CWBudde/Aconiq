import { useState, useMemo } from "react";
import {
  Package,
  ExternalLink,
  Copy,
  Check,
  Loader2,
  AlertCircle,
  Info,
  ChevronRight,
  FileText,
  FileCode,
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
import { getArtifactContentURL, useCreateExport, useRuns } from "@/api/hooks";
import { IS_WASM_MODE } from "@/api/mode";
import type { ArtifactRef, RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString([], {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// ---------------------------------------------------------------------------
// Export artifact kind labels / icons
// ---------------------------------------------------------------------------

const EXPORT_KIND_LABELS: Record<
  string,
  { label: () => string; icon: React.ComponentType<{ className?: string }> }
> = {
  "export.bundle": { label: m.export_artifact_label_bundle, icon: Package },
  "export.report_html": {
    label: m.export_artifact_label_html_report,
    icon: FileText,
  },
  "export.report_markdown": {
    label: m.export_artifact_label_markdown_report,
    icon: FileCode,
  },
  "export.report_context_json": {
    label: m.export_artifact_label_json_context,
    icon: FileCode,
  },
};

function kindMeta(kind: string) {
  return EXPORT_KIND_LABELS[kind] ?? { label: () => kind, icon: Package };
}

// ---------------------------------------------------------------------------
// Copy button (with confirmation flash)
// ---------------------------------------------------------------------------

function CopyButton({ text, label }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);

  function handleCopy() {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => {
        setCopied(false);
      }, 1500);
    });
  }

  return (
    <Button variant="outline" size="sm" onClick={handleCopy}>
      {copied ? (
        <Check className="mr-1.5 h-3.5 w-3.5 text-green-600" />
      ) : (
        <Copy className="mr-1.5 h-3.5 w-3.5" />
      )}
      {copied ? m.status_copied() : (label ?? m.action_copy())}
    </Button>
  );
}

// ---------------------------------------------------------------------------
// Export artifact row
// ---------------------------------------------------------------------------

function ExportArtifactRow({ artifact }: { artifact: ArtifactRef }) {
  const { label: labelFn, icon: Icon } = kindMeta(artifact.kind);
  const label = labelFn();
  const filename = artifact.path.split("/").pop() ?? artifact.path;
  const contentURL = getArtifactContentURL(artifact.id);

  return (
    <div className="flex items-center justify-between gap-3 rounded-md border bg-muted/30 px-3 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        <div className="min-w-0">
          <p className="text-xs font-medium">{label}</p>
          <p
            className="truncate font-mono text-xs text-muted-foreground"
            title={artifact.path}
          >
            {filename}
          </p>
          <p className="text-xs text-muted-foreground">
            {formatTime(artifact.created_at)}
          </p>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {artifact.kind === "export.report_markdown" ? (
          <span className="rounded-full bg-secondary px-2 py-0.5 text-xs font-medium">
            View
          </span>
        ) : null}
        {artifact.kind === "export.report_html" ? (
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              window.open(contentURL, "_blank");
            }}
          >
            <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
            {m.action_open_in_browser()}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Right panel (selected run)
// ---------------------------------------------------------------------------

function ExportDetail({ run }: { run: RunSummary }) {
  const exportArtifacts = run.artifacts.filter((a) =>
    a.kind.startsWith("export."),
  );
  const htmlArtifact = exportArtifacts.find(
    (a) => a.kind === "export.report_html",
  );
  const cliCommand = `aconiq export --run-id ${run.id}`;

  return (
    <div className="flex flex-col gap-6 overflow-y-auto p-5">
      {/* Export artifacts */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {m.section_export_artifacts()}
        </h3>
        {exportArtifacts.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            {m.msg_no_artifacts_for_run()}
          </p>
        ) : (
          <div className="space-y-2">
            {exportArtifacts.map((a) => (
              <ExportArtifactRow key={a.id} artifact={a} />
            ))}
          </div>
        )}
      </section>

      {/* Report preview */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {m.section_report_preview()}
        </h3>
        {htmlArtifact ? (
          <iframe
            src={getArtifactContentURL(htmlArtifact.id)}
            sandbox="allow-scripts"
            width="100%"
            height="400"
            className="rounded-md border"
            title="HTML Report Preview"
          />
        ) : (
          <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-4 text-sm text-muted-foreground">
            <Info className="mt-0.5 h-4 w-4 shrink-0" />
            Run{" "}
            <code className="rounded bg-muted px-1">
              aconiq export --run-id {run.id}
            </code>{" "}
            to generate a report.
          </div>
        )}
      </section>

      {/* Typst PDF placeholder */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {m.section_typst_pdf()}
        </h3>
        <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          {m.msg_pdf_generation_planned()}
        </div>
      </section>

      {/* CLI command */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {m.section_cli_command()}
        </h3>
        <div className="flex items-center gap-3 rounded-md border bg-muted/50 px-3 py-2">
          <code className="flex-1 font-mono text-xs">{cliCommand}</code>
          <CopyButton text={cliCommand} />
        </div>
      </section>
    </div>
  );
}

// ---------------------------------------------------------------------------
// New Export dialog
// ---------------------------------------------------------------------------

function NewExportDialog({
  open,
  onClose,
  runs,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  runs: RunSummary[];
  onCreated: (runId: string) => void;
}) {
  const [selectedRunId, setSelectedRunId] = useState<string>("");
  const createExport = useCreateExport();
  const cliCommand = selectedRunId
    ? `aconiq export --run-id ${selectedRunId}`
    : "aconiq export --run-id <run-id>";

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{m.dialog_title_new_export()}</DialogTitle>
          <DialogDescription>
            {IS_WASM_MODE
              ? "Generate an offline export bundle directly in the browser."
              : m.dialog_desc_new_export()}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <p className="text-xs font-medium">{m.label_select_run()}</p>
            <Select
              value={selectedRunId || "_none"}
              onValueChange={(v) => {
                setSelectedRunId(v === "_none" ? "" : v);
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder={m.placeholder_select_run()} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_none">
                  {m.option_select_run_placeholder()}
                </SelectItem>
                {runs.map((r) => (
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

          {IS_WASM_MODE ? null : (
            <div className="space-y-1.5">
              <p className="text-xs font-medium">{m.label_command()}</p>
              <div className="flex items-center gap-3 rounded-md border bg-muted/50 px-3 py-2">
                <code className="flex-1 font-mono text-xs">{cliCommand}</code>
                <CopyButton text={cliCommand} />
              </div>
            </div>
          )}
          {createExport.isError ? (
            <p className="text-sm text-destructive">
              {createExport.error.message}
            </p>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {m.action_close()}
          </Button>
          {IS_WASM_MODE ? (
            <Button
              onClick={() => {
                if (!selectedRunId) return;
                createExport.mutate(selectedRunId, {
                  onSuccess: (run) => {
                    onCreated(run.id);
                    onClose();
                  },
                });
              }}
              disabled={!selectedRunId || createExport.isPending}
            >
              {createExport.isPending ? "Generating…" : m.action_new_export()}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Export list item (left panel)
// ---------------------------------------------------------------------------

function ExportListItem({
  run,
  selected,
  onClick,
}: {
  run: RunSummary;
  selected: boolean;
  onClick: () => void;
}) {
  const bundleArtifact = run.artifacts.find((a) => a.kind === "export.bundle");
  const exportCount = run.artifacts.filter((a) =>
    a.kind.startsWith("export."),
  ).length;

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
          <Package className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate font-mono text-xs text-muted-foreground">
            {run.id}
          </span>
        </div>
        <p className="mt-0.5 truncate text-sm">
          {run.standard_id}
          {run.version ? ` / ${run.version}` : ""}
        </p>
        <p className="text-xs text-muted-foreground">
          {bundleArtifact
            ? formatTime(bundleArtifact.created_at)
            : formatTime(run.finished_at)}{" "}
          · {String(exportCount)} artifact{exportCount !== 1 ? "s" : ""}
        </p>
      </div>
      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
    </button>
  );
}

// ---------------------------------------------------------------------------
// Export page
// ---------------------------------------------------------------------------

export default function ExportPage() {
  const { data: runs = [], isLoading, error } = useRuns(5_000);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  const runsWithExports = useMemo(
    () =>
      runs.filter((r) => r.artifacts.some((a) => a.kind.startsWith("export."))),
    [runs],
  );

  const selectedRun = useMemo(
    () => runsWithExports.find((r) => r.id === selectedRunId) ?? null,
    [runsWithExports, selectedRunId],
  );

  // Auto-select first.
  if (!selectedRun && runsWithExports.length > 0 && !isLoading) {
    setSelectedRunId(runsWithExports[0].id);
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
          {m.msg_api_error_export()}
        </div>
      </div>
    );
  }

  return (
    <>
      <div className="flex flex-1 overflow-hidden">
        {/* Left panel */}
        <div className="flex w-72 shrink-0 flex-col overflow-hidden border-r">
          <div className="flex items-center justify-between border-b px-4 py-3">
            <div>
              <h2 className="text-sm font-semibold">
                {m.page_title_exports()}
              </h2>
              <p className="text-xs text-muted-foreground">
                {String(runsWithExports.length)}{" "}
                {runsWithExports.length !== 1
                  ? m.msg_runs_with_exports_plural()
                  : m.msg_runs_with_exports()}
              </p>
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                setDialogOpen(true);
              }}
            >
              {m.action_new_export()}
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto">
            {runsWithExports.length === 0 ? (
              <div className="px-4 py-6 text-center">
                <Package className="mx-auto h-8 w-8 text-muted-foreground" />
                <p className="mt-2 text-xs text-muted-foreground">
                  {m.msg_no_exports_empty_state()}
                </p>
              </div>
            ) : (
              runsWithExports.map((run) => (
                <ExportListItem
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
            <ExportDetail run={selectedRun} />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              {m.msg_select_run_for_details()}
            </div>
          )}
        </div>
      </div>

      <NewExportDialog
        open={dialogOpen}
        onClose={() => {
          setDialogOpen(false);
        }}
        runs={runs}
        onCreated={(runId) => {
          setSelectedRunId(runId);
        }}
      />
    </>
  );
}
