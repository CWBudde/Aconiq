import { ThemeProvider } from "@/ui/theme-provider";
import { AppShell } from "@/ui/app-shell";

export function App() {
  return (
    <ThemeProvider defaultTheme="system">
      <AppShell>
        <div className="flex flex-1 items-center justify-center p-8">
          <div className="text-center">
            <h2 className="text-2xl font-semibold tracking-tight">Aconiq</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Environmental noise modeling workspace
            </p>
          </div>
        </div>
      </AppShell>
    </ThemeProvider>
  );
}
