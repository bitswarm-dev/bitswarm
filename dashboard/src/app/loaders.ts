import type { QueryClient, QueryKey } from "@tanstack/react-query";
import type { RouterContext } from "./routerContext";

type Prefetchable = { queryKey: QueryKey; queryFn?: unknown };

export const prefetch =
  (...queries: Prefetchable[]) =>
  ({ context }: { context: RouterContext }) =>
    context.signedIn ? prefetchAll(context.queryClient, queries) : undefined;

export function prefetchAll(client: QueryClient, queries: Prefetchable[]) {
  return Promise.all(queries.map((q) => client.ensureQueryData(q as never)));
}
