"use client";

import { Chart as ChartJS, Animation } from "chart.js/auto";
import { chartChrome, type LegendPosition } from "@/lib/widget-config";

let registered = false;

function fallbackInterpolate(from: unknown, to: unknown, factor: number) {
  if (typeof from === "number" && typeof to === "number") {
    return from + (to - from) * factor;
  }
  return factor < 1 ? from : to;
}

function patchAnimationTicker() {
  const proto = Animation.prototype as Animation & { _fn?: unknown; tick: (date: number) => void };
  const original = proto.tick;
  if ((original as { _dobraPatched?: boolean })._dobraPatched) return;
  function tick(this: { _fn?: unknown }, date: number) {
    if (typeof this._fn !== "function") this._fn = fallbackInterpolate;
    return original.call(this, date);
  }
  (tick as { _dobraPatched?: boolean })._dobraPatched = true;
  proto.tick = tick;
}

export function registerCharts() {
  if (registered) return;
  patchAnimationTicker();
  ChartJS.defaults.font.family = "Inter, ui-sans-serif, system-ui, sans-serif";
  ChartJS.defaults.font.size = 11;
  if (ChartJS.defaults.animation && typeof ChartJS.defaults.animation === "object") {
    ChartJS.defaults.animation.duration = 450;
  }
  registered = true;
}

export function chartLegend(show: boolean, position: LegendPosition = "top", theme?: "light" | "dark") {
  const c = chartChrome(theme);
  return {
    display: show,
    position,
    labels: { color: c.mute, boxWidth: 10, boxHeight: 10, padding: 12, font: { size: 11 } },
  };
}

export function chartTooltip(theme?: "light" | "dark") {
  const c = chartChrome(theme);
  return {
    enabled: true,
    backgroundColor: c.surface,
    titleColor: c.ink,
    bodyColor: c.ink,
    borderColor: c.line,
    borderWidth: 1,
    cornerRadius: 10,
    padding: 10,
    displayColors: true,
  };
}

export const WEEKDAYS_PT = ["Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom"];
