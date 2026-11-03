import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { hooksConfig, listDeliveries, listHooks, previewHook } from "./api";
import type { Hook } from "./types";

export function useHooks() {
  return useSuspenseQuery({ queryKey: ["hooks"], queryFn: listHooks, refetchInterval: 3000 });
}

export function useDeliveries() {
  return useSuspenseQuery({ queryKey: ["hooks", "deliveries"], queryFn: listDeliveries, refetchInterval: 3000 });
}

export function useHooksConfig() {
  return useSuspenseQuery({ queryKey: ["hooks", "config"], queryFn: hooksConfig, staleTime: 60_000 });
}

export function useHookPreview(h: Omit<Hook, "id" | "count" | "last" | "lastEvent">) {
  return useQuery({ queryKey: ["hooks", "preview", h], queryFn: () => previewHook(h), placeholderData: (prev) => prev });
}
