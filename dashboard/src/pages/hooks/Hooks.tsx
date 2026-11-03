import { useState } from "react";
import { Svg } from "@/components/Icons/Svg";
import { useToast } from "@/components/Toasts/Toasts";
import { useDeleteHook, useDuplicateHook, useSetHookEnabled, useTestHook } from "@/features/hooks/mutations";
import { useDeliveries, useHooks, useHooksConfig } from "@/features/hooks/queries";
import { ALL_EVENTS, EVENT_LABEL, type Delivery, type HookView } from "@/features/hooks/types";
import { useCopy } from "@/pages/torrents/ContextMenu";
import { formatWhen, plural } from "@/utils/format";
import { HookEditor } from "./HookEditor";

const STATUS: Record<Delivery["status"], [string, string]> = {
  sent: ["seeding", "Sent"],
  ran: ["seeding", "Ran"],
  failed: ["error", "Failed"],
  not_run: ["queued", "Not run"],
};

const nDeliveries = (n: number) => (n === 1 ? "1 delivery" : `${n} deliveries`);

function EventChips({ h }: { h: HookView }) {
  const names = h.events.length === ALL_EVENTS.length ? ["All events"] : h.events.map((e) => EVENT_LABEL[e] ?? e);
  return (
    <span className="evchips">
      {names.slice(0, 5).map((e) => <span key={e} className="evchip">{e}</span>)}
      {names.length > 5 && <span className="evchip">+{names.length - 5} more</span>}
    </span>
  );
}

