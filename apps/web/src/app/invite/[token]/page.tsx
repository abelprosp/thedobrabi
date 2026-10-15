"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { api, setTokens, type Tokens } from "@/lib/api";
import { toast } from "sonner";
import { AuthCard, Field, PrimaryButton } from "@/components/auth";
import { postLoginPath, type OnboardingUser } from "@/components/onboarding/useOnboarding";

export default function InvitePage() {
  const { token } = useParams<{ token: string }>();
  const router = useRouter();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [mfaCode, setMfaCode] = useState("");
  const [mfaRequired, setMfaRequired] = useState(false);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api<{
        tokens?: Tokens;
        mfa_required?: boolean;
        user?: OnboardingUser;
      }>(`/api/v1/invites/${token}/accept`, {
        method: "POST",
        body: JSON.stringify({
          name,
          password,
          ...(mfaRequired ? { mfa_code: mfaCode } : {}),
        }),
      });
      if (res.mfa_required) {
        setMfaRequired(true);
        toast.message("Introduza o código da app autenticadora");
        return;
      }
      if (!res.tokens) {
        toast.error("Não foi possível aceitar o convite");
        return;
      }
      setTokens(res.tokens);
      router.replace(postLoginPath(res.user));
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthCard
      title="Aceitar convite"
      subtitle={
        mfaRequired
          ? "Confirme a autenticação em dois passos da sua conta."
          : "Use a senha da sua conta (ou crie uma se for a primeira vez) para entrar na organização."
      }
    >
      <form onSubmit={onSubmit} className="space-y-3">
        {!mfaRequired && (
          <>
            <Field label="Nome" value={name} onChange={setName} icon="user" placeholder="O seu nome" />
            <Field
              label="Senha"
              value={password}
              onChange={setPassword}
              type="password"
              icon="lock"
              required
              minLength={8}
              placeholder="Mínimo 8 caracteres"
            />
          </>
        )}
        {mfaRequired && (
          <Field
            label="Código MFA"
            value={mfaCode}
            onChange={setMfaCode}
            icon="lock"
            required
            placeholder="000000"
          />
        )}
        <PrimaryButton busy={busy}>
          {busy ? "A entrar…" : mfaRequired ? "Confirmar MFA" : "Aceitar e entrar"}
        </PrimaryButton>
      </form>
    </AuthCard>
  );
}
