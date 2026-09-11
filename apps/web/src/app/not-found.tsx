import Link from "next/link";
import { ArrowLeft, LayoutDashboard } from "lucide-react";
import { Button } from "@/components/ui";
import { Logo } from "@/components/brand";

export default function NotFound() {
  return (
    <main className="flex min-h-dvh items-center justify-center bg-bg px-4">
      <div className="w-full max-w-md text-center">
        <div className="mb-10 flex justify-center">
          <Logo size={34} />
        </div>
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/[0.035]">
          <LayoutDashboard size={24} />
        </div>
        <p className="mt-6 text-[11px] font-semibold uppercase tracking-[0.18em] text-primary">Erro 404</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-ink">Página não encontrada</h1>
        <p className="mt-3 text-sm leading-relaxed text-mute">
          Este endereço pode ter mudado ou já não estar disponível.
        </p>
        <div className="mt-7 flex justify-center">
          <Link href="/overview">
            <Button><ArrowLeft size={15} /> Voltar ao início</Button>
          </Link>
        </div>
      </div>
    </main>
  );
}
