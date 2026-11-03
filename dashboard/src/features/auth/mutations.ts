import { useMutation, useQueryClient } from "@tanstack/react-query";
import { login, logout } from "./api";
import { authKey } from "./queries";

export function useLogin() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: login,
    onSuccess: () => client.invalidateQueries(),
  });
}

export function useLogout() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      client.removeQueries({ predicate: (q) => q.queryKey[0] !== authKey[0] });
      await client.invalidateQueries({ queryKey: authKey });
    },
  });
}
