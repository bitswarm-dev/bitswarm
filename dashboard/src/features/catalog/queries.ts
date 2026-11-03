import { useSuspenseQuery } from "@tanstack/react-query";
import { getCatalog, getTagUsage } from "./api";

export const catalogKey = ["catalog"] as const;

export function useCatalog() {
  return useSuspenseQuery({ queryKey: catalogKey, queryFn: getCatalog, refetchInterval: 5000 });
}

export function useTagUsage() {
  return useSuspenseQuery({ queryKey: ["catalog", "usage"], queryFn: getTagUsage, staleTime: 0, gcTime: 0 });
}
