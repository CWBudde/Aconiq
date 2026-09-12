import { useSyncExternalStore } from "react";
import { Moon, Sun } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/ui/components/dropdown-menu";
import { useTheme } from "@/ui/theme-provider";
import { m } from "@/i18n/messages";

const DARK_SCHEME_QUERY = "(prefers-color-scheme: dark)";

function subscribeToScheme(onChange: () => void): () => void {
  const query = window.matchMedia(DARK_SCHEME_QUERY);
  query.addEventListener("change", onChange);
  return () => {
    query.removeEventListener("change", onChange);
  };
}

function prefersDark(): boolean {
  return window.matchMedia(DARK_SCHEME_QUERY).matches;
}

/**
 * The theme actually in effect: the stored preference, or the OS scheme when
 * the preference is `system`. The icon is chosen from this rather than
 * swapped through a per-theme class pair, which the design system reserves
 * for nothing — theme-conditional class patches bypass the token contract.
 */
function useResolvedTheme(): "light" | "dark" {
  const { theme } = useTheme();
  const systemDark = useSyncExternalStore(
    subscribeToScheme,
    prefersDark,
    () => false,
  );
  if (theme === "system") return systemDark ? "dark" : "light";
  return theme;
}

export function ThemeToggle() {
  const { setTheme } = useTheme();
  const resolved = useResolvedTheme();
  const Icon = resolved === "dark" ? Moon : Sun;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={m.tooltip_toggle_theme()}
        >
          <Icon aria-hidden="true" className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          onClick={() => {
            setTheme("light");
          }}
        >
          {m.theme_option_light()}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setTheme("dark");
          }}
        >
          {m.theme_option_dark()}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setTheme("system");
          }}
        >
          {m.theme_option_system()}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
