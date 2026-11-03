export type TorrentStatus =
  | "downloading"
  | "checking"
  | "rechecking"
  | "seeding"
  | "done"
  | "paused"
  | "queued"
  | "error";

export type Priority = "high" | "normal" | "skip";

export interface TorrentFile {
  name: string;
  size: number;
  prio: Priority;
  orig?: string;
  done: number;
}

export interface Peer {
  ip: string;
  client: string;
  cc: string;
  prog: number;
  w: number;
  enc: boolean;
  utp: boolean;
  dl: number;
  ul: number;
}

export interface Torrent {
  simulated: boolean;
  trackers: string[];
  id: number;
  name: string;
  hash: string;
  size: number;
  done: number;
  status: TorrentStatus;
  tags: string[];
  files: TorrentFile[];
  path: string;
  tracker: string;
  seeds: number;
  leech: number;
  dl: number;
  ul: number;
  uploaded: number;
  queue: number;
  force: boolean;
  seq: boolean;
  flp: boolean;
  dlLimit: number;
  ulLimit: number;
  ratioLimit: number | null;
  seedSince: number;
  lastActive: number;
  added: number;
  presets: number[];
  err: string;
  pieceSize: number;
  seed: number;
  peers: Peer[];
  hist: { dl: number[]; ul: number[] };
  superseed: boolean;
  wanted: number;
  progress: number;
  ratio: number;
  eta: number | null;
  queuePos: number | null;
}

export interface Contents {
  size: number;
  files: TorrentFile[];
}

export interface AddRequest {
  magnet?: string;
  fileName?: string;
  fileData?: string;
  path: string;
  tags: string[];
  preset: number;
  start: boolean;
  layout: string;
  skipHash: boolean;
  seq: boolean;
  flp: boolean;
  top: boolean;
  selected?: number[];
}

export interface BatchRequest {
  action: string;
  ids: number[];
  deleteData?: boolean;
  how?: string;
  tag?: string;
  tags?: string[];
  on?: boolean;
  path?: string;
  moveFiles?: boolean;
  dlLimit?: number;
  ulLimit?: number;
  ratioMode?: string;
  ratio?: number;
  preset?: number;
}

export interface Stats {
  free: number;
  dht: number;
  sessDl: number;
  sessUl: number;
  alt: boolean;
  altActive: boolean;
  hist: { dl: number[]; ul: number[] };
  histWindow?: number;
  histStep?: number;
  uptime: number;
  torrents: number;
  active: number;
  conns: number;
  real: boolean;
  engine: string;
  port: number;
  totalDl: number;
  totalUl: number;
}
