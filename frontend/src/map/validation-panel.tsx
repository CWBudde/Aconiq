import { AlertTriangle, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { useModelValidation } from "@/model/use-model-validation";
import { validationIssueText } from "@/model/validation-message";
import type { ValidationIssue } from "@/model/types";
import { m } from "@/i18n/messages";

interface ValidationPanelProps {
  onSelectFeature: (featureId: string) => void;
}

export function ValidationPanel({ onSelectFeature }: ValidationPanelProps) {
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

  const allIssues: ValidationIssue[] = [...report.errors, ...report.warnings];

  return (
    <div className="max-h-64 overflow-y-auto">
      <div className="border-b px-3 py-2 text-xs font-medium">
        {errorCount > 0
          ? errorCount === 1
            ? m.msg_validation_error_count_one({ count: errorCount })
            : m.msg_validation_error_count_other({ count: errorCount })
          : ""}
        {errorCount > 0 && warningCount > 0 ? ", " : ""}
        {warningCount > 0
          ? warningCount === 1
            ? m.msg_validation_warning_count_one({ count: warningCount })
            : m.msg_validation_warning_count_other({ count: warningCount })
          : ""}
      </div>
      <ul className="divide-y">
        {allIssues.map((issue, i) => (
          <li key={i} className="flex items-start gap-2 px-3 py-2">
            {issue.level === "error" ? (
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
              <p className="text-xs">{validationIssueText(issue)}</p>
              <p className="font-mono text-2xs text-muted-foreground">
                {issue.code}
              </p>
            </div>
            {issue.featureId ? (
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-2xs"
                onClick={() => {
                  onSelectFeature(issue.featureId);
                }}
              >
                {m.action_go_to()}
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
