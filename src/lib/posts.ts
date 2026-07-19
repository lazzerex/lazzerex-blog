import { writeFile, mkdir, access } from "node:fs/promises";
import { join, dirname } from "node:path";
import sharp from "sharp";
import { MarkdownContentParser, extractExcerpt, type RichContentBlock } from "./content-parser";
import { highlightCode, applyLanguageOverride } from "./code-highlight";
import { fetchNotionPublishedPosts, hasNotionConfig, type NotionPost } from "./notion";
import { normalizeTags } from "./tags";
import { resolveLocalCoverBySlug, resolvePublishedNotionUrl } from "../data/posts";

const SLUG_PATTERN = /^[a-z0-9-]+$/;
const DEFAULT_SUMMARY = "Summary will be available soon.";
const DEFAULT_COVER = "/images/folder-bg.jfif";
const DEFAULT_AUTHOR = "H. S. N. Bình";
const NOTION_COVERS_PUBLIC_DIR = join(process.cwd(), "public", "images", "notion-covers");
const NOTION_COVERS_DIST_DIR = join(process.cwd(), "dist", "images", "notion-covers");
const NOTION_COVERS_WEB_PATH = "/images/notion-covers";
const NOTION_CONTENT_PUBLIC_DIR = join(process.cwd(), "public", "images", "notion-content");
const NOTION_CONTENT_DIST_DIR = join(process.cwd(), "dist", "images", "notion-content");
const NOTION_CONTENT_WEB_PATH = "/images/notion-content";
const COVER_IMAGE_MAX_WIDTH = 1600;
const CONTENT_IMAGE_MAX_WIDTH = 1400;
const WEBP_QUALITY = 80;

interface OptimizedImage {
  path: string;
  width: number;
  height: number;
}

const MAX_CONCURRENT_IMAGE_DOWNLOADS = 6;
const IMAGE_DOWNLOAD_ATTEMPTS = 2;

function createLimiter(maxConcurrent: number) {
  let active = 0;
  const queue: (() => void)[] = [];

  return function limit<T>(fn: () => Promise<T>): Promise<T> {
    return new Promise((resolve, reject) => {
      const run = () => {
        active += 1;
        fn()
          .then(resolve, reject)
          .finally(() => {
            active -= 1;
            queue.shift()?.();
          });
      };

      if (active < maxConcurrent) {
        run();
      } else {
        queue.push(run);
      }
    });
  };
}

const limitImageDownload = createLimiter(MAX_CONCURRENT_IMAGE_DOWNLOADS);

async function downloadNotionAsset(
  publicPath: string,
  distPath: string,
  webPath: string,
  url: string,
  maxWidth: number
): Promise<OptimizedImage | undefined> {
  try {
    await access(publicPath);
    const metadata = await sharp(publicPath).metadata();
    return { path: webPath, width: metadata.width ?? 0, height: metadata.height ?? 0 };
  } catch {
    // not cached yet
  }

  return limitImageDownload(async () => {
    for (let attempt = 1; attempt <= IMAGE_DOWNLOAD_ATTEMPTS; attempt += 1) {
      try {
        const response = await fetch(url);
        if (!response.ok) {
          continue;
        }
        const original = Buffer.from(await response.arrayBuffer());
        const { data, info } = await sharp(original)
          .resize({ width: maxWidth, withoutEnlargement: true })
          .webp({ quality: WEBP_QUALITY })
          .toBuffer({ resolveWithObject: true });

        await mkdir(dirname(publicPath), { recursive: true });
        await writeFile(publicPath, data);
        // Astro copies public/ to dist/ before page generation runs in prod builds,
        // so also write directly to dist/ to ensure the image is in the final output.
        if (!import.meta.env.DEV) {
          await mkdir(dirname(distPath), { recursive: true });
          await writeFile(distPath, data);
        }
        return { path: webPath, width: info.width, height: info.height };
      } catch {
        // retry
      }
    }

    console.warn(`[image-pipeline] Failed to download/optimize asset after ${IMAGE_DOWNLOAD_ATTEMPTS} attempts: ${url.split("?")[0]}`);
    return undefined;
  });
}

