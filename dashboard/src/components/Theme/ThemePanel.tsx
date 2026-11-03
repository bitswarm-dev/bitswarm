import { useRef, type ChangeEvent } from "react";
import { ACCENTS, BGS, THEME_DEFAULT, useTheme, type AccentKey, type Mode } from "@/app/theme";
import { useDismiss } from "@/components/Menu/useDismiss";
import { useToast } from "@/components/Toasts/Toasts";

interface Props {
  open: boolean;
  onClose: () => void;
}

export function ThemePanel({ open, onClose }: Props) {
  const { theme, setTheme } = useTheme();
  const toast = useToast();
  const ref = useRef<HTMLDivElement>(null);
  useDismiss(ref, open, () => {
    const btn = document.getElementById("themeBtn");
    if (document.activeElement !== btn) onClose();
  });

  const onPhoto = (e: ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (!f) return;
    if (!f.type.startsWith("image/")) {
      toast("That file isn’t an image. Choose a JPG, PNG or WebP.");
      return;
    }
    const img = new Image();
    const url = URL.createObjectURL(f);
    img.onload = () => {
      const k = Math.min(1, 2000 / Math.max(img.width, img.height));
      const c = document.createElement("canvas");
      c.width = Math.round(img.width * k);
      c.height = Math.round(img.height * k);
      c.getContext("2d")?.drawImage(img, 0, 0, c.width, c.height);
      URL.revokeObjectURL(url);
      setTheme({ photo: c.toDataURL("image/jpeg", 0.82), bg: "photo", vis: 45 });
      toast("Background photo set");
    };
    img.onerror = () => toast("Couldn’t read that image. Try a different file.");
    img.src = url;
  };

  if (!open) return null;
  return (
    <div className="theme-pop" ref={ref} role="dialog" aria-label="Theme">
      <div className="top-row">
        <h2>Theme</h2>
        <button className="btn" onClick={onClose} aria-label="Close theme panel">✕</button>
      </div>
      <div>
        <h3>Mode</h3>
        <div className="seg">
          {(["system", "light", "dark"] as Mode[]).map((m) => (
            <button key={m} aria-pressed={theme.mode === m} onClick={() => setTheme({ mode: m })}>
              {m[0].toUpperCase() + m.slice(1)}
            </button>
          ))}
        </div>
      </div>
      <div>
        <h3>Accent color</h3>
        <div className="swatches">
          {(Object.keys(ACCENTS) as AccentKey[]).map((k) => (
            <button
              key={k}
              className="swatch"
              aria-pressed={theme.accent === k}
              style={{ "--c": ACCENTS[k].l[0] } as React.CSSProperties}
              onClick={() => setTheme({ accent: k })}
            >
              <i>
                {theme.accent === k && (
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="3">
                    <path d="m5 12 5 5 9-10" />
                  </svg>
                )}
              </i>
              {ACCENTS[k].name}
            </button>
          ))}
        </div>
      </div>
      <div>
        <h3>Background image</h3>
        <div className="bgs">
          {BGS.map(([k, label]) =>
            k === "photo" ? (
              <label
                key={k}
                className="bgopt"
                aria-pressed={theme.bg === "photo"}
                onClick={() => theme.photo && setTheme({ bg: "photo", vis: Math.min(theme.vis, 60) })}
              >
                <div className="th" style={theme.photo ? { backgroundImage: `url('${theme.photo}')` } : undefined}>
                  {!theme.photo && (
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M12 5v14M5 12h14" />
                    </svg>
                  )}
                  <input
                    type="file"
                    accept="image/*"
                    aria-label="Upload a background photo"
                    onChange={onPhoto}
                    hidden={!!theme.photo && theme.bg !== "photo"}
                  />
                </div>
                <span className="l">{theme.photo ? "Your photo · change" : "Upload photo"}</span>
              </label>
            ) : (
              <button key={k} className="bgopt" aria-pressed={theme.bg === k} onClick={() => setTheme({ bg: k })}>
                <span className={`th ${k !== "none" ? `bg-${k}` : ""}`}>{k === "none" ? "—" : ""}</span>
                <span className="l">{label}</span>
              </button>
            ),
          )}
        </div>
      </div>
      {theme.bg !== "none" && (
        <>
          <div className="slider">
            <label htmlFor="dim">
              <span>Image visibility</span>
              <span className="num">{theme.vis}%</span>
            </label>
            <input id="dim" type="range" min={0} max={90} step={5} value={theme.vis} onChange={(e) => setTheme({ vis: +e.target.value })} />
          </div>
          <div className="slider">
            <label htmlFor="blur">
              <span>Blur</span>
              <span className="num">{theme.blur} px</span>
            </label>
            <input id="blur" type="range" min={0} max={24} step={2} value={theme.blur} onChange={(e) => setTheme({ blur: +e.target.value })} />
          </div>
          <div className="slider">
            <label htmlFor="panel">
              <span>Panel opacity</span>
              <span className="num">{theme.panel}%</span>
            </label>
            <input id="panel" type="range" min={0} max={100} step={5} value={theme.panel} onChange={(e) => setTheme({ panel: +e.target.value })} />
          </div>
        </>
      )}
      <button
        className="btn reset"
        onClick={() => {
          setTheme({ ...THEME_DEFAULT, photo: theme.photo });
          toast("Theme reset to Mint");
        }}
      >
        Reset to default
      </button>
    </div>
  );
}
