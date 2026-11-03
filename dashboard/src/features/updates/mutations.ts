import { useMutation, useQueryClient } from "@tanstack/react-query";
import { checkUpdates } from "./api";
import { updatesKey } from "./queries";

export function useCheckUpdates() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: checkUpdates,
    onSuccess: (state) => {
      client.setQueryData(updatesKey, state);
      void client.invalidateQueries({ queryKey: ["log"] });
    },
  });
}
