import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatPt(n: number, opts?: Intl.NumberFormatOptions) {
  return n.toLocaleString("pt-BR", opts);
}

export function isNumericValue(v: unknown) {
  if (typeof v === "number") return Number.isFinite(v);
  if (typeof v !== "string" || v.trim() === "") return false;
  return Number.isFinite(Number(v.replace(",", ".")));
}
