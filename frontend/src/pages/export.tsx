import { useState, useMemo } from "react";
import { useNavigate, useParams } from "react-router";
import {
  Package,
  ExternalLink,
  Loader2,
  AlertCircle,
  AlertTriangle,
  Info,
  FileText,
  FileCode,
  FileType,
  FileCheck,
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
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Callout } from "@/ui/callout";
import { CopyField } from "@/ui/copy-field";
import { EmptyState } from "@/ui/empty-state";
import { ModeGate } from "@/ui/mode-gate";
import { formatDateTime } from "@/ui/format";
import { ItemList, ListItem, MasterDetail } from "@/ui/master-detail";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { getArtifactContentURL, useCreateExport } from "@/api/hooks";
import { getStandardLabel } from "@/run/standards-meta";
import { useRunFromRoute } from "@/run/use-run-from-route";
import { backend } from "@/api/backend";
import { exportCommand } from "@/api/cli";
import type { ArtifactRef, RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Export artifact kind labels / icons
// ---------------------------------------------------------------------------

/**
 * Every artifact kind `aconiq export` can write, keyed exactly as the CLI
 * stamps it. A kind missing here falls through `kindMeta` and prints its own
 * identifier at the reader, so the table has to track `export.go`: every kind
 * it appends an `ArtifactRef` for needs a row here, added with it.
 */
const EXPORT_KIND_LABELS: Record<
  string,
  { label: () => string; icon: React.ComponentType<{ className?: string }> }
> = {
  "export.bundle": { label: m.export_artifact_label_bundle, icon: Package },
  "export.report_html": {
    label: m.export_artifact_label_html_report,
    icon: FileText,
  },
  "export.report_pdf": {
    label: m.export_artifact_label_pdf_report,
    icon: FileType,
  },
  "export.report_markdown": {
    label: m.export_artifact_label_markdown_report,
    icon: FileCode,
  },
  "export.report_context_json": {
    label: m.export_artifact_label_json_context,
    icon: FileCode,
  },
  "export.report_typst": {
    label: m.export_artifact_label_typst_report,
    icon: FileCode,
  },
  "export.assessment_16bimschv_json": {
    label: m.export_artifact_label_bimschv16_assessment,
    icon: FileCheck,
  },
};

function kindMeta(kind: string) {
  return EXPORT_KIND_LABELS[kind] ?? { label: () => kind, icon: Package };
}

function isExportArtifact(artifact: ArtifactRef): boolean {
  return artifact.kind.startsWith("export.");
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
    <li className="flex items-center justify-between gap-3 rounded-md border bg-muted/30 px-3 py-2">
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
            {formatDateTime(artifact.created_at)}
          </p>
        </div>
      </div>
      {artifact.kind === "export.report_html" ? (
        <Button
          variant="outline"
          size="sm"
          className="shrink-0"
          onClick={() => {
            window.open(contentURL, "_blank");
          }}
        >
          <ExternalLink aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
          {m.action_open_in_browser()}
        </Button>
      ) : null}
    </li>
  );
}

// ---------------------------------------------------------------------------
// Right panel (selected run)
// ---------------------------------------------------------------------------

