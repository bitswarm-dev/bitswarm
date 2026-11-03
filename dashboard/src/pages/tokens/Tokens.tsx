import { Fragment, useState } from "react";
import { Svg } from "@/components/Icons/Svg";
import { useToast } from "@/components/Toasts/Toasts";
import { useCreateToken, useRevokeToken } from "@/features/tokens/mutations";
import { useTokens } from "@/features/tokens/queries";
import { SCOPES } from "@/features/tokens/types";
import { useCopy } from "@/pages/torrents/ContextMenu";
import { formatWhen } from "@/utils/format";

const dateText = (ts: number) => new Date(ts).toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" });

export function Tokens() {
  const { data: tokens } = useTokens();
  const create = useCreateToken();
  const revoke = useRevokeToken();
  const toast = useToast();
  const copy = useCopy();
  const [name, setName] = useState("");
  const [scope, setScope] = useState("read");
  const [days, setDays] = useState(90);
  const [error, setError] = useState("");
  const [fresh, setFresh] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<number | null>(null);

  const submit = () => {
    if (!name.trim()) return setError("Give the token a name, so you know what uses it.");
    setError("");
    create.mutate({ name, scope, days }, { onSuccess: (r) => { setFresh(r.token); setName(""); }, onError: (e) => setError(e.message) });
  };

  return (
    <section className="sched" aria-label="API tokens">
      <div className="sched-head">
        <div>
          <h2>API tokens</h2>
          <p>Let scripts and other apps use Bitswarm’s Web API without your password. Each token has its own access level and expiry, and you can revoke it at any time.</p>
        </div>
      </div>
      <div className="note">
        <Svg size={16}><circle cx="12" cy="12" r="9" /><path d="M12 8h.01M11 12h1v5h1" /></Svg>
        <span>Only a hash of each token is kept, never the token itself. Tokens work on every route their access level allows.</span>
      </div>

      {fresh && (
        <div className="upd ok" role="status">
          <Svg size={20}><path d="m5 12 5 5 9-10" /></Svg>
          <div style={{ flex: 1, minWidth: 0 }}>
            <h4>Copy your new token now</h4>
            <p>For security, you won’t be able to see it again. If you lose it, revoke it and create another.</p>
            <div className="tokshow">
              <code>{fresh}</code>
              <button type="button" className="btn primary" onClick={() => copy(fresh, "Token")}>Copy</button>
            </div>
            <div className="acts">
              <button type="button" className="btn" onClick={() => setFresh(null)}>I’ve copied it</button>
            </div>
          </div>
        </div>
      )}

      <fieldset className="sgroup">
        <legend>Create a token</legend>
        <div className="tokrow">
          <label className="field">
            <span>Name</span>
            <input type="text" maxLength={60} value={name} onChange={(e) => setName(e.target.value)} placeholder="What will use it, such as Home Assistant" autoComplete="off" />
          </label>
          <label className="field">
            <span>Access</span>
            <select value={scope} onChange={(e) => setScope(e.target.value)}>
              {Object.entries(SCOPES).map(([k, [l, d]]) => <option key={k} value={k}>{l}: {d}</option>)}
            </select>
          </label>
          <label className="field">
            <span>Expires</span>
            <select value={days} onChange={(e) => setDays(Number(e.target.value))}>
              <option value={30}>In 30 days</option>
              <option value={90}>In 90 days</option>
              <option value={365}>In 1 year</option>
              <option value={0}>Never</option>
            </select>
          </label>
        </div>
        {error && <p className="err-msg">{error}</p>}
        <div>
          <button type="button" className="btn primary" disabled={create.isPending} onClick={submit}>Create token</button>
        </div>
      </fieldset>

      <section className="sgroup">
        <h3 className="pvh">Your tokens</h3>
        {tokens.length ? (
          <div className="tbl-wrap">
            <table className="toktbl">
              <thead><tr><th>Name</th><th>Access</th><th>Token</th><th>Created</th><th>Expires</th><th>Last used</th><th /></tr></thead>
              <tbody>
                {tokens.map((k) => (
                  <Fragment key={k.id}>
                    <tr>
                      <td>{k.name}</td>
                      <td>{SCOPES[k.scope]?.[0] ?? k.scope}</td>
                      <td className="num">swm_…{k.last4}</td>
                      <td>{dateText(k.created)}</td>
                      <td>{k.expired ? <span className="pill st-error">Expired</span> : k.expires ? dateText(k.expires) : "Never"}</td>
                      <td>{k.lastUsed ? formatWhen(k.lastUsed) : <span className="z">Never</span>}</td>
                      <td className="r"><button type="button" className="btn danger" onClick={() => setRevoking(k.id)}>Revoke</button></td>
                    </tr>
                    {revoking === k.id && (
                      <tr>
                        <td colSpan={7} style={{ whiteSpace: "normal" }}>
                          <div className="confirm" style={{ borderRadius: 8, border: 0 }}>
                            <strong>Revoke “{k.name}”?</strong>
                            <span style={{ color: "var(--muted)" }}>Anything using it stops working right away.</span>
                            <span style={{ flex: 1 }} />
                            <button type="button" className="btn" onClick={() => setRevoking(null)}>Cancel</button>
                            <button type="button" className="btn danger solid" onClick={() => revoke.mutate(k.id, { onSuccess: () => { setRevoking(null); toast(`Revoked ${k.name}`); } })}>Revoke</button>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="z" style={{ margin: 0 }}>No tokens yet. Create one above.</p>
        )}
      </section>

      <section className="sgroup">
        <h3 className="pvh">Using a token</h3>
        <p className="snote" style={{ margin: 0 }}>Send it in the Authorization header of each request to Bitswarm’s Web API.</p>
        <pre className="req">{`curl -H 'Authorization: Bearer swm_…' ${location.origin}/api/v1/torrents`}</pre>
      </section>
    </section>
  );
}
