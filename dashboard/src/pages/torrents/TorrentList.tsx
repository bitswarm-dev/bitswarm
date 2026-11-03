import type { MouseEvent } from "react";
import { useEffect, useRef } from "react";
import { defaultDir, useActiveId, useListView } from "@/app/listView";
import { useUi, type SortKey } from "@/app/ui";
import { STATUS_TEXT } from "@/features/torrents/filters";
import type { Torrent } from "@/features/torrents/types";
import { formatBytes, formatEta, formatRate } from "@/utils/format";

function savesTo(t: Torrent): string {
  const base = t.path.replace(/\/+$/, "");
  if (t.files.length === 0) return `Saves to ${base}/${t.name}`;
  if (t.files.length === 1) return `Saves to ${base}/${t.files[0].name}`;
  const tops = new Set(t.files.map((f) => f.name.split("/")[0]));
  const root = tops.size === 1 && t.files.every((f) => f.name.includes("/")) ? `${[...tops][0]}/` : "";
  return `Saves to ${base}/${root} (${t.files.length} files)`;
}

const COLS: [SortKey, string, string][] = [
  ["queue", "#", "r"],
  ["name", "Name", ""],
  ["size", "Size", "r"],
  ["progress", "Progress", ""],
  ["status", "Status", ""],
  ["dl", "Down", "r"],
  ["ul", "Up", "r"],
  ["eta", "ETA", "r"],
  ["peers", "Peers", "r"],
  ["ratio", "Ratio", "r"],
];

interface Props {
  rows: Torrent[];
  onSelect: (ids: number[], active: number, anchor: number | null) => void;
  onContext: (x: number, y: number, id: number) => void;
  onToggle: (id: number) => void;
  onRemoveKey: () => void;
  onSpace: () => void;
  onDoubleClick: (t: Torrent) => void;
}

function mobileLine(t: Torrent): React.ReactNode {
  const etaText = t.status === "downloading" ? formatEta(t.eta) : t.status === "checking" ? "…" : "–";
  if (t.status === "downloading")
    return (
      <>
        {formatBytes(t.done)} of {formatBytes(t.wanted)} · <span className="d">↓ {formatRate(t.dl)}</span> · {etaText} left
      </>
    );
  if (t.status === "seeding")
    return (
      <>
        {formatBytes(t.size)} · <span className="u">↑ {formatRate(t.ul)}</span> · ratio {t.ratio.toFixed(2)}
      </>
    );
  if (t.status === "error") return <span className="bad">{t.err}</span>;
  return (
    <>
      {formatBytes(t.done)} of {formatBytes(t.wanted)}
    </>
  );
}

