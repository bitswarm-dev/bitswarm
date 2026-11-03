import { Suspense } from "react";
import { Route, Routes } from "react-router-dom";
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

const later = (title: string, blurb: string) => (
  <Suspense fallback={<PageSkeleton />}>
    <NotBuilt title={title} blurb={blurb} />
  </Suspense>
);

function Authed() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Suspense fallback={<TorrentsSkeleton />}><Torrents /></Suspense>} />
        <Route path="schedules" element={<Suspense fallback={<PageSkeleton />}><Schedules /></Suspense>} />
        <Route path="renames" element={<Suspense fallback={<PageSkeleton />}><Renames /></Suspense>} />
        <Route path="maint" element={<Suspense fallback={<PageSkeleton />}><Maintenance /></Suspense>} />
        <Route path="hooks" element={<HooksGate><Suspense fallback={<PageSkeleton />}><Hooks /></Suspense></HooksGate>} />
        <Route path="log" element={<Suspense fallback={<PageSkeleton />}><Log /></Suspense>} />
        <Route path="tokens" element={<Suspense fallback={<PageSkeleton />}><Tokens /></Suspense>} />
        <Route path="settings" element={<Suspense fallback={<PageSkeleton />}><Settings /></Suspense>} />
        <Route path="about" element={<Suspense fallback={<PageSkeleton />}><About /></Suspense>} />
        <Route path="*" element={later("Not found", "That page doesn’t exist.")} />
      </Route>
    </Routes>
  );
}

export function App() {
  const { data: status } = useAuthStatus();
  return status.signedIn ? <Authed /> : <SignIn />;
}
