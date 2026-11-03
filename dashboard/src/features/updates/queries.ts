import { useSuspenseQuery } from "@tanstack/react-query";
import { getUpdates } from "./api";

export const updatesKey = ["updates"] as const;

export function useUpdates() {
  return useSuspenseQuery({ queryKey: updatesKey, queryFn: getUpdates, refetchInterval: 15000 });
}
