import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

export default defineConfig({
  output: "static",
  site: "https://lazzerex-blog.vercel.app",
  prefetch: {
    defaultStrategy: "hover"
  },
  integrations: [sitemap()]
});
