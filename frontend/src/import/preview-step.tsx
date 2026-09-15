import { AlertTriangle, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Callout } from "@/ui/callout";
import { KeyValueList } from "@/ui/key-value-list";
import { PageHeader } from "@/ui/page-header";
import type { ModelFeature, ValidationReport } from "@/model/types";
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
  skippedCount,
  report,
  onBack,
  onImport,
}: {
  features: ModelFeature[];
  skippedCount: number;
  report: ValidationReport;
  onBack: () => void;
  onImport: () => void;
}) {
  const countByKind = (kind: ModelFeature["kind"]) =>
    String(features.filter((f) => f.kind === kind).length);

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
          ]}
        />
      </Card>

      {report.errors.length > 0 ? (
        <Callout
          variant="destructive"
          icon={XCircle}
          title={`${String(report.errors.length)} ${m.status_validation_errors()}`}
        >
          <ul className="space-y-1">
            {report.errors.slice(0, PREVIEW_ERROR_LIMIT).map((e, i) => (
              <li key={i}>{e.message}</li>
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

      {report.warnings.length > 0 ? (
        <Callout variant="warning" icon={AlertTriangle}>
          {String(report.warnings.length)} {m.status_validation_warnings()}
        </Callout>
      ) : null}

      <div className="flex gap-2">
        <Button variant="ghost" onClick={onBack}>
          {m.action_back()}
        </Button>
        <Button onClick={onImport}>
          {m.action_import_features({ count: features.length })}
        </Button>
      </div>
    </div>
  );
}
