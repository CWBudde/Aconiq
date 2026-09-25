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
 * What confirming an LGLN load will do, planned by `planReplaceBuildings`
 * before the reader chooses — and what the data must be shown with.
 */
export interface LglnPreview {
  /** OSM buildings inside the box that the replacement removes. */
  removed: number;
  /** LGLN buildings that land (ids the workspace already holds are skipped). */
  added: number;
  /** False when the workspace is projected, so nothing could be compared. */
  comparable: boolean;
  tileCount: number;
  attribution: string;
}

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
  lgln = null,
  onBack,
  onAdd,
  onReplace,
  onReplaceBuildings,
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
  /**
   * Set when the import came from the LGLN tab. The choice between Add and
   * Replace then gives way to the one thing an LGLN load is for: replacing
   * the OSM buildings in its box, which {@link onReplaceBuildings} does.
   */
  lgln?: LglnPreview | null;
  onBack: () => void;
  onAdd: () => void;
  onReplace: () => void;
  onReplaceBuildings?: () => void;
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
            {
              label: m.label_ground_zones(),
              value: countByKind("ground-zone"),
            },
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
          title={m.status_validation_errors({ count: report.errors.length })}
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
          {m.msg_validation_warning_count({ count: report.warnings.length })}
        </Callout>
      ) : null}

      {lgln === null ? null : (
        <Callout variant="info" icon={Info}>
          <ul className="space-y-1">
            {lgln.comparable ? (
              <>
                <li>{m.msg_lgln_replaces_osm({ count: lgln.removed })}</li>
                <li>{m.msg_lgln_adds({ count: lgln.added })}</li>
              </>
            ) : (
              <li>{m.msg_lgln_workspace_projected()}</li>
            )}
          </ul>
          <p className="mt-2 text-xs text-muted-foreground">
            {m.msg_lgln_tiles_read({ count: lgln.tileCount })}{" "}
            {lgln.attribution}
          </p>
        </Callout>
      )}

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
        {lgln !== null ? (
          // Named for what it does: with no OSM building in the box there is
          // nothing to replace, and the button is a plain import.
          // Disabled for a projected workspace: the buildings are in degrees
          // and would be stored as metres.
          <Button onClick={onReplaceBuildings} disabled={!lgln.comparable}>
            {lgln.comparable && lgln.removed > 0
              ? m.action_import_lgln_apply()
              : m.action_import_features({ count: importedCount })}
          </Button>
        ) : workspaceEmpty ? (
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
