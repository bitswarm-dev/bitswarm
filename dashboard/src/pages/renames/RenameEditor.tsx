import { useEffect, useState, type FormEvent } from "react";
import { Modal } from "@/components/Modal/Modal";
import { useCatalog } from "@/features/catalog/queries";
import { useCreatePreset, useUpdatePreset } from "@/features/renames/mutations";
import { usePreview } from "@/features/renames/queries";
import { pickKnown } from "@/utils/safe";
import { BLANK_PRESET, type Preset } from "@/features/renames/types";

interface Props {
  preset: Preset | null;
  onClose: () => void;
  onSaved: (p: Preset) => void;
}

export function RenameEditor({ preset, onClose, onSaved }: Props) {
  const { data: catalog } = useCatalog();
  const create = useCreatePreset();
  const update = useUpdatePreset();
  const [form, setForm] = useState<Omit<Preset, "id" | "count">>(() => pickKnown(BLANK_PRESET, preset));
  const [samples, setSamples] = useState("");
  const [error, setError] = useState("");
  const [debounced, setDebounced] = useState({ form, samples });

  useEffect(() => {
    const t = setTimeout(() => setDebounced({ form, samples }), 180);
    return () => clearTimeout(t);
  }, [form, samples]);

  const preview = usePreview(debounced.form, debounced.samples).data;
  const tests = (preview?.rows ?? []).filter((r) => r.src === "Your names");
  const set = <K extends keyof typeof form>(k: K, v: (typeof form)[K]) => setForm((f) => ({ ...f, [k]: v }));

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const done = { onSuccess: (p: Preset) => onSaved(p), onError: (err: Error) => setError(err.message) };
    if (preset) update.mutate({ ...form, id: preset.id }, done);
    else create.mutate(form, done);
  };

  return (
    <Modal title={preset ? "Edit rename preset" : "New rename preset"} subtitle="Find part of a name with a regular expression and replace it." wide onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <label className="field">
          <span>Preset name</span>
          <input type="text" value={form.name} onChange={(e) => set("name", e.target.value)} placeholder="Tidy Ubuntu ISO names" autoFocus />
        </label>
        <div className="row2">
          <label className="field">
            <span>Find (regex)</span>
            <input type="text" className="mono" value={form.find} onChange={(e) => set("find", e.target.value)} spellCheck={false} autoComplete="off" placeholder="^ubuntu-(\d+\.\d+)-desktop-(\w+)$" />
          </label>
          <label className="field">
            <span>Replace with</span>
            <input type="text" className="mono" value={form.replace} onChange={(e) => set("replace", e.target.value)} spellCheck={false} autoComplete="off" placeholder="Ubuntu $1 Desktop ($2)" />
          </label>
        </div>
        <div className="flagrow">
          <label className="check"><input type="checkbox" checked={form.i} onChange={(e) => set("i", e.target.checked)} /> Ignore case</label>
          <label className="check"><input type="checkbox" checked={form.g} onChange={(e) => set("g", e.target.checked)} /> Replace every match</label>
          <label className="check"><input type="checkbox" checked={form.keepExt} onChange={(e) => set("keepExt", e.target.checked)} /> Keep the file extension</label>
        </div>
        <p className="hint">
          Use <code>$1</code>, <code>$2</code> for captured groups, <code>$&amp;</code> for the whole match, or leave “Replace with” empty to delete the match. Put a <code>/</code> in “Replace with” to move a renamed file into a folder, such as <code>Show/Show - $1</code>. Other characters that aren’t allowed in file names become “-”.
          Patterns use Go’s regular expression syntax, which has no lookahead or backreferences.
        </p>
        {form.replace.includes("/") && form.target !== "torrent" && (
          <p className="note" role="note" style={{ margin: 0 }}>
            <span><strong>This moves files into a folder.</strong> The folder is created inside the torrent’s save folder if it doesn’t exist. If it already exists, the renamed file goes into it. A file that already has that name there is never overwritten: the new one is numbered instead, like “Show - 06 (2).mkv”.</span>
          </p>
        )}
        <div className="grid3">
          <label className="field">
            <span>Rename</span>
            <select value={form.target} onChange={(e) => set("target", e.target.value as Preset["target"])}>
              <option value="files">File names</option>
              <option value="torrent">Torrent name</option>
              <option value="both">Both</option>
            </select>
          </label>
          <label className="field">
            <span>Only for tag</span>
            <select value={form.tag} onChange={(e) => set("tag", e.target.value)}>
              <option value="">Any tag</option>
              {catalog.tags.map((g) => <option key={g}>{g}</option>)}
            </select>
          </label>
          <label className="field">
            <span>When</span>
            <select value={form.when} onChange={(e) => set("when", e.target.value as Preset["when"])}>
              <option value="complete">Download finishes</option>
              <option value="add">Torrent is added</option>
            </select>
          </label>
        </div>
        <label className="check">
          <input type="checkbox" checked={form.enabled} onChange={(e) => set("enabled", e.target.checked)} /> Auto-apply to every matching torrent (otherwise it runs only where you choose it)
        </label>
        <div className="step" style={{ border: 0, padding: 0, gap: 10 }}>
          <h3 className="pvh">Preview</h3>
          <div className="res rn">
            {!form.find ? (
              <p className="z" style={{ margin: 0 }}>Enter a pattern in “Find” to see which names it changes.</p>
            ) : preview?.error ? (
              <p className="err-msg" style={{ margin: 0 }}>{preview.error}</p>
            ) : preview ? (
              <>
                {preview.rows.some((r) => r.src !== "Your names") && (<p className="res-sum">
                  {preview.ok} of {preview.total} name{preview.total === 1 ? "" : "s"} would change{form.tag ? ` (torrents tagged ${form.tag} only)` : ""}.
                </p>)}
                {preview.rows.some((r) => r.src !== "Your names") ? (
                  <div className="tbl-wrap">
                    <table>
                      <thead><tr><th>Now</th><th></th><th>After</th><th>Result</th></tr></thead>
                      <tbody>
                        {preview.rows.filter((r) => r.src !== "Your names").map((r, i) => (
                          <tr key={`${r.from}-${i}`}>
                            <td className="fname num">{r.from}<span className="src">{r.src}</span></td>
                            <td className="arrow">→</td>
                            <td className="fname num">{r.empty ? <span className="z">(nothing left)</span> : r.to}</td>
                            <td>
                              {r.empty ? <span className="pill st-error">Skipped</span> : r.clash ? <span className="pill st-queued">Name taken, numbered</span> : <span className="pill st-seeding">Renamed</span>}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <p className="z" style={{ margin: 0 }}>None of your torrents match yet. Paste names in the test box below to try the pattern.</p>
                )}
              </>
            ) : null}
          </div>
        </div>
        <fieldset className="step" style={{ marginTop: 12 }}>
          <legend>Test your regex</legend>
          <textarea rows={3} value={samples} onChange={(e) => setSamples(e.target.value)} placeholder="Paste file names to test, one per line" spellCheck={false} aria-label="Names to test" />
          {tests.length > 0 && (
            <div className="tbl-wrap">
              <table>
                <thead><tr><th>Name</th><th></th><th>After</th><th>Result</th></tr></thead>
                <tbody>
                  {tests.map((r, i) => (
                    <tr key={`${r.from}-${i}`}>
                      <td className="fname num">{r.from}</td>
                      <td className="arrow">→</td>
                      <td className="fname num">{r.same ? <span className="z">{r.match ? "(unchanged)" : "(no match)"}</span> : r.empty ? <span className="z">(nothing left)</span> : r.to}</td>
                      <td>
                        {r.same ? (
                          <span className="pill st-error">{r.match ? "Matches, no change" : "No match"}</span>
                        ) : r.empty ? (
                          <span className="pill st-error">Skipped</span>
                        ) : (
                          <span className="pill st-seeding">Renamed</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {form.keepExt && tests.some((r) => r.same && !r.match && r.tested !== r.from) && (
            <p className="hint" style={{ margin: 0 }}>“Keep the file extension” is on, so the pattern is tested against the name without its extension. Leave the extension out of the pattern, or untick that box.</p>
          )}
        </fieldset>
        {error && <p className="err-msg">{error}</p>}
        <div className="actions">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn primary" disabled={create.isPending || update.isPending}>Save preset</button>
        </div>
      </form>
    </Modal>
  );
}
