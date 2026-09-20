import { useState } from "react";
import { AlertTriangle, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { ConfirmDialog } from "@/ui/confirm-dialog";
import { useModelValidation } from "@/model/use-model-validation";
import { validationIssueText } from "@/model/validation-message";
import type { IssueSeverity, ValidationIssue } from "@/model/types";
import { m } from "@/i18n/messages";

interface ValidationPanelProps {
  onSelectFeature: (featureId: string) => void;
  /**
   * Accepts every source in a group of review findings at once.
   *
   * Offered only for {@link REVIEW_CODE}, which is the one finding in this
   * model that a reader retires by looking rather than by editing. Every other
   * row names a defect, and a button that made those disappear in bulk would be
   * a button that hides them.
   */
  onSignOffGroup: (featureIds: string[]) => void;
}

/** The one finding a reader can retire by accepting it. */
const REVIEW_CODE = "source.rls19.review_required";

/** How many member features an expanded group lists before it stops. */
const GROUP_PREVIEW = 20;

/** One finding, and every feature that carries it. */
interface IssueGroup {
  key: string;
  level: IssueSeverity;
  /**
   * The codes that render this sentence, in first-appearance order. Usually
   * one, occasionally two — `geometry.linestring.self_intersection` and its
   * `multilinestring` twin are one sentence — and all of them are shown,
   * because the code is what a reader greps for and printing only the first
   * would name a finding half the group does not have.
   */
  codes: string[];
  text: string;
  featureIds: string[];
}

/**
 * Collapses findings that say the same thing into one row each.
 *
 * The key is the severity and the rendered **sentence**, and deliberately not
 * the code. Several codes take parameters — `source.rls19.traffic.invalid`
 * names a different field each time — so grouping by code alone would put one
 * headline over rows it is only half true for; and several codes share one
 * sentence, so keeping the code in the key would split a group whose rows a
 * reader cannot tell apart, which is the complaint this whole panel answers.
 * Grouping by what the reader actually reads merges exactly the findings that
 * are indistinguishable on screen, which on an OSM import of a city district is
 * 608 copies of "check the imported acoustics".
 *
 * That also makes `validation-message.ts` the thing that decides what "the same
 * finding" means, which is the right place for the decision: it is already the
 * one place a code becomes prose.
 *
 * The severity stays in the key because it is not in the sentence — it is the
 * icon — and an error must never be filed under a warning's row.
 *
 * Groups come out in first-appearance order over errors then warnings, so the
 * errors still lead.
 */
function groupIssues(issues: ValidationIssue[]): IssueGroup[] {
  const groups = new Map<string, IssueGroup>();

  for (const issue of issues) {
    const text = validationIssueText(issue);
    const key = `${issue.level}|${text}`;

    const existing = groups.get(key);
    if (existing) {
      if (!existing.codes.includes(issue.code)) existing.codes.push(issue.code);
      if (issue.featureId !== "") existing.featureIds.push(issue.featureId);
      continue;
    }

    groups.set(key, {
      key,
      level: issue.level,
      codes: [issue.code],
      text,
      featureIds: issue.featureId === "" ? [] : [issue.featureId],
    });
  }

  return [...groups.values()];
}

export function ValidationPanel({
  onSelectFeature,
  onSignOffGroup,
}: ValidationPanelProps) {
  const { state, errorCount, warningCount, report } = useModelValidation();

  // An empty model is not a valid one, and with the map now mounted from the
  // start this panel can be opened before anything has been drawn.
  if (state === "empty") {
    return (
      <div className="p-3 text-center text-xs text-muted-foreground">
        {m.msg_validation_nothing_yet()}
      </div>
    );
  }

  if (state === "valid" || report === null) {
    return (
      <div className="p-3 text-center text-xs text-muted-foreground">
        {m.msg_model_valid()}
      </div>
    );
  }

  const groups = groupIssues([...report.errors, ...report.warnings]);

  return (
    <div className="max-h-64 overflow-y-auto">
      <div className="border-b px-3 py-2 text-xs font-medium">
        {errorCount > 0
          ? m.msg_validation_error_count({ count: errorCount })
          : ""}
        {errorCount > 0 && warningCount > 0 ? ", " : ""}
        {warningCount > 0
          ? m.msg_validation_warning_count({ count: warningCount })
          : ""}
      </div>
      <ul className="divide-y">
        {groups.map((group) => (
          <IssueGroupRow
            key={group.key}
            group={group}
            onSelectFeature={onSelectFeature}
            onSignOffGroup={onSignOffGroup}
          />
        ))}
      </ul>
    </div>
  );
}

function IssueGroupRow({
  group,
  onSelectFeature,
  onSignOffGroup,
}: {
  group: IssueGroup;
  onSelectFeature: (featureId: string) => void;
  onSignOffGroup: (featureIds: string[]) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const count = group.featureIds.length;
  const first = group.featureIds[0];
  // Every code in the group, not just the first: grouping is by sentence, so a
  // row can carry two codes, and accepting a group must not quietly accept a
  // finding that has no sign-off of its own.
  const signOffable =
    count > 0 && group.codes.every((code) => code === REVIEW_CODE);
  // The member rows are capped rather than virtualised. 608 of them would be
  // 608 mounted nodes inside a 256-pixel scroller to render a list nobody
  // reads to the end; the map flags and the stepper are the real answers to
  // "where are the others".
  const shown = expanded ? group.featureIds.slice(0, GROUP_PREVIEW) : [];
  const hidden = expanded ? count - shown.length : 0;

  return (
    <li className="px-3 py-2">
      <div className="flex items-start gap-2">
        {group.level === "error" ? (
          <XCircle
            aria-hidden="true"
            className="mt-0.5 size-3.5 shrink-0 text-destructive"
          />
        ) : (
          <AlertTriangle
            aria-hidden="true"
            className="mt-0.5 size-3.5 shrink-0 text-warning"
          />
        )}
        <div className="min-w-0 flex-1">
          <p className="text-xs">{group.text}</p>
          <p className="font-mono text-2xs text-muted-foreground">
            {group.codes.join(", ")}
          </p>
        </div>
        {count > 1 ? (
          // No plural key of its own: the two count messages already say
          // "608 Warnungen", which is exactly what this number means.
          <span
            className="rounded-full bg-secondary px-1.5 py-0.5 text-2xs tabular-nums"
            aria-label={
              group.level === "error"
                ? m.msg_validation_error_count({ count })
                : m.msg_validation_warning_count({ count })
            }
          >
            {String(count)}
          </span>
        ) : null}
        {first !== undefined ? (
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-2xs"
            onClick={() => {
              onSelectFeature(first);
            }}
          >
            {m.action_go_to()}
          </Button>
        ) : null}
      </div>
      {count > 1 ? (
        <div className="mt-1 pl-5">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-2xs"
            aria-expanded={expanded}
            onClick={() => {
              setExpanded((open) => !open);
            }}
          >
            {expanded ? m.action_show_less() : m.action_show_all()}
          </Button>
          {expanded ? (
            <ul className="mt-1 space-y-0.5">
              {shown.map((featureId) => (
                <li key={featureId} className="flex items-center gap-2">
                  <span className="min-w-0 flex-1 truncate font-mono text-2xs text-muted-foreground">
                    {featureId}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-5 px-2 text-2xs"
                    onClick={() => {
                      onSelectFeature(featureId);
                    }}
                  >
                    {m.action_go_to()}
                  </Button>
                </li>
              ))}
              {hidden > 0 ? (
                <li className="pt-0.5 text-2xs text-muted-foreground">
                  {m.msg_validation_more_findings({ count: hidden })}
                </li>
              ) : null}
            </ul>
          ) : null}
        </div>
      ) : null}
      {signOffable ? (
        <div className="mt-1 pl-5">
          <Button
            variant="outline"
            size="sm"
            className="h-6 px-2 text-2xs"
            onClick={() => {
              setConfirming(true);
            }}
          >
            {m.action_mark_all_acoustics_reviewed()}
          </Button>
          <ConfirmDialog
            open={confirming}
            onOpenChange={setConfirming}
            title={m.label_confirm_mark_all_reviewed()}
            description={m.msg_confirm_mark_all_reviewed({ count })}
            confirmLabel={m.action_mark_all_acoustics_reviewed()}
            onConfirm={() => {
              onSignOffGroup(group.featureIds);
            }}
          />
        </div>
      ) : null}
    </li>
  );
}
