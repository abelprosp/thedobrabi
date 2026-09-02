"use client";

import { useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { AuthCard, Field, PrimaryButton } from "@/components/auth";

export default function ForgotPage() {
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api("/api/v1/auth/forgot", { method: "POST", body: JSON.stringify({ email }) });
      setSent(true);
      toast.success("Se a conta existir, enviámos a ligação de recuperação");
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthCard title="Recuperar senha" subtitle="Enviamos um e-mail se a conta existir.">
      {sent ? (
        <p className="text-sm text-mute">Verifique a caixa de entrada. Em desenvolvimento o link também aparece nos logs da API.</p>
      ) : (
        <form onSubmit={onSubmit} className="space-y-3">
          <Field label="E-mail" value={email} onChange={setEmail} type="email" icon="email" required placeholder="Introduza o seu e-mail" autoComplete="email" />
          <PrimaryButton busy={busy}>{busy ? "A enviar…" : "Enviar ligação"}</PrimaryButton>
        </form>
      )}
      <p className="mt-4 text-center text-[12px] text-mute">
        <Link href="/login" className="text-accent hover:underline">
          Voltar ao login
        </Link>
      </p>
    </AuthCard>
  );
}
