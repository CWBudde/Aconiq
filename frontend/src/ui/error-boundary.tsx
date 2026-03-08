import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Uncaught error:", error, errorInfo);
  }

  render() {
    if (this.state.error) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8">
          <div className="text-center">
            <h2 className="text-lg font-semibold">{m.error_heading()}</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {this.state.error.message}
            </p>
          </div>
          <Button
            variant="outline"
            onClick={() => {
              this.setState({ error: null });
            }}
          >
            {m.action_try_again()}
          </Button>
        </div>
      );
    }

    return this.props.children;
  }
}
