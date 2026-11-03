import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { safeNumber } from "@/utils/safe";
import { readJson, writeJson } from "./storage";

export const ACCENTS = {
  mint: { name: "Mint", l: ["#12916b", "#ffffff", "#d8f2e8"], d: ["#5fdcb2", "#04201a", "#123229"] },
  ocean: { name: "Ocean", l: ["#3654c9", "#ffffff", "#e2e8fb"], d: ["#8099ff", "#0e1220", "#222c4d"] },
  violet: { name: "Violet", l: ["#7442c8", "#ffffff", "#ece3fa"], d: ["#b594ff", "#170c2e", "#2b2147"] },
  coral: { name: "Coral", l: ["#c8483a", "#ffffff", "#fbe4e0"], d: ["#ff8a7a", "#2a0c08", "#3d211d"] },
  amber: { name: "Amber", l: ["#a76a00", "#ffffff", "#f8ecd2"], d: ["#f2b84b", "#261a02", "#36290f"] },
} as const;

export type AccentKey = keyof typeof ACCENTS;
export type Mode = "system" | "light" | "dark";

export const BGS: [string, string][] = [
  ["none", "None"],
  ["aurora", "Aurora"],
  ["dots", "Dot grid"],
  ["grid", "Grid"],
  ["swarm", "Bitswarm"],
  ["photo", "Your photo"],
];

export interface Theme {
  mode: Mode;
  accent: AccentKey;
  bg: string;
  photo: string | null;
  vis: number;
  blur: number;
  panel: number;
}

export const THEME_DEFAULT: Theme = { mode: "system", accent: "mint", bg: "aurora", photo: null, vis: 55, blur: 0, panel: 90 };
const KEY = "swarm-theme";

interface Ctx {
  theme: Theme;
  setTheme: (p: Partial<Theme>) => void;
}

const ThemeContext = createContext<Ctx | null>(null);

function load(): Theme {
  const t = readJson<Theme>(KEY, THEME_DEFAULT);
  if (!(t.accent in ACCENTS)) t.accent = "mint";
  if (!BGS.some(([k]) => k === t.bg) || (t.bg === "photo" && !t.photo)) t.bg = "aurora";
  t.panel = safeNumber(t.panel, THEME_DEFAULT.panel);
  return t;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(load);

  const setTheme = useCallback((p: Partial<Theme>) => {
    setThemeState((cur) => {
      const next = { ...cur, ...p };
      if (!writeJson(KEY, next)) writeJson(KEY, { ...next, photo: null, bg: next.bg === "photo" ? "aurora" : next.bg });
      return next;
    });
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    const a = ACCENTS[theme.accent];
    const vars: [string, string][] = [
      ["acc-l", a.l[0]], ["accf-l", a.l[1]], ["accs-l", a.l[2]],
      ["acc-d", a.d[0]], ["accf-d", a.d[1]], ["accs-d", a.d[2]],
    ];
    vars.forEach(([k, v]) => root.style.setProperty(`--${k}`, v));
    if (theme.mode === "system") root.removeAttribute("data-theme");
    else root.setAttribute("data-theme", theme.mode);
    root.style.setProperty("--bg-dim", (1 - theme.vis / 100).toFixed(2));
    root.style.setProperty("--bg-blur", `${theme.blur}px`);
    root.style.setProperty("--panel", `${theme.panel}%`);
    root.style.setProperty("--panel-head", `${Math.min(100, theme.panel * 1.06).toFixed(1)}%`);
    root.style.setProperty("--panel-spark", `${(theme.panel * 0.95).toFixed(1)}%`);
    root.style.setProperty("--panel-blur", `${((14 * theme.panel) / 100).toFixed(1)}px`);
    document.body.classList.toggle("has-bg", theme.bg !== "none");
  }, [theme]);

  const value = useMemo(() => ({ theme, setTheme }), [theme, setTheme]);
  const bgClass = theme.bg !== "none" && theme.bg !== "photo" ? ` bg-${theme.bg}` : "";

  return (
    <ThemeContext.Provider value={value}>
      {theme.bg !== "none" && (
        <div
          className={`bgimg${bgClass}`}
          style={theme.bg === "photo" && theme.photo ? { backgroundImage: `url("${theme.photo}")` } : undefined}
        />
      )}
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside ThemeProvider");
  return ctx;
}
