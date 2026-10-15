"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { loadRadar, sendRadarFeedback } from "@/lib/manage-client";
import { Badge, Button, EmptyState, ErrorState, PageHeader, PageSkeleton, Table, TableWrap, Td, Th } from "@/components/ui";
import { ActionDialog, type ActionPreset } from "@/components/manage/action-dialog";
import type { Discovery } from "@/components/manage/types";
import { Radar } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

export default function RadarPage() {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["manage-radar"],
    queryFn: loadRadar,
  });
  const items = normalizeArray<Discovery>(q.data);
  const material = items.filter((d) => d.kind !== "stable");
  const quiet = items.find((d) => d.kind === "stable");
  const [preset, setPreset] = useState<ActionPreset | undefined>();
  const [open, setOpen] = useState(false);

  const feedback = useMutation({
    mutationFn: ({ id, value }: { id: string; value: string }) => sendRadarFeedback(id, value),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["manage-radar"] });
      qc.invalidateQueries({ queryKey: ["overview"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="mx-auto max-w-5xl space-y-4">
      <PageHeader
        title="Radar"
        description="Descobertas priorizadas pelo impacto e pela qualidade da evidência. Quando nada relevante mudou, dizemos isso com clareza."
      />
      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {!q.isLoading && !q.isError && material.length === 0 && (
        <EmptyState
          icon={Radar}
          title={quiet?.title || "Nenhuma mudança relevante"}
          description={quiet?.body || "Volte quando a próxima actualização dos dados chegar."}
          action={
            <Link href="/data">
              <Button variant="secondary">Ver conjuntos</Button>
            </Link>
          }
        />
      )}
      {material.length > 0 && (
        <TableWrap>
          <Table>
            <thead>
              <tr>
                <Th>Descoberta</Th>
                <Th>Próximo passo</Th>
                <Th>Feedback</Th>
              </tr>
            </thead>
            <tbody>
              {material.map((d) => (
                <tr key={d.id}>
                  <Td>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{d.title}</span>
                      <Badge tone={d.kind === "risk" ? "warn" : d.kind === "opportunity" ? "ok" : "accent"}>{kindLabel(d.kind)}</Badge>
                      {d.watching && <Badge>a acompanhar</Badge>}
                    </div>
                    <p className="mt-1 max-w-xl text-[12px] text-mute">{d.body}</p>
                  </Td>
                  <Td>
                    <p className="text-[13px]">{d.next_step}</p>
                    <div className="mt-2 flex flex-wrap gap-2">
                      <Link href="/ask" className="text-[12px] font-medium text-primary hover:underline">
                        Ver análise
                      </Link>
                      <button
                        type="button"
                        className="text-[12px] font-medium text-ink hover:underline"
                        onClick={() => {
                          setPreset({
                            title: d.next_step || d.title,
                            body: `${d.title}. ${d.body}`,
                            discoveryId: d.id,
                            datasetId: d.dataset_id,
                            measure: typeof d.evidence?.metric === "string" ? d.evidence.metric : undefined,
                          });
                          setOpen(true);
                        }}
                      >
                        Criar acção
                      </button>
                    </div>
                  </Td>
                  <Td>
                    <div className="flex flex-col gap-1">
                      <Btn active={d.feedback === "useful"} onClick={() => feedback.mutate({ id: d.id, value: "useful" })}>
                        Foi útil
                      </Btn>
                      <Btn active={d.feedback === "not_applicable"} onClick={() => feedback.mutate({ id: d.id, value: "not_applicable" })}>
                        Não se aplica
                      </Btn>
                      <Btn active={d.feedback === "watch" || d.watching} onClick={() => feedback.mutate({ id: d.id, value: "watch" })}>
                        Acompanhar
                      </Btn>
                    </div>
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        </TableWrap>
      )}
      <ActionDialog open={open} onClose={() => setOpen(false)} preset={preset} />
    </div>
  );
}

function kindLabel(kind: string) {
  return { change: "Mudança", risk: "Desvio", opportunity: "Oportunidade", stable: "Estável" }[kind] || kind;
}

function Btn({ active, onClick, children }: { active?: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-full px-2.5 py-1 text-left text-[11px] ${active ? "bg-primary text-white" : "bg-surface-2 text-mute hover:text-ink"}`}
    >
      {children}
    </button>
  );
}
