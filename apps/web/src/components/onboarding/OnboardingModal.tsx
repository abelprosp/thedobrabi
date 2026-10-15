"use client";

import { useRouter, usePathname } from "next/navigation";
import { Button } from "@/components/ui";
import { ArrowRight, Sparkles, X } from "lucide-react";
import { useOnboarding } from "./useOnboarding";

/** Convite único para o assistente, para quem entra na app sem passar por /welcome (SSO, convites). */
export function OnboardingModal() {
  const router = useRouter();
  const pathname = usePathname();
  const { isLoading, completed, dismissed, dismiss } = useOnboarding();

  if (isLoading || completed || dismissed || pathname?.startsWith("/welcome")) return null;

  const start = () => {
    dismiss();
    router.push("/welcome");
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
      <div className="absolute inset-0 bg-ink/40 backdrop-blur-sm" onClick={dismiss} />
      <div className="relative w-full max-w-md overflow-hidden rounded-t-3xl border border-line bg-surface shadow-2xl sm:rounded-3xl">
        <div className="brand-gradient p-6 text-white">
          <div className="flex items-start justify-between">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/20">
              <Sparkles size={24} />
            </div>
            <button
              onClick={dismiss}
              className="flex h-9 w-9 items-center justify-center rounded-full text-white/80 hover:bg-white/20"
              aria-label="Fechar"
            >
              <X size={18} />
            </button>
          </div>
          <h2 className="mt-5 text-2xl font-semibold tracking-tight">Bem-vindo à TheDobra</h2>
          <p className="mt-2 text-sm text-white/85">
            Em 3 minutos deixamos os seus dados, o primeiro painel e a IA prontos. Pode saltar qualquer passo.
          </p>
        </div>
        <div className="space-y-3 p-6">
          <Button className="w-full" onClick={start}>
            Configurar agora <ArrowRight size={14} />
          </Button>
          <Button variant="ghost" className="w-full" onClick={dismiss}>
            Explorar sozinho
          </Button>
          <p className="text-center text-[11px] text-mute">Este aviso não volta a aparecer. O guia fica na página Início.</p>
        </div>
      </div>
    </div>
  );
}
