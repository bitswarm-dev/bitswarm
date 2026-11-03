import { useNavigate } from "@tanstack/react-router";

export type AppPath = "/" | "/schedules" | "/renames" | "/maint" | "/hooks" | "/log" | "/tokens" | "/settings" | "/about";

export function useNav() {
  const navigate = useNavigate();
  return (to: AppPath) => void navigate({ to });
}
