import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useDismiss } from "@/components/Menu/useDismiss";
import { useToast } from "@/components/Toasts/Toasts";
import type { Torrent } from "@/features/torrents/types";
import { useActions } from "./useActions";

interface Item {
  k: string;
  l: string;
  on?: boolean;
  check?: boolean;
  danger?: boolean;
}

interface Props {
  x: number;
  y: number;
  selection: Torrent[];
  onClose: () => void;
  onOption: (kind: "move" | "limits") => void;
  onMenu: (kind: "tags" | "rename") => void;
  onRemove: () => void;
}

export function magnetOf(t: Torrent): string {
  const tr = t.tracker && t.tracker !== "DHT only" ? `&tr=${encodeURIComponent(t.tracker)}` : "";
  return `magnet:?xt=urn:btih:${t.hash}&dn=${encodeURIComponent(t.name)}${tr}`;
}

export function useCopy() {
  const toast = useToast();
  return (text: string, what: string) => {
    const done = () => toast(`${what} copied`);
    const fallback = () => {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      let ok = false;
      try {
        ok = document.execCommand("copy");
      } catch {
        ok = false;
      }
      ta.remove();
      if (ok) done();
      else toast(`Couldn’t copy ${what.toLowerCase()}. Select it and press Ctrl+C.`);
    };
    if (navigator.clipboard) navigator.clipboard.writeText(text).then(done, fallback);
    else fallback();
  };
}

export function ContextMenu({ x, y, selection, onClose, onOption, onMenu, onRemove }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const run = useActions();
  const copy = useCopy();
  const [pos, setPos] = useState({ left: x, top: y });
  useDismiss(ref, true, onClose);

  useLayoutEffect(() => {
    const r = ref.current?.getBoundingClientRect();
    if (r) setPos({ left: Math.max(8, Math.min(x, innerWidth - r.width - 8)), top: Math.max(8, Math.min(y, innerHeight - r.height - 8)) });
  }, [x, y]);

  useEffect(() => {
    ref.current?.querySelector<HTMLButtonElement>("button:not(:disabled)")?.focus();
    const close = () => onClose();
    window.addEventListener("resize", close);
    return () => window.removeEventListener("resize", close);
  }, [onClose]);

  const ids = selection.map((t) => t.id);
  const all = (f: (t: Torrent) => boolean) => selection.length > 0 && selection.every(f);
  const some = (f: (t: Torrent) => boolean) => selection.some(f);
  const unfinished = some((t) => t.progress < 1);
  const many = selection.length > 1;

  const items: (Item | null)[] = [
    { k: "resume", l: "Resume", on: some((t) => ["paused", "done", "error", "queued"].includes(t.status)) },
    { k: "pause", l: "Pause", on: some((t) => ["downloading", "seeding", "checking", "queued"].includes(t.status)) },
    { k: "force", l: "Force start (ignore the queue)", check: all((t) => t.force) },
    null,
    { k: "recheck", l: "Force recheck" },
    { k: "reannounce", l: "Force reannounce" },
    { k: "magnet", l: many ? "Copy magnet links" : "Copy magnet link" },
    { k: "hash", l: many ? "Copy info hashes" : "Copy info hash" },
    null,
    { k: "move", l: "Move files…" },
    { k: "limits", l: "Speed and ratio limits…" },
    { k: "seq", l: "Download in sequential order", check: all((t) => t.seq), on: unfinished },
    { k: "flp", l: "Download first and last pieces first", check: all((t) => t.flp), on: unfinished },
    null,
    { k: "qtop", l: "Move to top of queue", on: unfinished },
    { k: "qup", l: "Move up in queue", on: unfinished },
    { k: "qdown", l: "Move down in queue", on: unfinished },
    { k: "qbottom", l: "Move to bottom of queue", on: unfinished },
    null,
    { k: "tags", l: "Tags…" },
    { k: "rename", l: "Rename with a preset…" },
    null,
    { k: "remove", l: "Remove…", danger: true },
  ];

  const act = (k: string) => {
    onClose();
    switch (k) {
      case "magnet":
        return copy(selection.map(magnetOf).join("\n"), many ? "Magnet links" : "Magnet link");
      case "hash":
        return copy(selection.map((t) => t.hash).join("\n"), many ? "Info hashes" : "Info hash");
      case "move":
      case "limits":
        return onOption(k);
      case "tags":
      case "rename":
        return onMenu(k);
      case "remove":
        return onRemove();
      case "qtop":
      case "qup":
      case "qdown":
      case "qbottom":
        return run({ action: "queue", how: k.slice(1), ids });
      default:
        return run({ action: k, ids });
    }
  };

  return createPortal(
    <div className="menu ctx" role="menu" aria-label="Torrent actions" ref={ref} style={{ left: pos.left, top: pos.top }}>
      <div className="ctx-title">{selection.length === 1 ? selection[0].name : `${selection.length} torrents`}</div>
      {items.map((i, n) =>
        i === null ? (
          <hr key={n} />
        ) : (
          <button
            key={i.k}
            role={i.check !== undefined ? "menuitemcheckbox" : "menuitem"}
            aria-checked={i.check}
            className={`lopt${i.danger ? " danger" : ""}`}
            disabled={i.on === false}
            onClick={() => act(i.k)}
          >
            <span className="ck" aria-hidden="true">{i.check ? "✓" : ""}</span>
            <span>{i.l}</span>
          </button>
        ),
      )}
    </div>,
    document.body,
  );
}
