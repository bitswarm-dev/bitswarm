import { Link } from "@tanstack/react-router";

export function NotFound() {
  return (
    <section className="sched" aria-label="Not found">
      <div className="sched-head">
        <div>
          <h2>Not found</h2>
          <p>That page doesn’t exist. It may have moved, or the address may be mistyped.</p>
        </div>
      </div>
      <p className="z">
        <Link to="/">Back to torrents</Link>
      </p>
    </section>
  );
}
