import { useEffect, useState, type DragEvent, type FormEvent } from "react";
import { Modal } from "@/components/Modal/Modal";
import { useToast } from "@/components/Toasts/Toasts";
import { useCatalog } from "@/features/catalog/queries";
import { useRenames } from "@/features/renames/queries";
import { useSettings } from "@/features/settings/queries";
import { useAddTorrent } from "@/features/torrents/mutations";
import { useContents, useStats } from "@/features/torrents/queries";
import { formatBytes } from "@/utils/format";
import { parseTags } from "@/utils/tags";

interface Props {
  defaultTag: string | null;
  onClose: () => void;
  onAdded: (id: number) => void;
}

function readBase64(f: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(String(r.result).split(",")[1] ?? "");
    r.onerror = () => reject(r.error);
    r.readAsDataURL(f);
  });
}

const MAGNET_OK = /^magnet:\?/i;
const BTIH = /xt=urn:btih:[0-9a-z]{32,40}/i;

export function AddTorrentDialog({ defaultTag, onClose, onAdded }: Props) {
  const { data: set } = useSettings();
  const { data: catalog } = useCatalog();
  const { data: stats } = useStats();
  const { data: presets } = useRenames();
  const add = useAddTorrent();
  const toast = useToast();

  const [magnet, setMagnet] = useState("");
  const [debounced, setDebounced] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [fileData, setFileData] = useState("");
  const [over, setOver] = useState(false);
  const [path, setPath] = useState(set.savePath);
  const [tags, setTags] = useState(defaultTag && defaultTag !== "__none" ? defaultTag : "");
  const [start, setStart] = useState(set.startAuto);
  const [layout, setLayout] = useState(set.createSubfolder ? "original" : "nosub");
  const [skipHash, setSkipHash] = useState(false);
  const [seq, setSeq] = useState(false);
  const [flp, setFlp] = useState(false);
  const [top, setTop] = useState(false);
  const [preset, setPreset] = useState(0);
  const [checked, setChecked] = useState<Set<number>>(new Set());
  const [error, setError] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(magnet.trim()), 300);
    return () => clearTimeout(t);
  }, [magnet]);

  const params = debounced && MAGNET_OK.test(debounced) && BTIH.test(debounced)
    ? { magnet: debounced }
    : !debounced && file
      ? fileData ? { fileName: file.name, fileData } : null
      : null;
  const contents = useContents(params);
  const files = contents.data?.contents.files;

  useEffect(() => {
    if (files) setChecked(new Set(files.map((_, i) => i)));
  }, [files]);

  const takeFile = (f: File | undefined) => {
    if (!f) return;
    if (!/\.torrent$/i.test(f.name)) return setError("That file isn’t a .torrent file. Choose a file ending in .torrent.");
    setError("");
    setFile(f);
    setFileData("");
    void readBase64(f).then(setFileData).catch(() => setError("Couldn’t read that file. Choose it again."));
  };

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setOver(false);
    takeFile(e.dataTransfer.files[0]);
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const m = magnet.trim();
    if (!m && !file) return setError("Paste a magnet link or choose a .torrent file first.");
    if (m && (!MAGNET_OK.test(m) || !BTIH.test(m)))
      return setError("This doesn’t look like a magnet link. It should start with “magnet:?xt=urn:btih:” followed by a 40-character hash.");
    if (files && checked.size === 0) return setError("Select at least one file to download.");
    setError("");
    add.mutate(
      {
        magnet: m || undefined,
        fileName: m ? undefined : file?.name,
        fileData: m ? undefined : fileData,
        path: path.trim() || set.savePath,
        tags: parseTags(tags),
        preset,
        start,
        layout,
        skipHash,
        seq,
        flp,
        top,
        selected: files ? [...checked].sort((a, b) => a - b) : undefined,
      },
      {
        onSuccess: (t) => {
          toast(skipHash ? `Added ${t.name} without checking files` : start ? `Added ${t.name} · fetching metadata` : `Added ${t.name} (paused)`);
          onAdded(t.id);
          onClose();
        },
        onError: (err) => setError(err.message),
      },
    );
  };

  const selected = files ? files.filter((_, i) => checked.has(i)) : [];
  const bytes = selected.reduce((a, f) => a + f.size, 0);
  const totalSize = contents.data?.contents.size ?? 0;

  return (
    <Modal title="Add torrent" subtitle="Paste a magnet link or choose a .torrent file." onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <label className="field">
          <span>Magnet link</span>
          <textarea value={magnet} onChange={(e) => setMagnet(e.target.value)} placeholder="magnet:?xt=urn:btih:…&dn=…" spellCheck={false} autoFocus />
        </label>
        <label
          className={`drop${over ? " over" : ""}`}
          onDragEnter={(e) => { e.preventDefault(); setOver(true); }}
          onDragOver={(e) => { e.preventDefault(); setOver(true); }}
          onDragLeave={() => setOver(false)}
          onDrop={onDrop}
        >
          <input type="file" accept=".torrent" onChange={(e) => takeFile(e.target.files?.[0])} />
          <span>
            {file ? <>Selected <b>{file.name}</b> · {formatBytes(file.size)}</> : <>Or drop a <b>.torrent</b> file here, or click to choose one</>}
          </span>
        </label>

        {params && (
          <div className="contents">
            <div className="ct-head">
              <b>Files</b>
              <span className="num">
                {files ? (
                  <>
                    {selected.length} of {files.length} files · {formatBytes(bytes)} of {formatBytes(totalSize)}
                    {bytes > stats.free && <> · <span className="bad">more than the free space</span></>}
                  </>
                ) : null}
              </span>
              <span style={{ flex: 1 }} />
              {files && (
                <>
                  <button type="button" className="linkish" onClick={() => setChecked(new Set(files.map((_, i) => i)))}>Select all</button>
                  <button type="button" className="linkish" onClick={() => setChecked(new Set())}>Select none</button>
                </>
              )}
            </div>
            <div className="ct-list">
              {!files ? (
                <p className="ct-wait">
                  <span className="spin" aria-hidden="true" />
                  {contents.isError ? contents.error.message : "magnet" in params ? "Getting the file list from peers…" : "Reading the .torrent file…"}
                </p>
              ) : (
                files.map((f, i) => (
                  <label key={`${f.name}-${i}`} className="ct-row">
                    <input
                      type="checkbox"
                      checked={checked.has(i)}
                      onChange={(e) =>
                        setChecked((cur) => {
                          const next = new Set(cur);
                          if (e.target.checked) next.add(i);
                          else next.delete(i);
                          return next;
                        })
                      }
                    />
                    <span className="ct-name">{f.name}</span>
                    <span className="num z">{formatBytes(f.size)}</span>
                  </label>
                ))
              )}
            </div>
          </div>
        )}

        <label className="field">
          <span>Save to</span>
          <input type="text" value={path} onChange={(e) => setPath(e.target.value)} />
        </label>
        <div className="row2">
          <label className="field">
            <span>Tags</span>
            <input type="text" list="add-tags" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="Comma-separated, such as 4K, keep" autoComplete="off" />
            <datalist id="add-tags">{catalog.tags.map((g) => <option key={g} value={g} />)}</datalist>
          </label>
          <label className="field">
            <span>Rename preset</span>
            <select value={preset} onChange={(e) => setPreset(Number(e.target.value))}>
              <option value={0}>Don’t rename</option>
              {presets.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          </label>
        </div>
        <label className="check">
          <input type="checkbox" checked={start} onChange={(e) => setStart(e.target.checked)} /> Start downloading right away
        </label>
        <details className="sample">
          <summary>More options</summary>
          <label className="field">
            <span>Content layout</span>
            <select value={layout} onChange={(e) => setLayout(e.target.value)}>
              <option value="original">Original</option>
              <option value="subfolder">Create a subfolder</option>
              <option value="nosub">Don’t create a subfolder</option>
            </select>
          </label>
          {stats.real ? (
            <>
              <label className="check"><input type="checkbox" disabled /> Skip hash check: not available. Bitswarm always checks files that are already on disk.</label>
              <label className="check"><input type="checkbox" disabled /> Download in sequential order: not supported by the BitTorrent engine yet.</label>
              <label className="check"><input type="checkbox" disabled /> Download first and last pieces first: not supported by the BitTorrent engine yet.</label>
            </>
          ) : (
            <>
              <label className="check"><input type="checkbox" checked={skipHash} onChange={(e) => setSkipHash(e.target.checked)} /> Skip hash check (only if the files are already complete on disk)</label>
              <label className="check"><input type="checkbox" checked={seq} onChange={(e) => setSeq(e.target.checked)} /> Download in sequential order</label>
              <label className="check"><input type="checkbox" checked={flp} onChange={(e) => setFlp(e.target.checked)} /> Download first and last pieces first</label>
            </>
          )}
          <label className="check"><input type="checkbox" checked={top} onChange={(e) => setTop(e.target.checked)} /> Add to the top of the queue</label>
        </details>
        {error && <p className="err-msg">{error}</p>}
        <div className="actions">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn primary" disabled={add.isPending}>Add torrent</button>
        </div>
      </form>
    </Modal>
  );
}
