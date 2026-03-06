import { Label } from "@/ui/components/label";
import { Input } from "@/ui/components/input";
import { cn } from "@/ui/lib/utils";

interface FormFieldProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
  hint?: string;
}

export function FormField({
  label,
  error,
  hint,
  id,
  className,
  ...props
}: FormFieldProps) {
  const fieldId = id ?? label.toLowerCase().replace(/\s+/g, "-");
  const errorId = `${fieldId}-error`;
  const hintId = `${fieldId}-hint`;

  return (
    <div className={cn("grid gap-1.5", className)}>
      <Label htmlFor={fieldId} className={error ? "text-destructive" : ""}>
        {label}
      </Label>
      <Input
        id={fieldId}
        aria-invalid={error ? true : undefined}
        aria-describedby={
          [error ? errorId : null, hint ? hintId : null]
            .filter(Boolean)
            .join(" ") || undefined
        }
        className={error ? "border-destructive" : ""}
        {...props}
      />
      {hint && !error ? (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
      {error ? (
        <p id={errorId} className="text-xs text-destructive" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}
