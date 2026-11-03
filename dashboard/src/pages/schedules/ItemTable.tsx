import { useMemo, useState } from "react";
import type { Item, ItemState } from "@/features/schedules/types";

export const ITEM_STATE: Record<ItemState, [string, string]> = {
  new: ["seeding", "Will download"],
  have: ["paused", "Already have"],
  filtered: ["paused", "Filtered out"],
  over: ["queued", "Over the limit"],
};

const LIMIT = 25;
const STATES: ItemState[] = ["new", "over", "have", "filtered"];

type Sort = "page" | "az" | "za" | "status";

export function ItemTable({ items }: { items: Item[] }) {
  const [filter, setFilter] = useState<ItemState | "all">("all");
  const [sort, setSort] = useState<Sort>("page");

  const counts = useMemo(() => {
    const c: Record<ItemState, number> = { new: 0, have: 0, filtered: 0, over: 0 };
    items.forEach((i) => c[i.state]++);
    return c;
  }, [items]);

  const shown = useMemo(() => {
    const list = filter === "all" ? [...items] : items.filter((i) => i.state === filter);
    if (sort === "az") list.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
    if (sort === "za") list.sort((a, b) => b.name.localeCompare(a.name, undefined, { numeric: true }));
    if (sort === "status") list.sort((a, b) => STATES.indexOf(a.state) - STATES.indexOf(b.state));
    return list;
  }, [items, filter, sort]);

  const rows = shown.slice(0, LIMIT);

  return (
    <>
      <div className="row2" style={{ gap: 10 }}>
        <label className="field" style={{ margin: 0 }}>
          <span>Show</span>
          <select value={filter} onChange={(e) => setFilter(e.target.value as ItemState | "all")}>
            <option value="all">All results ({items.length})</option>
            {STATES.map((s) => (
              <option key={s} value={s} disabled={counts[s] === 0}>{ITEM_STATE[s][1]} ({counts[s]})</option>
            ))}
          </select>
        </label>
        <label className="field" style={{ margin: 0 }}>
          <span>Sort by</span>
          <select value={sort} onChange={(e) => setSort(e.target.value as Sort)}>
            <option value="page">Order on the page</option>
            <option value="az">Name, A to Z</option>
            <option value="za">Name, Z to A</option>
            <option value="status">Result</option>
          </select>
        </label>
      </div>
      <div className="tbl-wrap" style={{ maxHeight: 240, overflowY: "auto" }}>
        <table>
          <thead style={{ position: "sticky", top: 0, background: "var(--surface)" }}><tr><th>Name</th><th>Type</th><th>Result</th></tr></thead>
          <tbody>
            {rows.map((i) => (
              <tr key={i.key}>
                <td className="fname">{i.name}</td>
                <td>{i.type === "magnet" ? "Magnet" : ".torrent"}</td>
                <td><span className={`pill st-${ITEM_STATE[i.state][0]}`}>{ITEM_STATE[i.state][1]}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="hint" style={{ margin: 0 }}>
        {shown.length > LIMIT ? `Showing the first ${LIMIT} of ${shown.length}. Narrow it with “Show” or the filters above.` : `Showing ${shown.length} of ${items.length}.`}
      </p>
    </>
  );
}
