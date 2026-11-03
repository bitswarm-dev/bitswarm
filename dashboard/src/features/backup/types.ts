export interface PlanPart {
  key: string;
  name: string;
  summary: string;
}

export interface Plan {
  from: string;
  at: string;
  parts: PlanPart[];
  theme: Record<string, unknown> | null;
}

export interface ApplyResult {
  done: string[];
  skipped: { part: string; name: string; reason: string }[];
  summary: string;
}

export const PARTS: [string, string][] = [
  ["settings", "Settings"],
  ["theme", "Theme"],
  ["tags", "Tags"],
  ["schedules", "Schedules"],
  ["presets", "Rename presets"],
  ["maintenance", "Maintenance tasks"],
  ["hooks", "Hooks"],
];
