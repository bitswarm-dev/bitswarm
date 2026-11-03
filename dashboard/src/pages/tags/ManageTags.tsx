import { Suspense, useState, type FormEvent } from "react";
import { useListView } from "@/app/listView";
import { Modal } from "@/components/Modal/Modal";
import { useToast } from "@/components/Toasts/Toasts";
import { useAddTag, useDeleteTag, useRenameTag } from "@/features/catalog/mutations";
import { useCatalog, useTagUsage } from "@/features/catalog/queries";

function Body() {
  const toast = useToast();
  const view = useListView();
  const { data: catalog } = useCatalog();
  const { data: usage, refetch } = useTagUsage();
  const add = useAddTag();
  const rename = useRenameTag();
  const del = useDeleteTag();
  const [newName, setNewName] = useState("");
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [deleting, setDeleting] = useState<string | null>(null);
  const [error, setError] = useState("");

  const fail = (e: Error) => setError(e.message);
  const done = () => {
    setError("");
    void refetch();
  };

  const submitAdd = (e: FormEvent) => {
    e.preventDefault();
    if (!newName.trim()) return;
    add.mutate(newName, { onSuccess: (r) => { setNewName(""); done(); toast(`Added ${r.name}`); }, onError: fail });
  };

  const submitRename = (from: string) => (e: FormEvent) => {
    e.preventDefault();
    rename.mutate(
      { from, to: draft },
      {
        onSuccess: (r) => {
          if (view.tag === from) view.go({ tag: r.name }, { replace: true });
          setEditing(null);
          done();
          toast(r.name === from ? "Name unchanged" : `Renamed ${from} to ${r.name}`);
        },
        onError: fail,
      },
    );
  };

  const confirmDelete = (name: string) =>
    del.mutate(name, {
      onSuccess: () => {
        if (view.tag === name) view.go({ tag: null }, { replace: true });
        setDeleting(null);
        done();
        toast(`Deleted ${name}`);
      },
      onError: fail,
    });

  return (
    <>
      <form className="field" onSubmit={submitAdd} style={{ flexDirection: "row", gap: 8, alignItems: "center" }}>
        <input type="text" aria-label="New tag name" value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="New tag name" maxLength={40} autoComplete="off" style={{ flex: 1 }} />
        <button type="submit" className="btn primary" disabled={add.isPending || !newName.trim()}>Add tag</button>
      </form>
      {error && <p className="err-msg">{error}</p>}
      {catalog.tags.length === 0 ? (
        <p className="z">No tags yet. Add one above, or tag a torrent from the toolbar.</p>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead><tr><th>Tag</th><th>Used by</th><th /></tr></thead>
            <tbody>
              {catalog.tags.map((t) => {
                const uses = usage[t] ?? [];
                return (
                  <tr key={t}>
                    <td className="fname">
                      {editing === t ? (
                        <form onSubmit={submitRename(t)} style={{ display: "flex", gap: 6 }}>
                          <input type="text" aria-label={`New name for ${t}`} value={draft} onChange={(e) => setDraft(e.target.value)} maxLength={40} autoFocus autoComplete="off" />
                          <button type="submit" className="btn primary" disabled={rename.isPending}>Save</button>
                          <button type="button" className="btn" onClick={() => setEditing(null)}>Cancel</button>
                        </form>
                      ) : (
                        <span className="tag">{t}</span>
                      )}
                    </td>
                    <td>
                      {uses.length === 0 ? (
                        <span className="z">Not used</span>
                      ) : (
                        uses.map((u) => (
                          <div key={u.kind} title={u.names.join(", ")}>
                            {u.count} {u.kind}
                          </div>
                        ))
                      )}
                    </td>
                    <td style={{ whiteSpace: "nowrap", textAlign: "right" }}>
                      {deleting === t ? (
                        <span className="confirm" style={{ padding: 0, border: 0, background: "none", justifyContent: "flex-end" }}>
                          <span>Remove “{t}” from {uses.length ? uses.map((u) => `${u.count} ${u.kind}`).join(", ") : "nothing"}?</span>
                          <button className="btn" onClick={() => setDeleting(null)}>Cancel</button>
                          <button className="btn danger solid" disabled={del.isPending} onClick={() => confirmDelete(t)}>Delete</button>
                        </span>
                      ) : (
                        <>
                          <button className="btn" onClick={() => { setEditing(t); setDraft(t); setDeleting(null); setError(""); }}>Rename</button>{" "}
                          <button className="btn danger" onClick={() => { setDeleting(t); setEditing(null); setError(""); }}>Delete</button>
                        </>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      <p className="hint" style={{ margin: 0 }}>
        Renaming changes the tag on every torrent, schedule, rename preset, maintenance task and hook that uses it. Renaming to a tag that already exists merges the two. Deleting removes the tag from all of them; the torrents and files stay.
      </p>
    </>
  );
}

export function ManageTags({ onClose }: { onClose: () => void }) {
  return (
    <Modal title="Manage tags" subtitle="Add, rename, merge or delete tags everywhere they are used." wide onClose={onClose}>
      <Suspense fallback={<p className="z">Loading tags…</p>}>
        <Body />
      </Suspense>
      <div className="actions">
        <button type="button" className="btn primary" onClick={onClose}>Done</button>
      </div>
    </Modal>
  );
}
