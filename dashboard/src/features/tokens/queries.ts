import { useSuspenseQuery } from "@tanstack/react-query";
import { listTokens } from "./api";

export function useTokens() {
  return useSuspenseQuery({ queryKey: ["tokens"], queryFn: listTokens, refetchInterval: 5000 });
}
