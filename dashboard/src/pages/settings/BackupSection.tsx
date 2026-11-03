import { useState, type ChangeEvent } from "react";
import { ACCENTS, BGS, useTheme, type AccentKey, type Mode, type Theme } from "@/app/theme";
import { useToast } from "@/components/Toasts/Toasts";
import { useApplyBackup, useCheckBackup, useExportBackup } from "@/features/backup/mutations";
import { useBackupCounts } from "@/features/backup/queries";
import { PARTS, type Plan } from "@/features/backup/types";
import { useCopy } from "@/pages/torrents/ContextMenu";
import { formatBytes } from "@/utils/format";

const MAX_BYTES = 5 * 1024 * 1024;

function sanitizeTheme(d: Record<string, unknown>): Partial<Theme> {
  const patch: Partial<Theme> = {};
  if (["system", "light", "dark"].includes(d.mode as string)) patch.mode = d.mode as Mode;
  if (typeof d.accent === "string" && d.accent in ACCENTS) patch.accent = d.accent as AccentKey;
  if (typeof d.bg === "string" && d.bg !== "photo" && BGS.some(([k]) => k === d.bg)) patch.bg = d.bg;
  if (typeof d.vis === "number" && Number.isFinite(d.vis)) patch.vis = Math.max(0, Math.min(90, d.vis));
  if (typeof d.blur === "number" && Number.isFinite(d.blur)) patch.blur = Math.max(0, Math.min(24, d.blur));
  return patch;
}

