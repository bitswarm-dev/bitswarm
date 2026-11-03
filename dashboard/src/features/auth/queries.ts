import { useSuspenseQuery } from "@tanstack/react-query";
import { getAuthStatus } from "./api";

export const authKey = ["auth"] as const;

export function useAuthStatus() {
  return useSuspenseQuery({ queryKey: authKey, queryFn: getAuthStatus });
}
