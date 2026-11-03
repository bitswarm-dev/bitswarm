import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createSchedule, deleteSchedule, downloadFromPreview, runSchedule, setScheduleEnabled, updateSchedule } from "./api";

function useRefresh() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: ["schedules"] }),
      client.invalidateQueries({ queryKey: ["torrents"] }),
      client.invalidateQueries({ queryKey: ["renames"] }),
      client.invalidateQueries({ queryKey: ["log"] }),
    ]);
}

export const useCreateSchedule = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: createSchedule, onSuccess: refresh });
};
export const useUpdateSchedule = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: updateSchedule, onSuccess: refresh });
};
export const useSetScheduleEnabled = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: setScheduleEnabled, onSuccess: refresh });
};
export const useDeleteSchedule = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: deleteSchedule, onSuccess: refresh });
};
export const useRunSchedule = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: runSchedule, onSuccess: refresh });
};
export const useDownloadFromPreview = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: downloadFromPreview, onSuccess: refresh });
};