export function BackupSection() {
  const { theme, setTheme } = useTheme();
  const toast = useToast();
  const copy = useCopy();
  const counts = useBackupCounts().data ?? {};
  const exporter = useExportBackup();
  const checker = useCheckBackup();
  const applier = useApplyBackup();

  const [parts, setParts] = useState<Set<string>>(new Set(PARTS.map(([k]) => k)));
  const [secrets, setSecrets] = useState(false);
  const [out, setOut] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [plan, setPlan] = useState<Plan | null>(null);
  const [use, setUse] = useState<Set<string>>(new Set());
  const [mode, setMode] = useState("merge");
  const [error, setError] = useState("");
  const [skipped, setSkipped] = useState<{ part: string; name: string; reason: string }[]>([]);

  const togglePart = (k: string, on: boolean) => {
    setParts((cur) => {
      const next = new Set(cur);
      if (on) next.add(k);
      else next.delete(k);
      return next;
    });
    setOut(null);
  };

  const create = () => {
    if (!parts.size) return toast("Choose at least one part to export");
    const server = [...parts].filter((p) => p !== "theme");
    const finish = (json: string) => {
      let final = json;
      if (parts.has("theme")) {
        const { photo: _photo, ...rest } = theme;
        const doc = JSON.parse(json);
        doc.parts.theme = rest;
        final = JSON.stringify(doc, null, 2);
      }
      setOut(final);
    };
    if (!server.length) return finish(JSON.stringify({ swarm_backup: 1, app_version: "0.9.0", exported_at: new Date().toISOString(), parts: {} }));
    exporter.mutate({ parts: server, secrets }, { onSuccess: (r) => finish(r.json), onError: (e) => toast(e.message) });
  };

  const download = () => {
    if (!out) return;
    const url = URL.createObjectURL(new Blob([out], { type: "application/json" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = "bitswarm-backup.json";
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  const check = (value: string) => {
    setText(value);
    setPlan(null);
    setSkipped([]);
    setError("");
    checker.mutate(value, {
      onSuccess: (p) => {
        setPlan(p);
        setUse(new Set(p.parts.map((x) => x.key)));
      },
      onError: (e) => setError(e.message),
    });
  };

  const onFile = (e: ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    e.target.value = "";
    if (!f) return;
    if (f.size > MAX_BYTES) return setError("That file is too large to be a Bitswarm export.");
    const rd = new FileReader();
    rd.onload = () => check(String(rd.result));
    rd.onerror = () => setError("Couldn’t read that file. Try pasting its contents instead.");
    rd.readAsText(f);
  };

  const apply = () => {
    if (!plan) return;
    const chosen = [...use];
    applier.mutate(
      { text, use: chosen.filter((k) => k !== "theme"), mode },
      {
        onSuccess: (r) => {
          const done = [...r.done];
          if (chosen.includes("theme") && plan.theme) {
            setTheme(sanitizeTheme(plan.theme));
            done.push("theme");
          }
          toast(`Imported ${done.join(", ")}`);
          setSkipped(r.skipped);
          setPlan(null);
          setText("");
        },
        onError: (e) => setError(e.message),
      },
    );
  };

  const when = plan?.at && !Number.isNaN(Date.parse(plan.at)) ? ` on ${new Date(plan.at).toLocaleString([], { dateStyle: "medium", timeStyle: "short" })}` : "";

  return (
    <>
      <fieldset className="sgroup">
        <legend>Export</legend>
        <p className="snote" style={{ margin: 0 }}>
          Save everything Bitswarm keeps as one JSON file, to move it to another computer or keep as a backup. Your Web UI password and API tokens are never exported.
        </p>
        <div className="bkparts">
          {PARTS.map(([k, l]) => (
            <label key={k} className="check">
              <input type="checkbox" checked={parts.has(k)} onChange={(e) => togglePart(k, e.target.checked)} /> {l} {counts[k] && <span className="z">{counts[k]}</span>}
            </label>
          ))}
        </div>
        <label className="check">
          <input type="checkbox" checked={secrets} onChange={(e) => { setSecrets(e.target.checked); setOut(null); }} /> Include secrets: hook signing secrets, Authorization headers and the proxy password
        </label>
        <div>
          <button type="button" className="btn primary" disabled={exporter.isPending} onClick={create}>Create export</button>
        </div>
        {out && (
          <div className="bkout">
            <textarea rows={9} readOnly value={out} aria-label="Exported settings as JSON" id="bk-out" />
            <div className="pv-foot">
              <span className="sum">{formatBytes(new Blob([out]).size)}. Download it as <code>bitswarm-backup.json</code>, or copy it.</span>
              <button type="button" className="btn" onClick={() => document.getElementById("bk-out")?.focus()}>Select all</button>
              <button type="button" className="btn" onClick={() => copy(out, "Export")}>Copy</button>
              <button type="button" className="btn primary" onClick={download}>Download</button>
            </div>
          </div>
        )}
      </fieldset>

      <fieldset className="sgroup">
        <legend>Import</legend>
        <p className="snote" style={{ margin: 0 }}>Paste an export or choose a .json file. You’ll see what it contains before anything changes.</p>
        <textarea className="bkin" rows={6} value={text} onChange={(e) => { setText(e.target.value); setPlan(null); }} placeholder='{"swarm_backup": 1, …}' spellCheck={false} aria-label="Exported JSON to import" />
        <div className="bkrow">
          <label className="btn bkfile">
            <input type="file" accept=".json,application/json" onChange={onFile} />
            Choose a file…
          </label>
          <button type="button" className="btn" onClick={() => check(text)} disabled={checker.isPending}>Check import</button>
        </div>
        {error && <p className="err-msg">{error}</p>}
        {plan && (
          <div className="bkplan">
            <h4>Ready to import</h4>
            <p className="snote" style={{ margin: 0 }}>Exported from Bitswarm {plan.from}{when}. Choose what to bring in:</p>
            {plan.parts.map((p) => (
              <label key={p.key} className="check">
                <input
                  type="checkbox"
                  checked={use.has(p.key)}
                  onChange={(e) => setUse((cur) => { const next = new Set(cur); if (e.target.checked) next.add(p.key); else next.delete(p.key); return next; })}
                />{" "}
                {p.name} <span className="z">{p.summary}</span>
              </label>
            ))}
            <div className="bkmode" role="radiogroup" aria-label="How to import">
              <label className="check"><input type="radio" name="bk-mode" value="merge" checked={mode === "merge"} onChange={() => setMode("merge")} /> Add to what’s here, skipping items with the same name</label>
              <label className="check"><input type="radio" name="bk-mode" value="replace" checked={mode === "replace"} onChange={() => setMode("replace")} /> Replace your schedules, presets and hooks with the imported ones</label>
            </div>
            <p className="snote" style={{ margin: 0 }}>
              Settings, theme and maintenance tasks are always replaced. Tags are always added, so your torrents keep theirs. Imported hooks start turned off so you can check them first.
            </p>
            <div className="pv-foot">
              <span className="sum" />
              <button type="button" className="btn" onClick={() => setPlan(null)}>Cancel</button>
              <button type="button" className="btn primary" disabled={!use.size || applier.isPending} onClick={apply}>Import</button>
            </div>
          </div>
        )}
        {skipped.length > 0 && (
          <div className="note">
            <span>
              <b>Skipped {skipped.length} item{skipped.length === 1 ? "" : "s"}:</b>{" "}
              {skipped.map((s) => `${s.part} “${s.name}” (${s.reason})`).join("; ")}
            </span>
          </div>
        )}
      </fieldset>
    </>
  );
}
