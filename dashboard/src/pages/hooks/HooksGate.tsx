import { useState, type FormEvent, type ReactNode } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useUi } from "@/app/ui";
import { readFlag, writeFlag } from "@/app/storage";
import { Modal } from "@/components/Modal/Modal";

export function HooksGate({ children }: { children: ReactNode }) {
  const ui = useUi();
  const navigate = useNavigate();
  const [ok, setOk] = useState(false);
  const [never, setNever] = useState(false);
  const acked = ui.hooksAck || readFlag("swarm-hooks-ack");

  if (acked) return <>{children}</>;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!ok) return;
    if (never) writeFlag("swarm-hooks-ack", true);
    ui.patch({ hooksAck: true });
  };

  return (
    <Modal
      title="Hooks are for advanced users"
      alert
      width="540px"
      onClose={() => void navigate({ to: "/" })}
      head={
        <header className="ack-head">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--warn)" strokeWidth="2" strokeLinejoin="round">
            <path d="M12 3 2 20h20z" />
            <path d="M12 10v4M12 17h.01" />
          </svg>
          <h2>Hooks are for advanced users</h2>
        </header>
      }
    >
      <form onSubmit={submit} noValidate>
        <div className="ack">
          <p>Hooks act on their own every time an event happens, with no confirmation:</p>
          <ul>
            <li><b>Commands</b> run on the machine where Bitswarm runs, with Bitswarm’s permissions. A wrong command can delete or overwrite files.</li>
            <li><b>API calls</b> send torrent names, folders and other details to the address you enter.</li>
            <li>A hook that fires often can flood a server or start many programs at once.</li>
          </ul>
          <p>Only set up hooks you understand, and test them on unimportant data first.</p>
          <p className="ack-legal">
            <b>You use hooks at your own risk.</b> Bitswarm and its developers aren’t responsible for any damage, data loss or other harm caused by hooks you create or run.
          </p>
        </div>
        <label className="check"><input type="checkbox" checked={ok} onChange={(e) => setOk(e.target.checked)} /> I understand the risks and I’m responsible for the hooks I set up</label>
        <label className="check"><input type="checkbox" checked={never} onChange={(e) => setNever(e.target.checked)} /> Don’t ask me again</label>
        <div className="actions">
          <button type="button" className="btn" onClick={() => void navigate({ to: "/" })}>Go back</button>
          <button type="submit" className="btn primary" disabled={!ok}>Continue to hooks</button>
        </div>
      </form>
    </Modal>
  );
}
