import { Suspense, useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { Logo, Svg } from "@/components/Icons/Svg";
import { ErrorBoundary } from "@/components/PageQuery/ErrorBoundary";
import { Sidebar } from "@/components/Sidebar/Sidebar";
import { StatusBar } from "@/components/StatusBar/StatusBar";
import { ThemePanel } from "@/components/Theme/ThemePanel";
import { useLogout } from "@/features/auth/mutations";
import { useStats } from "@/features/torrents/queries";
import { LegalNotice, shouldShowLegal } from "@/pages/auth/LegalNotice";
import { formatRateZero } from "@/utils/format";

function Rates() {
  const { data } = useStats();
  return (
    <>
      <span className="rate d" title="Total download speed">↓ <b className="num">{formatRateZero(data.totalDl)}</b></span>
      <span className="rate u" title="Total upload speed">↑ <b className="num">{formatRateZero(data.totalUl)}</b></span>
    </>
  );
}

export function Layout() {
  const logout = useLogout();
  const [themeOpen, setThemeOpen] = useState(false);
  const [legal, setLegal] = useState(false);

  useEffect(() => setLegal(shouldShowLegal()), []);

  return (
    <>
      <header className="top">
        <div className="brand">
          <Logo />
          Bitswarm
        </div>
        <div className="rates">
          <Suspense fallback={null}>
            <Rates />
          </Suspense>
          <button className="btn themebtn" id="themeBtn" aria-expanded={themeOpen} aria-controls="themePop" onClick={() => setThemeOpen((o) => !o)}>
            <Svg size={15}>
              <path d="M12 3a9 9 0 1 0 0 18c1.1 0 1.8-.8 1.8-1.7 0-.5-.2-.9-.5-1.2-.3-.3-.5-.7-.5-1.2 0-1 .8-1.7 1.8-1.7H17a4 4 0 0 0 4-4C21 6.6 17 3 12 3z" />
              <circle cx="7.5" cy="11" r="1.2" fill="currentColor" />
              <circle cx="10.5" cy="7" r="1.2" fill="currentColor" />
              <circle cx="15" cy="7.5" r="1.2" fill="currentColor" />
            </Svg>
            <span className="t">Theme</span>
          </button>
          <button className="btn themebtn" title="Sign out" onClick={() => logout.mutate()}>
            <Svg size={15}><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3" /></Svg>
            <span className="t">Sign out</span>
          </button>
        </div>
      </header>
      <ThemePanel open={themeOpen} onClose={() => setThemeOpen(false)} />

      <div className="app">
        <ErrorBoundary>
          <Suspense fallback={<aside />}>
            <Sidebar />
          </Suspense>
          <Outlet />
        </ErrorBoundary>
      </div>

      <Suspense fallback={<footer className="status" />}>
        <StatusBar />
      </Suspense>

      {legal && <LegalNotice onClose={() => setLegal(false)} />}
    </>
  );
}
