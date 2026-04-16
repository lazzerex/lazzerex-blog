CREATE TABLE IF NOT EXISTS post_reaction_counts (
  slug TEXT PRIMARY KEY,
  likes_count BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS post_reactions (
  slug TEXT NOT NULL,
  visitor_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (slug, visitor_hash)
);

CREATE TABLE IF NOT EXISTS reaction_events (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT NOT NULL,
  action TEXT NOT NULL,
  visitor_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK (action IN ('liked', 'unliked'))
);

CREATE INDEX IF NOT EXISTS idx_reaction_events_slug_created_at
  ON reaction_events (slug, created_at DESC);

CREATE TABLE IF NOT EXISTS post_comments (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT NOT NULL,
  author_name TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'approved',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_post_comments_slug_status_created_at
  ON post_comments (slug, status, created_at DESC);
