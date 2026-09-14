"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button, Card, PageHeader } from "@/components/ui";

export default function VerifyEmailPage() {
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("A confirmar o seu e-mail…");

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token");
    if (!token) {
      setStatus("error");
      setMessage("A ligação de confirmação está incompleta.");
      return;
    }
    api("/api/v1/auth/verify-email?token=" + encodeURIComponent(token))
      .then(() => {
        setStatus("success");
        setMessage("E-mail confirmado. A sua conta está pronta.");
      })
      .catch((error: Error) => {
        setStatus("error");
        setMessage(error.message || "Não foi possível confirmar o e-mail.");
      });
  }, []);

  return (
    <main className="flex min-h-dvh items-center justify-center bg-bg px-4">
      <Card className="w-full max-w-lg space-y-5 p-7">
        <PageHeader
          title={status === "success" ? "E-mail confirmado" : status === "error" ? "Confirmação não concluída" : "Confirmar e-mail"}
          description={message}
        />
        {status !== "loading" && (
          <Button onClick={() => { window.location.href = "/login"; }}>
            Ir para o login
          </Button>
        )}
      </Card>
    </main>
  );
}
