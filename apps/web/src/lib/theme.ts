export type Appearance = "light" | "dark";

export const THEME_STORAGE_KEY = "thedobra.theme";

export function isAppearance(v: unknown): v is Appearance {
  return v === "light" || v === "dark";
}

export function applyAppearance(theme: Appearance) {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.style.colorScheme = theme;
}

export function readStoredAppearance(): Appearance {
  try {
    const v = localStorage.getItem(THEME_STORAGE_KEY);
    if (isAppearance(v)) return v;
  } catch {
    /* ignore */
  }
  return "light";
}

export const THEME_BOOT_SCRIPT = `(function(){try{if(localStorage.getItem("${THEME_STORAGE_KEY}")==="dark"){document.documentElement.classList.add("dark");document.documentElement.style.colorScheme="dark"}}catch(e){}})();`;
