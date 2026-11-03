import { queryOptions, useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { hooksConfig, listDeliveries, listHooks, previewHook } from "./api";
import type { Hook } from "./types";

export const hooksQuery = queryOptions({ queryKey: ["hooks"], queryFn: listHooks, refetchInterval: 3000 });
export const deliveriesQuery = queryOptions({ queryKey: ["hooks", "deliveries"], queryFn: listDeliveries, refetchInterval: 3000 });
export const hooksConfigQuery = queryOptions({ queryKey: ["hooks", "config"], queryFn: hooksConfig, staleTime: 60_000 });

export function useHooks() {
  return useSuspenseQuery(hooksQuery);
}

export function useDeliveries() {
  return useSuspenseQuery(deliveriesQuery);
}

export function useHooksConfig() {
  return useSuspenseQuery(hooksConfigQuery);
}

export function useHookPreview(h: Omit<Hook, "id" | "count" | "last" | "lastEvent">) {
  return useQuery({ queryKey: ["hooks", "preview", h], queryFn: () => previewHook(h), placeholderData: (prev) => prev });
}
