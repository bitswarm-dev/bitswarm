import { useEffect, useState } from "react";
import { useToast } from "@/components/Toasts/Toasts";
import { useDownloadFromPreview } from "@/features/schedules/mutations";
import { useSchedulePreview } from "@/features/schedules/queries";
import type { RuleView } from "@/features/schedules/types";
import { ITEM_STATE } from "./ItemTable";

export function PreviewPanel({ rule, onClose }: { rule: RuleView; onClose: () => void }) {
  const q = useSchedulePreview(rule.id);
  const download = useDownloadFromPreview();
  const toast = useToast();
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const data = q.data;

  useEffect(() => {
    if (data && !data.extraction.error) setPicked(new Set(data.extraction.items.filter((i) => i.state === "new").map((i) => i.key)));
  }, [data]);

  const head = (
    <>
      <div className="pv-head">
        <h4>Preview of the next run</h4>
        <span>Nothing is downloaded until you choose to.</span>
      </div>
      <div className="urls">
        <span className="k">Searches</span>
        {rule.urls.map((u) => <code key={u}>{u}</code>)}
      </div>
    </>
  );
  const close = <button className="btn" onClick={onClose}>Close preview</button>;

  if (q.isLoading || !data)
    return <div className="preview">{head}<p className="z" style={{ margin: 0 }}>Loading the results page…</p><div className="pv-foot"><span className="sum" />{close}</div></div>;

  const ex = data.extraction;
  if (ex.error)
    return <div className="preview">{head}<p className="err-msg" style={{ margin: 0 }}>{ex.error}</p><div className="pv-foot"><span className="sum" />{close}</div></div>;

  const n = (s: string) => ex.items.filter((i) => i.state === s).length;
  const sum = `${ex.matched} element${ex.matched === 1 ? "" : "s"} matched · ${ex.items.length} usable link${ex.items.length === 1 ? "" : "s"} · ${n("new")} new${n("have") ? ` · ${n("have")} already have` : ""}${n("filtered") ? ` · ${n("filtered")} filtered out` : ""}${n("over") ? ` · ${n("over")} over the limit` : ""}`;

  if (!ex.items.length)
    return <div className="preview">{head}<p className="z" style={{ margin: 0 }}>The selector found no magnet or .torrent links on the results page. Edit the schedule to adjust step 3.</p><div className="pv-foot"><span className="sum">{sum}</span>{close}</div></div>;

  return (
    <div className="preview">
      {head}
      <div className="tbl-wrap">
        <table>
          <thead><tr><th className="c"><span className="z" aria-hidden="true">✓</span></th><th>Name</th><th>Type</th><th>Result</th><th>Link</th></tr></thead>
          <tbody>
            {ex.items.map((i, ix) => (
              <tr key={i.key} className={i.state === "new" ? "" : "dim"}>
                <td className="c">
                  <input
                    type="checkbox"
                    id={`pv-${rule.id}-${ix}`}
                    checked={picked.has(i.key)}
                    disabled={i.state === "have"}
                    aria-label={`Download ${i.name}`}
                    onChange={(e) => setPicked((cur) => { const next = new Set(cur); if (e.target.checked) next.add(i.key); else next.delete(i.key); return next; })}
                  />
                </td>
                <td className="fname"><label htmlFor={`pv-${rule.id}-${ix}`}>{i.name}</label></td>
                <td>{i.type === "magnet" ? "Magnet" : ".torrent"}</td>
                <td><span className={`pill st-${ITEM_STATE[i.state][0]}`}>{ITEM_STATE[i.state][1]}</span></td>
                <td className="num" style={{ maxWidth: 220, overflow: "hidden", textOverflow: "ellipsis" }} title={i.url}>{i.url.length > 48 ? `${i.url.slice(0, 48)}…` : i.url}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="pv-foot">
        <span className="sum">{sum}</span>
        {close}
        <button
          className="btn primary"
          disabled={picked.size === 0 || download.isPending}
          onClick={() =>
            download.mutate({ id: rule.id, keys: [...picked] }, {
              onSuccess: (r) => { toast(`${rule.name}: added ${r.added} torrent${r.added === 1 ? "" : "s"} from preview`); onClose(); },
              onError: (e) => toast(e.message),
            })
          }
        >
          <span>Download selected ({picked.size})</span>
        </button>
      </div>
    </div>
  );
}
