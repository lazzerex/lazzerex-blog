# Lazzerex Blog

<p align="center">
  <img src="https://img.shields.io/badge/Astro-BC52EE?style=flat&logo=astro&logoColor=white"/>
  <img src="https://img.shields.io/badge/TypeScript-3178C6?style=flat&logo=typescript&logoColor=white"/>
  <img src="https://img.shields.io/badge/Notion_API-000000?style=flat&logo=notion&logoColor=white"/>
  <img src="https://img.shields.io/badge/Node.js-20%2B-339933?style=flat&logo=node.js&logoColor=white"/>
  <img src="https://img.shields.io/badge/npm-CLI-CB3837?style=flat&logo=npm&logoColor=white"/>
  <img src="https://img.shields.io/badge/Lucide-Icons-f97316?style=flat"/>
  <img src="https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white"/>
  <img src="https://img.shields.io/badge/SQLite-003B57?style=flat&logo=sqlite&logoColor=white"/>
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?style=flat&logo=postgresql&logoColor=white"/>
  <img src="https://img.shields.io/badge/Neon-00E599?style=flat"/>
  <img src="https://img.shields.io/badge/Vercel-000000?style=flat&logo=vercel&logoColor=white"/>
  <img src="https://img.shields.io/badge/Rendering-Static%20Site-1f6feb?style=flat"/>
</p>

This repository powers my personal blog and publishing platform. Built with Astro and Notion CMS.

I am a university student writing about technology, systems, science, music, and gaming.
The personal part is simple: this is where I publish my learning journey.

## Core Dependencies

### Current Stack

- Frontend framework: Astro 5
- Language: TypeScript
- Content source: Notion API (`@notionhq/client`)
- Icons: Lucide (`@lucide/astro`, `lucide-astro`)
- Runtime/tooling: Node.js 20+, npm
- API service: Go 1.22+
- Runtime persistence: SQLite (default) or PostgreSQL (Neon-ready)
- Deployment: Vercel
- Rendering model: static site generation (SSG)

## Technical Highlights

- Notion-only content pipeline (build fails fast if env is missing)
- Static route generation for each post at `/blog/[slug]`
- Build-time slug validation and collision checks
- SEO tags: canonical, Open Graph, Twitter
- Componentized shell layout with responsive behavior

## Architecture Overview

1. Build process reads posts from Notion database.
2. Data is normalized and slug-validated in `src/lib/posts.ts`.
3. Astro generates static routes for listing and detail pages.
4. Metadata is injected for SEO and social previews.

Primary routes:

- `/` article explorer list
- `/blog/[slug]` article detail page
- `/about` author profile page
- `/contact` contact page

## Content Publishing Flow (Notion)

1. Write/edit post in Notion.
2. Set `Published = true`.
3. Set unique lowercase slug.
4. Rebuild/redeploy to publish.

## Local Setup

### 1. Install dependencies

```bash
npm install
```

### 2. Add environment variables

Create `.env` in the project root:

```env
NOTION_TOKEN=secret_xxxxxxxxxxxxxxxxxxxxxxxxx
NOTION_DATABASE_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
PUBLIC_GO_API_BASE_URL=http://localhost:8080
GO_API_PUBLISH_SYNC_SECRET=replace-with-long-random-string
```

Optional alias (kept for compatibility):

```env
NOTION_API_KEY=secret_xxxxxxxxxxxxxxxxxxxxxxxxx
```

`GO_API_PUBLISH_SYNC_SECRET` is read at Astro build time (server-side only) and sent to the Go API during published-post sync.

### 3. Run locally

```bash
npm run dev
```

### 4. Build

```bash
npm run build
```

### 5. Run Go API locally (optional but recommended)

```bash
go run ./cmd/server
```

Default local CORS allows Astro dev origins on port `4321`.

Relevant backend environment variables:

```env
GO_API_ADDRESS=:8080
GO_API_DB_DRIVER=sqlite
GO_API_DB_PATH=data/lazzerex.sqlite
GO_API_DATABASE_URL=
GO_API_ALLOWED_ORIGINS=http://localhost:4321,http://127.0.0.1:4321
GO_API_DISCORD_WEBHOOK_URL=
GO_API_DISCORD_REQUEST_TIMEOUT=4s
GO_API_PUBLISH_SYNC_SECRET=replace-with-long-random-string
```

For PostgreSQL/Neon, set:

```env
GO_API_DB_DRIVER=postgres
GO_API_DATABASE_URL=postgresql://USER:PASSWORD@HOST/DBNAME?sslmode=require
```

### 6. Preview production output

```bash
npm run preview
```

## Notion Schema (Minimum)

- `Published` (Checkbox)
- `Publish Date` (Date)
- `Title` (Title)
- `Slug` (Rich text or formula output)

Recommended:

- `Summary` (Rich text)
- `Tags` (Multi-select)
- `Cover Image URL` (URL or files/media)

## Tech Stack

Core libraries:

- `astro`
- `@notionhq/client`
- `@lucide/astro`
- `lucide-astro`

## Deployment

- Deploy target: Vercel
- Site URL: https://lazzerex-blog.vercel.app
- Required env vars in Vercel:
  - `NOTION_TOKEN`
  - `NOTION_DATABASE_ID`
  - `PUBLIC_GO_API_BASE_URL` (for example your Go service URL)

### Render + Neon deployment (Go API)

1. Create a Neon database and copy the connection string.
2. Create a new Render Web Service from this repository.
3. In Render service settings, use:
  - Build command: `go build -o bin/server ./cmd/server`
  - Start command: `./bin/server`
4. Add Render environment variables:
  - `GO_API_DB_DRIVER=postgres`
  - `GO_API_DATABASE_URL=<your-neon-connection-string>`
  - `GO_API_ALLOWED_ORIGINS=https://lazzerex-blog.vercel.app`
  - Optional: `GO_API_LOG_LEVEL=info`
5. Set frontend env (`PUBLIC_GO_API_BASE_URL`) in Vercel to the Render URL.

Notes:
- The API auto-binds to Render's `PORT` if `GO_API_ADDRESS` is not set.
- Keep Neon SSL enabled (`sslmode=require`) for production.
- Keep SQLite for local development unless you explicitly want local Postgres testing.

Discord notifications are triggered for:
- New likes (includes blog slug/title and total likes)
- New comments (includes blog slug/title, author name, and comment body)
- Newly published blogs detected during Astro build sync (includes title and summary, sent once per slug)

## Documentation

- Notion setup and troubleshooting: `notion_guide.md`
- Migration and phase tracking: `plan.md`
- Engineering workflow: `workflow.md`

## License

This is a personal project repository.
