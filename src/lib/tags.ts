const TAG_ALIASES: Record<string, string> = {
  tech: "Technology",
  technology: "Technology",
  ai: "Artificial Intelligence",
  "artificial intelligence": "Artificial Intelligence",
  cybersecurity: "Cyber Security",
  "cyber security": "Cyber Security",
  "operating system": "Operating System"
};

const EMPTY_TAG_FALLBACK = "General";

function toTitleCase(value: string): string {
  return value
    .split(" ")
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

export function normalizeTag(rawTag: string): string {
  const compact = rawTag.trim().replace(/\s+/g, " ");
  if (!compact) {
    return "";
  }

  const key = compact.toLowerCase();
  if (TAG_ALIASES[key]) {
    return TAG_ALIASES[key];
  }

  return toTitleCase(key);
}

export function normalizeTags(rawTags: string[]): string[] {
  const seen = new Set<string>();
  const normalizedTags: string[] = [];

  for (const rawTag of rawTags) {
    const normalizedTag = normalizeTag(rawTag);
    if (!normalizedTag || seen.has(normalizedTag)) {
      continue;
    }

    seen.add(normalizedTag);
    normalizedTags.push(normalizedTag);
  }

  if (normalizedTags.length === 0) {
    return [EMPTY_TAG_FALLBACK];
  }

  return normalizedTags;
}
