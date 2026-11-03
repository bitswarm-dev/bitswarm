export interface Preset {
  id: number;
  name: string;
  find: string;
  replace: string;
  i: boolean;
  g: boolean;
  keepExt: boolean;
  target: "files" | "torrent" | "both";
  tag: string;
  when: "complete" | "add";
  enabled: boolean;
  count: number;
}

export interface PresetView extends Preset {
  example: { from: string; to: string } | null;
  usedBy: string[];
}

export interface RenameLogEntry {
  id: number;
  t: number;
  rule: string;
  from: string;
  to: string;
  undone: boolean;
  torrentId: number;
  kind: string;
  also: boolean;
}

export interface PreviewRow {
  from: string;
  to: string;
  empty: boolean;
  clash: boolean;
  src: string;
  match: boolean;
  same: boolean;
  tested: string;
}

export interface Preview {
  error?: string;
  total: number;
  ok: number;
  rows: PreviewRow[];
}

export const BLANK_PRESET: Omit<Preset, "id" | "count"> = {
  name: "",
  find: "",
  replace: "",
  i: false,
  g: false,
  keepExt: true,
  target: "files",
  tag: "",
  when: "complete",
  enabled: true,
};
