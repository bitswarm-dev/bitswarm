import { useState, type FormEvent } from "react";
import { Modal } from "@/components/Modal/Modal";
import { readFlag, writeFlag } from "@/app/storage";

const SHOWN = "swarm-legal-shown";

export function shouldShowLegal(): boolean {
  if (readFlag("swarm-legal-ack")) return false;
  try {
    return sessionStorage.getItem(SHOWN) !== "1";
  } catch {
    return true;
  }
}

export function LegalNotice({ onClose }: { onClose: () => void }) {
  const [never, setNever] = useState(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (never) writeFlag("swarm-legal-ack", true);
    try {
      sessionStorage.setItem(SHOWN, "1");
    } catch {
      /* private mode: show again next time */
    }
    onClose();
  };

  return (
    <Modal
      title="Use BitSwarm legally"
      alert
      width="520px"
      onClose={() => undefined}
      head={
        <header className="ack-head">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" strokeWidth="2" strokeLinejoin="round">
            <path d="M12 3 4 6v6c0 4.5 3.4 8.3 8 9 4.6-.7 8-4.5 8-9V6z" />
            <path d="m9 12 2 2 4-4" />
          </svg>
          <h2>Use BitSwarm legally</h2>
        </header>
      }
    >
      <form onSubmit={submit} noValidate>
        <div className="ack">
          <p>BitSwarm is a general-purpose BitTorrent client. It isn’t built for piracy, and it doesn’t provide, host or index any content.</p>
          <p>
            Only download and share files you have the right to, such as open-source software, public-domain works and content released under free licenses. Copyright laws differ by country, and
            you’re responsible for following the laws where you live.
          </p>
          <p className="ack-legal">
            <b>The author of BitSwarm isn’t liable</b> for how you use it, or for any content you download or share with it.
          </p>
        </div>
        <label className="check">
          <input type="checkbox" checked={never} onChange={(e) => setNever(e.target.checked)} /> Don’t show this again
        </label>
        <div className="actions">
          <button type="submit" className="btn primary" autoFocus>
            I understand
          </button>
        </div>
      </form>
    </Modal>
  );
}
