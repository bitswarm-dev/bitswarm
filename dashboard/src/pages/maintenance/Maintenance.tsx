import { useEffect, useState, type FormEvent } from "react";
import { useToast } from "@/components/Toasts/Toasts";
import { useCatalog } from "@/features/catalog/queries";
import { useRunTask, useSetTaskEnabled, useUpdateTask } from "@/features/maintenance/mutations";
import { useMaintenanceLog, useTaskPreview, useTasks } from "@/features/maintenance/queries";
import { FIELDS, type Task, type TaskView } from "@/features/maintenance/types";
import { formatWhen, plural, relativeTime } from "@/utils/format";

const lower = (s: string) => s.charAt(0).toLowerCase() + s.slice(1);
const everyText = (t: Task) => `Every ${t.every === 1 ? t.unit.slice(0, -1) : `${t.every} ${t.unit}`}`;

function Preview({ task, onClose }: { task: TaskView; onClose: () => void }) {
  const q = useTaskPreview(task.id);
  const run = useRunTask();
  const toast = useToast();
  const data = q.data;
  return (
    <div className="preview">
      <div className="pv-head">
        <h4>Preview</h4>
        <span>What Run now would do right now. Nothing changes until you run it.</span>
      </div>
      {!data ? (
        <p className="z" style={{ margin: 0 }}>Checking…</p>
      ) : data.items.length ? (
        <div className="tbl-wrap">
          <table>
            <thead><tr><th>Item</th><th>Why</th><th>Action</th></tr></thead>
            <tbody>
              {data.items.map((i, n) => (
                <tr key={`${i.name}-${n}`}><td className="fname">{i.name}</td><td>{i.why}</td><td>{i.what}</td></tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p className="z" style={{ margin: 0 }}>{data.done}, so nothing would change.</p>
      )}
      <div className="pv-foot">
        <span className="sum">{plural(data?.items.length ?? 0, "item")}</span>
        <button className="btn" onClick={onClose}>Close preview</button>
        {!!data?.items.length && (
          <button
            className="btn primary"
            onClick={() => run.mutate(task.id, { onSuccess: (r) => { toast(`${task.title}: ${lower(r.result)}`); onClose(); }, onError: (e) => toast(e.message) })}
          >
            Run now
          </button>
        )}
      </div>
    </div>
  );
}

function Editor({ task, onClose }: { task: TaskView; onClose: () => void }) {
  const { data: catalog } = useCatalog();
  const update = useUpdateTask();
  const toast = useToast();
  const [form, setForm] = useState({ every: task.every, unit: task.unit, tag: task.tag, p: { ...task.p } });
  const [error, setError] = useState("");

  const submit = (e: FormEvent) => {
    e.preventDefault();
    update.mutate(
      { id: task.id, every: form.every, unit: form.unit, tag: form.tag, p: form.p },
      { onSuccess: () => { toast(`Saved ${task.title}`); onClose(); }, onError: (err) => setError(err.message) },
    );
  };

  return (
    <form className="preview medit" onSubmit={submit} noValidate>
      <div className="pv-head"><h4>Edit task</h4></div>
      {(FIELDS[task.type] ?? []).map((f) => {
        const id = `m-${task.id}-${f.key}`;
        if (f.type === "check")
          return (
            <label key={f.key} className="check">
              <input type="checkbox" id={id} checked={Boolean(form.p[f.key])} onChange={(e) => setForm((s) => ({ ...s, p: { ...s.p, [f.key]: e.target.checked } }))} /> {f.label}
            </label>
          );
        return (
          <div key={f.key} className="sfield">
            <label htmlFor={id}>{f.label}</label>
            <div className="inp">
              {f.type === "select" ? (
                <select id={id} value={String(form.p[f.key])} onChange={(e) => setForm((s) => ({ ...s, p: { ...s.p, [f.key]: e.target.value } }))}>
                  {f.opts?.map(([v, t]) => <option key={v} value={v}>{t}</option>)}
                </select>
              ) : (
                <input
                  id={id}
                  type="number"
                  min={f.min ?? 0}
                  max={f.max}
                  step={f.step ?? 1}
                  value={Number.isFinite(form.p[f.key] as number) ? String(form.p[f.key]) : ""}
                  onChange={(e) => setForm((s) => ({ ...s, p: { ...s.p, [f.key]: e.target.value.trim() === "" ? NaN : Number(e.target.value) } }))}
                />
              )}
              {f.unit && <span className="unit">{f.unit}</span>}
            </div>
            {f.hint && <small>{f.hint}</small>}
          </div>
        );
      })}
      {task.scoped && (
        <div className="sfield">
          <label htmlFor={`m-${task.id}-tag`}>Only for tag</label>
          <div className="inp">
            <select id={`m-${task.id}-tag`} value={form.tag} onChange={(e) => setForm((s) => ({ ...s, tag: e.target.value }))}>
              <option value="">All torrents</option>
              {catalog.tags.map((g) => <option key={g}>{g}</option>)}
            </select>
          </div>
        </div>
      )}
      <div className="sfield">
        <label htmlFor={`m-${task.id}-every`}>Run every</label>
        <div className="inp">
          <input id={`m-${task.id}-every`} type="number" min={1} max={999} value={Number.isFinite(form.every) ? form.every : ""} onChange={(e) => setForm((s) => ({ ...s, every: e.target.value.trim() === "" ? NaN : Number(e.target.value) }))} />
          <select aria-label="Interval unit" value={form.unit} onChange={(e) => setForm((s) => ({ ...s, unit: e.target.value as Task["unit"] }))}>
            {["minutes", "hours", "days"].map((u) => <option key={u}>{u}</option>)}
          </select>
        </div>
      </div>
      {error && <p className="err-msg">{error}</p>}
      <div className="pv-foot">
        <span className="sum" />
        <button type="button" className="btn" onClick={onClose}>Cancel</button>
        <button type="submit" className="btn primary" disabled={update.isPending}>Save</button>
      </div>
    </form>
  );
}

export function Maintenance() {
  const { data: tasks } = useTasks();
  const { data: log } = useMaintenanceLog();
  const toggle = useSetTaskEnabled();
  const run = useRunTask();
  const toast = useToast();
  const [now, setNow] = useState(Date.now());
  const [preview, setPreview] = useState<number | null>(null);
  const [edit, setEdit] = useState<number | null>(null);

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  return (
    <section className="sched" aria-label="Maintenance">
      <div className="sched-head">
        <div>
          <h2>Maintenance</h2>
          <p>Housekeeping tasks that keep Bitswarm tidy: retry failed torrents, clear out finished ones and watch disk space. Each task runs on its own timer.</p>
        </div>
      </div>
      <div className="rules">
        {tasks.map((m) => (
          <article key={m.id} className={`rule ${m.enabled ? "" : "off"}`}>
            <div>
              <div className="rule-top">
                <label className="switch" title={m.enabled ? "Turn off" : "Turn on"}>
                  <input
                    type="checkbox"
                    checked={m.enabled}
                    aria-label={`${m.title} on`}
                    onChange={(e) =>
                      toggle.mutate({ id: m.id, enabled: e.target.checked }, { onSuccess: (v) => toast(v.enabled ? `${v.title} is on · next run ${relativeTime(v.nextRun)}` : `${v.title} is off`) })
                    }
                  />
                  <span />
                </label>
                <h3>{m.title}</h3>
                {m.tag && <span className="tag">{m.tag}</span>}
              </div>
              <p className="mdesc">{m.desc}</p>
              <dl className="rule-spec">
                <div>
                  <dt>Rule</dt>
                  <dd>{m.summary}{m.scoped ? (m.tag ? ` · only #${m.tag}` : " · all torrents") : ""}</dd>
                </div>
              </dl>
            </div>
            <div className="rule-side">
              <div className="when">
                <span className="every">{everyText(m)}</span>
                <span className="next">{m.enabled ? <>Next run <b>{relativeTime(m.nextRun, now)}</b></> : <span className="z">Off</span>}</span>
                <span className="last">{m.lastRun ? `Last run ${formatWhen(m.lastRun)} · ${m.lastText}` : "Never run"}</span>
              </div>
              <div className="rule-actions">
                <button className="btn" aria-expanded={preview === m.id} onClick={() => { setPreview(preview === m.id ? null : m.id); setEdit(null); }}>Preview</button>
                <button
                  className="btn"
                  disabled={run.isPending}
                  onClick={() => run.mutate(m.id, { onSuccess: (r) => toast(`${m.title}: ${lower(r.result)}`), onError: (e) => toast(e.message) })}
                >
                  Run now
                </button>
                <button className="btn" aria-expanded={edit === m.id} onClick={() => { setEdit(edit === m.id ? null : m.id); setPreview(null); }}>Edit</button>
              </div>
            </div>
            {preview === m.id && <Preview task={m} onClose={() => setPreview(null)} />}
            {edit === m.id && <Editor task={m} onClose={() => setEdit(null)} />}
          </article>
        ))}
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
                  <b>{l.task}</b> · {l.text}
                  {l.added.length > 0 && <span className="added">{l.added.join(", ")}{l.more ? ` and ${l.more} more` : ""}</span>}
                </div>
              </li>
            ))}
          </ol>
        ) : (
          <p className="z" style={{ margin: 0 }}>No maintenance has run yet.</p>
        )}
      </section>
    </section>
  );
}
