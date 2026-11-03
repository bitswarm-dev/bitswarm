import { useUi, type Tab } from "@/app/ui";
import { LineChart } from "@/components/Charts/LineChart";
import { PieceMap } from "@/components/Charts/PieceMap";
import { useToast } from "@/components/Toasts/Toasts";
import { STATUS_TEXT } from "@/features/torrents/filters";
import { useRetryAnnounce, useSetPriority } from "@/features/torrents/mutations";
import type { Priority, Torrent } from "@/features/torrents/types";
import { useSettings } from "@/features/settings/queries";
import { formatBytes, formatEta, formatKib, formatRate } from "@/utils/format";

const TABS: [Tab, string][] = [
  ["general", "General"],
  ["files", "Files"],
  ["peers", "Peers"],
  ["trackers", "Trackers"],
  ["speed", "Speed"],
];

function pillColor(t: Torrent): React.CSSProperties {
  const c = t.status === "error" ? "err" : ["seeding", "done"].includes(t.status) ? "up" : ["paused", "queued"].includes(t.status) ? "idle" : "dl";
  return { "--c": `var(--${c})`, "--cs": `var(--${c}-soft)` } as React.CSSProperties;
}

function General({ t, avgTitle }: { t: Torrent; avgTitle: string }) {
  const retry = useRetryAnnounce();
  const toast = useToast();
  const nP = Math.ceil(t.size / t.pieceSize);
  const have = Math.floor(nP * t.progress);
  const avail = t.status === "checking" || !t.simulated ? "–" : t.seeds > 0 ? (t.seeds / Math.max(1, t.leech) + 1).toFixed(3) : (0.4 + (t.seed % 50) / 100).toFixed(3);

  const copyHash = () => {
    const done = () => toast("Info hash copied");
    if (navigator.clipboard) navigator.clipboard.writeText(t.hash).then(done, () => toast("Couldn’t copy. Select the hash and press Ctrl+C."));
    else toast("Couldn’t copy. Select the hash and press Ctrl+C.");
  };

  return (
    <>
      {t.status === "error" && (
        <div className="alert">
          <span>{t.err}</span>
          <button className="btn danger solid" onClick={() => retry.mutate(t.id, { onSuccess: () => toast("Tracker responded · 37 seeds") })}>
            Retry announce
          </button>
        </div>
      )}
      <div className="pieces-wrap">
        <div className="cap">
          <span>Pieces · {nP.toLocaleString()} × {formatBytes(t.pieceSize)}</span>
          <span className="num">{have.toLocaleString()} of {nP.toLocaleString()} verified</span>
        </div>
        <PieceMap seed={t.seed} progress={t.progress} status={t.status} />
      </div>
      <dl className="meta">
        <div><dt>Downloaded</dt><dd className="num">{formatBytes(t.done)} / {formatBytes(t.wanted)}</dd></div>
        <div><dt>Uploaded</dt><dd className="num">{formatBytes(t.uploaded)}</dd></div>
        <div><dt>Share ratio</dt><dd className="num">{t.ratio.toFixed(3)}</dd></div>
        <div><dt>Time left</dt><dd className="num">{t.status === "downloading" ? formatEta(t.eta) : "–"}</dd></div>
        <div><dt>Download speed</dt><dd className="num d">{formatRate(t.dl)}</dd></div>
        <div><dt>Upload speed</dt><dd className="num u">{formatRate(t.ul)}</dd></div>
        <div><dt>Seeds / peers</dt><dd className="num">{t.seeds} / {t.leech}</dd></div>
        <div><dt>Availability</dt><dd className="num">{avail}</dd></div>
        <div><dt>Save path</dt><dd className="num" title={t.path}>{t.path}</dd></div>
        <div><dt>Tags</dt><dd>{t.tags.length ? t.tags.map((g) => `#${g}`).join(" ") : "–"}</dd></div>
        <div><dt>Queue position</dt><dd className="num">{t.queuePos ?? "–"}{t.force ? " (forced)" : ""}</dd></div>
        <div>
          <dt>Own limits</dt>
          <dd className="num">
            {t.dlLimit || t.ulLimit ? `↓ ${formatKib(t.dlLimit)} · ↑ ${formatKib(t.ulLimit)}` : "None"}
            {t.ratioLimit !== null ? ` · ratio ${t.ratioLimit < 0 ? "no limit" : t.ratioLimit}` : ""}
          </dd>
        </div>
        <div><dt>Piece order</dt><dd>{t.seq ? "Sequential" : "Rarest first"}{t.flp ? ", first and last first" : ""}</dd></div>
        <div><dt>Added</dt><dd>{new Date(t.added).toLocaleString([], { dateStyle: "medium", timeStyle: "short" })}</dd></div>
      </dl>
      <div className="hash">
        <span className="k">Info hash (v1)</span>
        <code>{t.hash}</code>
        <button className="btn" onClick={copyHash}>Copy</button>
      </div>
      <span hidden>{avgTitle}</span>
    </>
  );
}

