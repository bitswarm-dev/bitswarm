import { useMutation, useQueryClient } from "@tanstack/react-query";
import { clearLog } from "./api";

export function useClearLog() {
  const client = useQueryClient();
  return useMutation({ mutationFn: clearLog, onSuccess: () => client.invalidateQueries({ queryKey: ["log"] }) });
}
