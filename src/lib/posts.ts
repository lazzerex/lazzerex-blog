import { MarkdownContentParser, extractExcerpt, type RichContentBlock } from "./content-parser";
import { fetchNotionPublishedPosts, hasNotionConfig, type NotionPost } from "./notion";
import { normalizeTags } from "./tags";
import { resolveLocalCoverBySlug, resolvePublishedNotionUrl } from "../data/posts";

const SLUG_PATTERN = /^[a-z0-9-]+$/;
const DEFAULT_SUMMARY = "Summary will be available soon.";
const DEFAULT_COVER = "/images/folder-bg.jfif";
const DEFAULT_AUTHOR = "H. S. N. Bình";
const runtimeApiBaseUrl = String(import.meta.env.PUBLIC_GO_API_BASE_URL || "")
  .trim()
  .replace(/\/+$/, "");
const publishSyncSecret = String(import.meta.env.GO_API_PUBLISH_SYNC_SECRET || "").trim();

const dateFormatter = new Intl.DateTimeFormat("en-US", {
  month: "long",
  day: "numeric",
  year: "numeric"
});

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

function mapNotionToReaderPost(post: NotionPost, parser: MarkdownContentParser): ReaderPost {
  const tags = normalizeTags(post.tags);
  const inferredSummary = extractExcerpt(post.content, 180);
  const summary = post.summary?.trim() || inferredSummary || DEFAULT_SUMMARY;
  const rawCover = post.cover?.trim() || "";
  const localCoverFallback = resolveLocalCoverBySlug(post.slug);
  const isExpiringNotionAsset =
    /^https?:\/\/prod-files-secure\.s3\.[^\s]+/i.test(rawCover) &&
    /[?&]X-Amz-Expires=/i.test(rawCover);
  const cover =
    (isExpiringNotionAsset ? localCoverFallback : undefined) ||
    rawCover ||
    localCoverFallback ||
    DEFAULT_COVER;
  const author = post.author?.trim() || DEFAULT_AUTHOR;
  const notionUrl = resolvePublishedNotionUrl(post.title, post.notionUrl);

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
    blocks: post.blocks.length > 0 ? post.blocks : parser.parse(post.content)
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
  const posts = notionPosts.map((post) => mapNotionToReaderPost(post, parser));

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
