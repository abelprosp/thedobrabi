"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { getAccess } from "@/lib/api";

export default function Home() {
  const router = useRouter();
  useEffect(() => {
    router.replace(getAccess() ? "/overview" : "/login");
  }, [router]);
  return <div className="flex min-h-screen items-center justify-center text-mute">A abrir TheDobra…</div>;
}
