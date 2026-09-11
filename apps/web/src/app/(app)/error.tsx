"use client";

import { useEffect } from "react";
import { ErrorState } from "@/components/ui";

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="mx-auto max-w-3xl py-8">
      <ErrorState
        message="Ocorreu um erro inesperado nesta página. Os seus dados não foram alterados."
        onRetry={reset}
      />
    </div>
  );
}
