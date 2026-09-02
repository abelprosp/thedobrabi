"use client";

import ReactECharts from "echarts-for-react";
import { Card } from "@/components/ui";

type ChartProps = {
  type?: "line" | "bar" | "area" | "pie";
  title?: string;
  columns?: string[];
  rows?: Record<string, any>[];
  height?: number;
  onClick?: (payload: { dimension: string; value: string }) => void;
};

// Paleta da marca: azul primário, indigo, ciano e complementares.
const PRIMARY = "#2563EB";
const PALETTE = ["#2563EB", "#6366F1", "#0EA5E9", "#F59E0B", "#8B5CF6", "#10B981"];

const tooltip = {
  backgroundColor: "#ffffff",
  borderColor: "#e2e8f0",
  textStyle: { color: "#0f172a", fontSize: 12 },
  extraCssText: "box-shadow: 0 8px 24px rgba(15,23,42,0.08); border-radius: 10px;",
};

const ISO_DATE = /^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/;

/** Formata categorias que sejam datas ISO (ex.: 2025-10-26T00:00:00Z → 26/10/2025). */
function formatCategory(v: unknown) {
  const s = String(v ?? "");
  if (!ISO_DATE.test(s)) return s;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  const hasTime = s.includes("T") && !/T00:00(:00(\.0+)?)?(Z|[+-]00:?00)?$/.test(s);
  return d.toLocaleDateString("pt-BR", hasTime ? { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" } : { day: "2-digit", month: "2-digit", year: "numeric" });
}

export function Chart({ type = "bar", title, columns = [], rows = [], height = 280, onClick }: ChartProps) {
  const dim = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
  const rawCats = rows.map((r) => String(r[dim] ?? ""));
  const cats = rows.map((r) => formatCategory(r[dim]));
  const meas = columns.find((c) => c !== dim) || columns[1] || columns[0];
  const data = rows.map((r) => Number(r[meas] ?? 0));
  const handleClick = (params: any) => {
    if (!onClick) return;
    if (type === "pie") {
      onClick({ dimension: dim, value: params?.name ?? "" });
    } else {
      const idx = params?.dataIndex ?? 0;
      onClick({ dimension: dim, value: rawCats[idx] ?? "" });
    }
  };

  const option =
    type === "pie"
      ? {
          backgroundColor: "transparent",
          tooltip: { ...tooltip, trigger: "item", formatter: "{b}: {c} ({d}%)" },
          color: PALETTE,
          series: [{ type: "pie", radius: ["45%", "70%"], data: cats.map((n, i) => ({ name: n, value: data[i] })) }],
        }
      : {
          backgroundColor: "transparent",
          title: title ? { text: title, left: 0, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
          tooltip: { ...tooltip, trigger: "axis" },
          grid: { left: 52, right: 16, top: title ? 36 : 16, bottom: 36 },
          xAxis: {
            type: "category",
            data: cats,
            axisLine: { lineStyle: { color: "#e2e8f0" } },
            axisTick: { show: false },
            axisLabel: { color: "#5b6470", fontSize: 11 },
          },
          yAxis: {
            type: "value",
            splitLine: { lineStyle: { color: "#f1f5f9" } },
            axisLabel: { color: "#5b6470", fontSize: 11 },
          },
          series: [
            {
              type: type === "area" ? "line" : type,
              data,
              smooth: true,
              barMaxWidth: 36,
              areaStyle:
                type === "area" || type === "line"
                  ? {
                      color: {
                        type: "linear",
                        x: 0,
                        y: 0,
                        x2: 0,
                        y2: 1,
                        colorStops: [
                          { offset: 0, color: "rgba(37,99,235,0.22)" },
                          { offset: 1, color: "rgba(37,99,235,0.02)" },
                        ],
                      },
                    }
                  : undefined,
              lineStyle: { color: PRIMARY, width: 2 },
              itemStyle: { color: PRIMARY, borderRadius: type === "bar" ? [6, 6, 0, 0] : 0 },
            },
          ],
        };

  return <ReactECharts option={option} style={{ height }} notMerge onEvents={{ click: handleClick }} />;
}

export function Kpi({ label, value, delta }: { label: string; value: string; delta?: number }) {
  const pos = delta === undefined ? null : delta >= 0;
  return (
    <Card>
      <div className="text-[12px] uppercase tracking-wide text-mute">{label}</div>
      <div className="mt-2 text-3xl font-semibold tracking-tight text-ink">{value}</div>
      {delta !== undefined && (
        <div className={`mt-2 text-[12px] ${pos ? "text-ok" : "text-danger"}`}>
          {pos ? "+" : ""}
          {delta.toFixed(1)}% vs. período anterior
        </div>
      )}
    </Card>
  );
}
