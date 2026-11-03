import { put, request } from "@/lib/http";
import type { Settings } from "./types";

export const getSettings = () => request<Settings>("/settings");

export const getDefaults = () => request<Settings>("/settings/defaults");

export const saveSettings = (s: Settings) => put<Settings>("/settings", s);
