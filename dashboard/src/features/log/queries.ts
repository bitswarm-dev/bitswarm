import { useSuspenseQuery } from "@tanstack/react-query";
import { getLog, type LogFilter } from "./api";

export function useLog(filter: LogFilter) {
  return useSuspenseQuery({ queryKey: ["log", filter], queryFn: () => getLog(filter), refetchInterval: 2000 });
}