export function TorrentList({ rows, onSelect, onContext, onToggle, onRemoveKey, onSpace, onDoubleClick }: Props) {
  const ui = useUi();
  const view = useListView();
  const activeId = useActiveId();
  const allRef = useRef<HTMLInputElement>(null);
  const sel = new Set(ui.sel);
  const selectedShown = rows.filter((t) => sel.has(t.id)).length;

  useEffect(() => {
    if (allRef.current) allRef.current.indeterminate = selectedShown > 0 && selectedShown < rows.length;
  }, [selectedShown, rows.length]);

  const sortBy = (key: SortKey) => view.go({ sort: { key, dir: view.sort.key === key ? (-view.sort.dir as 1 | -1) : defaultDir(key) } }, { replace: true });

  const click = (e: MouseEvent, t: Torrent) => {
    if ((e.target as HTMLElement).closest(".c-chk")) return;
    if (e.metaKey || e.ctrlKey) {
      onToggle(t.id);
    } else if (e.shiftKey && ui.anchor) {
      const ids = rows.map((r) => r.id);
      const a = ids.indexOf(ui.anchor);
      const b = ids.indexOf(t.id);
      if (a >= 0 && b >= 0) onSelect(ids.slice(Math.min(a, b), Math.max(a, b) + 1), t.id, ui.anchor);
    } else {
      onSelect([t.id], t.id, t.id);
    }
  };

  const keyDown = (e: React.KeyboardEvent, t: Torrent) => {
    const ids = rows.map((r) => r.id);
    const i = ids.indexOf(t.id);
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const next = ids[Math.max(0, Math.min(ids.length - 1, i + (e.key === "ArrowDown" ? 1 : -1)))];
      onSelect([next], next, next);
      requestAnimationFrame(() => document.querySelector<HTMLElement>(`.row[data-id="${next}"]`)?.focus());
    } else if (e.key === "Enter") {
      onSelect([t.id], t.id, t.id);
    } else if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
      e.preventDefault();
      const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
      onContext(r.left + 60, r.bottom, t.id);
    } else if (e.key === "Delete" || e.key === "Backspace") {
      e.preventDefault();
      onRemoveKey();
    } else if (e.key === " ") {
      e.preventDefault();
      onSpace();
    }
  };

  return (
    <div className="list cols" role="grid" aria-label="Torrents">
      <div className="head" role="row">
        <label className="c-chk" title="Select all shown">
          <input
            ref={allRef}
            type="checkbox"
            aria-label="Select all shown torrents"
            checked={rows.length > 0 && selectedShown === rows.length}
            disabled={rows.length === 0}
            onChange={(e) => {
              const ids = e.target.checked ? rows.map((r) => r.id) : [];
              onSelect(ids, ids.includes(activeId ?? -1) ? (activeId as number) : (ids[0] ?? activeId ?? 0), ui.anchor);
            }}
          />
        </label>
        {COLS.map(([k, label, c]) => (
          <button
            key={k}
            className={`${c} c-${k}`}
            aria-sort={view.sort.key === k ? (view.sort.dir > 0 ? "ascending" : "descending") : undefined}
            onClick={() => sortBy(k)}
          >
            {label}
            {view.sort.key === k ? (view.sort.dir > 0 ? " ↑" : " ↓") : ""}
          </button>
        ))}
      </div>
      {rows.length === 0 ? (
        <div className="empty">
          <strong>No torrents here</strong>
          {view.q ? `Nothing matches “${view.q}”.` : "Add a magnet link or .torrent file to get started."}
        </div>
      ) : (
        rows.map((t) => {
          const etaText = t.status === "downloading" ? formatEta(t.eta) : t.status === "checking" ? "…" : "–";
          const p = t.progress;
          return (
            <div
              key={t.id}
              className={`row st-${t.status} ${sel.has(t.id) ? "sel" : ""}`}
              role="row"
              tabIndex={0}
              data-id={t.id}
              aria-selected={sel.has(t.id)}
              onClick={(e) => click(e, t)}
              onDoubleClick={(e) => {
                if ((e.target as HTMLElement).closest(".c-chk")) return;
                onSelect([t.id], t.id, t.id);
                onDoubleClick(t);
              }}
              onContextMenu={(e) => {
                e.preventDefault();
                onContext(e.clientX, e.clientY, t.id);
              }}
              onKeyDown={(e) => keyDown(e, t)}
            >
              <label className="c-chk">
                <input type="checkbox" id={`chk-${t.id}`} checked={sel.has(t.id)} aria-label={`Select ${t.name}`} onChange={() => onToggle(t.id)} />
              </label>
              <div className="c-queue num r z" title="Queue position">{t.queuePos ?? "–"}</div>
              <div className="c-name">
                <span className="nm" title={savesTo(t)}>{t.name}</span>
                <span className="chips">
                  {t.tags.slice(0, 2).map((g) => (
                    <span key={g} className="tag">{g}</span>
                  ))}
                  {t.tags.length > 2 && <span className="tag" title={t.tags.slice(2).join(", ")}>+{t.tags.length - 2}</span>}
                  {t.seq && <span className="flag" title="Sequential download">SEQ</span>}
                  {(t.dlLimit > 0 || t.ulLimit > 0) && <span className="flag" title="Has its own speed limit">LIM</span>}
                </span>
              </div>
              <div className="c-size num r">{formatBytes(t.size)}</div>
              <div className="c-prog">
                <div className="bar"><i style={{ width: `${(p * 100).toFixed(2)}%` }} /></div>
                <span className="num">{p >= 1 ? "100%" : `${(p * 100).toFixed(1)}%`}</span>
              </div>
              <div className="c-status">
                <span className="pill">{t.force && ["downloading", "seeding"].includes(t.status) ? "[F] " : ""}{STATUS_TEXT[t.status]}</span>
              </div>
              <div className={`c-dl num r ${t.dl > 1 ? "d" : "z"}`}>{formatRate(t.dl)}</div>
              <div className={`c-ul num r ${t.ul > 1 ? "u" : "z"}`}>{formatRate(t.ul)}</div>
              <div className="c-eta num r">{etaText}</div>
              <div className="c-peers num r" title={`${t.seeds} seeds, ${t.leech} peers`}>
                {t.seeds}<span className="z">/{t.leech}</span>
              </div>
              <div className="c-ratio num r">{t.ratio.toFixed(2)}</div>
              <div className="c-mob">{mobileLine(t)}</div>
            </div>
          );
        })
      )}
    </div>
  );
}
