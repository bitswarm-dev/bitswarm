import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createHook, deleteHook, duplicateHook, parseCurl, setHookEnabled, testHook, updateHook } from "./api";

function useRefresh() {
  const client = useQueryClient();
  return () => Promise.all([client.invalidateQueries({ queryKey: ["hooks"] }), client.invalidateQueries({ queryKey: ["log"] })]);
}

export const useCreateHook = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: createHook, onSuccess: refresh });
};
export const useUpdateHook = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: updateHook, onSuccess: refresh });
};
export const useSetHookEnabled = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: setHookEnabled, onSuccess: refresh });
};
export const useDuplicateHook = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: duplicateHook, onSuccess: refresh });
};
export const useParseCurl = () => useMutation({ mutationFn: parseCurl });
export const useDeleteHook = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: deleteHook, onSuccess: refresh });
};
export const useTestHook = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: testHook, onSuccess: refresh });
};
