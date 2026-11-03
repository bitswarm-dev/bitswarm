export interface Hook {
  id: number;
  kind: "api" | "cmd" | "curl";
  name: string;
  method: string;
  url: string;
  headers: string;
  body: string;
  cmd: string;
  cwd: string;
  timeout: number;
  stdin: boolean;
  events: string[];
  tag: string;
  secret: string;
  clearSecret: boolean;
  retries: number;
  enabled: boolean;
  count: number;
  last: number;
  lastEvent: string;
}

export interface HookView extends Hook {
  hasSecret: boolean;
}

export interface Delivery {
  id: number;
  t: number;
  hookId: number;
  hook: string;
  event: string;
  test: boolean;
  kind: "api" | "cmd" | "curl";
  method: string;
  url: string;
  command: string;
  text: string;
  copy: string;
  status: "sent" | "failed" | "ran" | "not_run";
  attempts: number;
  error: string;
  response: number;
  output: string;
  millis: number;
}

export interface CurlImport {
  method: string;
  url: string;
  headers: string;
  body: string;
  warnings: string[];
}

export interface HookPreview {
  event: string;
  text: string;
  unknown: string[];
  empty: string[];
  signed: boolean;
  missingSecrets: string[];
}

export const EVENT_GROUPS: [string, [string, string][]][] = [
  ["Torrents", [["torrent.added", "Added"], ["torrent.metadata", "Metadata received"], ["torrent.resumed", "Resumed"], ["torrent.paused", "Paused"], ["torrent.completed", "Download finished"], ["torrent.error", "Error"], ["torrent.removed", "Removed"], ["torrent.renamed", "Renamed"], ["torrent.tagged", "Tags changed"], ["torrent.moved", "Files moved"]]],
  ["Schedules", [["schedule.search", "Search sent"], ["schedule.links", "Links found"], ["schedule.finished", "Run finished"]]],
  ["Maintenance and app", [["maintenance.run", "Maintenance task ran"], ["settings.saved", "Settings saved"], ["update.available", "Update available"]]],
];

export const EVENT_LABEL: Record<string, string> = Object.fromEntries(EVENT_GROUPS.flatMap(([, l]) => l));
export const ALL_EVENTS = Object.keys(EVENT_LABEL);

export const PLACEHOLDERS = ["payload", "event", "time", "torrent.name", "torrent.hash", "torrent.tags", "torrent.size", "torrent.bytes", "torrent.path", "torrent.ratio", "torrent.state", "schedule.name", "task.name", "count"];

export const BLANK_HOOK: Omit<Hook, "id" | "count" | "last" | "lastEvent"> = {
  kind: "api",
  name: "",
  method: "POST",
  url: "",
  headers: "Content-Type: application/json",
  body: "{{payload}}",
  cmd: "",
  cwd: "",
  timeout: 60,
  stdin: false,
  events: ["torrent.added", "torrent.completed"],
  tag: "",
  secret: "",
  clearSecret: false,
  retries: 3,
  enabled: true,
};

export const CURL_TEMPLATE = `curl -sf -X POST https://example.org/hooks/bitswarm \\
  -H 'Content-Type: application/json' \\
  -d {{payload}}`;
