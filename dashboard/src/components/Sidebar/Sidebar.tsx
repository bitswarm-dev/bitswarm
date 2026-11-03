import { useState } from "react";
import { createPortal } from "react-dom";
import { useLocation, useNavigate } from "react-router-dom";
import { useUi, type GroupKey } from "@/app/ui";
import { Svg } from "@/components/Icons/Svg";
import { LineChart } from "@/components/Charts/LineChart";
import { useCatalog } from "@/features/catalog/queries";
import { FILTERS } from "@/features/torrents/filters";
import { useStats, useTorrents } from "@/features/torrents/queries";
import { useLog } from "@/features/log/queries";
import { useUpdates } from "@/features/updates/queries";
import { useTokens } from "@/features/tokens/queries";
import { useHooks } from "@/features/hooks/queries";
import { useTasks } from "@/features/maintenance/queries";
import { useSchedules } from "@/features/schedules/queries";
import { useRenames } from "@/features/renames/queries";
import { ManageTags } from "@/pages/tags/ManageTags";

interface GroupProps {
  id: GroupKey;
  title: string;
  controls: string;
  summary: string;
  action?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}

function Group({ id, title, controls, summary, action, className = "", children }: GroupProps) {
  const ui = useUi();
  const collapsed = !!ui.collapsed[id];
  return (
    <div className={`group ${className} ${collapsed ? "collapsed" : ""}`}>
      <h3 className="gh">
        <button className="gtog" aria-expanded={!collapsed} aria-controls={controls} onClick={() => ui.toggleGroup(id)}>
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
            <path d="m6 9 6 6 6-6" />
          </svg>
          {title}
          <span className="gsel">{collapsed ? summary : ""}</span>
        </button>
        {action}
      </h3>
      <div className="flist" id={controls}>
        {children}
      </div>
    </div>
  );
}

const NAV_ICON = {
  schedules: <><circle cx="12" cy="13" r="8" /><path d="M12 9v4l2.5 2M9 2h6" /></>,
  renames: <><path d="M4 20h4L19 9l-4-4L4 16z" /><path d="M13 7l4 4" /></>,
  maint: <path d="M14.7 6.3a4 4 0 0 0-5.4 5.4L3 18v3h3l6.3-6.3a4 4 0 0 0 5.4-5.4l-2.5 2.5-2.4-.6-.6-2.4z" />,
  log: <path d="M4 6h16M4 12h16M4 18h10" />,
  tokens: <><circle cx="8" cy="15" r="4" /><path d="m10.8 12.2 8.2-8.2M17 6l2 2M15 8l2 2" /></>,
  hooks: <><path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" /><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7" /></>,
  settings: <><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" /></>,
  about: <><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 8h.01" /></>,
};

type Page = keyof typeof NAV_ICON;

