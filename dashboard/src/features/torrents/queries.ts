import { queryOptions, useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { getContents, getStats, listTorrents, type ContentsParams } from "./api";

export const torrentsKey = ["torrents"] as const;
export const statsKey = ["stats"] as const;

export const torrentsQuery = queryOptions({ queryKey: torrentsKey, queryFn: listTorrents, refetchInterval: 1000 });
export const statsQuery = queryOptions({ queryKey: statsKey, queryFn: getStats, refetchInterval: 1000 });

export function useTorrents() {
  return useSuspenseQuery(torrentsQuery);
}

export function useStats() {
  return useSuspenseQuery(statsQuery);
}

export function useContents(params: ContentsParams | null) {
  return useQuery({
    queryKey: ["contents", params?.magnet ?? "", params?.fileName ?? "", params?.fileData?.length ?? 0],
    queryFn: () => getContents(params ?? {}),
    enabled: params !== null,
    retry: false,
    staleTime: Infinity,
  });
}
