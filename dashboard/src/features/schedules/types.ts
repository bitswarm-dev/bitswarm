export interface Rule {
  id: number;
  name: string;
  url: string;
  terms: string;
  selector: string;
  attr: string;
  linkType: "any" | "magnet" | "torrent";
  include: string;
  exclude: string;
  tags: string[];
  path: string;
  max: number;
  start: boolean;
  skipSeen: boolean;
  every: number;
  unit: "hours" | "days";
  enabled: boolean;
  endDate: number;
  ended: boolean;
  renamePreset: number;
  lastRun: number;
  lastFound: number;
  lastNew: number;
  status: "idle" | "ok" | "error";
  lastError: string;
  nextRun: number;
}

export interface RuleView extends Rule {
  urls: string[];
  runsLeft: number;
  seenCount: number;
}

export type ItemState = "new" | "have" | "filtered" | "over";

export interface Item {
  name: string;
  type: "magnet" | "torrent";
  url: string;
  key: string;
  btih: string;
  state: ItemState;
}

export interface Extraction {
  error?: string;
  matched: number;
  items: Item[];
}

export interface PreviewResult {
  urls: string[];
  extraction: Extraction;
}

export interface ScheduleLogEntry {
  id: number;
  t: number;
  rule: string;
  kind: "ok" | "none" | "err";
  text: string;
  added: string[];
}

export const BLANK_RULE: Omit<Rule, "id"> = {
  name: "",
  url: "",
  terms: "",
  selector: "a[href^='magnet:']",
  attr: "href",
  linkType: "any",
  include: "",
  exclude: "",
  tags: [],
  path: "",
  max: 5,
  start: true,
  skipSeen: true,
  every: 12,
  unit: "hours",
  enabled: true,
  endDate: 0,
  ended: false,
  renamePreset: 0,
  lastRun: 0,
  lastFound: 0,
  lastNew: 0,
  status: "idle",
  lastError: "",
  nextRun: 0,
};
