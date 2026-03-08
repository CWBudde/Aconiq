import { useMemo } from "react";
import { AlertTriangle, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { useModelStore } from "@/model/model-store";
import { validateModel } from "@/model/validate";
import type { ValidationIssue } from "@/model/types";
import { m } from "@/i18n/messages";

interface ValidationPanelProps {
  onSelectFeature: (featureId: string) => void;
}

export function ValidationPanel({ onSelectFeature }: ValidationPanelProps) {
  const features = useModelStore((s) => s.features);
  const report = useMemo(() => validateModel(features), [features]);

  if (report.valid && report.warnings.length === 0) {
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
        {report.errors.length > 0
          ? `${String(report.errors.length)} ${m.msg_validation_errors_count()}`
          : ""}
        {report.errors.length > 0 && report.warnings.length > 0 ? ", " : ""}
        {report.warnings.length > 0
          ? `${String(report.warnings.length)} ${m.msg_validation_warnings_count()}`
          : ""}
      </div>
      <ul className="divide-y">
        {allIssues.map((issue, i) => (
          <li key={i} className="flex items-start gap-2 px-3 py-2">
            {issue.level === "error" ? (
              <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-destructive" />
            ) : (
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-yellow-500" />
            )}
            <div className="min-w-0 flex-1">
              <p className="text-xs">{issue.message}</p>
              <p className="font-mono text-[10px] text-muted-foreground">
                {issue.code}
              </p>
            </div>
            {issue.featureId ? (
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-[10px]"
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
