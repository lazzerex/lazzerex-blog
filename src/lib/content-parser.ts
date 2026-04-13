export type RichContentBlock =
  | {
      type: "heading";
      level: 1 | 2 | 3 | 4 | 5 | 6;
      text: string;
    }
  | {
      type: "paragraph";
      text: string;
    }
  | {
      type: "list-item";
      text: string;
      ordered: boolean;
    }
  | {
      type: "quote";
      text: string;
    }
  | {
      type: "code";
      language: string;
      code: string;
    }
  | {
      type: "image";
      alt: string;
      src: string;
    }
  | {
      type: "divider";
    };

export interface RichContentParser<Input> {
  parse(input: Input): RichContentBlock[];
}

const FRONTMATTER_PATTERN = /^---\s*[\s\S]*?\s*---\s*/;

function stripFrontmatter(markdown: string): string {
  return markdown.replace(FRONTMATTER_PATTERN, "");
}

export function extractExcerpt(markdown: string, maxLength = 180): string {
  const plainText = stripFrontmatter(markdown)
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/!\[[^\]]*\]\([^)]*\)/g, " ")
    .replace(/\[[^\]]+\]\([^)]*\)/g, (match) => {
      const labelMatch = /^\[([^\]]+)\]/.exec(match);
      return labelMatch ? ` ${labelMatch[1]} ` : " ";
    })
    .replace(/[>#*_`~-]/g, " ")
    .replace(/\s+/g, " ")
    .trim();

  if (!plainText) {
    return "";
  }

  if (plainText.length <= maxLength) {
    return plainText;
  }

  return `${plainText.slice(0, maxLength).trimEnd()}...`;
}

export function parseMarkdownBlocks(markdown: string): RichContentBlock[] {
  const blocks: RichContentBlock[] = [];
  const lines = stripFrontmatter(markdown).split(/\r?\n/);

  let paragraphBuffer: string[] = [];
  let inCodeBlock = false;
  let codeLanguage = "";
  let codeBuffer: string[] = [];

  const flushParagraph = () => {
    if (paragraphBuffer.length === 0) {
      return;
    }

    blocks.push({
      type: "paragraph",
      text: paragraphBuffer.join(" ").replace(/\s+/g, " ").trim()
    });

    paragraphBuffer = [];
  };

  for (const line of lines) {
    const trimmed = line.trim();

    if (inCodeBlock) {
      if (trimmed.startsWith("```")) {
        blocks.push({
          type: "code",
          language: codeLanguage,
          code: codeBuffer.join("\n")
        });

        inCodeBlock = false;
        codeLanguage = "";
        codeBuffer = [];
      } else {
        codeBuffer.push(line);
      }

      continue;
    }

    if (!trimmed) {
      flushParagraph();
      continue;
    }

    if (trimmed.startsWith("```")) {
      flushParagraph();
      inCodeBlock = true;
      codeLanguage = trimmed.slice(3).trim();
      continue;
    }

    const headingMatch = /^(#{1,6})\s+(.+)$/.exec(trimmed);
    if (headingMatch) {
      flushParagraph();
      blocks.push({
        type: "heading",
        level: headingMatch[1].length as 1 | 2 | 3 | 4 | 5 | 6,
        text: headingMatch[2].trim()
      });
      continue;
    }

    const quoteMatch = /^>\s?(.*)$/.exec(trimmed);
    if (quoteMatch) {
      flushParagraph();
      blocks.push({
        type: "quote",
        text: quoteMatch[1].trim()
      });
      continue;
    }

    if (/^(\*|-|\+)\s+.+$/.test(trimmed) || /^\d+\.\s+.+$/.test(trimmed)) {
      flushParagraph();
      blocks.push({
        type: "list-item",
        text: trimmed.replace(/^(\*|-|\+|\d+\.)\s+/, "").trim(),
        ordered: /^\d+\./.test(trimmed)
      });
      continue;
    }

    if (/^(-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
      flushParagraph();
      blocks.push({ type: "divider" });
      continue;
    }

    const imageMatch = /^!\[([^\]]*)\]\(([^)]+)\)$/.exec(trimmed);
    if (imageMatch) {
      flushParagraph();
      blocks.push({
        type: "image",
        alt: imageMatch[1].trim(),
        src: imageMatch[2].trim()
      });
      continue;
    }

    paragraphBuffer.push(trimmed);
  }

  flushParagraph();

  if (inCodeBlock) {
    blocks.push({
      type: "code",
      language: codeLanguage,
      code: codeBuffer.join("\n")
    });
  }

  return blocks;
}

export class MarkdownContentParser implements RichContentParser<string> {
  parse(input: string): RichContentBlock[] {
    return parseMarkdownBlocks(input);
  }
}
