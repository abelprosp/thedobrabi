import { useId } from "react";
import { cn } from "@/lib/cn";

/**
 * Símbolo TheDobra: três formas em gradiente indigo → azul → ciano
 * (barra pequena, barra média e a forma "D" aberta). Vectorial, escala sem perdas.
 */
export function LogoMark({ size = 32, className }: { size?: number; className?: string }) {
  const id = useId().replace(/:/g, "");
  const gradId = `td-mark-${id}`;
  // O símbolo original ocupa 96×57; adicionamos margens para ficar quadrado.
  return (
    <svg
      viewBox="0 0 104 104"
      width={size}
      height={size}
      className={cn("shrink-0", className)}
      aria-hidden
      focusable="false"
    >
      <defs>
        <linearGradient id={gradId} gradientUnits="userSpaceOnUse" x1="4" y1="0" x2="100" y2="0">
          <stop offset="0" stopColor="#6D3AFD" />
          <stop offset="0.3" stopColor="#4F46E5" />
          <stop offset="0.62" stopColor="#2563EB" />
          <stop offset="1" stopColor="#0EA5E9" />
        </linearGradient>
      </defs>
      <g transform="translate(4 23)" fill={`url(#${gradId})`}>
        <rect x="0" y="33" width="12" height="24" rx="2" />
        <rect x="17" y="15" width="13" height="42" rx="2" />
        <path d="M34 0H54.5A28.5 28.5 0 0 1 54.5 57H34V45H54.5A16.5 16.5 0 0 0 54.5 12H34Z" />
      </g>
    </svg>
  );
}

/**
 * Logo completo (símbolo + wordmark "TheDobra").
 *
 * - `variant="light"` — para fundos claros: texto em `--color-ink` (#0F172A).
 * - `variant="dark"`  — para fundos escuros: texto branco (como no ficheiro oficial).
 * - `size` — altura do símbolo em px; o wordmark escala proporcionalmente.
 * - `markOnly` — apenas o símbolo.
 */
export function Logo({
  variant = "light",
  size = 32,
  markOnly = false,
  className,
}: {
  variant?: "light" | "dark";
  size?: number;
  markOnly?: boolean;
  className?: string;
}) {
  if (markOnly) return <LogoMark size={size} className={className} />;
  // Proporção do wordmark relativa ao símbolo (o símbolo tem margem de ~23/104 em cima).
  const fontSize = Math.round(size * 0.56);
  return (
    <span className={cn("inline-flex items-center", className)} style={{ gap: Math.max(6, Math.round(size * 0.22)) }}>
      <LogoMark size={size} />
      <span
        className={cn("font-bold tracking-tight leading-none", variant === "dark" ? "text-white" : "text-ink")}
        style={{ fontSize, letterSpacing: "-0.03em" }}
      >
        TheDobra
      </span>
    </span>
  );
}
