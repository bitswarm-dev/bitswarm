export function safeArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

export function safeString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

export function safeNumber(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

export function safeBoolean(value: unknown, fallback = false): boolean {
  return typeof value === "boolean" ? value : fallback;
}

export function safeObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

export function pickKnown<T extends object>(template: T, source: object | null): T {
  if (!source) return { ...template };
  const src = source as Record<string, unknown>;
  const out = { ...template } as Record<string, unknown>;
  for (const key of Object.keys(template)) {
    if (src[key] !== undefined) out[key] = src[key];
  }
  return out as T;
}
