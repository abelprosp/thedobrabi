"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, setTokens, type Tokens } from "@/lib/api";
import { toast } from "sonner";
import { AuthFormShell, Field, PrimaryButton } from "@/components/auth";

export default function SignupPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [org, setOrg] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api<{ tokens: Tokens; user?: { onboarding_step?: string; onboarding_completed?: boolean } }>("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify({ name, email, password, organization: org }),
      });
      setTokens(res.tokens);
      if (res.user?.onboarding_completed) {
        router.replace("/overview");
      } else {
        router.replace("/welcome");
      }
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthFormShell mode="signup">
      <form onSubmit={onSubmit} className="space-y-4">
        <Field label="O seu nome" value={name} onChange={setName} icon="user" required placeholder="Nome completo" />
        <Field label="Organização" value={org} onChange={setOrg} icon="org" required placeholder="Nome da organização" />
        <Field
          label="Endereço de e-mail"
          value={email}
          onChange={setEmail}
          type="email"
          icon="email"
          required
          placeholder="Introduza o seu e-mail"
        />
        <Field
          label="Senha"
          value={password}
          onChange={setPassword}
          type="password"
          icon="lock"
          required
          placeholder="Mínimo 8 caracteres"
          minLength={8}
          error={password.length > 0 && password.length < 8 ? "A senha deve ter pelo menos 8 caracteres" : undefined}
          autoComplete="new-password"
        />
        <PrimaryButton busy={busy}>{busy ? "A criar…" : "Criar conta"}</PrimaryButton>
      </form>
    </AuthFormShell>
  );
}
