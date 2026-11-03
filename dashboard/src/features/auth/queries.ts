import { queryOptions, useSuspenseQuery } from "@tanstack/react-query";
import { getAuthStatus } from "./api";

export const authKey = ["auth"] as const;

export const authQuery = queryOptions({ queryKey: authKey, queryFn: getAuthStatus });

export function useAuthStatus() {
  return useSuspenseQuery(authQuery);
}