export function Sidebar() {
  const ui = useUi();
  const [managing, setManaging] = useState(false);
  const nav = useNavigate();
  const { pathname } = useLocation();
  const { data: torrents } = useTorrents();
  const { data: catalog } = useCatalog();
  const { data: stats } = useStats();
  const { data: renames } = useRenames();
  const { data: schedules } = useSchedules();
  const { data: tasks } = useTasks();
  const { data: hooks } = useHooks();
  const { data: tokens } = useTokens();
  const { data: upd } = useUpdates();
  const { data: errLog } = useLog({ levels: ["error"], src: "", q: "" });
  const logCount = Object.values(errLog.counts).reduce((a, b) => a + b, 0);
  const newErrors = (errLog.entries[0]?.t ?? 0) > ui.logSeen;
  const onTorrents = pathname === "/";
  const page = pathname.replace("/", "");

  const goTorrents = (p: Partial<{ filter: string; tag: string | null }>) => {
    ui.patch(p);
    nav("/");
  };

  const link = (to: Page, label: string, extra?: React.ReactNode) => (
    <button className="fbtn" aria-pressed={page === to} onClick={() => nav(`/${to}`)}>
      <Svg>{NAV_ICON[to]}</Svg>
      {label}
      {extra}
    </button>
  );

  const filterLabel = FILTERS.find((f) => f.key === ui.filter)?.label ?? "";
  const automationNames: Record<string, string> = { schedules: "Scheduled", renames: "Auto rename", maint: "Maintenance" };
  const developerNames: Record<string, string> = { hooks: "Hooks", log: "Execution log", tokens: "API tokens" };

  return (
    <aside>
      <Group id="status" title="Status" controls="filters" summary={onTorrents ? filterLabel : ""}>
        {FILTERS.map((f) => (
          <button key={f.key} className="fbtn" aria-pressed={onTorrents && ui.filter === f.key} onClick={() => goTorrents({ filter: f.key })}>
            <span className="sw" style={{ background: f.color }} />
            {f.label}
            <span className="n">{torrents.filter(f.test).length}</span>
          </button>
        ))}
      </Group>

      <Group id="automation" title="Automation" controls="autoNav" summary={automationNames[page] ?? ""}>
        {link("schedules", "Scheduled", <span className="n">{schedules.filter((r) => r.enabled).length}/{schedules.length}</span>)}
        {link("renames", "Auto rename", <span className="n">{renames.filter((r) => r.enabled).length}/{renames.length}</span>)}
        {link("maint", "Maintenance", <span className="n">{tasks.filter((t) => t.enabled).length}/{tasks.length}</span>)}
      </Group>

      <Group
        id="tags"
        title="Tags"
        controls="tagList"
        className="labels-g"
        action={<button className="gh-btn" onClick={() => setManaging(true)}>Manage</button>}
        summary={onTorrents ? (ui.tag === "__none" ? "Untagged" : ui.tag ? `#${ui.tag}` : "Any tag") : ""}
      >
        <button className="fbtn" aria-pressed={onTorrents && !ui.tag} onClick={() => goTorrents({ tag: null })}>
          <span className="sw" style={{ background: "var(--line)" }} />
          Any tag
          <span className="n">{torrents.length}</span>
        </button>
        <button className="fbtn" aria-pressed={onTorrents && ui.tag === "__none"} onClick={() => goTorrents({ tag: "__none" })}>
          <span className="sw0" />
          Untagged
          <span className="n">{torrents.filter((t) => t.tags.length === 0).length}</span>
        </button>
        {catalog.tags.map((g) => (
          <button key={g} className="fbtn" aria-pressed={onTorrents && ui.tag === g} onClick={() => goTorrents({ tag: g })}>
            <span className="taghash" aria-hidden="true">#</span>
            {g}
            <span className="n">{torrents.filter((t) => t.tags.includes(g)).length}</span>
          </button>
        ))}
      </Group>

      <Group id="developer" title="Developer" controls="devNav" summary={developerNames[page] ?? ""}>
        {link("log", "Execution log", newErrors && page !== "log" ? <span className="err-badge">New errors</span> : <span className="n">{logCount}</span>)}
        {link("tokens", "API tokens", <span className="n">{tokens.filter((k) => !k.expired).length}</span>)}
        {link("hooks", "Hooks", <span className="n">{hooks.filter((h) => h.enabled).length}/{hooks.length}</span>)}
      </Group>

      <div className="spark">
        <div className="legend">
          <span className="d">Download</span>
          <span className="u">Upload</span>
          <span>last 90 s</span>
        </div>
        <LineChart hist={stats.hist} axes={false} id="globalSpark" />
      </div>

      <div className="group set-g">
        <div className="flist">
          {link("settings", "Settings", ui.settingsDirty ? <span className="dirty">Unsaved</span> : null)}
          {link("about", "About", upd.state === "available" ? <span className="upd-badge">Update</span> : null)}
        </div>
      </div>
      {managing && createPortal(<ManageTags onClose={() => setManaging(false)} />, document.body)}
    </aside>
  );
}
