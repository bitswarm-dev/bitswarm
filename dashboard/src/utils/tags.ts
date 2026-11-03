export function parseTags(input: string): string[] {
  const seen = new Set<string>();
  for (const part of input.split(",")) {
    const tag = part.trim().replace(/^#/, "").replace(/\s+/g, " ").slice(0, 40);
    if (tag) seen.add(tag);
  }
  return [...seen];
}
