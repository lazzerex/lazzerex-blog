import { Client } from "@notionhq/client";
import type {
  BlockObjectResponse,
  PageObjectResponse,
  PartialBlockObjectResponse,
  PartialPageObjectResponse,
  RichTextItemResponse
} from "@notionhq/client/build/src/api-endpoints";
import type { RichContentBlock } from "./content-parser";

export interface NotionPost {
  id: string;
  title: string;
  slug: string;
  date: Date;
  author?: string;
  tags: string[];
  summary?: string;
  cover?: string;
  notionUrl: string;
  blocks: RichContentBlock[];
  content: string;
  published: boolean;
}

const notionToken = import.meta.env.NOTION_TOKEN ?? import.meta.env.NOTION_API_KEY;
const notionDatabaseId = import.meta.env.NOTION_DATABASE_ID;
const notionPublicSiteUrl = String(import.meta.env.NOTION_PUBLIC_SITE_URL || "https://lazzerex.notion.site")
  .trim()
  .replace(/\/+$/, "");

const SLUG_PATTERN = /^[a-z0-9-]+$/;

function normalizeKey(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]/g, "");
}

function slugify(value: string): string {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/["'`“”’]/g, "")
    .replace(/[^a-zA-Z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .toLowerCase();
}

function extractNotionSharePath(rawUrl: string): string | undefined {
  if (!rawUrl) {
    return undefined;
  }

  try {
    const parsedUrl = new URL(rawUrl);
    const segments = parsedUrl.pathname.split("/").filter(Boolean);
    return segments.length > 0 ? segments[segments.length - 1] : undefined;
  } catch (error) {
    return undefined;
  }
}

function richTextToPlainText(richText: RichTextItemResponse[] = []): string {
  return richText.map((item) => item.plain_text ?? "").join("").trim();
}

function getProperty(page: PageObjectResponse, candidates: string[]): unknown {
  const candidateKeys = candidates.map(normalizeKey);

  for (const [key, value] of Object.entries(page.properties)) {
    if (candidateKeys.includes(normalizeKey(key))) {
      return value;
    }
  }

  return undefined;
}

function getPropertyText(property: unknown): string {
  if (!property || typeof property !== "object") {
    return "";
  }

  const value = property as Record<string, any>;

  if (value.type === "title") {
    return richTextToPlainText(value.title);
  }

  if (value.type === "rich_text") {
    return richTextToPlainText(value.rich_text);
  }

  if (value.type === "formula") {
    if (value.formula?.type === "string") {
      return (value.formula.string ?? "").trim();
    }

    if (value.formula?.type === "number") {
      return String(value.formula.number ?? "");
    }
  }

  if (value.type === "url") {
    return (value.url ?? "").trim();
  }

  if (value.type === "select") {
    return (value.select?.name ?? "").trim();
  }

  return "";
}

function getPropertyDate(property: unknown): Date | undefined {
  if (!property || typeof property !== "object") {
    return undefined;
  }

  const value = property as Record<string, any>;
  let dateText: string | undefined;

  if (value.type === "date") {
    dateText = value.date?.start;
  }

  if (value.type === "formula" && value.formula?.type === "date") {
    dateText = value.formula.date?.start;
  }

  if (!dateText) {
    return undefined;
  }

  const parsedDate = new Date(dateText);
  if (Number.isNaN(parsedDate.getTime())) {
    return undefined;
  }

  return parsedDate;
}

function getPropertyTags(property: unknown): string[] {
  if (!property || typeof property !== "object") {
    return [];
  }

  const value = property as Record<string, any>;

  if (value.type === "multi_select") {
    return (value.multi_select ?? []).map((item: any) => String(item.name ?? "").trim()).filter(Boolean);
  }

  if (value.type === "select") {
    const selectedTag = String(value.select?.name ?? "").trim();
    return selectedTag ? [selectedTag] : [];
  }

  const textValue = getPropertyText(value);
  if (!textValue) {
    return [];
  }

  return textValue.split(",").map((tag) => tag.trim()).filter(Boolean);
}

function getPropertyCheckbox(property: unknown): boolean | undefined {
  if (!property || typeof property !== "object") {
    return undefined;
  }

  const value = property as Record<string, any>;

  if (value.type !== "checkbox") {
    return undefined;
  }

  return Boolean(value.checkbox);
}

function getPropertyPeople(property: unknown): string[] {
  if (!property || typeof property !== "object") {
    return [];
  }

  const value = property as Record<string, any>;

  if (value.type !== "people" || !Array.isArray(value.people)) {
    return [];
  }

  return value.people
    .map((person: any) => String(person?.name ?? "").trim())
    .filter(Boolean);
}

function getCoverFromFilesProperty(property: unknown): string | undefined {
  if (!property || typeof property !== "object") {
    return undefined;
  }

  const value = property as Record<string, any>;

  if (value.type !== "files" || !Array.isArray(value.files) || value.files.length === 0) {
    return undefined;
  }

  const firstFile = value.files[0];

  if (firstFile.type === "external") {
    return firstFile.external?.url;
  }

  if (firstFile.type === "file") {
    return firstFile.file?.url;
  }

  return undefined;
}

function getPageCover(page: PageObjectResponse): string | undefined {
  const coverProperty = getProperty(page, ["Cover Image URL", "Cover", "Cover URL"]);
  const coverText = getPropertyText(coverProperty);
  if (coverText) {
    return coverText;
  }

  const coverFromFiles = getCoverFromFilesProperty(coverProperty);
  if (coverFromFiles) {
    return coverFromFiles;
  }

  if (!page.cover) {
    return undefined;
  }

  if (page.cover.type === "external") {
    return page.cover.external.url;
  }

  if (page.cover.type === "file") {
    return page.cover.file.url;
  }

  return undefined;
}

function getPageTitle(page: PageObjectResponse): string {
  const titleProperty = getProperty(page, ["Title", "Name"]);
  const explicitTitle = getPropertyText(titleProperty);
  if (explicitTitle) {
    return explicitTitle;
  }

  for (const property of Object.values(page.properties)) {
    const inferredTitle = getPropertyText(property);
    if (inferredTitle) {
      return inferredTitle;
    }
  }

  return "Untitled Post";
}

function getPageSlug(page: PageObjectResponse, title: string): string {
  const slugProperty = getProperty(page, ["Slug"]);
  const slugValue = getPropertyText(slugProperty).toLowerCase();

  if (slugValue && SLUG_PATTERN.test(slugValue)) {
    return slugValue;
  }

  return slugify(title) || `notion-${page.id.replace(/-/g, "")}`;
}

function getPageDate(page: PageObjectResponse): Date {
  const dateProperty = getProperty(page, ["Publish Date", "PublishDate", "Date"]);
  const explicitDate = getPropertyDate(dateProperty);
  if (explicitDate) {
    return explicitDate;
  }

  const createdDate = new Date(page.created_time);
  if (!Number.isNaN(createdDate.getTime())) {
    return createdDate;
  }

  return new Date();
}

function getPagePublished(page: PageObjectResponse): boolean {
  const publishedProperty = getProperty(page, ["Published"]);
  const publishedValue = getPropertyCheckbox(publishedProperty);

  if (typeof publishedValue === "boolean") {
    return publishedValue;
  }

  return true;
}

function getPageSummary(page: PageObjectResponse): string | undefined {
  const summaryProperty = getProperty(page, ["Summary", "Description", "Excerpt"]);
  const summaryText = getPropertyText(summaryProperty);

  return summaryText || undefined;
}

function getPageAuthor(page: PageObjectResponse): string | undefined {
  const authorProperty = getProperty(page, ["Author", "Authors", "Writer"]);
  const people = getPropertyPeople(authorProperty);

  if (people.length > 0) {
    return people.join(", ");
  }

  const authorText = getPropertyText(authorProperty);
  return authorText || undefined;
}

function getPageTags(page: PageObjectResponse): string[] {
  const tagsProperty = getProperty(page, ["Tags", "Tag", "Category", "Categories"]);
  return getPropertyTags(tagsProperty);
}

function getPageNotionUrl(page: PageObjectResponse): string {
  const publicUrl = (page as PageObjectResponse & { public_url?: string | null }).public_url;
  if (publicUrl) {
    return publicUrl;
  }

  const sharePath = extractNotionSharePath(page.url);
  if (sharePath && notionPublicSiteUrl) {
    return `${notionPublicSiteUrl}/${sharePath}`;
  }

  return page.url;
}

function isFullPage(response: PageObjectResponse | PartialPageObjectResponse): response is PageObjectResponse {
  return response.object === "page" && "properties" in response;
}

function isFullBlock(response: BlockObjectResponse | PartialBlockObjectResponse): response is BlockObjectResponse {
  return response.object === "block" && "type" in response;
}

function blockTextToParagraph(text: string): RichContentBlock[] {
  if (!text) {
    return [];
  }

  return [{
    type: "paragraph",
    text
  }];
}

function mapBlockToRichBlocks(block: BlockObjectResponse): RichContentBlock[] {
  if (block.type === "paragraph") {
    return blockTextToParagraph(richTextToPlainText(block.paragraph.rich_text));
  }

  if (block.type === "heading_1") {
    const text = richTextToPlainText(block.heading_1.rich_text);
    return text ? [{ type: "heading", level: 1, text }] : [];
  }

  if (block.type === "heading_2") {
    const text = richTextToPlainText(block.heading_2.rich_text);
    return text ? [{ type: "heading", level: 2, text }] : [];
  }

  if (block.type === "heading_3") {
    const text = richTextToPlainText(block.heading_3.rich_text);
    return text ? [{ type: "heading", level: 3, text }] : [];
  }

  if (block.type === "bulleted_list_item") {
    const text = richTextToPlainText(block.bulleted_list_item.rich_text);
    return text ? [{ type: "list-item", ordered: false, text }] : [];
  }

  if (block.type === "numbered_list_item") {
    const text = richTextToPlainText(block.numbered_list_item.rich_text);
    return text ? [{ type: "list-item", ordered: true, text }] : [];
  }

  if (block.type === "quote") {
    const text = richTextToPlainText(block.quote.rich_text);
    return text ? [{ type: "quote", text }] : [];
  }

  if (block.type === "code") {
    const code = richTextToPlainText(block.code.rich_text);
    if (!code) {
      return [];
    }

    return [{
      type: "code",
      language: block.code.language ?? "",
      code
    }];
  }

  if (block.type === "divider") {
    return [{ type: "divider" }];
  }

  if (block.type === "image") {
    const src = block.image.type === "external" ? block.image.external.url : block.image.file.url;
    const alt = richTextToPlainText(block.image.caption);

    return src
      ? [{
          type: "image",
          src,
          alt
        }]
      : [];
  }

  if (block.type === "bookmark") {
    return blockTextToParagraph(block.bookmark.url);
  }

  if (block.type === "embed") {
    return blockTextToParagraph(block.embed.url);
  }

  if (block.type === "callout") {
    return blockTextToParagraph(richTextToPlainText(block.callout.rich_text));
  }

  if (block.type === "to_do") {
    const text = richTextToPlainText(block.to_do.rich_text);
    return text ? [{ type: "list-item", ordered: false, text }] : [];
  }

  if (block.type === "toggle") {
    return blockTextToParagraph(richTextToPlainText(block.toggle.rich_text));
  }

  if (block.type === "child_page") {
    return block.child_page.title
      ? [{ type: "heading", level: 3, text: block.child_page.title }]
      : [];
  }

  return [];
}

async function fetchBlocksForParent(client: Client, parentId: string): Promise<RichContentBlock[]> {
  let cursor: string | undefined;
  const blocks: RichContentBlock[] = [];

  do {
    const response = await client.blocks.children.list({
      block_id: parentId,
      start_cursor: cursor,
      page_size: 100
    });

    for (const rawBlock of response.results) {
      if (!isFullBlock(rawBlock)) {
        continue;
      }

      blocks.push(...mapBlockToRichBlocks(rawBlock));

      if (rawBlock.has_children) {
        blocks.push(...await fetchBlocksForParent(client, rawBlock.id));
      }
    }

    cursor = response.has_more ? response.next_cursor ?? undefined : undefined;
  } while (cursor);

  return blocks;
}

function contentFromBlocks(blocks: RichContentBlock[]): string {
  return blocks
    .map((block) => {
      if (block.type === "heading") {
        return block.text;
      }

      if (block.type === "paragraph") {
        return block.text;
      }

      if (block.type === "list-item") {
        return block.text;
      }

      if (block.type === "quote") {
        return block.text;
      }

      if (block.type === "code") {
        return block.code;
      }

      return "";
    })
    .filter(Boolean)
    .join("\n")
    .trim();
}

async function queryPages(client: Client, databaseId: string, useStrictFilter: boolean): Promise<(PageObjectResponse | PartialPageObjectResponse)[]> {
  const notionClient = client as any;
  const hasLegacyDatabaseQuery = typeof notionClient.databases?.query === "function";
  const hasDataSourceQuery = typeof notionClient.dataSources?.query === "function";

  if (!hasLegacyDatabaseQuery && !hasDataSourceQuery) {
    throw new Error("Installed Notion SDK does not expose a supported query API.");
  }

  let cursor: string | undefined;
  const pages: (PageObjectResponse | PartialPageObjectResponse)[] = [];
  let dataSourceId = databaseId;
  let dataSourceIdResolvedFromDatabase = false;

  do {
    const queryOptions = {
      start_cursor: cursor,
      page_size: 100,
      ...(useStrictFilter
        ? {
            filter: {
              property: "Published",
              checkbox: {
                equals: true
              }
            },
            sorts: [
              {
                property: "Publish Date",
                direction: "descending" as const
              }
            ]
          }
        : {})
    };

    let response: {
      results: Array<PageObjectResponse | PartialPageObjectResponse>;
      has_more: boolean;
      next_cursor: string | null;
    };

    if (hasLegacyDatabaseQuery) {
      response = await notionClient.databases.query({
        database_id: databaseId,
        ...queryOptions
      });
    } else {
      try {
        response = await notionClient.dataSources.query({
          data_source_id: dataSourceId,
          ...queryOptions
        });
      } catch (error) {
        if (dataSourceIdResolvedFromDatabase) {
          throw error;
        }

        const database = await notionClient.databases.retrieve({
          database_id: databaseId
        });

        const resolvedDataSourceId = database?.data_sources?.[0]?.id;
        if (!resolvedDataSourceId) {
          throw error;
        }

        dataSourceId = resolvedDataSourceId;
        dataSourceIdResolvedFromDatabase = true;

        response = await notionClient.dataSources.query({
          data_source_id: dataSourceId,
          ...queryOptions
        });
      }
    }

    for (const result of response.results) {
      if (result.object === "page") {
        pages.push(result);
      }
    }

    cursor = response.has_more ? response.next_cursor ?? undefined : undefined;
  } while (cursor);

  return pages;
}

function createNotionClient(): Client {
  if (!notionToken) {
    throw new Error("Missing NOTION_TOKEN or NOTION_API_KEY for Notion integration.");
  }

  return new Client({ auth: notionToken });
}

export function hasNotionConfig(): boolean {
  return Boolean(notionToken && notionDatabaseId);
}

export async function fetchNotionPublishedPosts(): Promise<NotionPost[]> {
  if (!hasNotionConfig() || !notionDatabaseId) {
    return [];
  }

  const client = createNotionClient();

  let pageResults: (PageObjectResponse | PartialPageObjectResponse)[];

  try {
    pageResults = await queryPages(client, notionDatabaseId, true);
  } catch (error) {
    console.warn("[Notion] Strict Published/Publish Date query failed, using broad query with local filtering.");
    pageResults = await queryPages(client, notionDatabaseId, false);
  }

  const fullPages = pageResults.filter(isFullPage);

  const posts = await Promise.all(
    fullPages.map(async (page) => {
      const title = getPageTitle(page);
      const slug = getPageSlug(page, title);
      const date = getPageDate(page);
      const author = getPageAuthor(page);
      const tags = getPageTags(page);
      const summary = getPageSummary(page);
      const cover = getPageCover(page);
      const notionUrl = getPageNotionUrl(page);
      const published = getPagePublished(page);
      const blocks = await fetchBlocksForParent(client, page.id);
      const content = contentFromBlocks(blocks);

      const post: NotionPost = {
        id: page.id,
        title,
        slug,
        date,
        author,
        tags,
        summary,
        cover,
        notionUrl,
        blocks,
        content,
        published
      };

      return post;
    })
  );

  return posts
    .filter((post) => post.published)
    .sort((left, right) => right.date.getTime() - left.date.getTime());
}
