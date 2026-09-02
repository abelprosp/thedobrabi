"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { ErrorState, PageHeader, PageSkeleton } from "@/components/ui";
import { FlowCanvasEditor } from "@/components/flows/canvas";
import { AutoRefreshCard } from "@/components/auto-refresh-card";
import type { Flow, Step } from "@/lib/flows";

export default function FlowEditorPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const q = useQuery({
    queryKey: ["flow", id],
    queryFn: () => api<{ flow: Flow; steps: Step[] }>(`/api/v1/flows/${id}`),
    enabled: !!id,
  });

  if (q.isLoading) return <PageSkeleton />;
  if (q.isError || !q.data?.flow) {
    return <ErrorState message={(q.error as Error)?.message || "Flow não encontrado"} onRetry={() => q.refetch()} />;
  }

  const flow = q.data.flow;

  return (
    <div className="mx-auto max-w-6xl space-y-5">
      <PageHeader
        title={flow.name}
        description={flow.description || "Pipeline visual até ao ClickHouse."}
        crumbs={[{ href: "/flows", label: "Flows" }]}
      />
      <AutoRefreshCard kind="flow" targetId={flow.id} />
      <FlowCanvasEditor flow={flow} initialSteps={q.data.steps} />
    </div>
  );
}
