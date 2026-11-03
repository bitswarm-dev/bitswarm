import { useEffect, useMemo, useState } from "react";
import { useActiveId, useListView } from "@/app/listView";
import { useUi } from "@/app/ui";
import { MenuWrap } from "@/components/Menu/MenuWrap";
import { Svg } from "@/components/Icons/Svg";
import { useToast } from "@/components/Toasts/Toasts";
import { visibleTorrents } from "@/features/torrents/filters";
import { useSetAlt } from "@/features/torrents/mutations";
import { useStats, useTorrents } from "@/features/torrents/queries";
import { useSettings } from "@/features/settings/queries";
import { formatKib } from "@/utils/format";
import { useUrlText } from "@/utils/useUrlText";
import { AddTorrentDialog } from "./AddTorrentDialog";
import { ContextMenu } from "./ContextMenu";
import { Inspector } from "./Inspector";
import { RenameMenu, TagMenu } from "./Menus";
import { OptionsDialog } from "./OptionsDialog";
import { TorrentList } from "./TorrentList";
import { useActions } from "./useActions";

type MenuKind = "tags" | "rename";

export function Torrents() {
  const ui = useUi();
  const view = useListView();
  const currentId = useActiveId();
  const { data: torrents } = useTorrents();
  const { data: stats } = useStats();
  const { data: set } = useSettings();
  const run = useActions();
  const setAlt = useSetAlt();
  const toast = useToast();

  const [adding, setAdding] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [deleteData, setDeleteData] = useState(false);
  const [ctx, setCtx] = useState<{ x: number; y: number } | null>(null);
  const [option, setOption] = useState<"move" | "limits" | null>(null);
  const [openMenu, setOpenMenu] = useState<MenuKind | null>(null);
  const [qText, setQText] = useUrlText(view.q, (q) => view.go({ q }, { replace: true }));

  const rows = useMemo(
    () => visibleTorrents(torrents, { filter: view.filter, tag: view.tag, q: view.q, sort: view.sort }),
    [torrents, view.filter, view.tag, view.q, view.sort.key, view.sort.dir], // eslint-disable-line react-hooks/exhaustive-deps
  );
  const known = new Set(torrents.map((t) => t.id));
  const selIds = ui.sel.filter((id) => known.has(id));
  const selection = torrents.filter((t) => selIds.includes(t.id));
  const active = torrents.find((t) => t.id === currentId) ?? null;

  useEffect(() => {
    if (torrents.length === 0 || ui.sel.length > 0) return;
    const id = view.active !== null && known.has(view.active) ? view.active : torrents[0].id;
    ui.patch({ sel: [id], anchor: id });
  }, [ui, torrents]); // eslint-disable-line react-hooks/exhaustive-deps

  const select = (ids: number[], activeId: number | null, anchor: number | null) => {
    setConfirming(false);
    ui.patch({ sel: ids, anchor });
    view.go({ active: activeId }, { replace: true });
  };

  const toggle = (id: number) => {
    const has = ui.sel.includes(id);
    const sel = has ? ui.sel.filter((x) => x !== id) : [...ui.sel, id];
    select(sel, has ? currentId : id, id);
  };

  const ids = selection.map((t) => t.id);
  const n = selection.length;
  const canResume = selection.some((t) => ["paused", "done", "error", "queued"].includes(t.status));
  const canPause = selection.some((t) => ["downloading", "seeding", "checking", "queued"].includes(t.status));
  const count = (label: string) => (n > 1 ? `${label} (${n})` : label);

  const toggleRun = () => {
    const any = selection.some((t) => ["downloading", "seeding", "checking"].includes(t.status));
    run({ action: any ? "pause" : "resume", ids });
  };

  const confirmRemove = () => {
    run({ action: "remove", ids, deleteData }, () => {
      setConfirming(false);
      setDeleteData(false);
      ui.patch({ sel: [], anchor: null });
      view.go({ active: null }, { replace: true });
    });
  };

  const openContext = (x: number, y: number, id: number) => {
    if (!ui.sel.includes(id)) select([id], id, id);
    else view.go({ active: id }, { replace: true });
    setCtx({ x, y });
  };

  const showMenu = (kind: MenuKind) => setOpenMenu(kind);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).matches?.("input,textarea,select,button,a,summary")) return;
      if (document.querySelector(".scrim") || ctx) return;
      if (e.key === "Escape") setConfirming(false);
      else if (e.key === "Delete" || e.key === "Backspace") {
        if (n) {
          e.preventDefault();
          setConfirming(true);
        }
      } else if (e.key === " " && n) {
        e.preventDefault();
        toggleRun();
        document.querySelector<HTMLElement>(`.row[data-id="${currentId}"]`)?.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  });

  return (
    <main>
      <div>
        <div className="tools">
          <button className="btn primary" onClick={() => setAdding(true)}>
            <Svg size={15} strokeWidth={2.4}><path d="M12 5v14M5 12h14" /></Svg>
            <span className="t">Add torrent</span>
          </button>
          <label className="search">
            <Svg size={15}><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></Svg>
            <input type="search" placeholder="Search torrents" aria-label="Search torrents" value={qText} onChange={(e) => setQText(e.target.value)} />
          </label>
          <span className="sep" />
          <button className="btn" disabled={!canResume} title="Resume (Space)" onClick={() => run({ action: "resume", ids })}>
            <Svg fill="currentColor"><path d="M7 4.5v15l13-7.5z" /></Svg>
            <span className="t">{count("Resume")}</span>
          </button>
          <button className="btn" disabled={!canPause} title="Pause (Space)" onClick={() => run({ action: "pause", ids })}>
            <Svg fill="currentColor"><rect x="6" y="4.5" width="4" height="15" rx="1" /><rect x="14" y="4.5" width="4" height="15" rx="1" /></Svg>
            <span className="t">{count("Pause")}</span>
          </button>
          <button
            className="btn danger"
            disabled={!n}
            title="Remove (Delete)"
            onClick={() => {
              setDeleteData(false);
              setConfirming(true);
            }}
          >
            <Svg><path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" /></Svg>
            <span className="t">{count("Remove")}</span>
          </button>
          <MenuWrap
            open={openMenu === "tags"}
            onOpenChange={(o) => setOpenMenu(o ? "tags" : null)}
            button={(p) => (
              <button className="btn" disabled={!n} title="Add or remove tags on the selected torrents" {...p}>
                <Svg><path d="M5 9h14M5 15h14M10 3 8 21M16 3l-2 18" /></Svg>
                <span className="t">Tags</span>
              </button>
            )}
          >
            {() => <TagMenu selection={selection} />}
          </MenuWrap>
          <MenuWrap
            open={openMenu === "rename"}
            onOpenChange={(o) => setOpenMenu(o ? "rename" : null)}
            button={(p) => (
              <button className="btn" disabled={!n} title="Rename the selected torrents with a preset" {...p}>
                <Svg><path d="M4 20h4L19 9l-4-4L4 16z" /><path d="M13 7l4 4" /></Svg>
                <span className="t">Rename</span>
              </button>
            )}
          >
            {(close) => <RenameMenu selection={selection} close={close} />}
          </MenuWrap>
          <button
            className="btn"
            disabled={!n}
            aria-haspopup="menu"
            aria-expanded={!!ctx}
            title="More actions (or right-click a torrent)"
            onClick={(e) => {
              if (ctx) return setCtx(null);
              const r = e.currentTarget.getBoundingClientRect();
              setCtx({ x: r.left, y: r.bottom + 4 });
            }}
          >
            <Svg fill="currentColor"><circle cx="5" cy="12" r="1.8" /><circle cx="12" cy="12" r="1.8" /><circle cx="19" cy="12" r="1.8" /></Svg>
            <span className="t">More</span>
          </button>
          <span className="selcount">
            {n > 0 ? (
              <>
                <b className="num">{n}</b>selected
                {rows.some((t) => !ui.sel.includes(t.id)) && (
                  <button className="btn" onClick={() => select(rows.map((t) => t.id), currentId ?? rows[0]?.id ?? null, ui.anchor)}>Select all</button>
                )}
                <button className="btn" onClick={() => select([], currentId, ui.anchor)}>Clear</button>
              </>
            ) : (
              rows.length > 0 && <button className="btn" onClick={() => select(rows.map((t) => t.id), rows[0].id, rows[0].id)}>Select all</button>
            )}
          </span>
          <span className="spacer" />
          <button
            className="btn toggle"
            aria-pressed={stats.alt}
            title={`Use the alternative speed limits: ${formatKib(set.altDl)} down, ${formatKib(set.altUl)} up`}
            onClick={() => setAlt.mutate(!stats.alt, { onSuccess: () => toast(!stats.alt ? "Speed limit on" : "Speed limit off") })}
          >
            <Svg size={15}><path d="M12 13l4-4" /><path d="M3.5 17a9 9 0 1 1 17 0" /></Svg>
            <span className="t">Speed limit</span>
          </button>
        </div>
        {confirming && n > 0 && (
          <div className="confirm">
            <strong>{n === 1 ? `Remove “${selection[0].name}”?` : `Remove ${n} torrents?`}</strong>
            <label>
              <input type="checkbox" checked={deleteData} onChange={(e) => setDeleteData(e.target.checked)} /> Also delete downloaded data
            </label>
            <span className="spacer" style={{ flex: 1 }} />
            <button className="btn" onClick={() => setConfirming(false)}>Cancel</button>
            <button className="btn danger solid" autoFocus onClick={confirmRemove}>Remove</button>
          </div>
        )}
      </div>

      <TorrentList
        rows={rows}
        onSelect={(sel, a, anchor) => select(sel, a, anchor)}
        onContext={openContext}
        onToggle={toggle}
        onRemoveKey={() => n && setConfirming(true)}
        onSpace={toggleRun}
        onDoubleClick={(t) => run({ action: ["paused", "done", "error", "queued"].includes(t.status) ? "resume" : "pause", ids: [t.id] })}
      />

      <Inspector torrent={active} />

      {adding && (
        <AddTorrentDialog
          defaultTag={view.tag}
          onClose={() => setAdding(false)}
          onAdded={(id) => {
            ui.patch({ sel: [id], anchor: id });
            view.go({ active: id, filter: "all" }, { replace: true });
          }}
        />
      )}
      {ctx && n > 0 && (
        <ContextMenu
          x={ctx.x}
          y={ctx.y}
          selection={selection}
          onClose={() => setCtx(null)}
          onOption={setOption}
          onMenu={showMenu}
          onRemove={() => { setDeleteData(false); setConfirming(true); }}
        />
      )}
      {option && n > 0 && <OptionsDialog kind={option} selection={selection} onClose={() => setOption(null)} />}
    </main>
  );
}
