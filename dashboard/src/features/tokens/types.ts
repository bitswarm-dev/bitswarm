export interface TokenView {
  id: number;
  name: string;
  scope: "read" | "control" | "admin";
  created: number;
  expires: number;
  last4: string;
  lastUsed: number;
  expired: boolean;
}

export const SCOPES: Record<string, [string, string]> = {
  read: ["Read only", "see torrents, stats and the log"],
  control: ["Control torrents", "also add, pause, resume and remove torrents"],
  admin: ["Full access", "also change settings, hooks and tokens"],
};
