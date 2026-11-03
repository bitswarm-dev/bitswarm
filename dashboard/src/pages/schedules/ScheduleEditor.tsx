import { useEffect, useState, type FormEvent } from "react";
import { Modal } from "@/components/Modal/Modal";
import { useToast } from "@/components/Toasts/Toasts";
import { useCatalog } from "@/features/catalog/queries";
import { useRenames } from "@/features/renames/queries";
import { useSettings } from "@/features/settings/queries";
import { RenameEditor } from "@/pages/renames/RenameEditor";
import { useCreateSchedule, useRunSchedule, useUpdateSchedule } from "@/features/schedules/mutations";
import { useTestSchedule } from "@/features/schedules/queries";
import { pickKnown } from "@/utils/safe";
import { parseTags } from "@/utils/tags";
import { BLANK_RULE, type Rule, type RuleView } from "@/features/schedules/types";
import { ItemTable } from "./ItemTable";

const DAY = 864e5;

const ymd = (ts: number) => {
  const d = new Date(ts);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

function searchUrls(r: Pick<Rule, "url" | "terms">): string[] {
  const u = r.url.trim();
  if (!u) return [];
  if (!u.includes("{query}")) return [u];
  return r.terms
    .split("\n")
    .map((q) => q.trim())
    .filter(Boolean)
    .map((q) => u.split("{query}").join(encodeURIComponent(q).replace(/%20/g, "+")));
}

function nameFor(r: Pick<Rule, "url" | "terms">): string {
  const first = r.terms.split("\n").map((q) => q.trim()).find(Boolean);
  if (first) return first;
  try {
    const u = new URL(r.url.trim());
    for (const k of ["q", "query", "search", "s", "keywords", "term", "k", "find"]) {
      const v = (u.searchParams.get(k) ?? "").split(/\s+/).filter(Boolean).join(" ");
      if (v) return v;
    }
    return u.host;
  } catch {
    return "";
  }
}

interface Props {
  rule: RuleView | null;
  onClose: () => void;
  onSaved: (r: RuleView) => void;
}

export function ScheduleEditor({ rule, onClose, onSaved }: Props) {
  const { data: catalog } = useCatalog();
  const { data: presets } = useRenames();
  const { data: sys } = useSettings();
  const [newPreset, setNewPreset] = useState(false);
  const create = useCreateSchedule();
  const update = useUpdateSchedule();
  const run = useRunSchedule();
  const toast = useToast();
  const [runNow, setRunNow] = useState(false);
  const test = useTestSchedule();
  const [form, setForm] = useState<Omit<Rule, "id">>(() => pickKnown(rule ? BLANK_RULE : { ...BLANK_RULE, start: sys.startAuto }, rule));
  const [tagText, setTagText] = useState(() => form.tags.join(", "));
  const [hasEnd, setHasEnd] = useState(!!rule?.endDate);
  const [endDay, setEndDay] = useState(ymd(rule?.endDate || Date.now() + 30 * DAY));
  const [error, setError] = useState("");
  const [testError, setTestError] = useState("");

  const set = <K extends keyof Omit<Rule, "id">>(k: K, v: Omit<Rule, "id">[K]) => setForm((f) => ({ ...f, [k]: v }));
  const urls = searchUrls(form);
  const validUrl = urls.length > 0 && urls.every((u) => /^https?:\/\/[^\s/]+/i.test(u));

  const runTest = () => {
    setTestError("");
    test.mutate(form, { onError: (e) => setTestError(e.message) });
  };

  useEffect(() => {
    if (!validUrl || !form.selector.trim()) return;
    const t = setTimeout(runTest, 600);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.url, form.terms, form.selector, form.attr, form.linkType, form.include, form.exclude, form.max, form.skipSeen]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    let endDate = 0;
    if (hasEnd) {
      if (!endDay) return setError("Pick an end date, or untick “Stop after”.");
      endDate = new Date(`${endDay}T23:59:59.999`).getTime();
    }
    const body = { ...form, endDate };
    const finish = (saved: RuleView) => {
      if (!runNow || !form.enabled) return onSaved(saved);
      run.mutate(saved.id, {
        onSuccess: (v) => {
          toast(v.status === "error" ? `${v.name}: run failed` : v.lastNew ? `${v.name}: added ${v.lastNew} torrent${v.lastNew > 1 ? "s" : ""}` : `${v.name}: nothing new`);
          onSaved(v);
        },
        onError: (err) => {
          toast(err.message);
          onSaved(saved);
        },
      });
    };
    const done = { onSuccess: finish, onError: (err: Error) => setError(err.message) };
    if (rule) update.mutate({ ...body, id: rule.id }, done);
    else create.mutate(body, done);
  };

  const res = test.data;

  return (
    <Modal title={rule ? "Edit schedule" : "New schedule"} subtitle="Search a site on a timer, pick the magnet or .torrent links out of the results, and add the new ones." wide onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <fieldset className="step">
          <legend><span className="sn">1</span>Search</legend>
          <label className="field">
            <span>Schedule name <em className="z">(optional)</em></span>
            <input type="text" value={form.name} onChange={(e) => set("name", e.target.value)} placeholder={nameFor(form) || "Named after the first search term"} autoFocus />
            <small>Left empty, it is named after the first search term.</small>
          </label>
          <label className="field">
            <span>Search URL</span>
            <input type="text" className="mono" value={form.url} onChange={(e) => set("url", e.target.value)} placeholder="https://example.org/search?q={query}" spellCheck={false} />
            <small>Put <code>{"{query}"}</code> where the search terms go, or paste a finished results address and leave the terms empty.</small>
          </label>
          <label className="field">
            <span>Search terms</span>
            <textarea rows={2} value={form.terms} onChange={(e) => set("terms", e.target.value)} placeholder="One per line" spellCheck={false} />
            <small>Each line becomes its own search.</small>
          </label>
          {urls.length > 0 && (
            <div className="urls">
              <span className="k">Each run fetches</span>
              {urls.map((u) => <code key={u}>{u}</code>)}
            </div>
          )}
        </fieldset>

        <fieldset className="step">
          <legend><span className="sn">2</span>Find links on the results page</legend>
          <div className="row3">
            <label className="field">
              <span>CSS selector</span>
              <input type="text" className="mono" value={form.selector} onChange={(e) => set("selector", e.target.value)} placeholder="a[href^='magnet:']" spellCheck={false} />
            </label>
            <label className="field">
              <span>Read link from</span>
              <select value={form.attr} onChange={(e) => set("attr", e.target.value)}>
                <option value="href">href</option>
                <option value="data-magnet">data-magnet</option>
                <option value="data-href">data-href</option>
                <option value="data-url">data-url</option>
                <option value="text">Element text</option>
              </select>
            </label>
            <label className="field">
              <span>Link type</span>
              <select value={form.linkType} onChange={(e) => set("linkType", e.target.value as Rule["linkType"])}>
                <option value="any">Magnet or .torrent</option>
                <option value="magnet">Magnet only</option>
                <option value="torrent">.torrent only</option>
              </select>
            </label>
          </div>
          <div className="row2">
            <label className="field">
              <span>Name must contain</span>
              <input type="text" value={form.include} onChange={(e) => set("include", e.target.value)} placeholder="e.g. amd64, 1080p" />
              <small>Every word listed, separated by commas.</small>
            </label>
            <label className="field">
              <span>Skip if name contains</span>
              <input type="text" value={form.exclude} onChange={(e) => set("exclude", e.target.value)} placeholder="e.g. server, beta" />
              <small>Any word listed.</small>
            </label>
          </div>
          <div className="sample">
            <h4 style={{ margin: "4px 0" }}>Preview results</h4>
            <p className="hint">Fetched live from the search URL and refreshed as you change the selector and filters.</p>
            <div className="res">
              {!validUrl && <p className="hint" style={{ margin: 0, color: "var(--muted)" }}>Enter the search URL (and a search term if it has <code>{"{query}"}</code>) to see the results.</p>}
              {validUrl && test.isPending && <p className="hint" style={{ margin: 0 }}>Fetching the results page…</p>}
              {testError && <p className="err-msg" style={{ margin: 0 }}>{testError}</p>}
              {res?.error && <p className="err-msg" style={{ margin: 0 }}>{res.error}</p>}
              {validUrl && res && !res.error && (
                <>
                  <p className="res-sum">
                    The selector matched {res.matched} element{res.matched === 1 ? "" : "s"}; {res.items.length} {res.items.length === 1 ? "is a usable link" : "are usable links"}, {res.items.filter((i) => i.state === "new").length} would be added.
                  </p>
                  {res.items.length ? (
                    <ItemTable items={res.items} />
                  ) : (
                    <p className="hint" style={{ margin: 0, color: "var(--muted)" }}>
                      No magnet or .torrent links found. Check that the selector points at the link elements and that “Read link from” names the attribute holding the URL.
                    </p>
                  )}
                </>
              )}
            </div>
          </div>
        </fieldset>

        <fieldset className="step">
          <legend><span className="sn">3</span>Download</legend>
          <div className="row3">
            <label className="field">
              <span>Save to</span>
              <input type="text" className="mono" value={form.path} placeholder={sys.savePath} onChange={(e) => set("path", e.target.value)} />
              <small>Leave empty to use the default save folder from Settings.</small>
            </label>
            <label className="field">
              <span>Tags</span>
              <input type="text" list="rule-tags" value={tagText} onChange={(e) => { setTagText(e.target.value); set("tags", parseTags(e.target.value)); }} placeholder="Comma-separated, such as 4K, keep" autoComplete="off" />
              <datalist id="rule-tags">{catalog.tags.map((g) => <option key={g} value={g} />)}</datalist>
            </label>
            <label className="field">
              <span>Max new per run</span>
              <input type="number" min={1} max={50} value={Number.isFinite(form.max) ? form.max : ""} onChange={(e) => set("max", e.target.value === "" ? NaN : Number(e.target.value))} />
            </label>
          </div>
          <label className="field">
            <span>Rename preset</span>
            <select value={form.renamePreset} onChange={(e) => (e.target.value === "new" ? setNewPreset(true) : set("renamePreset", Number(e.target.value)))}>
              <option value={0}>Don’t rename</option>
              {presets.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              <option value="new">New rename preset…</option>
            </select>
            <small>Applied to everything this schedule adds, at the time the preset is set to run.</small>
          </label>
          <label className="check"><input type="checkbox" checked={form.start} onChange={(e) => set("start", e.target.checked)} /> Start downloading right away</label>
          <label className="check"><input type="checkbox" checked={form.skipSeen} onChange={(e) => set("skipSeen", e.target.checked)} /> Skip links that were already downloaded</label>
        </fieldset>

        <fieldset className="step" style={{ border: 0, paddingBottom: 0 }}>
          <legend><span className="sn">4</span>Repeat</legend>
          <div className="repeat">
            <span>Run every</span>
            <input type="number" min={1} max={999} aria-label="Interval" value={Number.isFinite(form.every) ? form.every : ""} onChange={(e) => set("every", e.target.value === "" ? NaN : Number(e.target.value))} />
            <select aria-label="Interval unit" value={form.unit} onChange={(e) => set("unit", e.target.value as Rule["unit"])}>
              <option value="hours">hours</option>
              <option value="days">days</option>
            </select>
            <label className="check"><input type="checkbox" checked={form.enabled} onChange={(e) => set("enabled", e.target.checked)} /> Schedule is on</label>
            <label className="check" title="Searches once as soon as you save, instead of waiting for the first interval."><input type="checkbox" checked={runNow && form.enabled} disabled={!form.enabled} onChange={(e) => setRunNow(e.target.checked)} /> Run right away</label>
          </div>
          <div className="repeat">
            <label className="check"><input type="checkbox" checked={hasEnd} onChange={(e) => setHasEnd(e.target.checked)} /> Stop after</label>
            <input
              type="date"
              aria-label="End date"
              min={ymd(Date.now())}
              value={endDay}
              disabled={!hasEnd}
              onChange={(e) => setEndDay(e.target.value)}
              style={{ background: "var(--bg)", border: "1px solid var(--line)", borderRadius: 8, padding: "7px 10px" }}
            />
            <small style={{ color: "var(--muted)" }}>Optional. The schedule turns itself off after this day.</small>
          </div>
        </fieldset>

        {error && <p className="err-msg">{error}</p>}
        <div className="actions">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn primary" disabled={create.isPending || update.isPending || run.isPending}>{run.isPending ? "Running…" : "Save schedule"}</button>
        </div>
      </form>
      {newPreset && (
        <RenameEditor
          preset={null}
          onClose={() => setNewPreset(false)}
          onSaved={(p) => {
            set("renamePreset", p.id);
            setNewPreset(false);
          }}
        />
      )}
    </Modal>
  );
}
