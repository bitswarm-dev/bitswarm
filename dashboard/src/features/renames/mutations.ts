import { useMutation, useQueryClient } from "@tanstack/react-query";
import { applyPreset, createPreset, deletePreset, duplicatePreset, setPresetEnabled, undoRename, updatePreset } from "./api";

function useRefresh() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: ["renames"] }),
      client.invalidateQueries({ queryKey: ["torrents"] }),
      client.invalidateQueries({ queryKey: ["log"] }),
      client.invalidateQueries({ queryKey: ["schedules"] }),
    ]);
}

export const useCreatePreset = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: createPreset, onSuccess: refresh });
};
export const useUpdatePreset = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: updatePreset, onSuccess: refresh });
};
export const useSetPresetEnabled = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: setPresetEnabled, onSuccess: refresh });
};
export const useDuplicatePreset = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: duplicatePreset, onSuccess: refresh });
};
export const useDeletePreset = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: deletePreset, onSuccess: refresh });
};
export const useApplyPreset = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: applyPreset, onSuccess: refresh });
};
export const useUndoRename = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: undoRename, onSuccess: refresh });
};
