import { queryOptions, useSuspenseQuery } from "@tanstack/react-query";
import { listTokens } from "./api";

export const tokensQuery = queryOptions({ queryKey: ["tokens"], queryFn: listTokens, refetchInterval: 5000 });

export function useTokens() {
  return useSuspenseQuery(tokensQuery);
}
