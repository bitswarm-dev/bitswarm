import { post, request } from "@/lib/http";

export interface AuthStatus {
  signedIn: boolean;
  demo: boolean;
  user: string;
  localBypass: boolean;
}

export const getAuthStatus = () => request<AuthStatus>("/auth/status");

export const login = (body: { user: string; pass: string; keep: boolean }) =>
  post<{ ok: boolean }>("/auth/login", body);

export const logout = () => post<void>("/auth/logout");
