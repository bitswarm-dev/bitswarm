import { post, request } from "@/lib/http";
import type { UpdateState } from "./types";

export const getUpdates = () => request<UpdateState>("/updates");
export const checkUpdates = () => post<UpdateState>("/updates/check");
