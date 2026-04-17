/// <reference types="astro/client" />

interface ImportMetaEnv {
	readonly NOTION_TOKEN?: string;
	readonly NOTION_API_KEY?: string;
	readonly NOTION_DATABASE_ID?: string;
	readonly PUBLIC_GO_API_BASE_URL?: string;
	readonly GO_API_PUBLISH_SYNC_SECRET?: string;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
}
