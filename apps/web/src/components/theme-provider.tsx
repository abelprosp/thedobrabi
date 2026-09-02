"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { applyAppearance, isAppearance, readStoredAppearance, THEME_STORAGE_KEY, type Appearance } from "@/lib/theme";

type ThemeContextValue = {
  theme: Appearance;
  setTheme: (theme: Appearance) => void;
  toggle: () => void;
};

const ThemeContext = createContext<ThemeContextValue>({
  theme: "light",
  setTheme: () => {},
  toggle: () => {},
});

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Appearance>("light");

  useEffect(() => {
    const next = readStoredAppearance();
    setThemeState(next);
    applyAppearance(next);
  }, []);

  const setTheme = useCallback((next: Appearance) => {
    setThemeState(next);
    applyAppearance(next);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, next);
    } catch {
      /* ignore */
    }
  }, []);

  const toggle = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [setTheme, theme]);

  const value = useMemo(() => ({ theme, setTheme, toggle }), [theme, setTheme, toggle]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  return useContext(ThemeContext);
}

export function parseLayoutTheme(layout: unknown): Appearance | null {
  if (!layout || typeof layout !== "object") return null;
  const theme = (layout as { theme?: unknown }).theme;
  return isAppearance(theme) ? theme : null;
}