export function Hooks() {
  const { data: hooks } = useHooks();
  const { data: deliveries } = useDeliveries();
  const { data: config } = useHooksConfig();
  const toggle = useSetHookEnabled();
  const dup = useDuplicateHook();
  const del = useDeleteHook();
  const test = useTestHook();
  const toast = useToast();
  const copy = useCopy();
  const [editing, setEditing] = useState<HookView | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [open, setOpen] = useState<Set<number>>(new Set());

  return (
    <section className="sched" aria-label="API hooks">
      <div className="sched-head">
        <div>
          <h2>Hooks</h2>
          <p>Call an API or run a command at each step: when torrents are added, finish or fail, when a schedule searches and finds links, when presets rename files, and when maintenance runs.</p>
        </div>
        <button className="btn primary" onClick={() => setEditing(null)}>
          <Svg size={15} strokeWidth={2.4}><path d="M12 5v14M5 12h14" /></Svg>
          New hook
        </button>
      </div>
      <div className="note">
        <Svg size={16}><circle cx="12" cy="12" r="9" /><path d="M12 8h.01M11 12h1v5h1" /></Svg>
        <span>
          API hooks are sent from this server, with retries when the other side fails.{" "}
          {config.commandsEnabled
            ? "Command hooks are on, so they run on this machine with BitSwarm’s permissions."
            : "Command hooks are off: they’re recorded as “Not run” until the server is started with SWARM_ALLOW_COMMANDS=1."}{" "}
          Hooks are for advanced users and you use them at your own risk: BitSwarm and its developers aren’t responsible for any damage they cause.
        </span>
      </div>

      <div className="rules">
        {hooks.length === 0 ? (
          <div className="empty"><strong>No hooks yet</strong>Create one to call an API or run a command when something happens in BitSwarm.</div>
        ) : (
          hooks.map((h) => (
            <article key={h.id} className={`rule ${h.enabled ? "" : "off"}`}>
              <div>
                <div className="rule-top">
                  <label className="switch" title={h.enabled ? "Turn off" : "Turn on"}>
                    <input type="checkbox" checked={h.enabled} aria-label={`${h.name} on`} onChange={(e) => toggle.mutate({ id: h.id, enabled: e.target.checked }, { onSuccess: (v) => toast(v.enabled ? `${v.name} is on` : `${v.name} is off`) })} />
                    <span />
                  </label>
                  <h3>{h.name}</h3>
                  <span className="chip">{h.kind === "cmd" ? "Command" : h.kind === "curl" ? "curl" : h.method}</span>
                  {h.tag && <span className="tag">{h.tag}</span>}
                  {h.kind === "api" && h.hasSecret && <span className="chip">Signed</span>}
                </div>
                <dl className="rule-spec">
                  <div><dt>{h.kind !== "api" ? "Runs" : "URL"}</dt><dd className="num">{h.kind !== "api" ? `$ ${h.cmd}` : h.url}</dd></div>
                  <div><dt>Events</dt><dd><EventChips h={h} /></dd></div>
                </dl>
              </div>
              <div className="rule-side">
                <div className="side-top">
                  <div className="when">
                    <span className="every">{plural(h.events.length, "event")}</span>
                    <span className="last">{h.count ? nDeliveries(h.count) : "No deliveries yet"}</span>
                    {h.last > 0 && <span className="last">Last: {EVENT_LABEL[h.lastEvent] ?? h.lastEvent} at {formatWhen(h.last)}</span>}
                  </div>
                  <button className="btn icon-del" title="Delete hook" aria-label={`Delete ${h.name}`} onClick={() => setDeleting(h.id)}>
                    <Svg size={15}><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" /></Svg>
                  </button>
                </div>
                <div className="rule-actions">
                  <button
                    className="btn"
                    disabled={test.isPending}
                    onClick={() =>
                      test.mutate(h.id, {
                        onSuccess: (d) => {
                          setOpen((cur) => new Set(cur).add(d.id));
                          toast(`Test ${d.event}: ${STATUS[d.status][1].toLowerCase()}`);
                        },
                        onError: (e) => toast(e.message),
                      })
                    }
                  >
                    Send test
                  </button>
                  <button className="btn" onClick={() => setEditing(h)}>Edit</button>
                  <button className="btn" onClick={() => dup.mutate(h.id, { onSuccess: (c) => setEditing(c) })}>Duplicate</button>
                </div>
              </div>
              {deleting === h.id && (
                <div className="confirm">
                  <strong>Delete “{h.name}”?</strong>
                  <span style={{ color: "var(--muted)" }}>Its past deliveries stay in the list.</span>
                  <span style={{ flex: 1 }} />
                  <button className="btn" onClick={() => setDeleting(null)}>Cancel</button>
                  <button className="btn danger solid" onClick={() => del.mutate(h.id, { onSuccess: () => { setDeleting(null); toast(`Deleted ${h.name}`); } })}>Delete</button>
                </div>
              )}
            </article>
          ))
        )}
      </div>

      <section className="log">
        <h3>Recent deliveries</h3>
        {deliveries.length ? (
          <div className="dels">
            {deliveries.map((d) => (
              <details
                key={d.id}
                className="del"
                open={open.has(d.id)}
                onToggle={(e) => {
                  const isOpen = (e.currentTarget as HTMLDetailsElement).open;
                  setOpen((cur) => {
                    const next = new Set(cur);
                    if (isOpen) next.add(d.id);
                    else next.delete(d.id);
                    return next;
                  });
                }}
              >
                <summary>
                  <time>{formatWhen(d.t)}</time>
                  <code className="evc">{d.event}</code>
                  <span className="dh">{d.hook}{d.test ? " · test" : ""}</span>
                  <span className="du num">{d.kind !== "api" ? `$ ${d.command}` : `${d.method} ${d.url}`}</span>
                  <span className={`pill st-${STATUS[d.status][0]}`}>{STATUS[d.status][1]}</span>
                </summary>
                <pre className="req">
                  {d.text}
                  {"\n\n"}
                  {d.status === "sent" && `# The server answered ${d.response}${d.attempts > 1 ? ` after ${d.attempts} tries` : ""}`}
                  {d.status === "failed" && `# Failed after ${plural(d.attempts, "try")}: ${d.error}`}
                  {d.status === "not_run" && `# Not run: ${d.error}`}
                  {d.status === "ran" && "# Ran and exited with 0"}
                  {d.output ? `\n# Output\n${d.output}` : ""}
                </pre>
                <div className="pv-foot">
                  <span className="sum" />
                  <button className="btn" onClick={() => copy(d.copy, d.kind !== "api" ? "Command" : "curl command")}>{d.kind !== "api" ? "Copy command" : "Copy as curl"}</button>
                </div>
              </details>
            ))}
          </div>
        ) : (
          <p className="z" style={{ margin: 0 }}>No deliveries yet. They appear here when an event fires, or when you send a test.</p>
        )}
      </section>

      {editing !== undefined && (
        <HookEditor hook={editing} onClose={() => setEditing(undefined)} onSaved={(h) => { setEditing(undefined); toast(`Saved ${h.name}`); }} />
      )}
    </section>
  );
}
