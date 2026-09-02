"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, setTokens, type Tokens } from "@/lib/api";
import { toast } from "sonner";
import { AuthCard, Field, PrimaryButton } from "@/components/auth";
import { Button } from "@/components/ui";
import { Sparkles, X } from "lucide-react";

export default function InvitePage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [showWelcome, setShowWelcome] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api<{ tokens: Tokens; user?: { onboarding_step?: string; onboarding_completed?: boolean } }>(`/api/v1/invites/${token}/accept`, {
        method: "POST",
        body: JSON.stringify({ name, password }),
      });
      setTokens(res.tokens);
      if (res.user?.onboarding_completed) {
        router.replace("/overview");
      } else {
        setShowWelcome(true);
      }
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthCard title="Aceitar convite" subtitle="Crie a senha para entrar na organização.">
      <form onSubmit={onSubmit} className="space-y-3">
        <Field label="Nome" value={name} onChange={setName} icon="user" required placeholder="O seu nome" />
        <Field label="Senha" value={password} onChange={setPassword} type="password" icon="lock" required minLength={8} placeholder="Mínimo 8 caracteres" />
        <PrimaryButton busy={busy}>{busy ? "A entrar…" : "Aceitar e entrar"}</PrimaryButton>
      </form>
      {showWelcome && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-ink/40 backdrop-blur-sm" onClick={() => setShowWelcome(false)} />
          <div className="relative w-full max-w-md rounded-3xl border border-line bg-white p-6 shadow-2xl">
            <button onClick={() => setShowWelcome(false)} className="absolute top-4 right-4 text-mute hover:text-ink" aria-label="Fechar">
              <X size={18} />
            </button>
            <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <Sparkles size={28} />
            </div>
            <h2 className="text-center text-xl font-semibold text-ink">Bem-vindo à equipa</h2>
            <p className="mt-2 text-center text-[13px] text-mute">
              Já tem acesso à organização. Comece pela visão geral para descobrir os dados e dashboards.
            </p>
            <div className="mt-5 flex justify-center gap-2">
              <Button variant="secondary" onClick={() => setShowWelcome(false)}>
                Explorar depois
              </Button>
              <Button onClick={() => router.replace("/overview")}>Ir para visão geral</Button>
            </div>
          </div>
        </div>
      )}
    </AuthCard>
  );
}
