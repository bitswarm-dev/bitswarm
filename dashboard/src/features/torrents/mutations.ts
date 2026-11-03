import { useMutation, useQueryClient } from "@tanstack/react-query";
import { addTorrent, batch, retryAnnounce, setAlt, setPriority } from "./api";
import { statsKey, torrentsKey } from "./queries";

function useRefresh() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: torrentsKey }),
      client.invalidateQueries({ queryKey: statsKey }),
      client.invalidateQueries({ queryKey: ["catalog"] }),
      client.invalidateQueries({ queryKey: ["log"] }),
    ]);
}

export function useAddTorrent() {
  const refresh = useRefresh();
  return useMutation({ mutationFn: addTorrent, onSuccess: refresh });
}

export function useBatch() {
  const refresh = useRefresh();
  return useMutation({ mutationFn: batch, onSuccess: refresh });
}

export function useSetPriority() {
  const refresh = useRefresh();
  return useMutation({ mutationFn: setPriority, onSuccess: refresh });
}

export function useRetryAnnounce() {
  const refresh = useRefresh();
  return useMutation({ mutationFn: retryAnnounce, onSuccess: refresh });
}

export function useSetAlt() {
  const refresh = useRefresh();
  return useMutation({ mutationFn: setAlt, onSuccess: refresh });
}
