import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";
import { readJson, writeJson } from "./storage";

export type SortKey = "queue" | "name" | "size" | "progress" | "status" | "dl" | "ul" | "eta" | "peers" | "ratio" | "added";
export type Tab = "general" | "files" | "peers" | "trackers" | "speed";
export type GroupKey = "status" | "automation" | "tags" | "developer";

interface UiState {
  filter: string;
  tag: string | null;
  q: string;
  sort: { key: SortKey; dir: 1 | -1 };
  sel: number[];
  active: number | null;
  anchor: number | null;
  tab: Tab;
  settingsDirty: boolean;
  logSeen: number;
  hooksAck: boolean;
  collapsed: Partial<Record<GroupKey, boolean>>;
}

interface UiCtx extends UiState {
  patch: (p: Partial<UiState>) => void;
  toggleGroup: (g: GroupKey) => void;
}

const SIDEBAR_KEY = "swarm-sidebar";
const UiContext = createContext<UiCtx | null>(null);

export function UiProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<UiState>(() => ({
    filter: "all",
    tag: null,
    q: "",
    sort: { key: "added", dir: -1 },
    sel: [],
    active: null,
    anchor: null,
    tab: "general",
    settingsDirty: false,
    logSeen: Date.now(),
    hooksAck: false,
    collapsed: readJson<Partial<Record<GroupKey, boolean>>>(SIDEBAR_KEY, {}),
  }));

  const patch = useCallback((p: Partial<UiState>) => setState((s) => ({ ...s, ...p })), []);
  const toggleGroup = useCallback((g: GroupKey) => {
    setState((s) => {
      const collapsed = { ...s.collapsed, [g]: !s.collapsed[g] };
      writeJson(SIDEBAR_KEY, collapsed);
      return { ...s, collapsed };
    });
  }, []);

  const value = useMemo(() => ({ ...state, patch, toggleGroup }), [state, patch, toggleGroup]);
  return <UiContext.Provider value={value}>{children}</UiContext.Provider>;
}

export function useUi() {
  const ctx = useContext(UiContext);
  if (!ctx) throw new Error("useUi must be used inside UiProvider");
  return ctx;
}
