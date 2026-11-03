import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createToken, revokeToken } from "./api";

function useRefresh() {
  const client = useQueryClient();
  return () => Promise.all([client.invalidateQueries({ queryKey: ["tokens"] }), client.invalidateQueries({ queryKey: ["log"] })]);
}

export const useCreateToken = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: createToken, onSuccess: refresh });
};
export const useRevokeToken = () => {
  const refresh = useRefresh();
  return useMutation({ mutationFn: revokeToken, onSuccess: refresh });
};
