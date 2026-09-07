import type { Layout } from "react-grid-layout";
import type { Widget, WidgetType } from "@/components/WidgetView";

export type GridPos = { x: number; y: number; w: number; h: number };

export const DESKTOP_COLS = 12;
export const MOBILE_COLS = 4;
export const PHONE_FRAME_WIDTH = 390;

export function mobileHeightFor(type: WidgetType | string, desktopH: number) {
  if (type === "kpi" || type === "sparkline") return Math.max(2, Math.min(3, desktopH));
  if (type === "kpi_goal" || type === "metric_group") return Math.max(3, Math.min(4, desktopH));
  if (type === "slicer") return Math.max(3, Math.min(6, desktopH + 1));
  if (type === "table") return Math.max(4, desktopH);
  if (type === "text" || type === "markdown") return Math.max(2, desktopH);
  return Math.max(3, Math.min(5, desktopH));
}

export function readingOrder<T extends { layout: GridPos }>(widgets: T[]) {
  return [...widgets].sort((a, b) => a.layout.y - b.layout.y || a.layout.x - b.layout.x);
}

export function toLayoutItem(id: string, pos: GridPos, minW = 2, minH = 2): Layout {
  return { i: id, x: pos.x, y: pos.y, w: pos.w, h: pos.h, minW, minH };
}

export function resolveDesktopLayout(widgets: Widget[]): Layout[] {
  return widgets.map((w) => toLayoutItem(w.id, w.layout));
}

export function resolveMobileLayout(widgets: Widget[]): Layout[] {
  const withMobile = widgets.filter((w) => w.layoutMobile);
  if (withMobile.length === widgets.length && widgets.length > 0) {
    return widgets.map((w) => toLayoutItem(w.id, w.layoutMobile!));
  }
  let y = withMobile.reduce((max, w) => Math.max(max, (w.layoutMobile?.y || 0) + (w.layoutMobile?.h || 0)), 0);
  const items: Layout[] = [];
  for (const w of readingOrder(widgets)) {
    if (w.layoutMobile) {
      items.push(toLayoutItem(w.id, {
        ...w.layoutMobile,
        w: Math.min(MOBILE_COLS, Math.max(2, w.layoutMobile.w)),
      }));
      continue;
    }
    const h = mobileHeightFor(w.type, w.layout.h);
    items.push(toLayoutItem(w.id, { x: 0, y, w: MOBILE_COLS, h }));
    y += h;
  }
  return items;
}

export function applyMobileLayoutChange(widgets: Widget[], next: Layout[]): Widget[] {
  return widgets.map((w) => {
    const l = next.find((x) => x.i === w.id);
    if (!l) return w;
    return { ...w, layoutMobile: { x: l.x, y: l.y, w: l.w, h: l.h } };
  });
}

export function applyDesktopLayoutChange(widgets: Widget[], next: Layout[]): Widget[] {
  return widgets.map((w) => {
    const l = next.find((x) => x.i === w.id);
    if (!l) return w;
    return { ...w, layout: { x: l.x, y: l.y, w: l.w, h: l.h } };
  });
}
