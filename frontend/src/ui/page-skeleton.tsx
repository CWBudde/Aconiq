import { Skeleton } from "@/ui/components/skeleton";

/**
 * The placeholder shown while a lazily loaded page chunk arrives: a heading
 * line, a description line and two card-sized blocks, so the layout does not
 * jump when the page lands.
 */
export function PageSkeleton() {
  return (
    <div className="flex flex-1 flex-col gap-4 p-6">
      <Skeleton className="h-7 w-48" />
      <Skeleton className="h-4 w-96 max-w-full" />
      <div className="mt-4 grid gap-4">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    </div>
  );
}