async function downloadNotionCover(slug: string, url: string): Promise<string | undefined> {
  const filename = `${slug}.webp`;
  const result = await downloadNotionAsset(
    join(NOTION_COVERS_PUBLIC_DIR, filename),
    join(NOTION_COVERS_DIST_DIR, filename),
    `${NOTION_COVERS_WEB_PATH}/${filename}`,
    url,
    COVER_IMAGE_MAX_WIDTH
  );
  return result?.path;
}

async function downloadNotionContentImage(slug: string, index: number, url: string): Promise<OptimizedImage | undefined> {
  const filename = `${slug}-${index}.webp`;
  return downloadNotionAsset(
    join(NOTION_CONTENT_PUBLIC_DIR, filename),
    join(NOTION_CONTENT_DIST_DIR, filename),
    `${NOTION_CONTENT_WEB_PATH}/${filename}`,
    url,
    CONTENT_IMAGE_MAX_WIDTH
  );
}
const runtimeApiBaseUrl = String(import.meta.env.PUBLIC_GO_API_BASE_URL || "")
  .trim()
  .replace(/\/+$/, "");
const publishSyncSecret = String(import.meta.env.GO_API_PUBLISH_SYNC_SECRET || "").trim();

const dateFormatter = new Intl.DateTimeFormat("en-US", {
  month: "long",
  day: "numeric",
  year: "numeric"
});

const READING_WORDS_PER_MINUTE = 220;

function estimateReadTimeMinutes(blocks: RichContentBlock[]): number {
  const wordCount = blocks.reduce((total, block) => {
    if (
      block.type === "paragraph" ||
      block.type === "heading" ||
      block.type === "quote" ||
      block.type === "list-item"
    ) {
      return total + block.text.trim().split(/\s+/).filter(Boolean).length;
    }
    return total;
  }, 0);

  return Math.max(1, Math.round(wordCount / READING_WORDS_PER_MINUTE));
}

export interface ExplorerPost {
  title: string;
  slug: string;
  date: Date;
  dateLabel: string;
  author: string;
  tags: string[];
  primaryTag: string;
  summary: string;
  cover: string;
  notionUrl?: string;
}

export interface ReaderPost extends ExplorerPost {
  content: string;
  blocks: RichContentBlock[];
  readTimeMinutes: number;
}

let cachedPosts: ReaderPost[] | null = null;
let publishedPostsSyncAttempted = false;

function validateReaderPostSlugs(posts: ReaderPost[], source: string): void {
  const errors: string[] = [];
  const seenSlugs = new Map<string, string>();

  for (const post of posts) {
    const slug = post.slug.trim();

    if (!SLUG_PATTERN.test(slug)) {
      errors.push(`Invalid slug "${slug}" from ${source} (${post.title}).`);
      continue;
    }

    const existingTitle = seenSlugs.get(slug);
    if (existingTitle) {
      errors.push(`Slug collision for "${slug}" between ${existingTitle} and ${post.title}.`);
      continue;
    }

    seenSlugs.set(slug, post.title);
  }

  if (errors.length > 0) {
    throw new Error(`[Phase 3] Post slug validation failed:\n${errors.join("\n")}`);
  }
}

