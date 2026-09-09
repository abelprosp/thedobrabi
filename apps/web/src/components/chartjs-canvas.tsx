"use client";

import { Chart } from "react-chartjs-2";
import type { ChartType } from "chart.js";
import { registerCharts } from "@/lib/chartjs";

registerCharts();

export function ChartJsCanvas({
  type,
  data,
  options,
}: {
  type: ChartType;
  data: any;
  options?: any;
}) {
  return (
    <div className="relative h-full min-h-[7rem] w-full">
      <Chart
        type={type}
        data={data}
        options={{
          responsive: true,
          maintainAspectRatio: false,
          ...options,
        }}
      />
    </div>
  );
}
