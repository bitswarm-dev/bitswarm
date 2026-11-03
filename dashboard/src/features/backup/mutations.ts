import { useMutation, useQueryClient } from "@tanstack/react-query";
import { applyBackup, checkBackup, exportBackup } from "./api";

export const useExportBackup = () => {
  const client = useQueryClient();
  return useMutation({ mutationFn: exportBackup, onSuccess: () => client.invalidateQueries({ queryKey: ["log"] }) });
};
export const useCheckBackup = () => useMutation({ mutationFn: checkBackup });
export const useApplyBackup = () => {
  const client = useQueryClient();
  return useMutation({
    mutationFn: applyBackup,
    onSuccess: () => client.invalidateQueries(),
  });
};
