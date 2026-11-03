import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useUi } from "@/app/ui";
import { useToast } from "@/components/Toasts/Toasts";
import type { Level } from "@/features/log/api";
import { useClearLog } from "@/features/log/mutations";
import { useLog } from "@/features/log/queries";
import { plural } from "@/utils/format";
import { useCopy } from "@/pages/torrents/ContextMenu";
import { useUrlText } from "@/utils/useUrlText";
import { levelsOf, toLogSearch } from "./logSearch";

const LEVELS: [Level, string][] = [["error", "Errors"], ["warn", "Warnings"], ["info", "Info"], ["debug", "Debug"]];
const NAME: Record<Level, string> = { error: "Error", warn: "Warning", info: "Info", debug: "Debug" };
const entries = (n: number) => plural(n, "entry").replace("entrys", "entries");

export function Log() {
  const search = useSearch({ from: "/log" });
  const navigate = useNavigate({ from: "/log" });
  const levels = levelsOf(search);
  const src = search.source ?? "";
  const q = search.text ?? "";
  const change = (next: Partial<{ levels: Level[]; source: string; text: string }>) =>
    void navigate({ search: toLogSearch({ levels, source: src, text: q, ...next }), replace: true });
  const setLevels = (fn: (cur: Level[]) => Level[]) => change({ levels: fn(levels) });
  const setSrc = (source: string) => change({ source });
  const [qText, setQText] = useUrlText(q, (text) => change({ text }));
  const [confirming, setConfirming] = useState(false);
  const { data } = useLog({ levels, src, q });
  const ui = useUi();
  const clear = useClearLog();
  const copy = useCopy();
  const toast = useToast();
  const { patch } = ui;
  useEffect(() => {
    patch({ logSeen: Date.now() });
  }, [patch, data.total]);
  const total = Object.values(data.counts).reduce((a, b) => a + b, 0);

  const toggle = (l: Level) => setLevels((cur) => (cur.includes(l) ? cur.filter((x) => x !== l) : [...cur, l]));

  const copyShown = () => {
    if (!data.entries.length) return toast("Nothing to copy with these filters");
    copy(
      data.entries.map((l) => `${new Date(l.t).toISOString()} ${NAME[l.level].toUpperCase().padEnd(7)} [${l.src}] ${l.msg}`).join("\n"),
      entries(data.entries.length),
    );
  };

  return (
    <section className="sched" aria-label="Execution log">
      <div className="sched-head">
        <div>
          <h2>Execution log</h2>
          <p>Everything Bitswarm did, newest first: torrents, schedules, renames, maintenance, hooks, sign-ins and settings.</p>
        </div>
      </div>
      <div className="logbar">
        <div className="lvchips" role="group" aria-label="Show levels">
          {LEVELS.map(([k, l]) => (
            <button key={k} type="button" className="lvchip" aria-pressed={levels.includes(k)} onClick={() => toggle(k)}>
              {l} <span className="num">{data.counts[k]}</span>
            </button>
          ))}
        </div>
        <select aria-label="Source" value={src} onChange={(e) => setSrc(e.target.value)}>
          <option value="">All sources</option>
          {[...data.sources].sort().map((s) => <option key={s}>{s}</option>)}
        </select>
        <label className="search">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
          <input type="search" placeholder="Search the log" aria-label="Search the log" value={qText} onChange={(e) => setQText(e.target.value)} />
        </label>
        <span style={{ flex: 1 }} />
        <button type="button" className="btn" onClick={copyShown}>Copy shown</button>
        <button type="button" className="btn danger" onClick={() => setConfirming(true)}>Clear</button>
      </div>
      {confirming && (
        <div className="confirm" style={{ borderRadius: 8, border: 0 }}>
          <strong>Clear the execution log?</strong>
          <span style={{ color: "var(--muted)" }}>This removes all {entries(total)}.</span>
          <span style={{ flex: 1 }} />
          <button type="button" className="btn" onClick={() => setConfirming(false)}>Cancel</button>
          <button type="button" className="btn danger solid" onClick={() => clear.mutate(undefined, { onSuccess: () => setConfirming(false) })}>Clear</button>
        </div>
      )}
      <div className="tbl-wrap">
        <table className="logtbl">
          <thead><tr><th>Time</th><th>Level</th><th>Source</th><th>Message</th></tr></thead>
          <tbody>
            {data.entries.length ? (
              data.entries.map((l) => (
                <tr key={l.id} className={`lg-${l.level}`}>
                  <td className="num">{new Date(l.t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</td>
                  <td><span className={`lvpill lv-${l.level}`}>{NAME[l.level]}</span></td>
                  <td>{l.src}</td>
                  <td className="lmsg">{l.msg}</td>
                </tr>
              ))
            ) : (
              <tr><td colSpan={4} className="z" style={{ padding: "20px 10px", whiteSpace: "normal" }}>No entries match these filters.</td></tr>
            )}
          </tbody>
        </table>
      </div>
      <p className="z" style={{ margin: 0 }}>
        {data.total > data.entries.length ? `Showing the newest ${data.entries.length} of ${entries(data.total)}.` : `${entries(data.total)} shown.`}
      </p>
    </section>
  );
}
