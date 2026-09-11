import { PageSkeleton } from "@/components/ui";

export default function AppLoading() {
  return (
    <div className="mx-auto max-w-6xl">
      <PageSkeleton cards={4} />
    </div>
  );
}
