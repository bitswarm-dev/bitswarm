import { useRouter, type ErrorComponentProps } from "@tanstack/react-router";

export function RouteError({ error, reset }: ErrorComponentProps) {
  const router = useRouter();
  const retry = () => {
    reset();
    void router.invalidate();
  };
  const message = error instanceof Error ? error.message : String(error);
  return (
    <div className="sched">
      <div className="note" role="alert">
        <span>
          <strong>Couldn’t load this page.</strong> {message}. Check that the BitSwarm server is running, then{" "}
          <button className="btn" onClick={retry}>
            Try again
          </button>
        </span>
      </div>
    </div>
  );
}
