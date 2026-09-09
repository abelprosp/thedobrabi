"use client";

import { DynamicIcon } from "lucide-react/dynamic";
import { cn } from "@/lib/cn";
import { hexToRgba } from "@/lib/widget-config";
import { isKpiIconUrl, isLikelyEmoji, kpiIconUrl, KPI_ICON_MAP, toKebabIconName } from "@/lib/kpi-icons";

export function KpiIcon({
  value,
  size = 20,
  color,
  className,
}: {
  value?: string;
  size?: number;
  color?: string;
  className?: string;
}) {
  const raw = (value || "").trim();
  if (!raw) return null;
  if (isKpiIconUrl(raw)) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={kpiIconUrl(raw)} alt="" width={size} height={size} className={cn("object-contain", className)} />
    );
  }
  if (isLikelyEmoji(raw)) {
    return (
      <span className={cn("leading-none", className)} style={{ fontSize: size }} aria-hidden>
        {raw}
      </span>
    );
  }
  const name = toKebabIconName(raw);
  const Curated = KPI_ICON_MAP.get(name);
  if (Curated) return <Curated size={size} color={color} className={className} />;
  return <DynamicIcon name={name as any} size={size} color={color} className={className} />;
}

export function KpiIconBadge({
  value,
  color,
  align,
  size = 22,
}: {
  value?: string;
  color?: string;
  align?: "left" | "center";
  size?: number;
}) {
  if (!value) return null;
  return (
    <div
      className={cn(
        "flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary",
        align === "center" && "mx-auto",
      )}
      style={color ? { backgroundColor: hexToRgba(color, 0.14), color } : undefined}
    >
      <KpiIcon value={value} size={size} color={color} />
    </div>
  );
}
