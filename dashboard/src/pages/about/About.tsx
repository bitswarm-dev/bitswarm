import { useState } from "react";
import { Link } from "react-router-dom";
import { writeFlag } from "@/app/storage";
import { useToast } from "@/components/Toasts/Toasts";
import { useSaveSettings } from "@/features/settings/mutations";
import { useSettings } from "@/features/settings/queries";
import { useCheckUpdates } from "@/features/updates/mutations";
import { useUpdates } from "@/features/updates/queries";
import { useStats } from "@/features/torrents/queries";
import { formatBytes, formatEta, formatWhen } from "@/utils/format";

export const VERSION = "0.9.0";

export function About() {
  const { data: set } = useSettings();
  const { data: stats } = useStats();
  const toast = useToast();
  const { data: upd } = useUpdates();
  const check = useCheckUpdates();
  const save = useSaveSettings();
  const [wiping, setWiping] = useState(false);
  const [show, setShow] = useState<"how" | "notes" | null>(null);

  const runCheck = () =>
    check.mutate(undefined, {
      onSuccess: (st) => {
        if (st.state === "current") toast("You’re using the latest version");
        if (st.state === "available" && st.latest) toast(`Bitswarm ${st.latest.version} is available. You’re using ${st.current}, so please update.`);
      },
      onError: (e) => toast(e.message),
    });

  const dateText = (iso: string) => new Date(iso).toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" });
  const latest = upd.latest;

  const up = stats.uptime < 60 ? `${stats.uptime}s` : formatEta(stats.uptime);

  const wipe = () => {
    for (const k of ["swarm-theme", "swarm-sidebar", "swarm-legal-ack", "swarm-hooks-ack"]) {
      try {
        localStorage.removeItem(k);
      } catch {
        /* blocked storage: nothing to clear */
      }
    }
    toast("Browser data reset. Reloading…");
    setTimeout(() => location.reload(), 600);
  };

  return (
    <section className="sched" aria-label="About Bitswarm">
      <div className="sched-head">
        <div>
          <h2>About Bitswarm</h2>
          <p>Version, session numbers and the small print.</p>
        </div>
      </div>
      <div className="set-body" style={{ maxWidth: 820 }}>
        <fieldset className="sgroup">
          <legend>Updates</legend>
          {check.isPending || upd.state === "checking" ? (
            <div className="upd"><span className="spin" aria-hidden="true" /><div><h4>Checking for updates…</h4><p>Looking for a newer version of Bitswarm.</p></div></div>
          ) : upd.state === "available" && latest ? (
            <div className="upd avail" role="status">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--warn)" strokeWidth="2"><path d="M12 3v12M7 10l5 5 5-5M5 21h14" /></svg>
              <div style={{ minWidth: 0, flex: 1 }}>
                <h4>Please update Bitswarm</h4>
                <p>You’re using version <b className="num">{upd.current}</b>. The latest version is <b className="num">{latest.version}</b>, released {dateText(latest.date)}.</p>
                <div className="acts">
                  <button className="btn primary" aria-expanded={show === "how"} onClick={() => setShow(show === "how" ? null : "how")}>How to update</button>
                  <button className="btn" aria-expanded={show === "notes"} onClick={() => setShow(show === "notes" ? null : "notes")}>What’s new in {latest.version}</button>
                  <button className="btn" onClick={runCheck}>Check again</button>
                </div>
                {show === "notes" && <ul>{latest.notes.map((n) => <li key={n}>{n}</li>)}</ul>}
                {show === "how" && (
                  <ol>
                    <li>Download Bitswarm {latest.version} from where you installed it, or pull the new image if you run it in Docker.</li>
                    <li>Stop Bitswarm. Your torrents keep their progress.</li>
                    <li>Replace the old files with the new ones.</li>
                    <li>Start Bitswarm again. Your settings, schedules, presets and tags are kept.</li>
                  </ol>
                )}
                <p className="snote" style={{ margin: "8px 0 0" }}>Last checked {formatWhen(upd.checked)}</p>
              </div>
            </div>
          ) : upd.state === "current" ? (
            <div className="upd ok">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--up)" strokeWidth="2.4"><path d="m5 12 5 5 9-10" /></svg>
              <div>
                <h4>Bitswarm is up to date</h4>
                <p>You’re using the latest version, <b className="num">{upd.current}</b>. Last checked {formatWhen(upd.checked)}.</p>
                <div className="acts"><button className="btn" onClick={runCheck}>Check again</button></div>
              </div>
            </div>
          ) : upd.state === "error" ? (
            <div className="upd">
              <div>
                <h4>Couldn’t check for updates</h4>
                <p>{upd.error}</p>
                <div className="acts"><button className="btn" onClick={runCheck}>Try again</button></div>
              </div>
            </div>
          ) : !upd.configured ? (
            <div className="upd">
              <div>
                <h4>Version {upd.current}</h4>
                <p>Update checks need a release feed. Start the server with <code>SWARM_UPDATE_FEED</code> set to the address of a JSON list of releases, or with <code>SWARM_DEMO=1</code> to try a sample feed.</p>
              </div>
            </div>
          ) : (
            <div className="upd">
              <div>
                <h4>Version {upd.current}</h4>
                <p>Check whether a newer version of Bitswarm is available.</p>
                <div className="acts"><button className="btn primary" onClick={runCheck}>Check for updates</button></div>
              </div>
            </div>
          )}
          <label className="check">
            <input
              type="checkbox"
              checked={set.autoUpdate}
              onChange={(e) =>
                save.mutate({ ...set, autoUpdate: e.target.checked }, { onSuccess: () => toast(e.target.checked ? "Bitswarm will check for updates when it starts" : "Automatic update checks are off"), onError: (err) => toast(err.message) })
              }
            />{" "}
            Check for updates automatically when Bitswarm starts
          </label>
          {upd.sample && <p className="snote" style={{ margin: 0 }}>This server is using a sample release feed, so “1.0.0” isn’t a real release.</p>}
        </fieldset>
        <fieldset className="sgroup">
          <legend>Bitswarm</legend>
          <p style={{ margin: 0, maxWidth: "60ch" }}>A BitTorrent client with a web interface, with scheduled downloads, regex rename presets and tags.</p>
          <dl className="meta" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(250px,1fr))" }}>
            <div><dt>Version</dt><dd className="num">{VERSION}</dd></div>
            <div><dt>Source code</dt><dd><a href="https://github.com/keithics/bitswarm" target="_blank" rel="noopener noreferrer">github.com/keithics/bitswarm</a></dd></div>
            <div><dt>Engine</dt><dd>{stats.real ? `${stats.engine}, built into the server` : "Simulated, built into the server"}</dd></div>
            <div><dt>Web UI port</dt><dd className="num">{set.webPort}{set.webHttps ? " (HTTPS)" : ""}</dd></div>
            <div><dt>Listening port</dt><dd className="num">{set.port}</dd></div>
          </dl>
          {!stats.real && <p className="snote" style={{ margin: 0 }}>Demo mode: torrents, peers and speeds are generated by the server and nothing is downloaded. Start Bitswarm without demo mode for real downloads.</p>}
        </fieldset>
        <fieldset className="sgroup">
          <legend>This session</legend>
          <dl className="meta" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(250px,1fr))" }}>
            <div><dt>Running for</dt><dd className="num">{up}</dd></div>
            <div><dt>Downloaded</dt><dd className="num d">{formatBytes(stats.sessDl)}</dd></div>
            <div><dt>Uploaded</dt><dd className="num u">{formatBytes(stats.sessUl)}</dd></div>
            <div><dt>Session ratio</dt><dd className="num">{stats.sessDl ? (stats.sessUl / stats.sessDl).toFixed(3) : "–"}</dd></div>
            <div><dt>Torrents</dt><dd className="num">{stats.torrents} ({stats.active} active)</dd></div>
            <div><dt>Free space</dt><dd className="num">{formatBytes(stats.free)}</dd></div>
          </dl>
        </fieldset>
        <fieldset className="sgroup">
          <legend>Disclaimer</legend>
          <p style={{ margin: 0, color: "var(--muted)", maxWidth: "68ch" }}>
            Bitswarm is provided as is, without any warranty. Its developers aren’t responsible for any damage, data loss or other harm from using it, including downloads, automation, maintenance tasks and hooks you
            set up. Only download and share content you have the right to.
          </p>
        </fieldset>
        <fieldset className="sgroup">
          <legend>Credits</legend>
          <p style={{ margin: 0, color: "var(--muted)" }}>Type set in Bricolage Grotesque, Figtree and JetBrains Mono, all under the SIL Open Font License.</p>
        </fieldset>
        <fieldset className="sgroup">
          <legend>Data in this browser</legend>
          <p style={{ margin: 0, color: "var(--muted)" }}>Your theme and sidebar layout are saved in this browser only. Settings, tags and tokens live on the server.</p>
          {wiping ? (
            <div className="confirm" style={{ borderRadius: 8, border: 0 }}>
              <strong>Reset theme and sidebar layout?</strong>
              <span style={{ flex: 1 }} />
              <button type="button" className="btn" onClick={() => setWiping(false)}>Cancel</button>
              <button type="button" className="btn danger solid" onClick={wipe}>Reset</button>
            </div>
          ) : (
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <button type="button" className="btn" style={{ borderColor: "var(--line)" }} onClick={() => { writeFlag("swarm-legal-ack", false); writeFlag("swarm-hooks-ack", false); toast("The legal notice and hooks warning will show again."); }}>
                Show warnings again
              </button>
              <button type="button" className="btn danger" style={{ borderColor: "var(--line)" }} onClick={() => setWiping(true)}>Reset saved data</button>
            </div>
          )}
        </fieldset>
        <p className="z"><Link to="/">Back to torrents</Link></p>
      </div>
    </section>
  );
}
