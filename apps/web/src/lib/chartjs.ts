"use client";

import { Chart as ChartJS } from "chart.js/auto";
import { chartChrome, type LegendPosition } from "@/lib/widget-config";

let registered = false;

export function registerCharts() {
  if (registered) return;
  ChartJS.defaults.font.family = "Inter, ui-sans-serif, system-ui, sans-serif";
  ChartJS.defaults.font.size = 11;
  ChartJS.defaults.animation = { duration: 450 };
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
