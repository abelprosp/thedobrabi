"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function Home() {
  const router = useRouter();
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/api/v1/auth/me", { credentials: "include" });
        if (cancelled) return;
        router.replace(res.ok ? "/overview" : "/login");
      } catch {
        if (!cancelled) router.replace("/login");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [router]);
  return <div className="flex min-h-screen items-center justify-center text-mute">A abrir TheDobra…</div>;
}
