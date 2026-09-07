"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { cn } from "@/lib/cn";
import {
  applyAppearance,
  isAppearance,
  readStoredAppearance,
  readStoredDashboardAppearance,
  writeStoredDashboardAppearance,
  writeStoredSystemAppearance,
  type Appearance,
} from "@/lib/theme";

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

const AppearanceScopeContext = createContext<Appearance | null>(null);

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
    writeStoredSystemAppearance(next);
  }, []);

  const toggle = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [setTheme, theme]);

  const value = useMemo(() => ({ theme, setTheme, toggle }), [theme, setTheme, toggle]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function AppearanceScope({
  appearance,
  className,
  children,
}: {
  appearance: Appearance;
  className?: string;
  children: ReactNode;
}) {
  return (
    <AppearanceScopeContext.Provider value={appearance}>
      <div
        className={cn(appearance === "dark" ? "dark" : "light", className)}
        data-appearance={appearance}
        style={{ colorScheme: appearance }}
      >
        {children}
      </div>
    </AppearanceScopeContext.Provider>
  );
}

export function useTheme() {
  const global = useContext(ThemeContext);
  const scoped = useContext(AppearanceScopeContext);
  if (!scoped) return global;
  return { ...global, theme: scoped };
}

export function useSystemTheme() {
  return useContext(ThemeContext);
}

export function useDashboardThemePreference() {
  const [theme, setThemeState] = useState<Appearance>("light");

  useEffect(() => {
    setThemeState(readStoredDashboardAppearance());
  }, []);

  const setTheme = useCallback((next: Appearance) => {
    setThemeState(next);
    writeStoredDashboardAppearance(next);
  }, []);

  return { theme, setTheme };
}

export function parseLayoutTheme(layout: unknown): Appearance | null {
  if (!layout || typeof layout !== "object") return null;
  const theme = (layout as { theme?: unknown }).theme;
  return isAppearance(theme) ? theme : null;
}
