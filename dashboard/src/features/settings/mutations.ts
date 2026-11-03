import { useMutation, useQueryClient } from "@tanstack/react-query";
import { saveSettings } from "./api";
import { settingsKey } from "./queries";

export function useSaveSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: saveSettings,
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: settingsKey });
      await client.invalidateQueries({ queryKey: ["log"] });
    },
  });
}
