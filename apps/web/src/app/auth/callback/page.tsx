"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { setTokens, type Tokens } from "@/lib/api";
import { Suspense } from "react";

export default function CallbackPage() {
  return (
    <Suspense>
      <Inner />
    </Suspense>
  );
}

function Inner() {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState(false);

  useEffect(() => {
    const code = params.get("code");
    if (!code) {
      router.replace("/login");
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/api/v1/auth/oauth/exchange", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include",
          body: JSON.stringify({ code }),
        });
        const json = await res.json().catch(() => ({}));
        const tokens = (json?.data?.tokens ?? json?.tokens) as Tokens | undefined;
        if (!res.ok || !tokens?.access_token || !tokens?.refresh_token) {
          throw new Error("exchange failed");
        }
        if (cancelled) return;
        setTokens(tokens); // clears localStorage leftovers; session is HttpOnly cookie
        // Strip code from the address bar / history so it cannot be reused via share/back.
        window.history.replaceState({}, "", "/auth/callback");
        router.replace("/overview");
      } catch {
        if (!cancelled) {
          setError(true);
          router.replace("/login");
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [params, router]);

  return (
    <div className="flex min-h-screen items-center justify-center text-mute">
      {error ? "Falha no SSO…" : "A concluir SSO…"}
    </div>
  );
}
