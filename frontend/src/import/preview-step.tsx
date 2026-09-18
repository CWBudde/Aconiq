import { AlertTriangle, Info, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Callout } from "@/ui/callout";
import { KeyValueList } from "@/ui/key-value-list";
import { PageHeader } from "@/ui/page-header";
import { countModelObjects } from "@/model/model-store";
import { validationIssueText } from "@/model/validation-message";
import type { MergeSkips } from "@/model/model-store";
import type {
  CalcArea,
  ModelFeature,
  ModelReceiver,
  ValidationReport,
} from "@/model/types";
import { m } from "@/i18n/messages";

/** How many validation errors the preview lists before it summarises the rest. */
const PREVIEW_ERROR_LIMIT = 5;

/**
 * What the import found, before anything of it reaches the workspace: the
 * counts, the skipped features, and the validation findings.
 *
 * It holds no state. Every decision the reader can take from here — back, or
 * import — is the page's to make.
 */
export function PreviewStep({
  features,
  receivers,
  calcArea,
  skippedCount,
  report,
  workspaceEmpty,
  mergeSkips,
  onBack,
  onAdd,
  onReplace,
}: {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  skippedCount: number;
  /** null when there was nothing for the validator to check. */
  report: ValidationReport | null;
  /** No workspace to lose: one button, and no confirmation. */
  workspaceEmpty: boolean;
  /** What Add would leave behind, so the reader reads it before choosing. */
  mergeSkips: MergeSkips;
  onBack: () => void;
  onAdd: () => void;
  onReplace: () => void;
}) {
  const countByKind = (kind: ModelFeature["kind"]) =>
    String(features.filter((f) => f.kind === kind).length);

  const skippedOnMerge = mergeSkips.features + mergeSkips.receivers;
  const importedCount = countModelObjects({ features, receivers, calcArea });

  return (
    <div className="space-y-4">
      <PageHeader title={m.heading_import_preview()} />
      <Card className="space-y-3 p-4 text-sm">
        <p>
          {String(features.length)} {m.msg_features_normalized()}
        </p>
        {skippedCount > 0 ? (
          <p className="text-warning">
            {String(skippedCount)} {m.msg_features_skipped()}
          </p>
        ) : null}
        <KeyValueList
          items={[
            { label: m.label_sources(), value: countByKind("source") },
            {
              label: m.label_buildings(),
              value: countByKind("building"),
            },
            { label: m.label_barriers(), value: countByKind("barrier") },
            // Receivers and the calculation area are counted because the
            // import now carries them. Listing only source/building/barrier
            // was true of the wizard that dropped the other two.
            {
              label: m.label_receivers(),
              value: String(receivers.length),
            },
            ...(calcArea === null
              ? []
              : [
                  {
                    label: m.label_calc_area(),
                    value: m.msg_calc_area_included(),
                  },
                ]),
          ]}
        />
      </Card>

      {report && report.errors.length > 0 ? (
        <Callout
          variant="destructive"
          icon={XCircle}
          title={`${String(report.errors.length)} ${m.status_validation_errors()}`}
        >
          <ul className="space-y-1">
            {/* The id is named but not linked. These features are not in the
                store yet — the reader has not chosen Add or Replace — so
                there is nothing for a map selection to open. The done step
                links the findings that survived the import. */}
            {report.errors.slice(0, PREVIEW_ERROR_LIMIT).map((e, i) => (
              <li key={i}>
                {validationIssueText(e)}
                {e.featureId === "" ? null : (
                  <span className="ml-1 font-mono text-xs opacity-80">
                    {e.featureId}
                  </span>
                )}
              </li>
            ))}
            {report.errors.length > PREVIEW_ERROR_LIMIT ? (
              <li className="text-muted-foreground">
                {m.msg_and_more({
                  count: report.errors.length - PREVIEW_ERROR_LIMIT,
                })}
              </li>
            ) : null}
          </ul>
        </Callout>
      ) : null}

      {report && report.warnings.length > 0 ? (
        <Callout variant="warning" icon={AlertTriangle}>
          {String(report.warnings.length)} {m.status_validation_warnings()}
        </Callout>
      ) : null}

      {/* What Add would skip, said here rather than in a dialog: the reader
          decides between Add and Replace on this screen, so this is where the
          consequence has to be readable. */}
      {!workspaceEmpty && (skippedOnMerge > 0 || mergeSkips.calcArea) ? (
        <Callout variant="info" icon={Info}>
          <ul className="space-y-1">
            {skippedOnMerge > 0 ? (
              <li>{m.msg_import_skips_existing({ count: skippedOnMerge })}</li>
            ) : null}
            {mergeSkips.calcArea ? (
              <li>{m.msg_import_skips_calc_area()}</li>
            ) : null}
          </ul>
        </Callout>
      ) : null}

      <div className="flex flex-wrap gap-2">
        <Button variant="ghost" onClick={onBack}>
          {m.action_back()}
        </Button>
        {/* An empty workspace gets one button and no dialog, because there is
            nothing to lose and therefore nothing to choose between: Add and
            Replace would do the same thing. The asymmetry is deliberate — the
            choice appears exactly when it has a consequence. */}
        {workspaceEmpty ? (
          <Button onClick={onAdd}>
            {m.action_import_features({ count: importedCount })}
          </Button>
        ) : (
          <>
            <Button onClick={onAdd}>{m.action_import_add()}</Button>
            <Button variant="outline" onClick={onReplace}>
              {m.action_import_replace()}
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