function ExportDetail({ run }: { run: RunSummary }) {
  const exportArtifacts = run.artifacts.filter(isExportArtifact);
  const htmlArtifact = exportArtifacts.find(
    (a) => a.kind === "export.report_html",
  );

  return (
    <div className="flex flex-col gap-6 p-5">
      {/* Export artifacts */}
      <section>
        <SectionHeading variant="eyebrow" className="mb-2">
          {m.section_export_artifacts()}
        </SectionHeading>
        {exportArtifacts.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            {m.msg_no_artifacts_for_run()}
          </p>
        ) : (
          <ul className="m-0 list-none space-y-2 p-0">
            {exportArtifacts.map((a) => (
              <ExportArtifactRow key={a.id} artifact={a} />
            ))}
          </ul>
        )}
      </section>

      {/* Report preview */}
      <section>
        <SectionHeading variant="eyebrow" className="mb-2">
          {m.section_report_preview()}
        </SectionHeading>
        {htmlArtifact ? (
          <iframe
            src={getArtifactContentURL(htmlArtifact.id)}
            sandbox="allow-scripts"
            width="100%"
            height="400"
            className="rounded-md border"
            title={m.label_html_report_preview()}
          />
        ) : (
          <Callout variant="neutral" icon={Info}>
            {m.msg_no_html_report_yet()}
          </Callout>
        )}
      </section>

      {/* CLI command */}
      <section>
        <SectionHeading variant="eyebrow" className="mb-2">
          {m.section_cli_command()}
        </SectionHeading>
        <div className="space-y-2">
          <CopyField value={exportCommand(run.id)} />
          {/* `--pdf` compiles report.pdf with Typst beside the offline report
              bundle. Offered as its own line rather than behind a toggle: a
              read-only hand-off should be copyable at a glance, not stateful. */}
          <CopyField
            label={m.label_command_with_pdf()}
            value={exportCommand(run.id, { pdf: true })}
          />
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
  // No fallback here: `exportCommand` substitutes its own placeholder for a
  // blank id, because a blank `--run-id` exports the latest run rather than
  // the one the dialog has selected.
  const cliCommand = exportCommand(selectedRunId);

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
            {backend.capabilities.canExport
              ? m.dialog_desc_new_export_browser()
              : m.dialog_desc_new_export()}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="export-run" className="text-xs">
              {m.label_select_run()}
            </Label>
            <Select
              value={selectedRunId || "_none"}
              onValueChange={(v) => {
                setSelectedRunId(v === "_none" ? "" : v);
              }}
            >
              <SelectTrigger id="export-run">
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
                      ({getStandardLabel(r.standard_id)} / {r.version})
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {backend.capabilities.canExport ? null : (
            <CopyField label={m.label_command()} value={cliCommand} />
          )}
          {createExport.isError ? (
            <Callout variant="destructive" icon={AlertCircle}>
              {createExport.error.message}
            </Callout>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {m.action_close()}
          </Button>
          {/* Disabled with a reason rather than absent: a hidden button leaves
              the user hunting for a control that is not there. The CLI command
              above stays — the gate says why this is dead, the copy field says
              what to do instead. */}
          <ModeGate
            capability="canExport"
            reason={m.tooltip_export_unavailable()}
          >
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
              {createExport.isPending
                ? m.status_generating()
                : m.action_new_export()}
            </Button>
          </ModeGate>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Export page
// ---------------------------------------------------------------------------

/** The list row's second line: when the bundle was written, and how many files came with it. */
function exportMeta(run: RunSummary): string {
  const bundle = run.artifacts.find((a) => a.kind === "export.bundle");
  const count = run.artifacts.filter(isExportArtifact).length;
  const when = formatDateTime(bundle ? bundle.created_at : run.finished_at);
  const files =
    count === 1
      ? m.msg_artifact_count_one({ count })
      : m.msg_artifact_count_other({ count });
  return `${when} · ${files}`;
}

export default function ExportPage() {
  const { runId } = useParams();
  const navigate = useNavigate();
  const [dialogOpen, setDialogOpen] = useState(false);

  // Resolved against **every** run, so no eligibility predicate: the dialog
  // offers every run, and generating a bundle selects one that is not in the
  // list below yet. `ExportDetail` has a no-artifacts branch for that, and
  // calling such a run "unknown" would be a lie.
  const {
    runs,
    run: selectedRun,
    state,
    isLoading,
    error,
  } = useRunFromRoute(runId);

  const runsWithExports = useMemo(
    () => runs.filter((r) => r.artifacts.some(isExportArtifact)),
    [runs],
  );

  // The heading stays above both transient states so every state of the page
  // keeps its landmark structure (`waitForPage` in e2e/app.ts needs it).
  if (isLoading || error) {
    return (
      <div className="flex flex-1 flex-col">
        <PageHeader
          className="border-b px-4 py-3"
          title={m.page_title_exports()}
        />
        {error ? (
          <div className="flex flex-1 items-start justify-center p-8">
            <Callout variant="destructive" icon={AlertCircle}>
              {m.msg_api_error_export()}
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
    <>
      <MasterDetail
        listLabel={m.page_title_exports()}
        header={
          <PageHeader
            className="border-b px-4 py-3"
            title={m.page_title_exports()}
            description={
              <span className="text-xs">
                {String(runsWithExports.length)}{" "}
                {runsWithExports.length === 1
                  ? m.msg_runs_with_exports()
                  : m.msg_runs_with_exports_plural()}
              </span>
            }
            actions={
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  setDialogOpen(true);
                }}
              >
                {m.action_new_export()}
              </Button>
            }
          />
        }
        list={
          runsWithExports.length === 0 ? (
            <EmptyState
              compact
              icon={Package}
              title={m.msg_no_exports_empty_state()}
            />
          ) : (
            <ItemList>
              {runsWithExports.map((run) => (
                <ListItem
                  key={run.id}
                  selected={run.id === runId}
                  to={`/export/${run.id}`}
                  badge={
                    <Package
                      aria-hidden="true"
                      className="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                    />
                  }
                  code={run.id}
                  title={`${getStandardLabel(run.standard_id)}${run.version ? ` / ${run.version}` : ""}`}
                  meta={exportMeta(run)}
                />
              ))}
            </ItemList>
          )
        }
      >
        {selectedRun ? (
          <ExportDetail run={selectedRun} />
        ) : runId != null && state === "unknown" ? (
          <div className="flex flex-1 items-start justify-center p-8">
            <Callout variant="warning" icon={AlertTriangle}>
              {m.msg_unknown_run_id({ runId })}
            </Callout>
          </div>
        ) : (
          <EmptyState title={m.msg_select_run_for_details()} />
        )}
      </MasterDetail>

      <NewExportDialog
        open={dialogOpen}
        onClose={() => {
          setDialogOpen(false);
        }}
        runs={runs}
        onCreated={(createdRunId) => {
          // A push, not a replace: the user did a thing, and Back should undo
          // it. By the time this fires the runs list has already refetched —
          // `useCreateExport` awaits the invalidation in its own `onSuccess`.
          void navigate(`/export/${createdRunId}`);
        }}
      />
    </>
  );
}
