import type { Settings } from "./types";

export type FieldKey = keyof Settings;

export interface FieldDef {
  key: FieldKey;
  type: "text" | "number" | "check" | "select" | "password" | "time";
  label: string;
  dep?: FieldKey;
  mono?: boolean;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  hint?: string;
  random?: boolean;
  opts?: [string, string][];
}

export interface GroupDef {
  title: string;
  note?: string;
  fields: FieldDef[];
}

export const SECTION_IDS = ["downloads", "connection", "speed", "bittorrent", "webui", "backup"] as const;
export type SectionId = (typeof SECTION_IDS)[number];

export function isSectionId(v: string): v is SectionId {
  return (SECTION_IDS as readonly string[]).includes(v);
}

export interface SectionDef {
  id: SectionId;
  title: string;
  groups?: GroupDef[];
}

const f = (key: FieldKey, type: FieldDef["type"], label: string, o: Partial<FieldDef> = {}): FieldDef => ({ key, type, label, ...o });

export const SECTIONS: SectionDef[] = [
  {
    id: "downloads",
    title: "Downloads",
    groups: [
      {
        title: "Saving files",
        fields: [
          f("savePath", "text", "Default save path", { mono: true }),
          f("useIncomplete", "check", "Keep incomplete torrents in a separate folder"),
          f("incompletePath", "text", "Incomplete folder", { dep: "useIncomplete", mono: true }),
          f("moveCompleted", "check", "Move completed downloads to another folder"),
          f("completedPath", "text", "Completed folder", { dep: "moveCompleted", mono: true }),
          f("appendExt", "check", "Add .part to the names of incomplete files"),
          f("preallocate", "check", "Pre-allocate disk space for all files"),
        ],
      },
      {
        title: "When adding a torrent",
        fields: [
          f("startAuto", "check", "Start the download automatically"),
          f("createSubfolder", "check", "Create a subfolder for torrents with several files"),
          f("deleteTorrentFile", "check", "Delete the .torrent file after adding it"),
          f("watchEnabled", "check", "Add torrents automatically from a watched folder"),
          f("watchPath", "text", "Watched folder", { dep: "watchEnabled", mono: true }),
        ],
      },
      {
        title: "Scheduled searches",
        fields: [
          f("searchDelay", "number", "Wait between searches", { unit: "seconds", max: 3600, hint: "BitSwarm waits this long between fetching one search page and the next, across all schedules, so sites don’t rate-limit you. 0 turns the wait off." }),
        ],
      },
    ],
  },
  {
    id: "connection",
    title: "Connection",
    groups: [
      {
        title: "Listening port",
        fields: [
          f("port", "number", "Port for incoming connections", { min: 1, max: 65535, random: true }),
          f("upnp", "check", "Use UPnP / NAT-PMP to forward the port from my router"),
        ],
      },
      {
        title: "Connection limits",
        note: "0 means unlimited.",
        fields: [
          f("maxConn", "number", "Global maximum connections", { max: 99999 }),
          f("maxConnPer", "number", "Maximum connections per torrent", { max: 99999 }),
          f("maxUpSlots", "number", "Global maximum upload slots", { max: 9999 }),
          f("maxUpSlotsPer", "number", "Upload slots per torrent", { max: 9999 }),
        ],
      },
      {
        title: "Proxy server",
        fields: [
          f("proxyType", "select", "Type", { opts: [["none", "None"], ["socks5", "SOCKS5"], ["socks4", "SOCKS4"], ["http", "HTTP"]] }),
          f("proxyHost", "text", "Host", { dep: "proxyType", mono: true }),
          f("proxyPort", "number", "Port", { dep: "proxyType", min: 1, max: 65535 }),
          f("proxyPeersOnly", "check", "Use the proxy for peer connections only", { dep: "proxyType" }),
          f("proxyAuth", "check", "The proxy needs a username and password", { dep: "proxyType" }),
          f("proxyUser", "text", "Username", { dep: "proxyAuth" }),
          f("proxyPass", "password", "Password", { dep: "proxyAuth" }),
        ],
      },
    ],
  },
  {
    id: "speed",
    title: "Speed",
    groups: [
      {
        title: "Global rate limits",
        note: "0 means unlimited.",
        fields: [
          f("dlLimit", "number", "Download", { unit: "KiB/s", max: 10000000 }),
          f("ulLimit", "number", "Upload", { unit: "KiB/s", max: 10000000 }),
        ],
      },
      {
        title: "Alternative rate limits",
        note: "Used while Speed limit is on in the toolbar, or during the schedule below.",
        fields: [
          f("altDl", "number", "Download", { unit: "KiB/s", max: 10000000 }),
          f("altUl", "number", "Upload", { unit: "KiB/s", max: 10000000 }),
          f("altSchedule", "check", "Turn on alternative limits on a schedule"),
          f("altFrom", "time", "From", { dep: "altSchedule" }),
          f("altTo", "time", "To", { dep: "altSchedule" }),
          f("altDays", "select", "Days", { dep: "altSchedule", opts: [["every", "Every day"], ["weekdays", "Weekdays"], ["weekends", "Weekends"]] }),
        ],
      },
      {
        title: "Rate limit settings",
        fields: [
          f("limitUtp", "check", "Apply rate limits to µTP connections"),
          f("limitLan", "check", "Apply rate limits to peers on my local network"),
        ],
      },
      {
        title: "Speed chart",
        fields: [
          f("speedWindow", "number", "Time shown", { unit: "seconds", min: 30, max: 3600, hint: "How far back the speed chart in the sidebar looks, from 30 seconds to 3600 (1 hour). For example 300 is 5 minutes and 900 is 15 minutes. The chart always uses the same memory, so a longer time averages more seconds into each point. Changing this restarts the chart." }),
        ],
      },
    ],
  },
  {
    id: "bittorrent",
    title: "BitTorrent",
    groups: [
      {
        title: "Privacy",
        fields: [
          f("dht", "check", "Use DHT (decentralized network) to find more peers"),
          f("pex", "check", "Use Peer Exchange (PeX) to find more peers"),
          f("lsd", "check", "Use Local Peer Discovery to find more peers"),
          f("encryption", "select", "Encryption", { opts: [["prefer", "Prefer encryption"], ["require", "Require encryption"], ["allow", "Allow encryption"], ["off", "Turn encryption off"]] }),
          f("anonymous", "check", "Anonymous mode (hide client name and version from peers)"),
        ],
      },
      {
        title: "Torrent queueing",
        fields: [
          f("queueing", "check", "Queue torrents beyond these limits"),
          f("maxActiveDl", "number", "Maximum active downloads", { dep: "queueing", min: 1, max: 999 }),
          f("maxActiveUp", "number", "Maximum active uploads", { dep: "queueing", min: 1, max: 999 }),
          f("maxActive", "number", "Maximum active torrents", { dep: "queueing", min: 1, max: 999 }),
          f("ignoreSlow", "check", "Don’t count slow torrents in these limits", { dep: "queueing" }),
        ],
      },
      {
        title: "Seeding limits",
        fields: [
          f("ratioOn", "check", "Stop seeding when the share ratio reaches a limit"),
          f("ratio", "number", "Ratio", { dep: "ratioOn", step: 0.1, max: 9999 }),
          f("seedTimeOn", "check", "Stop seeding after a set time"),
          f("seedTime", "number", "Seeding time", { dep: "seedTimeOn", unit: "minutes", max: 525600 }),
          f("seedAction", "select", "Then", { opts: [["pause", "Pause the torrent"], ["remove", "Remove the torrent"], ["removeData", "Remove the torrent and its files"], ["superseed", "Turn on super seeding"]] }),
        ],
      },
    ],
  },
  {
    id: "webui",
    title: "Web UI",
    groups: [
      {
        title: "Web interface",
        fields: [f("webPort", "number", "Port", { min: 1, max: 65535 }), f("webHttps", "check", "Use HTTPS instead of HTTP")],
      },
      {
        title: "Authentication",
        fields: [
          f("webUser", "text", "Username"),
          f("webPass", "password", "Password", { hint: "Leave empty to keep the current password." }),
          f("webLocalBypass", "check", "Skip sign-in for clients on this computer (localhost)"),
        ],
      },
    ],
  },
  { id: "backup", title: "Backup" },
];

export function sectionOf(key: string): SectionId | undefined {
  return SECTIONS.find((s) => s.groups?.some((g) => g.fields.some((x) => x.key === key)))?.id;
}

export function depOff(f: FieldDef, d: Settings): boolean {
  if (!f.dep) return false;
  const v = d[f.dep];
  return v === false || v === "none";
}
