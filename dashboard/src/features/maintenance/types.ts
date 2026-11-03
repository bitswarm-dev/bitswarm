export interface Params {
  days: number;
  ratio: number;
  files: string;
  hours: number;
  action: string;
  gb: number;
  resume: boolean;
}

export interface Task {
  id: number;
  type: string;
  enabled: boolean;
  every: number;
  unit: "minutes" | "hours" | "days";
  tag: string;
  p: Params;
  lastRun: number;
  lastText: string;
  nextRun: number;
}

export interface TaskView extends Task {
  title: string;
  desc: string;
  summary: string;
  scoped: boolean;
}

export interface MaintItem {
  name: string;
  what: string;
  why: string;
}

export interface MaintPreview {
  items: MaintItem[];
  done: string;
}

export interface MaintLogEntry {
  id: number;
  t: number;
  task: string;
  kind: "ok" | "none";
  text: string;
  added: string[];
  more: number;
}

export interface FieldDef {
  key: keyof Params;
  type: "number" | "select" | "check";
  label: string;
  unit?: string;
  min?: number;
  max?: number;
  step?: number;
  hint?: string;
  opts?: [string, string][];
}

export const FIELDS: Record<string, FieldDef[]> = {
  cleanup: [
    { key: "days", type: "number", label: "Finished at least", unit: "days ago", min: 0, max: 3650 },
    { key: "ratio", type: "number", label: "With a share ratio of at least", step: 0.1, min: 0, max: 9999, hint: "0 removes them whatever the ratio." },
    { key: "files", type: "select", label: "Downloaded files", opts: [["keep", "Keep them on disk"], ["delete", "Delete them"]] },
  ],
  stalled: [
    { key: "hours", type: "number", label: "No data received for", unit: "hours", min: 1, max: 720 },
    { key: "action", type: "select", label: "Then", opts: [["pause", "Pause the download"], ["reannounce", "Re-announce to the trackers"], ["remove", "Remove it and keep its files"]] },
  ],
  disk: [
    { key: "gb", type: "number", label: "Free space below", unit: "GB", min: 1, max: 100000 },
    { key: "resume", type: "check", label: "Resume them when there’s enough space again" },
  ],
  logs: [{ key: "days", type: "number", label: "Entries older than", unit: "days", min: 0, max: 3650 }],
};
