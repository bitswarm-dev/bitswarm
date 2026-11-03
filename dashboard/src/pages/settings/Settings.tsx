import { useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { useUi } from "@/app/ui";
import { useToast } from "@/components/Toasts/Toasts";
import { getDefaults } from "@/features/settings/api";
import { useSaveSettings } from "@/features/settings/mutations";
import { useSettings } from "@/features/settings/queries";
import { useStats } from "@/features/torrents/queries";
import { SECTIONS, depOff, sectionOf, type FieldDef, type SectionId } from "@/features/settings/sections";
import type { Settings as SettingsData } from "@/features/settings/types";
import { BackupSection } from "./BackupSection";
import { ApiError } from "@/lib/http";
import { safeObject } from "@/utils/safe";

const ENGINE_NOTES: Record<string, string> = {
  connection:
    "The port, UPnP, DHT and encryption options are read when Bitswarm starts, so restart Bitswarm after changing them. Connection limits, upload slots, the proxy, peer exchange, local peer discovery and uTP options are saved but the BitTorrent engine doesn’t use them yet.",
  speed: "Global limits and the alternative speed schedule apply right away. Limits for a single torrent aren’t supported yet.",
  webui: "The Web UI port, HTTPS and proxy options are saved but not used yet. Bitswarm listens on the PORT environment variable.",
};

const KEYS = (s: SettingsData) => Object.keys(s) as (keyof SettingsData)[];
const changed = (a: SettingsData, b: SettingsData) => KEYS(a).some((k) => a[k] !== b[k]);

export function Settings() {
  const { data: saved } = useSettings();
  const save = useSaveSettings();
  const { data: stats } = useStats();
  const toast = useToast();
  const ui = useUi();
  const [draft, setDraft] = useState<SettingsData>(saved);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const { section } = useParams({ from: "/settings/$section" });
  const navigate = useNavigate();
  const setSection = (id: SectionId, replace = false) => void navigate({ to: "/settings/$section", params: { section: id }, replace });
  const base = useRef(saved);

  useEffect(() => {
    if (!changed(base.current, saved) && !changed(draft, base.current)) return;
    if (!changed(draft, base.current)) setDraft(saved);
    base.current = saved;
  }, [saved, draft]);

  const dirty = changed(draft, saved) || draft.webPass !== "";
  useEffect(() => {
    if (ui.settingsDirty !== dirty) ui.patch({ settingsDirty: dirty });
  }, [dirty, ui]);
  useEffect(() => () => ui.patch({ settingsDirty: false }), []); // eslint-disable-line react-hooks/exhaustive-deps

  const set = <K extends keyof SettingsData>(k: K, v: SettingsData[K]) => {
    setDraft((d) => ({ ...d, [k]: v }));
    setErrors((e) => {
      const { [k]: _gone, ...rest } = e;
      return rest;
    });
  };

  const submit = () => {
    save.mutate(draft, {
      onSuccess: () => {
        setErrors({});
        setDraft((d) => ({ ...d, webPass: "", proxyPass: d.proxyPass }));
        toast("Settings saved");
      },
      onError: (e) => {
        const errs = e instanceof ApiError ? (safeObject(e.details.errors) as Record<string, string>) : {};
        setErrors(Object.keys(errs).length ? errs : { _: e.message });
        const first = Object.keys(errs)[0];
        const sec = first ? sectionOf(first) : undefined;
        if (sec) setSection(sec, true);
        if (first) requestAnimationFrame(() => document.getElementById(`set-${first}`)?.focus());
      },
    });
  };

  const restore = async () => {
    const d = await getDefaults();
    setDraft({ ...d, webPass: draft.webPass, proxyPass: draft.proxyPass });
    setErrors({});
    toast("Defaults restored. Save to keep them.");
  };

  const sec = SECTIONS.find((s) => s.id === section) ?? SECTIONS[0];
  const errList = Object.values(errors);

  const renderField = (fd: FieldDef) => {
    const id = `set-${fd.key}`;
    const disabled = depOff(fd, draft);
    const sub = fd.dep ? "sub" : "";
    const value = draft[fd.key];
    if (fd.type === "check")
      return (
        <label key={fd.key} className={`check ${sub}`}>
          <input type="checkbox" id={id} checked={value as boolean} disabled={disabled} onChange={(e) => set(fd.key, e.target.checked as never)} /> {fd.label}
        </label>
      );
    const msg = errors[fd.key] || fd.hint;
    return (
      <div key={fd.key} className={`sfield ${sub}`}>
        <label htmlFor={id}>{fd.label}</label>
        <div className="inp">
          {fd.type === "select" ? (
            <select id={id} value={value as string} disabled={disabled} onChange={(e) => set(fd.key, e.target.value as never)}>
              {fd.opts?.map(([v, t]) => <option key={v} value={v}>{t}</option>)}
            </select>
          ) : (
            <input
              id={id}
              type={fd.type}
              className={fd.mono ? "mono" : undefined}
              value={typeof value === "number" && Number.isNaN(value) ? "" : String(value)}
              min={fd.type === "number" ? (fd.min ?? 0) : undefined}
              max={fd.type === "number" ? fd.max : undefined}
              step={fd.type === "number" ? (fd.step ?? 1) : undefined}
              autoComplete={fd.type === "password" ? "new-password" : "off"}
              spellCheck={false}
              disabled={disabled}
              aria-invalid={errors[fd.key] ? true : undefined}
              onChange={(e) => {
                const raw = e.target.value;
                set(fd.key, (fd.type === "number" ? (raw.trim() === "" ? NaN : Number(raw)) : raw) as never);
              }}
            />
          )}
          {fd.unit && <span className="unit">{fd.unit}</span>}
          {fd.random && (
            <button type="button" className="btn" onClick={() => set("port", 49152 + Math.floor(Math.random() * 16383))}>Random</button>
          )}
        </div>
        {msg && <small className={errors[fd.key] ? "bad" : ""}>{msg}</small>}
      </div>
    );
  };

  return (
    <section className="sched" aria-label="Settings">
      <div className="sched-head">
        <div>
          <h2>Settings</h2>
          <p>Options for downloads, connections, speed and the BitTorrent network, grouped the way most torrent clients do.</p>
        </div>
        {section !== "backup" && (
          <button className="btn" onClick={restore} style={{ marginLeft: "auto", borderColor: "var(--line)" }}>Restore defaults</button>
        )}
      </div>
      <div className="set-wrap">
        <nav className="set-nav" aria-label="Settings sections">
          {SECTIONS.map((s) => (
            <button key={s.id} aria-current={s.id === section ? "page" : "false"} onClick={() => setSection(s.id)}>{s.title}</button>
          ))}
        </nav>
        <form className="set-body" noValidate onSubmit={(e) => e.preventDefault()}>
          {stats.real && ENGINE_NOTES[section] && <p className="note">{ENGINE_NOTES[section]}</p>}
          {sec.groups ? (
            sec.groups.map((g) => (
              <fieldset key={g.title} className="sgroup">
                <legend>{g.title}</legend>
                {g.note && <p className="snote">{g.note}</p>}
                {g.fields.map(renderField)}
              </fieldset>
            ))
          ) : (
            <BackupSection />
          )}
        </form>
      </div>
      <div className="savebar" hidden={!dirty && errList.length === 0}>
        <span>
          {errList.length ? (
            <span className="bad">{errList[0]}{errList.length > 1 ? ` (and ${errList.length - 1} more)` : ""}</span>
          ) : (
            "You have unsaved changes."
          )}
        </span>
        <span style={{ flex: 1 }} />
        <button type="button" className="btn" onClick={() => { setDraft(saved); setErrors({}); toast("Changes discarded"); }}>Discard</button>
        <button type="button" className="btn primary" disabled={save.isPending} onClick={submit}>Save changes</button>
      </div>
    </section>
  );
}