function Files({ t }: { t: Torrent }) {
  const setPrio = useSetPriority();
  const toast = useToast();
  return (
    <div className="tbl-wrap">
      <table>
        <thead>
          <tr><th>Name</th><th className="r">Size</th><th>Progress</th><th>Priority</th></tr>
        </thead>
        <tbody>
          {t.files.map((f, i) => {
            const skip = f.prio === "skip";
            const pct = t.simulated ? t.progress : f.size > 0 ? f.done / f.size : 1;
            return (
              <tr key={`${f.name}-${i}`}>
                <td className="fname">
                  {f.name}
                  {f.orig && f.orig !== f.name && <span className="was">Renamed from {f.orig}</span>}
                </td>
                <td className="r num">{formatBytes(f.size)}</td>
                <td className={`st-${skip ? "paused" : pct >= 1 ? "seeding" : "downloading"}`}>
                  <span className="bar"><i style={{ width: `${skip ? 0 : pct * 100}%` }} /></span>
                  <span className="num">{skip && pct === 0 ? "Skipped" : `${(pct * 100).toFixed(1)}%`}</span>
                </td>
                <td>
                  <select
                    className="prio"
                    aria-label={`Priority for ${f.name}`}
                    value={f.prio}
                    onChange={(e) => setPrio.mutate({ id: t.id, idx: i, prio: e.target.value as Priority }, { onError: (err) => toast(err.message) })}
                  >
                    <option value="high">High</option>
                    <option value="normal">Normal</option>
                    <option value="skip">Do not download</option>
                  </select>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function Peers({ t }: { t: Torrent }) {
  if (!["downloading", "seeding"].includes(t.status))
    return <p className="blank" style={{ height: "auto" }}>No peer connections while this torrent is {STATUS_TEXT[t.status].toLowerCase()}.</p>;
  const peers = [...t.peers].sort((a, b) => b.dl + b.ul - (a.dl + a.ul));
  return (
    <div className="tbl-wrap">
      <table>
        <thead>
          <tr>
            <th>Address</th><th>Client</th>
            <th title="E = encrypted, P = µTP, D = downloading from, U = uploading to">Flags</th>
            <th>Country</th><th className="r">Progress</th><th className="r">Down</th><th className="r">Up</th>
          </tr>
        </thead>
        <tbody>
          {peers.map((p) => {
            const flags = (p.dl > 1 ? "D" : "") + (p.ul > 1 ? "U" : "") + (p.enc ? "E" : "") + (p.utp ? "P" : "");
            return (
              <tr key={p.ip}>
                <td className="num">{p.ip}</td><td>{p.client}</td><td className="num flags">{flags || "–"}</td><td>{p.cc}</td>
                <td className="r num">{(p.prog * 100).toFixed(1)}%</td>
                <td className={`r num ${p.dl > 1 ? "d" : "z"}`}>{formatRate(p.dl)}</td>
                <td className={`r num ${p.ul > 1 ? "u" : "z"}`}>{formatRate(p.ul)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function Trackers({ t }: { t: Torrent }) {
  const { data: set } = useSettings();
  if (!t.simulated)
    return (
      <div className="tbl-wrap">
        <table>
          <thead>
            <tr><th>URL</th><th>Status</th></tr>
          </thead>
          <tbody>
            {t.trackers.length === 0 ? (
              <tr><td colSpan={2} className="z">This torrent lists no trackers. It finds peers through DHT{set.dht ? "" : " (turned off in Settings)"}.</td></tr>
            ) : (
              t.trackers.map((u) => <tr key={u}><td className="num">{u}</td><td className="z">Status isn’t reported by the BitTorrent engine yet</td></tr>)
            )}
          </tbody>
        </table>
      </div>
    );
  const sec = 1800 - Math.floor((Date.now() / 1000 + t.seed) % 1800);
  const err = t.status === "error";
  const ok = <span className="ok">Working</span>;
  const off = <span className="z">Disabled</span>;
  const rows: [string, React.ReactNode, React.ReactNode, React.ReactNode, string][] = [];
  if (t.tracker !== "DHT only")
    rows.push([
      t.tracker,
      err ? <span className="bad">Timed out</span> : ok,
      err ? "–" : t.seeds,
      err ? "–" : t.leech,
      err ? `Retrying in 0:${String(sec % 60).padStart(2, "0")}` : `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, "0")}`,
    ]);
  rows.push(["** [DHT] **", set.dht ? ok : off, set.dht ? Math.round(t.seeds * 0.6) : 0, set.dht ? Math.round(t.leech * 0.5) : 0, "–"]);
  rows.push(["** [PeX] **", set.pex ? ok : off, set.pex ? Math.round(t.seeds * 0.1) : 0, set.pex ? Math.round(t.leech * 0.3) : 0, "–"]);
  rows.push(["** [LSD] **", set.lsd ? <span className="z">Not contacted</span> : off, 0, 0, "–"]);
  return (
    <div className="tbl-wrap">
      <table>
        <thead>
          <tr><th>URL</th><th>Status</th><th className="r">Seeds</th><th className="r">Peers</th><th className="r">Next announce</th></tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r[0]}>
              <td className="num">{r[0]}</td><td>{r[1]}</td><td className="r num">{r[2]}</td><td className="r num">{r[3]}</td><td className="r num">{r[4]}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Speed({ t }: { t: Torrent }) {
  const avg = (a: number[]) => a.reduce((x, y) => x + y, 0) / Math.max(1, a.length);
  return (
    <>
      <div className="pieces-wrap">
        <div className="cap">
          <span><span className="d">● Download</span> &nbsp; <span className="u">● Upload</span></span>
          <span className="num">avg ↓ {formatRate(avg(t.hist.dl))} · ↑ {formatRate(avg(t.hist.ul))} · last 90 s</span>
        </div>
      </div>
      <LineChart hist={t.hist} axes id="speed" />
    </>
  );
}

export function Inspector({ torrent }: { torrent: Torrent | null }) {
  const ui = useUi();
  const t = torrent;
  const count = (k: Tab) => (!t ? "" : k === "files" ? t.files.length : k === "peers" ? (["downloading", "seeding"].includes(t.status) ? t.peers.length : 0) : "");

  return (
    <section className="insp" aria-label="Torrent details">
      <div className="tabs" role="tablist">
        {TABS.map(([k, l]) => (
          <button key={k} className="tab" role="tab" aria-selected={ui.tab === k} onClick={() => ui.patch({ tab: k })}>
            {l}
            {count(k) !== "" && <span className="n">{count(k)}</span>}
          </button>
        ))}
      </div>
      <div className="pane">
        {!t ? (
          <div className="blank">Select a torrent to see its pieces, files, peers and trackers.</div>
        ) : (
          <>
            <div className="pane-title">
              <h2>{t.name}</h2>
              <span className={`pill st-${t.status}`} style={pillColor(t)}>{STATUS_TEXT[t.status]}</span>
            </div>
            {ui.tab === "general" && <General t={t} avgTitle="" />}
            {ui.tab === "files" && <Files t={t} />}
            {ui.tab === "peers" && <Peers t={t} />}
            {ui.tab === "trackers" && <Trackers t={t} />}
            {ui.tab === "speed" && <Speed t={t} />}
          </>
        )}
      </div>
    </section>
  );
}
