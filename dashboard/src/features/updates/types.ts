export interface Release {
  version: string;
  date: string;
  notes: string[];
}

export interface UpdateState {
  state: "idle" | "checking" | "current" | "available" | "error" | "unconfigured";
  current: string;
  latest: Release | null;
  checked: number;
  configured: boolean;
  sample: boolean;
  error: string;
}
