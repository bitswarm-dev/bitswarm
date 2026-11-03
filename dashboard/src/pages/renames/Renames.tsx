import { useState } from "react";
import { Svg } from "@/components/Icons/Svg";
import { useToast } from "@/components/Toasts/Toasts";
import { useApplyPreset, useDeletePreset, useDuplicatePreset, useSetPresetEnabled, useUndoRename } from "@/features/renames/mutations";
import { useRenameLog, useRenames } from "@/features/renames/queries";
import type { Preset } from "@/features/renames/types";
import { plural, formatWhen } from "@/utils/format";
import { RenameEditor } from "./RenameEditor";

const WHEN_TXT = { complete: "When the download finishes", add: "As soon as a torrent is added" };
const TARGET_TXT = { files: "File names", torrent: "Torrent name", both: "Files and torrent" };
const flags = (p: Preset) => (p.g ? "g" : "") + (p.i ? "i" : "");

export function Renames() {
  const { data: presets } = useRenames();
  const { data: log } = useRenameLog();
  const toast = useToast();
  const toggle = useSetPresetEnabled();
  const dup = useDuplicatePreset();
  const del = useDeletePreset();
  const apply = useApplyPreset();
  const undo = useUndoRename();
  const [editing, setEditing] = useState<Preset | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<number | null>(null);

  return (
    <section className="sched" aria-label="Auto rename">
      <div className="sched-head">
        <div>
          <h2>Auto rename</h2>
          <p>
            Presets find part of a name with a regular expression and replace it. Turn on auto-apply to run a preset on every matching torrent, or pick it in a schedule, in Add torrent, or from Rename in
            the torrent toolbar. Presets run top to bottom.
          </p>
        </div>
        <button className="btn primary" onClick={() => setEditing(null)}>
          <Svg size={15} strokeWidth={2.4}><path d="M12 5v14M5 12h14" /></Svg>
          New preset
        </button>
      </div>

      <div className="rules">
        {presets.length === 0 ? (
          <div className="empty">
            <strong>No rename presets yet</strong>Create one to tidy file names with a regular expression.
          </div>
        ) : (
          presets.map((r) => (
            <article key={r.id} className={`rule ${r.enabled ? "" : "off"}`}>
              <div>
                <div className="rule-top">
                  <label className="switch" title="Auto-apply">
                    <input type="checkbox" checked={r.enabled} aria-label={`Auto-apply ${r.name}`} onChange={(e) => toggle.mutate({ id: r.id, enabled: e.target.checked }, { onSuccess: () => toast(e.target.checked ? `${r.name} now runs automatically` : `${r.name} runs only when chosen`) })} />
                    <span />
                  </label>
                  <h3>{r.name}</h3>
                  <span className="chip">{TARGET_TXT[r.target]}</span>
                  {r.tag && <span className="tag">{r.tag}</span>}
                </div>
                <dl className="rule-spec">
                  <div><dt>Find</dt><dd className="re"><span className="sl">/</span>{r.find}<span className="sl">/</span><span className="fl">{flags(r)}</span></dd></div>
                  <div><dt>Replace</dt><dd className="re">{r.replace ? r.replace : <span className="z">(delete the match)</span>}</dd></div>
                  <div><dt>Runs</dt><dd>{WHEN_TXT[r.when]}{r.keepExt ? " · extension kept" : ""}</dd></div>
                </dl>
                {r.example ? (
                  <div className="ex"><span>e.g.</span><code>{r.example.from}</code><span>→</span><code className="to">{r.example.to}</code></div>
                ) : (
                  <div className="ex">No current names match this preset.</div>
                )}
              </div>
              <div className="rule-side">
                <div className="side-top">
                  <div className="when">
                    <span className="every">{r.enabled ? "Auto-apply on" : "Only when chosen"}</span>
                    <span className="last">
                      {r.enabled ? (r.tag ? `Every torrent tagged ${r.tag}` : "Every torrent") : "Pick it in a schedule, in Add torrent, or from Rename"}
                    </span>
                    {r.usedBy.length > 0 && <span className="last">Used by {r.usedBy.join(", ")}</span>}
                    <span className="last">{r.count} renamed so far</span>
                  </div>
                  <button className="btn icon-del" title="Delete preset" aria-label={`Delete ${r.name}`} onClick={() => setDeleting(r.id)}>
                    <Svg size={15}><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" /></Svg>
                  </button>
                </div>
                <div className="rule-actions">
                  <button
                    className="btn"
                    onClick={() =>
                      apply.mutate({ id: r.id }, {
                        onSuccess: (res) => toast(res.names ? `${r.name}: renamed ${plural(res.names, "name")} in ${plural(res.torrents, "torrent")}` : `${r.name}: no current names match`),
                        onError: (e) => toast(e.message),
                      })
                    }
                  >
                    Apply to existing
                  </button>
                  <button className="btn" onClick={() => setEditing(r)}>Edit</button>
                  <button className="btn" onClick={() => dup.mutate(r.id, { onSuccess: (c) => setEditing(c) })}>Duplicate</button>
                </div>
              </div>
              {deleting === r.id && (
                <div className="confirm">
                  <strong>Delete “{r.name}”?</strong>
                  <span style={{ color: "var(--muted)" }}>Names it already changed stay changed.{r.usedBy.length ? " Schedules using it will stop renaming." : ""}</span>
                  <span style={{ flex: 1 }} />
                  <button className="btn" onClick={() => setDeleting(null)}>Cancel</button>
                  <button className="btn danger solid" onClick={() => del.mutate(r.id, { onSuccess: () => { setDeleting(null); toast(`Deleted ${r.name}`); } })}>Delete</button>
                </div>
              )}
            </article>
          ))
        )}
      </div>

      <section className="log">
        <h3>Recent renames</h3>
        {log.length ? (
          <ol>
            {log.map((l) => (
              <li key={l.id} className={l.undone ? "undone" : "ok"}>
                <time>{formatWhen(l.t)}</time>
                <span className="dotk" />
                <div>
                  <b>{l.rule}</b> · <span className="num">{l.from}</span> → <span className="num">{l.to}</span>
                  {l.undone ? (
                    <span className="z"> · undone</span>
                  ) : (
                    <button className="btn undo" onClick={() => undo.mutate(l.id, { onSuccess: () => toast(`Renamed back to ${l.from}`) })}>Undo</button>
                  )}
                </div>
              </li>
            ))}
          </ol>
        ) : (
          <p className="z" style={{ margin: 0 }}>Nothing renamed yet. Presets run when downloads finish, or use Apply to existing.</p>
        )}
      </section>

      {editing !== undefined && (
        <RenameEditor preset={editing} onClose={() => setEditing(undefined)} onSaved={(p) => { setEditing(undefined); toast(`Saved ${p.name}`); }} />
      )}
    </section>
  );
}
