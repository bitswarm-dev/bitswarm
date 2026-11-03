import { useToast } from "@/components/Toasts/Toasts";
import { useBatch } from "@/features/torrents/mutations";
import type { BatchRequest } from "@/features/torrents/types";

export function useActions() {
  const batch = useBatch();
  const toast = useToast();

  return (req: BatchRequest, after?: (message: string) => void) =>
    batch.mutate(req, {
      onSuccess: ({ message }) => {
        if (message) toast(message);
        after?.(message);
      },
      onError: (e) => toast(e.message),
    });
}
