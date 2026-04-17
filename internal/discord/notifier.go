package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	likeColor          = 0x57F287
	commentColor       = 0x5865F2
	publishedPostColor = 0xFEE75C
)

type Notifier struct {
	webhookURL string
	client     *http.Client
	logger     *slog.Logger
}

type LikeNotification struct {
	Slug      string
	PostTitle string
	LikeCount int64
}

type CommentNotification struct {
	Slug       string
	PostTitle  string
	AuthorName string
	Body       string
}

type PublishedPostNotification struct {
	Slug    string
	Title   string
	Summary string
}

type webhookPayload struct {
	Embeds []webhookEmbed `json:"embeds,omitempty"`
}

type webhookEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []webhookEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

type webhookEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

func NewNotifier(webhookURL string, timeout time.Duration, logger *slog.Logger) *Notifier {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Notifier{
		webhookURL: strings.TrimSpace(webhookURL),
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

func (notifier *Notifier) Enabled() bool {
	if notifier == nil {
		return false
	}

	return strings.TrimSpace(notifier.webhookURL) != ""
}

func (notifier *Notifier) NotifyLike(ctx context.Context, payload LikeNotification) error {
	if !notifier.Enabled() {
		return nil
	}

	postTitle := normalizePostTitle(payload.PostTitle, payload.Slug)
	embed := webhookEmbed{
		Title:       "New Like",
		Description: "A reader liked one of your blog posts.",
		Color:       likeColor,
		Fields: []webhookEmbedField{
			{
				Name:  "Blog",
				Value: postTitle,
			},
			{
				Name:   "Slug",
				Value:  normalizeSlug(payload.Slug),
				Inline: true,
			},
			{
				Name:   "Total Likes",
				Value:  strconv.FormatInt(payload.LikeCount, 10),
				Inline: true,
			},
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return notifier.send(ctx, webhookPayload{Embeds: []webhookEmbed{embed}})
}

func (notifier *Notifier) NotifyComment(ctx context.Context, payload CommentNotification) error {
	if !notifier.Enabled() {
		return nil
	}

	postTitle := normalizePostTitle(payload.PostTitle, payload.Slug)
	authorName := normalizeAuthor(payload.AuthorName)
	commentBody := truncateDiscordField(strings.TrimSpace(payload.Body), 1000, "(empty comment)")

	embed := webhookEmbed{
		Title:       "New Comment",
		Description: "A reader added a new comment.",
		Color:       commentColor,
		Fields: []webhookEmbedField{
			{
				Name:  "Blog",
				Value: postTitle,
			},
			{
				Name:   "Slug",
				Value:  normalizeSlug(payload.Slug),
				Inline: true,
			},
			{
				Name:   "Author",
				Value:  authorName,
				Inline: true,
			},
			{
				Name:  "Comment",
				Value: commentBody,
			},
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return notifier.send(ctx, webhookPayload{Embeds: []webhookEmbed{embed}})
}

func (notifier *Notifier) NotifyPostPublished(ctx context.Context, payload PublishedPostNotification) error {
	if !notifier.Enabled() {
		return nil
	}

	postTitle := normalizePostTitle(payload.Title, payload.Slug)
	summary := truncateDiscordField(strings.TrimSpace(payload.Summary), 1800, "Summary not provided.")

	embed := webhookEmbed{
		Title:       "New Blog Published",
		Description: summary,
		Color:       publishedPostColor,
		Fields: []webhookEmbedField{
			{
				Name:  "Title",
				Value: postTitle,
			},
			{
				Name:  "Slug",
				Value: normalizeSlug(payload.Slug),
			},
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return notifier.send(ctx, webhookPayload{Embeds: []webhookEmbed{embed}})
}

func (notifier *Notifier) send(ctx context.Context, payload webhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, notifier.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := notifier.client.Do(request)
	if err != nil {
		return fmt.Errorf("send discord request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}

	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	return fmt.Errorf("discord webhook returned status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
}

func normalizePostTitle(postTitle, slug string) string {
	trimmedTitle := strings.TrimSpace(postTitle)
	if trimmedTitle == "" {
		trimmedTitle = normalizeSlug(slug)
	}

	if trimmedTitle == "" {
		trimmedTitle = "Unknown post"
	}

	return truncateDiscordField(trimmedTitle, 240, "Unknown post")
}

func normalizeSlug(slug string) string {
	trimmedSlug := strings.TrimSpace(strings.ToLower(slug))
	if trimmedSlug == "" {
		return "unknown"
	}

	return truncateDiscordField(trimmedSlug, 180, "unknown")
}

func normalizeAuthor(authorName string) string {
	trimmedAuthor := strings.TrimSpace(authorName)
	if trimmedAuthor == "" {
		trimmedAuthor = "Anonymous"
	}

	return truncateDiscordField(trimmedAuthor, 120, "Anonymous")
}

func truncateDiscordField(value string, maxRunes int, fallback string) string {
	if maxRunes <= 0 {
		return fallback
	}

	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return fallback
	}

	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}

	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}

	return strings.TrimSpace(string(runes[:maxRunes-3])) + "..."
}
