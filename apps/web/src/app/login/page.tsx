"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { Suspense } from "react";
import { api, setTokens, type Tokens } from "@/lib/api";
import { toast } from "sonner";
import { AuthFormShell, Field, PrimaryButton } from "@/components/auth";

export default function LoginPage() {
  return (
    <Suspense>
      <LoginInner />
    </Suspense>
  );
}

function LoginInner() {
  const router = useRouter();
  const params = useSearchParams();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [providers, setProviders] = useState<Record<string, boolean>>({});
  const [mfaToken, setMfaToken] = useState("");
  const [mfaCode, setMfaCode] = useState("");
  const [errors, setErrors] = useState<{ email?: string; password?: string; mfa?: string }>({});

  useEffect(() => {
    api<Record<string, boolean>>("/api/v1/auth/oauth/providers").then(setProviders).catch(() => {});
    const err = params.get("erro");
    if (err) toast.error(err);
  }, [params]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const next: typeof errors = {};
    if (!email.trim()) next.email = "Indique o e-mail";
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) next.email = "E-mail inválido";
    if (!mfaToken && !password) next.password = "Indique a senha";
    if (mfaToken && !mfaCode.trim()) next.mfa = "Indique o código MFA";
    setErrors(next);
    if (Object.keys(next).length) return;
    setBusy(true);
    try {
      if (mfaToken) {
        const res = await api<{ tokens: Tokens; user?: { onboarding_step?: string; onboarding_completed?: boolean } }>("/api/v1/auth/mfa/verify", {
          method: "POST",
          body: JSON.stringify({ mfa_token: mfaToken, code: mfaCode }),
        });
        setTokens(res.tokens);
        if (res.user?.onboarding_completed) {
          router.replace("/overview");
        } else {
          router.replace("/welcome");
        }
        return;
      }
      const res = await api<{ tokens?: Tokens; mfa_required?: boolean; mfa_token?: string; user?: { onboarding_step?: string; onboarding_completed?: boolean } }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      if (res.mfa_required && res.mfa_token) {
        setMfaToken(res.mfa_token);
        toast.message("Introduza o código da app autenticadora");
        return;
      }
      if (res.tokens) {
        setTokens(res.tokens);
        if (res.user?.onboarding_completed) {
          router.replace("/overview");
        } else {
          router.replace("/welcome");
        }
      }
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }

  const oauth = [
    providers.google && { id: "google", href: "/api/v1/auth/oauth/google/start", label: "Google", icon: GoogleIcon },
    providers.github && { id: "github", href: "/api/v1/auth/oauth/github/start", label: "GitHub", icon: GitHubIcon },
    providers.oidc && { id: "oidc", href: "/api/v1/auth/oauth/oidc/start", label: "OIDC", icon: OidcIcon },
  ].filter(Boolean) as { id: string; href: string; label: string; icon: typeof GoogleIcon }[];

  return (
    <AuthFormShell mode="login">
      <form onSubmit={onSubmit} className="space-y-4">
        <Field
          label="Endereço de e-mail"
          value={email}
          onChange={setEmail}
          type="email"
          icon="email"
          required
          placeholder="Introduza o seu e-mail"
          error={errors.email}
          autoComplete="email"
        />
        <Field
          label="Senha"
          value={password}
          onChange={setPassword}
          type="password"
          icon="lock"
          required
          placeholder="Introduza a sua senha"
          error={errors.password}
          autoComplete="current-password"
        />
        {mfaToken && (
          <Field label="Código MFA" value={mfaCode} onChange={setMfaCode} required placeholder="Código de 6 dígitos" error={errors.mfa} />
        )}
        <PrimaryButton busy={busy}>{busy ? "A entrar…" : mfaToken ? "Confirmar MFA" : "Entrar"}</PrimaryButton>
      </form>
      <p className="mt-3 text-center text-[12px] text-mute">
        <Link href="/forgot" className="text-accent hover:underline">
          Esqueci a senha
        </Link>
      </p>
      {oauth.length > 0 && (
        <>
          <div className="my-6 flex items-center gap-3">
            <div className="h-px flex-1 bg-line" />
            <span className="text-[12px] text-mute">Ou continuar com</span>
            <div className="h-px flex-1 bg-line" />
          </div>
          <div className="flex justify-center gap-3">
            {oauth.map((p) => (
              <a
                key={p.id}
                href={p.href}
                title={p.label}
                aria-label={`Continuar com ${p.label}`}
                className="flex h-11 w-11 items-center justify-center rounded-full border border-line bg-white shadow-sm transition hover:border-accent/40 hover:shadow"
              >
                <p.icon />
              </a>
            ))}
          </div>
        </>
      )}
    </AuthFormShell>
  );
}

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" aria-hidden>
      <path fill="#EA4335" d="M12 10.2v3.6h5.1c-.2 1.2-1.5 3.6-5.1 3.6-3.1 0-5.6-2.5-5.6-5.6S8.9 6.2 12 6.2c1.8 0 3 .7 3.7 1.4l2.5-2.4C16.7 3.7 14.6 2.8 12 2.8 6.9 2.8 2.8 6.9 2.8 12S6.9 21.2 12 21.2c5.5 0 9.1-3.9 9.1-9.3 0-.6-.1-1.1-.2-1.7H12z" />
      <path fill="#4285F4" d="M21.1 11.9c.1.6.2 1.1.2 1.7 0 5.4-3.6 9.3-9.1 9.3-1.8 0-3.4-.5-4.8-1.3l3.7-2.8c1 .7 2.3 1.1 3.7 1.1 3.6 0 4.9-2.4 5.1-3.6H12v-3.6h9.1z" opacity=".0" />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="#111" aria-hidden>
      <path d="M12 2C6.48 2 2 6.58 2 12.26c0 4.52 2.87 8.35 6.84 9.71.5.1.68-.22.68-.49 0-.24-.01-.87-.01-1.71-2.78.62-3.37-1.37-3.37-1.37-.45-1.18-1.11-1.5-1.11-1.5-.91-.64.07-.63.07-.63 1 .07 1.53 1.06 1.53 1.06.9 1.57 2.36 1.12 2.94.86.09-.67.35-1.12.63-1.37-2.22-.26-4.56-1.14-4.56-5.07 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.7 0 0 .84-.27 2.75 1.05A9.3 9.3 0 0 1 12 6.8c.85 0 1.71.12 2.51.35 1.9-1.32 2.74-1.05 2.74-1.05.55 1.4.21 2.44.1 2.7.64.72 1.03 1.63 1.03 2.75 0 3.94-2.34 4.8-4.58 5.06.36.32.68.94.68 1.9 0 1.37-.01 2.47-.01 2.8 0 .27.18.6.69.49A10.03 10.03 0 0 0 22 12.26C22 6.58 17.52 2 12 2z" />
    </svg>
  );
}

function OidcIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="8" stroke="#2563EB" strokeWidth="1.8" />
      <path d="M12 8v8M8 12h8" stroke="#2563EB" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}
