export type Appearance = "light" | "dark";

/** @deprecated prefer SYSTEM_THEME_STORAGE_KEY */
export const THEME_STORAGE_KEY = "thedobra.theme";
export const SYSTEM_THEME_STORAGE_KEY = "thedobra.theme.system";
export const DASHBOARD_THEME_STORAGE_KEY = "thedobra.theme.dashboard";

export function isAppearance(v: unknown): v is Appearance {
  return v === "light" || v === "dark";
}

export function applyAppearance(theme: Appearance) {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.classList.remove("light");
  document.documentElement.style.colorScheme = theme;
}

function readKey(key: string): Appearance | null {
  try {
    const v = localStorage.getItem(key);
    return isAppearance(v) ? v : null;
  } catch {
    return null;
  }
}

export function writeStoredAppearance(theme: Appearance, key: string) {
  try {
    localStorage.setItem(key, theme);
  } catch {
    /* ignore */
  }
}

export function readStoredAppearance(): Appearance {
  return readKey(SYSTEM_THEME_STORAGE_KEY) || readKey(THEME_STORAGE_KEY) || "light";
}

export function writeStoredSystemAppearance(theme: Appearance) {
  writeStoredAppearance(theme, SYSTEM_THEME_STORAGE_KEY);
}

export function readStoredDashboardAppearance(): Appearance {
  return readKey(DASHBOARD_THEME_STORAGE_KEY) || "light";
}

export function writeStoredDashboardAppearance(theme: Appearance) {
  writeStoredAppearance(theme, DASHBOARD_THEME_STORAGE_KEY);
}

export const THEME_BOOT_SCRIPT = `(function(){try{var v=localStorage.getItem("${SYSTEM_THEME_STORAGE_KEY}")||localStorage.getItem("${THEME_STORAGE_KEY}");if(v==="dark"){document.documentElement.classList.add("dark");document.documentElement.style.colorScheme="dark"}else{document.documentElement.classList.remove("dark");document.documentElement.style.colorScheme="light"}}catch(e){}})();`;
