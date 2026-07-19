import rss from "@astrojs/rss";
import type { APIContext } from "astro";
import { getAllPosts } from "../lib/posts";

export async function GET(context: APIContext) {
  const posts = await getAllPosts();

  return rss({
    title: "Lazzerex Blog",
    description: "Lazzerex personal blog about technology, science, music, and gaming.",
    site: context.site!,
    items: posts.map((post) => ({
      title: post.title,
      pubDate: post.date,
      description: post.summary,
      link: `/blog/${post.slug}/`,
      categories: post.tags
    })),
    customData: "<language>en-us</language>"
  });
}
