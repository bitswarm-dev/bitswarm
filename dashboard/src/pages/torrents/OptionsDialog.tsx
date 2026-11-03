import { useState, type FormEvent } from "react";
import { Modal } from "@/components/Modal/Modal";
import { useSettings } from "@/features/settings/queries";
import type { Torrent } from "@/features/torrents/types";
import { useBatch } from "@/features/torrents/mutations";
import { useToast } from "@/components/Toasts/Toasts";

interface Props {
  kind: "move" | "limits";
  selection: Torrent[];
  onClose: () => void;
}

const same = <K,>(ts: Torrent[], f: (t: Torrent) => K) => ts.every((x) => f(x) === f(ts[0]));

const SEED_ACTION: Record<string, string> = {
  pause: "pause it",
  remove: "remove it",
  removeData: "remove it and its files",
  superseed: "turn on super seeding",
};

export function OptionsDialog({ kind, selection, onClose }: Props) {
  const { data: set } = useSettings();
  const batch = useBatch();
  const toast = useToast();
  const t = selection[0];
  const who = selection.length > 1 ? `${selection.length} torrents` : t.name;
  const ids = selection.map((x) => x.id);
  const [error, setError] = useState("");

  const [path, setPath] = useState(same(selection, (x) => x.path) ? t.path : "");
  const [moveFiles, setMoveFiles] = useState(true);

  const dlSame = same(selection, (x) => x.dlLimit);
  const ulSame = same(selection, (x) => x.ulLimit);
  const [dl, setDl] = useState(dlSame ? String(t.dlLimit) : "");
  const [ul, setUl] = useState(ulSame ? String(t.ulLimit) : "");
  const initialMode = !same(selection, (x) => x.ratioLimit) ? "global" : t.ratioLimit === null ? "global" : t.ratioLimit < 0 ? "none" : "custom";
  const [mode, setMode] = useState(initialMode);
  const [ratio, setRatio] = useState(String(initialMode === "custom" ? t.ratioLimit : 2));

  const parse = (v: string, mixed: boolean): number | null | undefined => {
    if (v.trim() === "") return mixed ? null : 0;
    const n = Number(v);
    return Number.isInteger(n) && n >= 0 && n <= 10000000 ? n : undefined;
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const fail = (m: string) => setError(m);
    if (kind === "move") {
      batch.mutate({ action: "move", ids, path, moveFiles }, { onSuccess: (r) => { toast(r.message); onClose(); }, onError: (err) => fail(err.message) });
      return;
    }
    const d = parse(dl, !dlSame);
    const u = parse(ul, !ulSame);
    if (d === undefined) return fail("The download limit must be a whole number of KiB/s, 0 or more.");
    if (u === undefined) return fail("The upload limit must be a whole number of KiB/s, 0 or more.");
    const r = Number(ratio);
    if (mode === "custom" && !(r > 0 && r <= 9999)) return fail("The ratio must be a number greater than 0, such as 1.5 or 2.");
    batch.mutate(
      { action: "limits", ids, dlLimit: d ?? undefined, ulLimit: u ?? undefined, ratioMode: mode, ratio: r },
      { onSuccess: (res) => { toast(res.message); onClose(); }, onError: (err) => fail(err.message) },
    );
  };

  return (
    <Modal
      title={kind === "move" ? "Move files" : "Speed and ratio limits"}
      subtitle={kind === "move" ? `Choose a new folder for ${who}.` : `These override the global limits for ${who}.`}
      onClose={onClose}
    >
      <form onSubmit={submit} noValidate>
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {kind === "move" ? (
            <>
              <label className="field">
                <span>New folder</span>
                <input type="text" className="mono" value={path} onChange={(e) => setPath(e.target.value)} placeholder="/downloads/somewhere" spellCheck={false} autoComplete="off" autoFocus />
              </label>
              <label className="check">
                <input type="checkbox" checked={moveFiles} onChange={(e) => setMoveFiles(e.target.checked)} /> Move the files already downloaded
              </label>
              <p className="hint" style={{ margin: 0 }}>
                Now in <code>{t.path}</code>
                {same(selection, (x) => x.path) ? "" : " and other folders"}. Untick the box to only change where new data goes.
              </p>
            </>
          ) : (
            <>
              <div className="row2">
                <label className="field">
                  <span>Download limit (KiB/s)</span>
                  <input type="number" min={0} max={10000000} value={dl} onChange={(e) => setDl(e.target.value)} placeholder={dlSame ? "" : "Mixed"} />
                  <small>0 means no limit of its own.</small>
                </label>
                <label className="field">
                  <span>Upload limit (KiB/s)</span>
                  <input type="number" min={0} max={10000000} value={ul} onChange={(e) => setUl(e.target.value)} placeholder={ulSame ? "" : "Mixed"} />
                </label>
              </div>
              <div className="row2">
                <label className="field">
                  <span>Share ratio limit</span>
                  <select value={mode} onChange={(e) => setMode(e.target.value)}>
                    <option value="global">Use the global setting ({set.ratioOn ? set.ratio : "off"})</option>
                    <option value="none">No limit</option>
                    <option value="custom">Custom</option>
                  </select>
                </label>
                <label className="field">
                  <span>Ratio</span>
                  <input type="number" min={0.1} max={9999} step={0.1} value={ratio} onChange={(e) => setRatio(e.target.value)} disabled={mode !== "custom"} />
                </label>
              </div>
              <p className="hint" style={{ margin: 0 }}>
                When it reaches the ratio, BitSwarm will {SEED_ACTION[set.seedAction]}, as set in Settings → BitTorrent.
              </p>
            </>
          )}
        </div>
        {error && <p className="err-msg">{error}</p>}
        <div className="actions">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn primary" disabled={batch.isPending}>{kind === "move" ? "Move" : "Save"}</button>
        </div>
      </form>
    </Modal>
  );
}
