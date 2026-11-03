import { useEffect, useState, type FormEvent } from "react";
import { VERSION } from "@/app/version";
import { Logo } from "@/components/Icons/Svg";
import { useAuthStatus } from "@/features/auth/queries";
import { useLogin } from "@/features/auth/mutations";
import { ApiError } from "@/lib/http";
import { safeNumber } from "@/utils/safe";

export function SignIn() {
  const { data: status } = useAuthStatus();
  const login = useLogin();
  const [user, setUser] = useState(status.user);
  const [pass, setPass] = useState("");
  const [show, setShow] = useState(false);
  const [keep, setKeep] = useState(false);
  const [error, setError] = useState("");
  const [locked, setLocked] = useState(0);

  useEffect(() => {
    if (locked <= 0) return;
    const t = setTimeout(() => setLocked((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [locked]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!user.trim()) return setError("Enter your username.");
    if (!pass) return setError("Enter your password.");
    setError("");
    login.mutate(
      { user, pass, keep },
      {
        onError: (err) => {
          if (err instanceof ApiError) {
            setError(err.message);
            setLocked(safeNumber(err.details.lockedFor) > 1 && err.status === 429 ? safeNumber(err.details.lockedFor) : 0);
          } else setError("Can’t reach the Bitswarm server.");
          setPass("");
        },
      },
    );
  };

  const disabled = locked > 0 || login.isPending;

  return (
    <div className="login">
      <form className="login-card" onSubmit={submit} noValidate aria-labelledby="lgTitle">
        <div className="brand">
          <Logo size={28} />
          Bitswarm
        </div>
        <div>
          <h1 id="lgTitle">Sign in</h1>
          <p className="sub">Sign in to manage your torrents.</p>
        </div>
        <label className="field">
          <span>Username</span>
          <input type="text" value={user} onChange={(e) => setUser(e.target.value)} autoComplete="username" autoCapitalize="none" spellCheck={false} disabled={disabled} />
        </label>
        <label className="field">
          <span>Password</span>
          <span className="pw">
            <input type={show ? "text" : "password"} value={pass} onChange={(e) => setPass(e.target.value)} autoComplete="current-password" disabled={disabled} autoFocus />
            <button type="button" className="btn" aria-pressed={show} aria-label={show ? "Hide password" : "Show password"} onClick={() => setShow(!show)}>
              {show ? "Hide" : "Show"}
            </button>
          </span>
        </label>
        <label className="check">
          <input type="checkbox" checked={keep} onChange={(e) => setKeep(e.target.checked)} /> Keep me signed in on this browser
        </label>
        {(error || locked > 0) && (
          <p className="err-msg" role="alert">
            {locked > 0 ? `Too many tries. Try again in ${locked} second${locked === 1 ? "" : "s"}.` : error}
          </p>
        )}
        <button type="submit" className="btn primary lg-go" disabled={disabled}>
          Sign in
        </button>
        {status.demo && (
          <p className="lg-hint">
            Demo sign-in: <b>{status.user}</b> / <b>swarm</b>. Change the password in Settings → Web UI.
          </p>
        )}
      </form>
      <p className="lg-foot">Bitswarm {VERSION} · The server checks your password.</p>
    </div>
  );
}
