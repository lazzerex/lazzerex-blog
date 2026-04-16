package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidSlug         = errors.New("invalid slug")
	ErrInvalidEvent        = errors.New("invalid event")
	ErrInvalidVisitorToken = errors.New("invalid visitor token")
	ErrInvalidComment      = errors.New("invalid comment")

	slugPattern         = regexp.MustCompile(`^[a-z0-9-]+$`)
	visitorTokenPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,128}$`)
)

// TrackEvent represents a non-blocking analytics event posted from the frontend.
type TrackEvent struct {
	Slug     string         `json:"slug"`
	Event    string         `json:"event"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ReactionState struct {
	Slug  string `json:"slug"`
	Count int64  `json:"count"`
	Liked bool   `json:"liked"`
}

type CreateCommentPayload struct {
	Slug       string `json:"slug"`
	AuthorName string `json:"authorName"`
	Body       string `json:"body"`
}

type Comment struct {
	ID         int64  `json:"id"`
	Slug       string `json:"slug"`
	AuthorName string `json:"authorName"`
	Body       string `json:"body"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

type Store struct {
	dbConn  *sql.DB
	logger  *slog.Logger
	dialect string
}

func New(dbConn *sql.DB, logger *slog.Logger, dialect ...string) *Store {
	if logger == nil {
		logger = slog.Default()
	}

	normalizedDialect := "sqlite"
	if len(dialect) > 0 {
		candidate := strings.ToLower(strings.TrimSpace(dialect[0]))
		if candidate == "postgres" {
			normalizedDialect = "postgres"
		}
	}

	return &Store{dbConn: dbConn, logger: logger, dialect: normalizedDialect}
}

func ValidateSlug(slug string) bool {
	return slugPattern.MatchString(strings.TrimSpace(strings.ToLower(slug)))
}

func ValidateVisitorToken(token string) bool {
	normalizedToken := strings.TrimSpace(token)
	return visitorTokenPattern.MatchString(normalizedToken)
}

func (store *Store) Ping(ctx context.Context) error {
	return store.dbConn.PingContext(ctx)
}

func (store *Store) q(query string) string {
	if store.dialect != "postgres" {
		return query
	}

	var builder strings.Builder
	builder.Grow(len(query) + 8)

	placeholder := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			placeholder++
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(placeholder))
			continue
		}

		builder.WriteByte(query[i])
	}

	return builder.String()
}

func (store *Store) IncrementView(ctx context.Context, slug, ipHash string) (int64, error) {
	normalizedSlug := strings.TrimSpace(strings.ToLower(slug))
	if !ValidateSlug(normalizedSlug) {
		return 0, ErrInvalidSlug
	}

	tx, err := store.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, store.q(`
		INSERT INTO post_views (slug, count, updated_at)
		VALUES (?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(slug)
		DO UPDATE SET count = count + 1, updated_at = CURRENT_TIMESTAMP;
	`), normalizedSlug); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, store.q(`
		INSERT INTO view_events (slug, ip_hash)
		VALUES (?, ?);
	`), normalizedSlug, ipHash); err != nil {
		return 0, err
	}

	var count int64
	if err := tx.QueryRowContext(ctx, store.q(`SELECT count FROM post_views WHERE slug = ?`), normalizedSlug).Scan(&count); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return count, nil
}

func (store *Store) RecordTrackEvent(ctx context.Context, payload TrackEvent, ipHash string) error {
	normalizedSlug := strings.TrimSpace(strings.ToLower(payload.Slug))
	if !ValidateSlug(normalizedSlug) {
		return ErrInvalidSlug
	}

	normalizedEvent := strings.TrimSpace(payload.Event)
	if normalizedEvent == "" {
		return ErrInvalidEvent
	}

	metadata := payload.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = store.dbConn.ExecContext(ctx, store.q(`
		INSERT INTO track_events (slug, event, metadata_json, ip_hash)
		VALUES (?, ?, ?, ?);
	`), normalizedSlug, normalizedEvent, string(metadataJSON), ipHash)

	return err
}

func (store *Store) ToggleReaction(ctx context.Context, slug, visitorHash string) (ReactionState, error) {
	normalizedSlug := strings.TrimSpace(strings.ToLower(slug))
	if !ValidateSlug(normalizedSlug) {
		return ReactionState{}, ErrInvalidSlug
	}

	normalizedVisitorHash := strings.TrimSpace(visitorHash)
	if normalizedVisitorHash == "" {
		return ReactionState{}, ErrInvalidVisitorToken
	}

	tx, err := store.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return ReactionState{}, err
	}

	defer tx.Rollback()

	var existing int
	err = tx.QueryRowContext(ctx, store.q(`
		SELECT 1
		FROM post_reactions
		WHERE slug = ? AND visitor_hash = ?
	`), normalizedSlug, normalizedVisitorHash).Scan(&existing)

	liked := false
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, store.q(`
			INSERT INTO post_reactions (slug, visitor_hash)
			VALUES (?, ?)
		`), normalizedSlug, normalizedVisitorHash); err != nil {
			return ReactionState{}, err
		}

		if _, err := tx.ExecContext(ctx, store.q(`
			INSERT INTO reaction_events (slug, action, visitor_hash)
			VALUES (?, 'liked', ?)
		`), normalizedSlug, normalizedVisitorHash); err != nil {
			return ReactionState{}, err
		}

		if _, err := tx.ExecContext(ctx, store.q(`
			INSERT INTO post_reaction_counts (slug, likes_count, updated_at)
			VALUES (?, 1, CURRENT_TIMESTAMP)
			ON CONFLICT(slug)
			DO UPDATE SET likes_count = likes_count + 1, updated_at = CURRENT_TIMESTAMP
		`), normalizedSlug); err != nil {
			return ReactionState{}, err
		}

		liked = true
	} else if err != nil {
		return ReactionState{}, err
	} else {
		if _, err := tx.ExecContext(ctx, store.q(`
			DELETE FROM post_reactions
			WHERE slug = ? AND visitor_hash = ?
		`), normalizedSlug, normalizedVisitorHash); err != nil {
			return ReactionState{}, err
		}

		if _, err := tx.ExecContext(ctx, store.q(`
			INSERT INTO reaction_events (slug, action, visitor_hash)
			VALUES (?, 'unliked', ?)
		`), normalizedSlug, normalizedVisitorHash); err != nil {
			return ReactionState{}, err
		}

		if _, err := tx.ExecContext(ctx, store.q(`
			INSERT INTO post_reaction_counts (slug, likes_count, updated_at)
			VALUES (?, 0, CURRENT_TIMESTAMP)
			ON CONFLICT(slug)
			DO UPDATE SET
				likes_count = CASE WHEN likes_count > 0 THEN likes_count - 1 ELSE 0 END,
				updated_at = CURRENT_TIMESTAMP
		`), normalizedSlug); err != nil {
			return ReactionState{}, err
		}
	}

	var count int64
	err = tx.QueryRowContext(ctx, store.q(`
		SELECT likes_count
		FROM post_reaction_counts
		WHERE slug = ?
	`), normalizedSlug).Scan(&count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ReactionState{}, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		count = 0
	}

	if err := tx.Commit(); err != nil {
		return ReactionState{}, err
	}

	return ReactionState{Slug: normalizedSlug, Count: count, Liked: liked}, nil
}

func (store *Store) GetReactionState(ctx context.Context, slug, visitorHash string) (ReactionState, error) {
	normalizedSlug := strings.TrimSpace(strings.ToLower(slug))
	if !ValidateSlug(normalizedSlug) {
		return ReactionState{}, ErrInvalidSlug
	}

	state := ReactionState{Slug: normalizedSlug, Count: 0, Liked: false}

	err := store.dbConn.QueryRowContext(ctx, store.q(`
		SELECT likes_count
		FROM post_reaction_counts
		WHERE slug = ?
	`), normalizedSlug).Scan(&state.Count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ReactionState{}, err
	}

	normalizedVisitorHash := strings.TrimSpace(visitorHash)
	if normalizedVisitorHash != "" {
		var liked int
		err = store.dbConn.QueryRowContext(ctx, store.q(`
			SELECT 1
			FROM post_reactions
			WHERE slug = ? AND visitor_hash = ?
		`), normalizedSlug, normalizedVisitorHash).Scan(&liked)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			state.Liked = false
		case err != nil:
			return ReactionState{}, err
		default:
			state.Liked = true
		}
	}

	return state, nil
}

func (store *Store) ListApprovedComments(ctx context.Context, slug string, limit int) ([]Comment, error) {
	normalizedSlug := strings.TrimSpace(strings.ToLower(slug))
	if !ValidateSlug(normalizedSlug) {
		return nil, ErrInvalidSlug
	}

	boundedLimit := limit
	if boundedLimit <= 0 {
		boundedLimit = 20
	}

	if boundedLimit > 100 {
		boundedLimit = 100
	}

	rows, err := store.dbConn.QueryContext(ctx, store.q(`
		SELECT id, slug, author_name, body, status, `+store.createdAtSelectExpr()+`
		FROM post_comments
		WHERE slug = ? AND status = 'approved'
		ORDER BY created_at DESC
		LIMIT ?
	`), normalizedSlug, boundedLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]Comment, 0, boundedLimit)
	for rows.Next() {
		var comment Comment
		var createdAt any
		if err := rows.Scan(&comment.ID, &comment.Slug, &comment.AuthorName, &comment.Body, &comment.Status, &createdAt); err != nil {
			return nil, err
		}

		comment.CreatedAt = normalizeCreatedAtValue(createdAt)

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

func (store *Store) CreateComment(ctx context.Context, payload CreateCommentPayload) (Comment, error) {
	normalizedSlug := strings.TrimSpace(strings.ToLower(payload.Slug))
	if !ValidateSlug(normalizedSlug) {
		return Comment{}, ErrInvalidSlug
	}

	authorName := normalizeAuthorName(payload.AuthorName)
	body := strings.TrimSpace(payload.Body)
	if !isValidCommentBody(body) {
		return Comment{}, ErrInvalidComment
	}

	if store.dialect == "postgres" {
		comment := Comment{}
		var createdAt any
		err := store.dbConn.QueryRowContext(ctx, store.q(`
			INSERT INTO post_comments (slug, author_name, body, status)
			VALUES (?, ?, ?, 'approved')
			RETURNING id, slug, author_name, body, status, created_at
		`), normalizedSlug, authorName, body).Scan(
			&comment.ID,
			&comment.Slug,
			&comment.AuthorName,
			&comment.Body,
			&comment.Status,
			&createdAt,
		)
		if err != nil {
			return Comment{}, err
		}

		comment.CreatedAt = normalizeCreatedAtValue(createdAt)

		return comment, nil
	}

	result, err := store.dbConn.ExecContext(ctx, store.q(`
		INSERT INTO post_comments (slug, author_name, body, status)
		VALUES (?, ?, ?, 'approved')
	`), normalizedSlug, authorName, body)
	if err != nil {
		return Comment{}, err
	}

	commentID, err := result.LastInsertId()
	if err != nil {
		return Comment{}, err
	}

	var createdAt any
	if err := store.dbConn.QueryRowContext(ctx, store.q(`
		SELECT `+store.createdAtSelectExpr()+`
		FROM post_comments
		WHERE id = ?
	`), commentID).Scan(&createdAt); err != nil {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}

	return Comment{
		ID:         commentID,
		Slug:       normalizedSlug,
		AuthorName: authorName,
		Body:       body,
		Status:     "approved",
		CreatedAt:  normalizeCreatedAtValue(createdAt),
	}, nil
}

func (store *Store) createdAtSelectExpr() string {
	if store.dialect == "postgres" {
		return "created_at::text"
	}

	return "created_at"
}

func normalizeCreatedAtValue(value any) string {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed != "" {
			return trimmed
		}
	case []byte:
		trimmed := strings.TrimSpace(string(typed))
		if trimmed != "" {
			return trimmed
		}
	}

	return time.Now().UTC().Format(time.RFC3339)
}

func normalizeAuthorName(authorName string) string {
	normalized := strings.TrimSpace(authorName)
	if normalized == "" {
		return "Anonymous"
	}

	if utf8.RuneCountInString(normalized) <= 60 {
		return normalized
	}

	runes := []rune(normalized)
	return strings.TrimSpace(string(runes[:60]))
}

func isValidCommentBody(body string) bool {
	normalizedBody := strings.TrimSpace(body)
	if normalizedBody == "" {
		return false
	}

	runeCount := utf8.RuneCountInString(normalizedBody)
	if runeCount < 3 || runeCount > 1200 {
		return false
	}

	return !strings.ContainsRune(normalizedBody, rune(0))
}
