export function readJson<T>(key: string, fallback: T, store: "local" | "session" = "local"): T {
  try {
    const raw = (store === "local" ? localStorage : sessionStorage).getItem(key);
    return raw ? ({ ...fallback, ...JSON.parse(raw) } as T) : fallback;
  } catch {
    return fallback;
  }
}

export function writeJson(key: string, value: unknown, store: "local" | "session" = "local"): boolean {
  try {
    (store === "local" ? localStorage : sessionStorage).setItem(key, JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}

export function readFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === "1";
  } catch {
    return false;
  }
}

export function writeFlag(key: string, on: boolean): void {
  try {
    if (on) localStorage.setItem(key, "1");
    else localStorage.removeItem(key);
  } catch {
    return;
  }
}
