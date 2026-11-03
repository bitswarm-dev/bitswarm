import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";
import { Suspense, type ReactNode } from "react";
import { PageSkeleton } from "@/components/PageSkeleton/PageSkeleton";
import { useAuthStatus } from "@/features/auth/queries";
import { SignIn } from "@/pages/auth/SignIn";
import { Hooks } from "@/pages/hooks/Hooks";
import { HooksGate } from "@/pages/hooks/HooksGate";
import { Maintenance } from "@/pages/maintenance/Maintenance";
import { Tokens } from "@/pages/tokens/Tokens";
import { About } from "@/pages/about/About";
import { Log } from "@/pages/log/Log";
import { NotBuilt } from "@/pages/NotBuilt";
import { Schedules } from "@/pages/schedules/Schedules";
import { Renames } from "@/pages/renames/Renames";
import { Settings } from "@/pages/settings/Settings";
import { Torrents } from "@/pages/torrents/Torrents";
import { TorrentsSkeleton } from "@/pages/torrents/TorrentsSkeleton";
import { Layout } from "./Layout";

const inPage = (page: ReactNode, fallback: ReactNode = <PageSkeleton />) => <Suspense fallback={fallback}>{page}</Suspense>;

function Root() {
  const { data: status } = useAuthStatus();
  return status.signedIn ? <Layout /> : <SignIn />;
}

const rootRoute = createRootRoute({
  component: Root,
  notFoundComponent: () => inPage(<NotBuilt title="Not found" blurb="That page doesn’t exist." />),
});

const routeTree = rootRoute.addChildren([
  createRoute({ getParentRoute: () => rootRoute, path: "/", component: () => inPage(<Torrents />, <TorrentsSkeleton />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/schedules", component: () => inPage(<Schedules />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/renames", component: () => inPage(<Renames />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/maint", component: () => inPage(<Maintenance />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/hooks", component: () => <HooksGate>{inPage(<Hooks />)}</HooksGate> }),
  createRoute({ getParentRoute: () => rootRoute, path: "/log", component: () => inPage(<Log />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/tokens", component: () => inPage(<Tokens />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/settings", component: () => inPage(<Settings />) }),
  createRoute({ getParentRoute: () => rootRoute, path: "/about", component: () => inPage(<About />) }),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
