export function NotBuilt({ title, blurb }: { title: string; blurb: string }) {
  return (
    <section className="sched" aria-label={title}>
      <div className="sched-head">
        <div>
          <h2>{title}</h2>
          <p>{blurb}</p>
        </div>
      </div>
      <div className="note">
        <span>This screen isn’t built in the new dashboard yet. The prototype in <code>draft.html</code> still has the full version.</span>
      </div>
    </section>
  );
}
