import { useQuery } from "@tanstack/react-query";
import { backupCounts } from "./api";

export function useBackupCounts() {
  return useQuery({ queryKey: ["backup", "counts"], queryFn: backupCounts });
}
