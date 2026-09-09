"use client";

import { Chart } from "react-chartjs-2";
import type { ChartType } from "chart.js";
import { registerCharts } from "@/lib/chartjs";

registerCharts();

function finiteNum(v: unknown, fallback = 0) {
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function sanitizeData(data: any) {
  if (!data?.datasets) return data;
  return {
    ...data,
    labels: data.labels,
    datasets: data.datasets.map((ds: any) => ({
      ...ds,
      data: Array.isArray(ds.data)
        ? ds.data.map((v: any) => {
            if (v == null) return 0;
            if (typeof v === "number") return finiteNum(v);
            if (typeof v === "object") {
              const next = { ...v };
              if ("x" in next) next.x = finiteNum(next.x);
              if ("y" in next) next.y = finiteNum(next.y);
              if ("r" in next) next.r = Math.max(0, finiteNum(next.r, 4));
              return next;
            }
            return finiteNum(v);
          })
        : ds.data,
    })),
  };
}

function stripUndefined(value: any): any {
  if (value === undefined) return value;
  if (Array.isArray(value)) return value.map(stripUndefined);
  if (value && typeof value === "object" && Object.getPrototypeOf(value) === Object.prototype) {
    const out: Record<string, any> = {};
    for (const [k, v] of Object.entries(value)) {
      if (v === undefined) continue;
      out[k] = stripUndefined(v);
    }
    return out;
  }
  return value;
}

export function ChartJsCanvas({
  type,
  data,
  options,
}: {
  type: ChartType;
  data: any;
  options?: any;
}) {
  const animation = options?.animation;
  const merged = stripUndefined({
    responsive: true,
    maintainAspectRatio: false,
    ...options,
    animation:
      animation === false
        ? false
        : { duration: 450, ...(typeof animation === "object" && animation ? animation : {}) },
  });

  return (
    <div className="relative h-full min-h-[7rem] w-full">
      <Chart key={type} type={type} data={sanitizeData(data)} options={merged} />
    </div>
  );
}
