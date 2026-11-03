import { createFileRoute } from "@tanstack/react-router";
import { logQuery } from "@/features/log/queries";
import { levelsOf, parseLogSearch } from "@/pages/log/logSearch";
import { Log } from "@/pages/log/Log";

export const Route = createFileRoute("/log")({
  validateSearch: parseLogSearch,
  loaderDeps: ({ search }) => ({ levels: levelsOf(search), src: search.source ?? "", q: search.text ?? "" }),
  loader: ({ context, deps }) => (context.signedIn ? context.queryClient.ensureQueryData(logQuery(deps)) : undefined),
  component: Log,
});
