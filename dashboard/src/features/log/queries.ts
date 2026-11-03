import { queryOptions, useSuspenseQuery } from "@tanstack/react-query";
import { getLog, type LogFilter } from "./api";

export const logQuery = (filter: LogFilter) => queryOptions({ queryKey: ["log", filter], queryFn: () => getLog(filter), refetchInterval: 2000 });

export function useLog(filter: LogFilter) {
  return useSuspenseQuery(logQuery(filter));
}
