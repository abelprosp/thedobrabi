"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { setTokens } from "@/lib/api";
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
  useEffect(() => {
    const access = params.get("access_token");
    const refresh = params.get("refresh_token");
    if (access && refresh) {
      setTokens({ access_token: access, refresh_token: refresh, expires_in: 900 });
      router.replace("/overview");
    } else {
      router.replace("/login");
    }
  }, [params, router]);
  return <div className="flex min-h-screen items-center justify-center text-mute">A concluir SSO…</div>;
}
