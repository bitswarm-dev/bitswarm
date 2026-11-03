import { useMutation, useQueryClient } from "@tanstack/react-query";
import { addTag, deleteTag, renameTag } from "./api";

function useRefresh() {
  const client = useQueryClient();
  return () =>
    Promise.all(
      ["catalog", "torrents", "schedules", "renames", "maintenance", "hooks", "log"].map((k) => client.invalidateQueries({ queryKey: [k] })),
    );
}

export const useAddTag = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: addTag, onSuccess: refresh });
};

export const useRenameTag = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: renameTag, onSuccess: refresh });
};

export const useDeleteTag = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: deleteTag, onSuccess: refresh });
};
