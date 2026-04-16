CREATE TABLE IF NOT EXISTS post_views (
  slug TEXT PRIMARY KEY,
  count BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS view_events (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT NOT NULL,
  ip_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_view_events_slug_created_at
  ON view_events (slug, created_at DESC);

CREATE TABLE IF NOT EXISTS track_events (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT NOT NULL,
  event TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  ip_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_track_events_slug_created_at
  ON track_events (slug, created_at DESC);
