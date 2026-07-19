import { createHighlighter, type Highlighter } from "shiki";

const THEME = "github-dark-dimmed";

const BUNDLED_LANGS = [
  "javascript",
  "typescript",
  "python",
  "bash",
  "c",
  "cpp",
  "csharp",
  "css",
  "html",
  "json",
  "haxe",
  "go",
  "rust",
  "java",
  "sql",
  "yaml",
  "markdown",
  "jsx",
  "tsx",
  "php",
  "ruby",
  "proto"
];

const LANGUAGE_ALIASES: Record<string, string> = {
  "c#": "csharp",
  "c++": "cpp",
  sh: "bash",
  shell: "bash",
  yml: "yaml",
  md: "markdown",
  protobuf: "proto",
  protocolbuffers: "proto",
  "protocol buffers": "proto"
};

let highlighterPromise: Promise<Highlighter> | null = null;

function getHighlighter(): Promise<Highlighter> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighter({ themes: [THEME], langs: BUNDLED_LANGS });
  }
  return highlighterPromise;
}

function normalizeLanguage(rawLanguage: string): string | undefined {
  const normalized = rawLanguage.trim().toLowerCase();
  if (!normalized || normalized === "plain text" || normalized === "text" || normalized === "plaintext") {
    return undefined;
  }

  const aliased = LANGUAGE_ALIASES[normalized] || normalized;
  return BUNDLED_LANGS.includes(aliased) ? aliased : undefined;
}

// Overrides Notion's reported language when its dropdown lacks one (e.g. Haxe).
const LANGUAGE_OVERRIDE_PATTERN = /^@lang:([a-zA-Z0-9+#._-]+)[ \t]*\r?\n/;

export function applyLanguageOverride(code: string, language: string): { code: string; language: string } {
  const match = LANGUAGE_OVERRIDE_PATTERN.exec(code);
  if (!match) {
    return { code, language };
  }

  return { code: code.slice(match[0].length), language: match[1] };
}

export async function highlightCode(code: string, rawLanguage: string): Promise<string | undefined> {
  const lang = normalizeLanguage(rawLanguage);
  if (!lang) {
    return undefined;
  }

  try {
    const highlighter = await getHighlighter();
    return highlighter.codeToHtml(code, { lang, theme: THEME });
  } catch {
    return undefined;
  }
}
