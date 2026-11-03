import { createRootRouteWithContext } from "@tanstack/react-router";
import { Layout } from "@/app/Layout";
import { prefetchAll } from "@/app/loaders";
import type { RouterContext } from "@/app/routerContext";
import { authQuery, useAuthStatus } from "@/features/auth/queries";
import { catalogQuery } from "@/features/catalog/queries";
import { hooksQuery } from "@/features/hooks/queries";
import { logQuery } from "@/features/log/queries";
import { tasksQuery } from "@/features/maintenance/queries";
import { renamesQuery } from "@/features/renames/queries";
import { schedulesQuery } from "@/features/schedules/queries";
import { tokensQuery } from "@/features/tokens/queries";
import { statsQuery, torrentsQuery } from "@/features/torrents/queries";
import { updatesQuery } from "@/features/updates/queries";
import { SignIn } from "@/pages/auth/SignIn";

function Root() {
  const { data: status } = useAuthStatus();
  return status.signedIn ? <Layout /> : <SignIn />;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async ({ context }) => {
    const status = await context.queryClient.ensureQueryData(authQuery);
    return { signedIn: status.signedIn };
  },
  loader: ({ context }) =>
    context.signedIn
      ? prefetchAll(context.queryClient, [statsQuery, torrentsQuery, catalogQuery, renamesQuery, schedulesQuery, tasksQuery, hooksQuery, tokensQuery, updatesQuery, logQuery({ levels: ["error"], src: "", q: "" })])
      : undefined,
  pendingComponent: () => null,
  component: Root,
});
