import { createRouter, defaultStringifySearch } from "@tanstack/react-router";
import { PageSkeleton } from "@/components/PageSkeleton/PageSkeleton";
import { RouteError } from "@/components/PageQuery/RouteError";
import { NotFound } from "@/pages/NotFound";
import { queryClient } from "@/app/queryClient";
import { routeTree } from "./routeTree.gen";

export const router = createRouter({
  routeTree,
  context: { queryClient, signedIn: false },
  defaultPreload: "intent",
  defaultPreloadDelay: 60,
  defaultPreloadStaleTime: 0,
  defaultPendingMs: 200,
  defaultPendingMinMs: 300,
  defaultPendingComponent: PageSkeleton,
  defaultErrorComponent: RouteError,
  defaultNotFoundComponent: NotFound,
  stringifySearch: (search) => defaultStringifySearch(search).replaceAll("%2C", ","),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
