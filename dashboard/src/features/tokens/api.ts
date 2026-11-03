import { del, post, request } from "@/lib/http";
import { safeArray } from "@/utils/safe";
import type { TokenView } from "./types";

export async function listTokens(): Promise<TokenView[]> {
  return safeArray(await request<TokenView[] | null>("/tokens"));
}

export const createToken = (v: { name: string; scope: string; days: number }) => post<{ token: string; view: TokenView }>("/tokens", v);
export const revokeToken = (id: number) => del<void>(`/tokens/${id}`);
