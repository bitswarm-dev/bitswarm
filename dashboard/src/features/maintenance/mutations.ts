import { useMutation, useQueryClient } from "@tanstack/react-query";
import { runTask, setTaskEnabled, updateTask } from "./api";

function useRefresh() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: ["maintenance"] }),
      client.invalidateQueries({ queryKey: ["torrents"] }),
      client.invalidateQueries({ queryKey: ["log"] }),
    ]);
}

export const useRunTask = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: runTask, onSuccess: refresh });
};
export const useSetTaskEnabled = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: setTaskEnabled, onSuccess: refresh });
};
export const useUpdateTask = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: updateTask, onSuccess: refresh });
};
