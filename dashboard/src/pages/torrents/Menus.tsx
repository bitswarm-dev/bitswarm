import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useToast } from "@/components/Toasts/Toasts";
import { useCatalog } from "@/features/catalog/queries";
import { useApplyPreset } from "@/features/renames/mutations";
import { useRenames } from "@/features/renames/queries";
import type { PresetView } from "@/features/renames/types";
import type { Torrent } from "@/features/torrents/types";
import { plural } from "@/utils/format";
import { parseTags } from "@/utils/tags";
import { useActions } from "./useActions";

interface Props {
  selection: Torrent[];
  close: () => void;
}

export function TagMenu({ selection }: Omit<Props, "close">) {
  const { data: catalog } = useCatalog();
  const run = useActions();
  const [text, setText] = useState("");
  const ids = selection.map((t) => t.id);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const tags = parseTags(text);
    if (!tags.length) return;
    setText("");
    run({ action: "tags", how: "add", tags, ids });
  };

  return (
    <>
      {catalog.tags.length ? (
        catalog.tags.map((g) => {
          const n = selection.filter((t) => t.tags.includes(g)).length;
          const state = n && n === selection.length ? "all" : n ? "some" : "none";
          return (
            <button
              key={g}
              role="menuitemcheckbox"
              aria-checked={state === "all" ? true : state === "some" ? "mixed" : false}
              className="lopt"
              onClick={() => run({ action: "tags", how: "toggle", tag: g, on: state !== "all", ids })}
            >
              <span className="ck" aria-hidden="true">{state === "all" ? "✓" : state === "some" ? "–" : ""}</span>
              <span>{g}</span>
            </button>
          );
        })
      ) : (
        <p className="z" style={{ margin: "6px 10px" }}>No tags yet. Add one below.</p>
      )}
      <hr />
      <form className="tagnew" onSubmit={submit}>
        <input type="text" value={text} onChange={(e) => setText(e.target.value)} placeholder="New tag, or several with commas" aria-label="New tag name" maxLength={120} autoComplete="off" />
        <button type="submit" className="btn">Add</button>
      </form>
      {selection.some((t) => t.tags.length > 0) && (
        <button role="menuitem" className="lopt" onClick={() => run({ action: "tags", how: "clear", ids })}>
          <span className="ck" />
          <span>Remove all tags from the selection</span>
        </button>
      )}
      <button role="menuitem" className="lopt" onClick={() => run({ action: "tags", how: "prune", ids })}>
        <span className="ck" />
        <span>Delete tags no torrent uses</span>
      </button>
    </>
  );
}

export function RenameMenu({ selection, close }: Props) {
  const nav = useNavigate();
  const toast = useToast();
  const { data: presets } = useRenames();
  const apply = useApplyPreset();
  const ids = selection.map((t) => t.id);

  const pick = (p: PresetView) => {
    close();
    apply.mutate(
      { id: p.id, ids },
      {
        onSuccess: (r) =>
          toast(r.names ? `Renamed ${plural(r.names, "name")} in ${plural(r.torrents, "torrent")} with ${p.name}` : `${p.name}: no names matched in the selected torrents`),
        onError: (e) => toast(e.message),
      },
    );
  };

  return (
    <>
      {presets.length ? (
        presets.map((p) => (
          <button key={p.id} role="menuitem" onClick={() => pick(p)}>
            <span>{p.name}</span>
            <small>/{p.find}/{(p.g ? "g" : "") + (p.i ? "i" : "")} → {p.replace || "(delete)"}</small>
          </button>
        ))
      ) : (
        <p className="z" style={{ margin: "6px 10px" }}>No presets yet.</p>
      )}
      <hr />
      <button
        role="menuitem"
        onClick={() => {
          close();
          nav("/renames");
        }}
      >
        <span>Manage presets…</span>
      </button>
    </>
  );
}