async function mapNotionToReaderPost(post: NotionPost, parser: MarkdownContentParser): Promise<ReaderPost> {
  const tags = normalizeTags(post.tags);
  const inferredSummary = extractExcerpt(post.content, 180);
  const summary = post.summary?.trim() || inferredSummary || DEFAULT_SUMMARY;
  const rawCover = post.cover?.trim() || "";
  const localCoverFallback = resolveLocalCoverBySlug(post.slug);
  const isExpiringNotionAsset =
    /^https?:\/\/prod-files-secure\.s3\.[^\s]+/i.test(rawCover) &&
    /[?&]X-Amz-Expires=/i.test(rawCover);

  let cover: string;
  if (isExpiringNotionAsset) {
    const downloaded = await downloadNotionCover(post.slug, rawCover);
    cover = downloaded || localCoverFallback || DEFAULT_COVER;
  } else {
    cover = rawCover || localCoverFallback || DEFAULT_COVER;
  }

  const author = post.author?.trim() || DEFAULT_AUTHOR;
  const notionUrl = resolvePublishedNotionUrl(post.title, post.notionUrl);

  const rawBlocks = post.blocks.length > 0 ? post.blocks : parser.parse(post.content);
  const blocks = await Promise.all(
    rawBlocks.map(async (block, index) => {
      if (block.type === "code") {
        const { code, language } = applyLanguageOverride(block.code, block.language);
        const html = await highlightCode(code, language);
        return { ...block, code, language, html };
      }

      if (block.type !== "image") return block;
      const isExpiring =
        /^https?:\/\/prod-files-secure\.s3\.[^\s]+/i.test(block.src) &&
        /[?&]X-Amz-Expires=/i.test(block.src);
      if (!isExpiring) return block;
      const downloaded = await downloadNotionContentImage(post.slug, index, block.src);
      return downloaded ? { ...block, src: downloaded.path, width: downloaded.width, height: downloaded.height } : block;
    })
  );

  return {
    title: post.title,
    slug: post.slug,
    date: post.date,
    dateLabel: dateFormatter.format(post.date),
    author,
    tags,
    primaryTag: tags[0],
    summary,
    cover,
    notionUrl,
    content: post.content,
    blocks,
    readTimeMinutes: estimateReadTimeMinutes(blocks)
  };
}

function assertNotionConfig(): void {
  if (hasNotionConfig()) {
    return;
  }

  throw new Error("[Phase 3] Missing Notion configuration. Set NOTION_TOKEN (or NOTION_API_KEY) and NOTION_DATABASE_ID.");
}

async function syncPublishedPosts(posts: ReaderPost[]): Promise<void> {
  if (publishedPostsSyncAttempted) {
    return;
  }

  publishedPostsSyncAttempted = true;

  if (!runtimeApiBaseUrl || posts.length === 0) {
    return;
  }

  const headers: Record<string, string> = {
    "Content-Type": "application/json"
  };

  if (publishSyncSecret) {
    headers["X-Lazzerex-Publish-Secret"] = publishSyncSecret;
  }

  for (const post of posts) {
    try {
      const response = await fetch(`${runtimeApiBaseUrl}/api/blogs/published`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          slug: post.slug,
          title: post.title,
          summary: post.summary
        })
      });

      if (!response.ok) {
        console.warn(
          `[Phase 6] Published-post sync returned ${response.status} for slug "${post.slug}".`
        );
      }
    } catch (error) {
      console.warn(`[Phase 6] Published-post sync failed for slug "${post.slug}".`, error);
    }
  }
}

export async function getAllPosts(forceRefresh = false): Promise<ReaderPost[]> {
  if (cachedPosts && !forceRefresh) {
    return cachedPosts;
  }

  assertNotionConfig();

  const parser = new MarkdownContentParser();
  const notionPosts = await fetchNotionPublishedPosts();
  const posts = await Promise.all(notionPosts.map((post) => mapNotionToReaderPost(post, parser)));

  validateReaderPostSlugs(posts, "Notion");

  posts.sort((a, b) => b.date.getTime() - a.date.getTime());

  await syncPublishedPosts(posts);

  cachedPosts = posts;
  return posts;
}

export async function getExplorerPosts(): Promise<ExplorerPost[]> {
  const posts = await getAllPosts();

  return posts.map(({ content, blocks, ...explorerPost }) => explorerPost);
}

export async function getPostBySlug(slug: string): Promise<ReaderPost | undefined> {
  const posts = await getAllPosts();
  return posts.find((post) => post.slug === slug);
}
