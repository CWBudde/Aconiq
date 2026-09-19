import { createContext, useContext, useEffect, useState } from "react";

type Theme = "dark" | "light" | "system";

interface ThemeProviderProps {
  children: React.ReactNode;
  defaultTheme?: Theme;
  storageKey?: string;
}

interface ThemeProviderState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

const initialState: ThemeProviderState = {
  theme: "system",
  setTheme: () => undefined,
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

const DARK_SCHEME_QUERY = "(prefers-color-scheme: dark)";

/**
 * Where the chosen theme is remembered.
 *
 * Exported because the settings page lists the keys this app writes to
 * `localStorage`, and a key spelled a second time there is a key that goes
 * stale the moment this one changes.
 */
export const THEME_STORAGE_KEY = "aconiq-theme";

export function ThemeProvider({
  children,
  defaultTheme = "system",
  storageKey = THEME_STORAGE_KEY,
  ...props
}: ThemeProviderProps) {
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem(storageKey) as Theme | null) ?? defaultTheme,
  );

  useEffect(() => {
    const root = window.document.documentElement;

    const apply = (resolved: "light" | "dark") => {
      root.classList.remove("light", "dark");
      root.classList.add(resolved);
    };

    if (theme !== "system") {
      apply(theme);
      return;
    }

    // Under `system` the palette follows the OS, which can change while the
    // app is open. Without this subscription the class stays on whichever
    // scheme was current when the preference was chosen, and anything that
    // resolves the scheme itself — the theme toggle's icon — drifts out of
    // step with the palette actually painted.
    const query = window.matchMedia(DARK_SCHEME_QUERY);
    const sync = () => {
      apply(query.matches ? "dark" : "light");
    };
    sync();
    query.addEventListener("change", sync);
    return () => {
      query.removeEventListener("change", sync);
    };
  }, [theme]);

  const value = {
    theme,
    setTheme: (newTheme: Theme) => {
      localStorage.setItem(storageKey, newTheme);
      setTheme(newTheme);
    },
  };

  return (
    <ThemeProviderContext {...props} value={value}>
      {children}
    </ThemeProviderContext>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTheme() {
  return useContext(ThemeProviderContext);
}
