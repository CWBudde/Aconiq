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
import { useRuns } from "@/api/hooks";
import type { ArtifactRef, RunSummary } from "@/api/client";

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
  { label: string; icon: React.ComponentType<{ className?: string }> }
> = {
  "export.bundle": { label: "Export Bundle", icon: Package },
  "export.report_html": { label: "HTML Report", icon: FileText },
  "export.report_markdown": { label: "Markdown Report", icon: FileCode },
  "export.report_context_json": {
    label: "Report Context (JSON)",
    icon: FileCode,
  },
};

function kindMeta(kind: string) {
  return EXPORT_KIND_LABELS[kind] ?? { label: kind, icon: Package };
}

// ---------------------------------------------------------------------------
// Copy button (with confirmation flash)
// ---------------------------------------------------------------------------

function CopyButton({
  text,
  label = "Copy",
}: {
  text: string;
  label?: string;
}) {
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
      {copied ? "Copied!" : label}
    </Button>
  );
}

// ---------------------------------------------------------------------------
// Export artifact row
// ---------------------------------------------------------------------------

function ExportArtifactRow({ artifact }: { artifact: ArtifactRef }) {
  const { label, icon: Icon } = kindMeta(artifact.kind);
  const filename = artifact.path.split("/").pop() ?? artifact.path;
  const contentURL = `/api/v1/artifacts/${artifact.id}/content`;

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
            Open in browser
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
  const cliCommand = `noise export --run-id ${run.id}`;

  return (
    <div className="flex flex-col gap-6 overflow-y-auto p-5">
      {/* Export artifacts */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Export Artifacts
        </h3>
        {exportArtifacts.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            No export artifacts found for this run.
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
          Report Preview
        </h3>
        {htmlArtifact ? (
          <iframe
            src={`/api/v1/artifacts/${htmlArtifact.id}/content`}
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
              noise export --run-id {run.id}
            </code>{" "}
            to generate a report.
          </div>
        )}
      </section>

      {/* Typst PDF placeholder */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Typst PDF
        </h3>
        <div className="flex items-start gap-2 rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          PDF generation via Typst is planned for Phase 20b. HTML reports are
          available now.
        </div>
      </section>

      {/* CLI command */}
      <section>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          CLI Command
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
}: {
  open: boolean;
  onClose: () => void;
  runs: RunSummary[];
}) {
  const [selectedRunId, setSelectedRunId] = useState<string>("");
  const cliCommand = selectedRunId
    ? `noise export --run-id ${selectedRunId}`
    : "noise export --run-id <run-id>";

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>New Export</DialogTitle>
          <DialogDescription>
            Export bundles are created via the CLI. Select a run and copy the
            command below.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <p className="text-xs font-medium">Select run</p>
            <Select
              value={selectedRunId || "_none"}
              onValueChange={(v) => {
                setSelectedRunId(v === "_none" ? "" : v);
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a run…" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="_none">— Select a run —</SelectItem>
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

          <div className="space-y-1.5">
            <p className="text-xs font-medium">Command</p>
            <div className="flex items-center gap-3 rounded-md border bg-muted/50 px-3 py-2">
              <code className="flex-1 font-mono text-xs">{cliCommand}</code>
              <CopyButton text={cliCommand} />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
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
          Could not load runs. Is the API server running?
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
              <h2 className="text-sm font-semibold">Exports</h2>
              <p className="text-xs text-muted-foreground">
                {String(runsWithExports.length)} run
                {runsWithExports.length !== 1 ? "s" : ""} with exports
              </p>
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                setDialogOpen(true);
              }}
            >
              New Export
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto">
            {runsWithExports.length === 0 ? (
              <div className="px-4 py-6 text-center">
                <Package className="mx-auto h-8 w-8 text-muted-foreground" />
                <p className="mt-2 text-xs text-muted-foreground">
                  No exports yet. Use{" "}
                  <code className="rounded bg-muted px-1">noise export</code>{" "}
                  from the CLI to create an export bundle.
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
              Select a run to view its export artifacts.
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
      />
    </>
  );
}
