import { useEffect, useState } from "react";
import { Svg } from "@/components/Icons/Svg";
import { useToast } from "@/components/Toasts/Toasts";
import { useRenames } from "@/features/renames/queries";
import { useCreateSchedule, useDeleteSchedule, useRunSchedule, useSetScheduleEnabled } from "@/features/schedules/mutations";
import { useScheduleLog, useSchedules } from "@/features/schedules/queries";
import { BLANK_RULE, type RuleView } from "@/features/schedules/types";
import { pickKnown } from "@/utils/safe";
import { formatWhen, relativeTime } from "@/utils/format";
import { PreviewPanel } from "./PreviewPanel";
import { ScheduleEditor } from "./ScheduleEditor";

const TYPE_TXT = { any: "magnet or .torrent", magnet: "magnet links", torrent: ".torrent files" };
const hostOf = (u: string) => {
  try {
    return new URL(u).host;
  } catch {
    return u || "—";
  }
};
const dayTxt = (ts: number) => {
  const d = new Date(ts);
  return d.toLocaleDateString([], { month: "short", day: "numeric", year: d.getFullYear() === new Date().getFullYear() ? undefined : "numeric" });
};

function useNow(ms = 1000) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(t);
  }, [ms]);
  return now;
}

export function Schedules() {
  const { data: rules } = useSchedules();
  const { data: log } = useScheduleLog();
  const { data: presets } = useRenames();
  const toast = useToast();
  const toggle = useSetScheduleEnabled();
  const run = useRunSchedule();
  const del = useDeleteSchedule();
  const create = useCreateSchedule();
  const now = useNow();
  const [editing, setEditing] = useState<RuleView | null | undefined>(undefined);
  const [previewing, setPreviewing] = useState<number | null>(null);
  const [deleting, setDeleting] = useState<number | null>(null);

  return (
    <section className="sched" aria-label="Scheduled downloads">
      <div className="sched-head">
        <div>
          <h2>Scheduled downloads</h2>
          <p>Each schedule searches a website on a timer, reads the magnet or .torrent links out of the results with a CSS selector, and adds anything new to your list.</p>
        </div>
        <button className="btn primary" onClick={() => setEditing(null)}>
          <Svg size={15} strokeWidth={2.4}><path d="M12 5v14M5 12h14" /></Svg>
          New schedule
        </button>
      </div>
      <div className="note">
        <Svg size={16}><circle cx="12" cy="12" r="9" /><path d="M12 8h.01M11 12h1v5h1" /></Svg>
        <span>
          Timed runs, Run now and Preview all fetch the live search URL from this server.
        </span>
      </div>

      <div className="rules">
        {rules.length === 0 ? (
          <div className="empty"><strong>No schedules yet</strong>Create one to search a site on a timer and add new torrents automatically.</div>
        ) : (
          rules.map((r) => {
            const preset = presets.find((p) => p.id === r.renamePreset);
            return (
              <article key={r.id} className={`rule ${r.enabled ? "" : "off"}`}>
                <div>
                  <div className="rule-top">
                    <label className="switch" title={r.enabled ? "Turn off" : "Turn on"}>
                      <input
                        type="checkbox"
                        checked={r.enabled}
                        aria-label={`${r.name} schedule on`}
                        onChange={(e) =>
                          toggle.mutate({ id: r.id, enabled: e.target.checked }, {
                            onSuccess: (v) => toast(v.enabled ? `${v.name} is on · next run ${relativeTime(v.nextRun)}` : `${v.name} is off`),
                            onError: (err) => toast(err.message),
                          })
                        }
                      />
                      <span />
                    </label>
                    <h3>{r.name}</h3>
                    <span className="chip">{hostOf(r.urls[0] ?? r.url)}</span>
                    {r.status === "error" && <span className="pill st-error" title={r.lastError}>Last run failed</span>}
                  </div>
                  <dl className="rule-spec">
                    <div>
                      <dt>Search</dt>
                      <dd className="num">{r.urls[0] ?? r.url}{r.urls.length > 1 && <> <span className="z">+{r.urls.length - 1} more</span></>}</dd>
                    </div>
                    <div>
                      <dt>Find</dt>
                      <dd>
                        <span className="num">{r.selector}</span> <span className="z">→</span> {r.attr === "text" ? "text" : <span className="num">{r.attr}</span>}, {TYPE_TXT[r.linkType]}
                        {r.include && <> · must contain <i>{r.include}</i></>}
                        {r.exclude && <> · skip <i>{r.exclude}</i></>}
                      </dd>
                    </div>
                    <div>
                      <dt>Then</dt>
                      <dd>
                        Add up to {r.max} new to <span className="num">{r.path}</span>
                        {r.tags.length > 0 && <> tagged {r.tags.map((g) => <span key={g} className="tag">{g}</span>)}</>}, {r.start ? "start right away" : "added paused"}
                        {preset && <>, rename with <b>{preset.name}</b></>}
                      </dd>
                    </div>
                  </dl>
                </div>
                <div className="rule-side">
                  <div className="side-top">
                    <div className="when">
                      <span className="every">Every {r.every} {r.every === 1 ? r.unit.slice(0, -1) : r.unit}</span>
                      <span className="next">
                        {r.enabled ? <>Next run <b>{relativeTime(r.nextRun, now)}</b></> : <span className="z">{r.ended ? "Ended" : "Off"}</span>}
                      </span>
                      {r.endDate > 0 && (
                        <span className="last">
                          {r.ended ? "Ended" : "Ends"} {dayTxt(r.endDate)}
                          {!r.ended && r.enabled ? ` · ${r.runsLeft} run${r.runsLeft === 1 ? "" : "s"} left` : ""}
                        </span>
                      )}
                      <span className="last">
                        {r.lastRun ? `Last run ${formatWhen(r.lastRun)} · ${r.status === "error" ? r.lastError : r.lastNew ? `${r.lastNew} new of ${r.lastFound}` : `${r.lastFound} found, none new`}` : "Never run"}
                      </span>
                    </div>
                    <button className="btn icon-del" title="Delete schedule" aria-label={`Delete ${r.name}`} onClick={() => setDeleting(r.id)}>
                      <Svg size={15}><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" /></Svg>
                    </button>
                  </div>
                  <div className="rule-actions">
                    <button className="btn" aria-expanded={previewing === r.id} onClick={() => setPreviewing(previewing === r.id ? null : r.id)}>
                      <Svg><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z" /><circle cx="12" cy="12" r="3" /></Svg>
                      Preview
                    </button>
                    <button
                      className="btn"
                      disabled={run.isPending}
                      onClick={() =>
                        run.mutate(r.id, {
                          onSuccess: (v) => toast(v.status === "error" ? `${v.name}: run failed` : v.lastNew ? `${v.name}: added ${v.lastNew} torrent${v.lastNew > 1 ? "s" : ""}` : `${v.name}: nothing new`),
                          onError: (err) => toast(err.message),
                        })
                      }
                    >
                      <Svg size={13} fill="currentColor"><path d="M7 4.5v15l13-7.5z" /></Svg>
                      Run now
                    </button>
                    <button className="btn" onClick={() => setEditing(r)}>Edit</button>
                    <button
                      className="btn"
                      disabled={create.isPending}
                      onClick={() => {
                        const copy = { ...pickKnown(BLANK_RULE, r), name: `${r.name} copy`, enabled: false, ended: false, endDate: r.endDate > Date.now() ? r.endDate : 0, lastRun: 0, lastFound: 0, lastNew: 0, status: "idle" as const, lastError: "", nextRun: 0 };
                        create.mutate(copy, { onSuccess: (v) => setEditing(v), onError: (err) => toast(err.message) });
                      }}
                    >
                      Duplicate
                    </button>
                  </div>
                </div>
                {previewing === r.id && <PreviewPanel rule={r} onClose={() => setPreviewing(null)} />}
                {deleting === r.id && (
                  <div className="confirm">
                    <strong>Delete “{r.name}”?</strong>
                    <span style={{ color: "var(--muted)" }}>Torrents it already added stay in your list.</span>
                    <span style={{ flex: 1 }} />
                    <button className="btn" onClick={() => setDeleting(null)}>Cancel</button>
                    <button className="btn danger solid" onClick={() => del.mutate(r.id, { onSuccess: () => { setDeleting(null); toast(`Deleted ${r.name}`); } })}>Delete</button>
                  </div>
                )}
              </article>
            );
          })
        )}
      </div>

      <section className="log">
        <h3>Activity</h3>
        {log.length ? (
          <ol>
            {log.map((l) => (
              <li key={l.id} className={l.kind}>
                <time>{formatWhen(l.t)}</time>
                <span className="dotk" />
                <div>
                  <b>{l.rule}</b> · {l.text}
                  {l.added.length > 0 && <span className="added">Added {l.added.join(", ")}</span>}
                </div>
              </li>
            ))}
          </ol>
        ) : (
          <p className="z">No runs yet.</p>
        )}
      </section>

      {editing !== undefined && (
        <ScheduleEditor
          rule={editing}
          onClose={() => setEditing(undefined)}
          onSaved={(r) => {
            setEditing(undefined);
            toast(r.enabled ? `Saved ${r.name} · next run ${relativeTime(r.nextRun)}` : `Saved ${r.name} (off)`);
          }}
        />
      )}
    </section>
  );
}
