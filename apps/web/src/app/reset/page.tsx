"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { AuthCard, Field, PrimaryButton } from "@/components/auth";

export default function ResetPage() {
  return (
    <Suspense>
      <ResetInner />
    </Suspense>
  );
}

function ResetInner() {
  const router = useRouter();
  const params = useSearchParams();
  const token = params.get("token") || "";
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await api("/api/v1/auth/reset", { method: "POST", body: JSON.stringify({ token, password }) });
      toast.success("Senha actualizada");
      router.replace("/login");
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthCard title="Nova senha" subtitle="Escolha uma senha com pelo menos 8 caracteres.">
      <form onSubmit={onSubmit} className="space-y-3">
        <Field label="Nova senha" value={password} onChange={setPassword} type="password" icon="lock" required minLength={8} placeholder="Mínimo 8 caracteres" />
        <PrimaryButton busy={busy} disabled={!token}>
          {busy ? "A guardar…" : "Guardar senha"}
        </PrimaryButton>
      </form>
    </AuthCard>
  );
}
