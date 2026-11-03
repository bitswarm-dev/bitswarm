import { useEffect, useRef, useState, type FormEvent } from "react";
import { Modal } from "@/components/Modal/Modal";
import { useCatalog } from "@/features/catalog/queries";
import { useCreateHook, useParseCurl, useUpdateHook } from "@/features/hooks/mutations";
import { useHookPreview, useHooksConfig } from "@/features/hooks/queries";
import { pickKnown } from "@/utils/safe";
import { BLANK_HOOK, EVENT_GROUPS, PLACEHOLDERS, type Hook, type HookView } from "@/features/hooks/types";

type Form = Omit<Hook, "id" | "count" | "last" | "lastEvent">;
type Field = "url" | "body" | "headers" | "cmd";

interface Props {
  hook: HookView | null;
  onClose: () => void;
  onSaved: (h: HookView) => void;
}

export function HookEditor({ hook, onClose, onSaved }: Props) {
  const { data: catalog } = useCatalog();
  const { data: config } = useHooksConfig();
  const create = useCreateHook();
  const update = useUpdateHook();
  const parse = useParseCurl();
  const [form, setForm] = useState<Form>(() => ({ ...pickKnown(BLANK_HOOK, hook), secret: "", clearSecret: false }));
  const [error, setError] = useState("");
  const [curlOpen, setCurlOpen] = useState(false);
  const [curlText, setCurlText] = useState("");
  const [curlError, setCurlError] = useState("");
  const [imported, setImported] = useState<string[] | null>(null);
  const [debounced, setDebounced] = useState(form);
  const last = useRef<Field>("body");
  const refs = { url: useRef<HTMLInputElement>(null), body: useRef<HTMLTextAreaElement>(null), headers: useRef<HTMLTextAreaElement>(null), cmd: useRef<HTMLTextAreaElement>(null) };

  useEffect(() => {
    const t = setTimeout(() => setDebounced(form), 180);
    return () => clearTimeout(t);
  }, [form]);

  const hasSecret = hook?.hasSecret ?? false;
  const preview = useHookPreview({ ...debounced, secret: debounced.secret || (hasSecret && !debounced.clearSecret ? "•" : "") }).data;
  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }));
  const cmd = form.kind === "cmd";

  const setKind = (k: Form["kind"]) => {
    last.current = k === "cmd" ? "cmd" : "body";
    setCurlOpen(false);
    setImported(null);
    set("kind", k);
  };

  const importCurl = () => {
    setCurlError("");
    parse.mutate(curlText, {
      onSuccess: (r) => {
        setForm((f) => ({ ...f, kind: "api", method: r.method, url: r.url, headers: r.headers, body: r.body }));
        setImported(r.warnings);
        setCurlOpen(false);
        setCurlText("");
        last.current = "body";
      },
      onError: (err: Error) => setCurlError(err.message),
    });
  };

  const insert = (name: string) => {
    const el = refs[last.current].current;
    const text = `{{${name}}}`;
    if (!el) return;
    const s = el.selectionStart ?? el.value.length;
    const e = el.selectionEnd ?? el.value.length;
    const next = el.value.slice(0, s) + text + el.value.slice(e);
    set(last.current, next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(s + text.length, s + text.length);
    });
  };

  const toggleEvent = (ev: string, on: boolean) => set("events", on ? [...form.events, ev] : form.events.filter((x) => x !== ev));

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const body = { ...form, timeout: Number.isFinite(form.timeout) ? form.timeout : 0, retries: Number.isFinite(form.retries) ? form.retries : -1 };
    const done = { onSuccess: (h: HookView) => onSaved(h), onError: (err: Error) => setError(err.message) };
    if (hook) update.mutate({ ...body, id: hook.id }, done);
    else create.mutate(body, done);
  };

  return (
    <Modal title={hook ? "Edit hook" : "New hook"} subtitle="Call an API or run a command whenever something happens in Bitswarm." wide onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <fieldset className="step">
          <legend><span className="sn">1</span>What to do</legend>
          <label className="field">
            <span>Name</span>
            <input type="text" value={form.name} onChange={(e) => set("name", e.target.value)} placeholder="Notify my server" autoFocus />
          </label>
          <div className="seg kindseg" role="group" aria-label="Hook type">
            <button type="button" aria-pressed={!cmd && !curlOpen} onClick={() => setKind("api")}>Call an API</button>
            <button type="button" aria-pressed={cmd} onClick={() => setKind("cmd")}>Run a command</button>
            <button type="button" aria-pressed={!cmd && curlOpen} onClick={() => { set("kind", "api"); setCurlOpen(true); }}>Paste curl</button>
          </div>
          {!cmd && curlOpen ? (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <label className="field">
                <span>curl command</span>
                <textarea rows={8} className="mono" value={curlText} onChange={(e) => setCurlText(e.target.value)} spellCheck={false} placeholder={'curl -X POST "https://api.example.com/send" \\\n  -H "Authorization: Bearer {{secret.API_KEY}}" \\\n  -H "Content-Type: application/json" \\\n  -d \'{"text": "Finished {{torrent.name}}"}\''} autoFocus />
                <small>Paste a command from your service’s docs. Bitswarm fills in the method, address, headers and body, then switches to Call an API so you can check them. Nothing is run and no request is sent.</small>
              </label>
              {curlError && <p className="bad" style={{ margin: 0 }}>{curlError}</p>}
              <div>
                <button type="button" className="btn primary" disabled={!curlText.trim() || parse.isPending} onClick={importCurl}>Fill the form</button>
              </div>
            </div>
          ) : !cmd ? (
            <>
              {imported && (
                <p className="note" style={{ margin: 0 }}>
                  <span>
                    Filled from curl.{imported.length > 0 ? " Please check:" : " Nothing was left out."}
                    {imported.length > 0 && <ul style={{ margin: "6px 0 0", paddingLeft: 18 }}>{imported.map((w) => <li key={w}>{w}</li>)}</ul>}
                  </span>
                </p>
              )}
            <div className="hrow">
              <label className="field" style={{ flex: "none" }}>
                <span>Method</span>
                <select value={form.method} onChange={(e) => set("method", e.target.value)}>
                  {["POST", "PUT", "PATCH", "GET"].map((m) => <option key={m}>{m}</option>)}
                </select>
              </label>
              <label className="field" style={{ flex: 1, minWidth: 0 }}>
                <span>URL</span>
                <input ref={refs.url} type="text" className="mono" value={form.url} onFocus={() => (last.current = "url")} onChange={(e) => set("url", e.target.value)} placeholder="https://example.org/hooks/bitswarm" spellCheck={false} autoComplete="off" />
              </label>
            </div>
            </>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              {!config.commandsEnabled && (
                <p className="note" style={{ margin: 0 }}>
                  <span>Command hooks are turned off on this server, so saved commands are recorded as “Not run”. Start the server with <code>SWARM_ALLOW_COMMANDS=1</code> to run them.</span>
                </p>
              )}
              <label className="field">
                <span>Command</span>
                <textarea ref={refs.cmd} rows={2} value={form.cmd} onFocus={() => (last.current = "cmd")} onChange={(e) => set("cmd", e.target.value)} spellCheck={false} placeholder="/home/me/scripts/done.sh {{torrent.path}} {{torrent.name}}" />
                <small>Runs on the machine where Bitswarm runs, with Bitswarm’s permissions. Placeholder values are quoted for the shell automatically, so don’t put quotes around them.</small>
              </label>
              <div className="row2">
                <label className="field">
                  <span>Working folder</span>
                  <input type="text" className="mono" value={form.cwd} onChange={(e) => set("cwd", e.target.value)} placeholder="Optional, such as /home/me/scripts" spellCheck={false} autoComplete="off" />
                </label>
                <label className="field">
                  <span>Time limit in seconds</span>
                  <input type="number" min={1} max={3600} value={Number.isFinite(form.timeout) ? form.timeout : ""} onChange={(e) => set("timeout", e.target.value === "" ? NaN : Number(e.target.value))} />
                  <small>The command is stopped after this.</small>
                </label>
              </div>
            </div>
          )}
        </fieldset>

        <fieldset className="step">
          <legend><span className="sn">2</span>When to call it</legend>
          <div className="hevents">
            {EVENT_GROUPS.map(([group, list]) => {
              const all = list.every(([k]) => form.events.includes(k));
              return (
                <div key={group} className="hgroup">
                  <div className="hg-head">
                    <b>{group}</b>
                    <button
                      type="button"
                      className="linkish"
                      onClick={() => set("events", all ? form.events.filter((e) => !list.some(([k]) => k === e)) : [...new Set([...form.events, ...list.map(([k]) => k)])])}
                    >
                      {all ? "Clear all" : "Select all"}
                    </button>
                  </div>
                  <div className="hg-list">
                    {list.map(([k, l]) => (
                      <label key={k} className="check">
                        <input type="checkbox" checked={form.events.includes(k)} onChange={(e) => toggleEvent(k, e.target.checked)} /> {l} <code>{k}</code>
                      </label>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
          <label className="field">
            <span>Only for tag</span>
            <select value={form.tag} onChange={(e) => set("tag", e.target.value)}>
              <option value="">Any tag</option>
              {catalog.tags.map((g) => <option key={g}>{g}</option>)}
            </select>
            <small>Applies to torrent events. Other events always call the hook.</small>
          </label>
        </fieldset>

        <fieldset className="step">
          <legend><span className="sn">3</span>{cmd ? "Input and environment" : "Headers and body"}</legend>
          {cmd ? (
            <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
              <label className="check"><input type="checkbox" checked={form.stdin} onChange={(e) => set("stdin", e.target.checked)} /> Send the event as JSON to the command’s standard input</label>
              <p className="hint" style={{ margin: 0 }}>
                The command also gets these environment variables: <code>SWARM_EVENT</code>, <code>SWARM_TIME</code>, <code>SWARM_TORRENT_NAME</code>, <code>SWARM_TORRENT_HASH</code>, <code>SWARM_TORRENT_PATH</code>,{" "}
                <code>SWARM_TORRENT_SIZE</code>, and <code>SWARM_SCHEDULE</code> or <code>SWARM_TASK</code> when they apply. Click a placeholder below to add it to the command.
              </p>
            </div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <label className="field">
                <span>Headers</span>
                <textarea ref={refs.headers} rows={3} value={form.headers} onFocus={() => (last.current = "headers")} onChange={(e) => set("headers", e.target.value)} spellCheck={false} placeholder="Content-Type: application/json" />
                <small>
                  One per line, written as Name: value. Keys in headers such as <code>Authorization</code> and <code>X-Api-Key</code> are hidden after you save, and <code>********</code> means the saved key is kept. To keep a key out of Bitswarm altogether, write <code>{"{{secret.NAME}}"}</code> and set <code>SWARM_SECRET_NAME</code> on the server.
                </small>
              </label>
              <label className="field">
                <span>Body</span>
                <textarea ref={refs.body} rows={5} value={form.body} onFocus={() => (last.current = "body")} onChange={(e) => set("body", e.target.value)} spellCheck={false} />
                <small>Not sent for GET. <code>{"{{payload}}"}</code> is the whole event as JSON. Click a placeholder to insert it where your cursor is.</small>
              </label>
            </div>
          )}
          <div className="phs">
            {PLACEHOLDERS.map((p) => (
              <button key={p} type="button" className="ph" onClick={() => insert(p)}>{`{{${p}}}`}</button>
            ))}
          </div>
        </fieldset>

        <fieldset className="step" style={{ border: 0, paddingBottom: 0 }}>
          <legend><span className="sn">4</span>Delivery</legend>
          <div className="row2">
            {!cmd && (
              <label className="field">
                <span>Signing secret</span>
                <input type="password" value={form.secret} onChange={(e) => set("secret", e.target.value)} autoComplete="new-password" placeholder={hasSecret && !form.clearSecret ? "Unchanged" : "Optional"} />
                <small>Adds an X-Swarm-Signature header: an HMAC-SHA256 of the body.</small>
                {hasSecret && (
                  <label className="check">
                    <input type="checkbox" checked={form.clearSecret} onChange={(e) => set("clearSecret", e.target.checked)} /> Remove the saved secret
                  </label>
                )}
              </label>
            )}
            <label className="field">
              <span>Retries if it fails</span>
              <input type="number" min={0} max={10} value={Number.isFinite(form.retries) ? form.retries : ""} onChange={(e) => set("retries", e.target.value === "" ? NaN : Number(e.target.value))} />
              {cmd && <small>Commands run once; this is kept for API hooks.</small>}
            </label>
          </div>
          <label className="check"><input type="checkbox" checked={form.enabled} onChange={(e) => set("enabled", e.target.checked)} /> Hook is on</label>
          <h3 className="pvh">Preview</h3>
          <p className="hint">
            {preview && (
              <>
                A sample <code>{preview.event}</code> event.
                {preview.unknown.length > 0 && <span className="bad"> Unknown placeholder{preview.unknown.length > 1 ? "s" : ""}: {preview.unknown.map((k) => `{{${k}}}`).join(", ")}.</span>}
                {preview.empty.length > 0 && <> {preview.empty.map((k) => `{{${k}}}`).join(", ")} {preview.empty.length > 1 ? "are" : "is"} empty for this event.</>}
                {(preview.missingSecrets?.length ?? 0) > 0 && <span className="bad"> Not set on the server: {preview.missingSecrets.map((k) => `SWARM_SECRET_${k}`).join(", ")}. The hook can’t send until you set {preview.missingSecrets.length > 1 ? "them" : "it"} and restart Bitswarm.</span>}
              </>
            )}
          </p>
          <pre className="req">{preview?.text ?? ""}</pre>
        </fieldset>

        {error && <p className="err-msg">{error}</p>}
        <div className="actions">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn primary" disabled={create.isPending || update.isPending}>Save hook</button>
        </div>
      </form>
    </Modal>
  );
}
