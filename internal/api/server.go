package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lazzerex-blog/internal/config"
	"lazzerex-blog/internal/discord"
	"lazzerex-blog/internal/ratelimit"
	"lazzerex-blog/internal/store"
)

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type responseEnvelope struct {
	OK    bool           `json:"ok"`
	Data  any            `json:"data,omitempty"`
	Error *responseError `json:"error,omitempty"`
}

type viewsResponse struct {
	Slug  string `json:"slug"`
	Count int64  `json:"count"`
}

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

type trackResponse struct {
	Accepted bool   `json:"accepted"`
	Event    string `json:"event"`
	Slug     string `json:"slug"`
}

type commentsResponse struct {
	Slug     string          `json:"slug"`
	Comments []store.Comment `json:"comments"`
}

type commentCreateResponse struct {
	Comment store.Comment `json:"comment"`
}

type reactionToggleRequest struct {
	Slug         string `json:"slug"`
	VisitorToken string `json:"visitorToken"`
	PostTitle    string `json:"postTitle,omitempty"`
}

type commentCreateRequest struct {
	Slug       string `json:"slug"`
	AuthorName string `json:"authorName"`
	Body       string `json:"body"`
	PostTitle  string `json:"postTitle,omitempty"`
}

type postPublishedRequest struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type postPublishedResponse struct {
	Slug          string `json:"slug"`
	NewlyNotified bool   `json:"newlyNotified"`
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

const (
	readOperationTimeout  = 5 * time.Second
	writeOperationTimeout = 8 * time.Second
)

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

type Server struct {
	cfg             config.Config
	logger          *slog.Logger
	store           *store.Store
	limiter         *ratelimit.IPLimiter
	reactionLimiter *ratelimit.IPLimiter
	commentLimiter  *ratelimit.IPLimiter
	mux             *http.ServeMux
	allowAnyOrigin  bool
	allowedOrigins  map[string]struct{}
	discordNotifier *discord.Notifier
}

func NewServer(cfg config.Config, logger *slog.Logger, storeLayer *store.Store, notifiers ...*discord.Notifier) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	var discordNotifier *discord.Notifier
	if len(notifiers) > 0 {
		discordNotifier = notifiers[0]
	}

	allowAnyOrigin, allowedOrigins := buildAllowedOrigins(cfg.AllowedOrigins)

	server := &Server{
		cfg:             cfg,
		logger:          logger,
		store:           storeLayer,
		limiter:         ratelimit.NewIPLimiter(cfg.ViewsRateLimit, cfg.ViewsRateWindow),
		reactionLimiter: ratelimit.NewIPLimiter(20, time.Minute),
		commentLimiter:  ratelimit.NewIPLimiter(6, time.Minute),
		mux:             http.NewServeMux(),
		allowAnyOrigin:  allowAnyOrigin,
		allowedOrigins:  allowedOrigins,
		discordNotifier: discordNotifier,
	}

	server.registerRoutes()
	return server
}

func (server *Server) registerRoutes() {
	server.mux.HandleFunc("GET /health", server.handleHealth)
	server.mux.HandleFunc("GET /api/views/{slug}", server.handleGetViews)
	server.mux.HandleFunc("POST /api/views/{slug}", server.handleViews)
	server.mux.HandleFunc("POST /api/track", server.handleTrack)
	server.mux.HandleFunc("GET /api/reactions/{slug}", server.handleGetReactionState)
	server.mux.HandleFunc("POST /api/reactions/toggle", server.handleToggleReaction)
	server.mux.HandleFunc("GET /api/comments/{slug}", server.handleGetComments)
	server.mux.HandleFunc("POST /api/comments", server.handleCreateComment)
	server.mux.HandleFunc("POST /api/blogs/published", server.handleSyncPublishedPost)
}

func (server *Server) Handler() http.Handler {
	return server.withRequestLogging(server.withCORS(server.mux))
}

func (server *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	statusCode := http.StatusOK
	status := "ok"
	if err := server.store.Ping(ctx); err != nil {
		statusCode = http.StatusServiceUnavailable
		status = "degraded"
	}

	server.writeJSON(writer, statusCode, responseEnvelope{
		OK: statusCode == http.StatusOK,
		Data: healthResponse{
			Status:    status,
			Service:   "lazzerex-go-api",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func (server *Server) handleGetViews(writer http.ResponseWriter, request *http.Request) {
	slug := strings.TrimSpace(strings.ToLower(request.PathValue("slug")))
	if !store.ValidateSlug(slug) {
		server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), readOperationTimeout)
	defer cancel()

	count, err := server.store.GetViewCount(ctx, slug)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSlug) {
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
			return
		}

		server.logger.Error("read view count failed",
			"slug", slug,
			"error", err,
		)

		server.writeError(writer, http.StatusInternalServerError, "views_read_failed", "unable to read view count")
		return
	}

	server.writeJSON(writer, http.StatusOK, responseEnvelope{
		OK: true,
		Data: viewsResponse{
			Slug:  slug,
			Count: count,
		},
	})
}

func (server *Server) handleViews(writer http.ResponseWriter, request *http.Request) {
	slug := strings.TrimSpace(strings.ToLower(request.PathValue("slug")))
	if !store.ValidateSlug(slug) {
		server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		return
	}

	clientIP := resolveClientIP(request, server.cfg.TrustProxyHeaders)
	allowed, retryAfter := server.limiter.Allow(clientIP, time.Now())
	if !allowed {
		writer.Header().Set("Retry-After", fmt.Sprintf("%.0f", math.Ceil(retryAfter.Seconds())))
		server.writeError(writer, http.StatusTooManyRequests, "rate_limited", "too many view events from this IP")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), writeOperationTimeout)
	defer cancel()

	count, err := server.store.IncrementView(ctx, slug, hashIP(clientIP))
	if err != nil {
		if errors.Is(err, store.ErrInvalidSlug) {
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
			return
		}

		server.logger.Error("increment view failed",
			"slug", slug,
			"client_ip", clientIP,
			"error", err,
		)

		server.writeError(writer, http.StatusInternalServerError, "views_failed", "unable to increment view count")
		return
	}

	server.writeJSON(writer, http.StatusOK, responseEnvelope{
		OK: true,
		Data: viewsResponse{
			Slug:  slug,
			Count: count,
		},
	})
}

func (server *Server) handleTrack(writer http.ResponseWriter, request *http.Request) {
	var payload store.TrackEvent
	if !server.decodeJSONPayload(writer, request, &payload) {
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), writeOperationTimeout)
	defer cancel()

	if err := server.store.RecordTrackEvent(ctx, payload, hashIP(resolveClientIP(request, server.cfg.TrustProxyHeaders))); err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidSlug):
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		case errors.Is(err, store.ErrInvalidEvent):
			server.writeError(writer, http.StatusBadRequest, "invalid_event", "event is required")
		default:
			server.logger.Error("track event write failed",
				"slug", strings.TrimSpace(strings.ToLower(payload.Slug)),
				"event", strings.TrimSpace(payload.Event),
				"error", err,
			)
			server.writeError(writer, http.StatusInternalServerError, "track_failed", "unable to store analytics event")
		}
		return
	}

	normalizedSlug := strings.TrimSpace(strings.ToLower(payload.Slug))
	server.writeJSON(writer, http.StatusAccepted, responseEnvelope{
		OK: true,
		Data: trackResponse{
			Accepted: true,
			Event:    strings.TrimSpace(payload.Event),
			Slug:     normalizedSlug,
		},
	})
}

func (server *Server) handleGetReactionState(writer http.ResponseWriter, request *http.Request) {
	slug := strings.TrimSpace(strings.ToLower(request.PathValue("slug")))
	if !store.ValidateSlug(slug) {
		server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		return
	}

	visitorToken := strings.TrimSpace(request.URL.Query().Get("visitor_token"))
	visitorHash := ""
	if visitorToken != "" {
		if !store.ValidateVisitorToken(visitorToken) {
			server.writeError(writer, http.StatusBadRequest, "invalid_visitor_token", "visitor token is invalid")
			return
		}

		visitorHash = hashVisitorToken(visitorToken)
	}

	ctx, cancel := context.WithTimeout(request.Context(), readOperationTimeout)
	defer cancel()

	reactionState, err := server.store.GetReactionState(ctx, slug, visitorHash)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSlug) {
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
			return
		}

		server.logger.Error("read reaction state failed",
			"slug", slug,
			"has_visitor_hash", visitorHash != "",
			"error", err,
		)

		server.writeError(writer, http.StatusInternalServerError, "reaction_state_failed", "unable to read reaction state")
		return
	}

	server.writeJSON(writer, http.StatusOK, responseEnvelope{OK: true, Data: reactionState})
}

func (server *Server) handleToggleReaction(writer http.ResponseWriter, request *http.Request) {
	clientIP := resolveClientIP(request, server.cfg.TrustProxyHeaders)
	allowed, retryAfter := server.reactionLimiter.Allow(clientIP, time.Now())
	if !allowed {
		writer.Header().Set("Retry-After", fmt.Sprintf("%.0f", math.Ceil(retryAfter.Seconds())))
		server.writeError(writer, http.StatusTooManyRequests, "rate_limited", "too many reaction updates from this IP")
		return
	}

	var payload reactionToggleRequest
	if !server.decodeJSONPayload(writer, request, &payload) {
		return
	}

	if !store.ValidateVisitorToken(payload.VisitorToken) {
		server.writeError(writer, http.StatusBadRequest, "invalid_visitor_token", "visitor token is invalid")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), writeOperationTimeout)
	defer cancel()

	reactionState, err := server.store.ToggleReaction(ctx, payload.Slug, hashVisitorToken(payload.VisitorToken))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidSlug):
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		case errors.Is(err, store.ErrInvalidVisitorToken):
			server.writeError(writer, http.StatusBadRequest, "invalid_visitor_token", "visitor token is invalid")
		default:
			server.logger.Error("toggle reaction failed",
				"slug", strings.TrimSpace(strings.ToLower(payload.Slug)),
				"client_ip", clientIP,
				"error", err,
			)
			server.writeError(writer, http.StatusInternalServerError, "reaction_toggle_failed", "unable to toggle reaction")
		}
		return
	}

	if reactionState.Liked {
		server.dispatchLikeNotification(reactionState.Slug, payload.PostTitle, reactionState.Count)
	}

	server.writeJSON(writer, http.StatusOK, responseEnvelope{OK: true, Data: reactionState})
}

func (server *Server) handleGetComments(writer http.ResponseWriter, request *http.Request) {
	slug := strings.TrimSpace(strings.ToLower(request.PathValue("slug")))
	if !store.ValidateSlug(slug) {
		server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(request.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil {
			server.writeError(writer, http.StatusBadRequest, "invalid_limit", "limit must be an integer")
			return
		}

		limit = parsedLimit
	}

	ctx, cancel := context.WithTimeout(request.Context(), readOperationTimeout)
	defer cancel()

	comments, err := server.store.ListApprovedComments(ctx, slug, limit)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSlug) {
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
			return
		}

		server.logger.Error("list comments failed",
			"slug", slug,
			"limit", limit,
			"error", err,
		)

		server.writeError(writer, http.StatusInternalServerError, "comments_read_failed", "unable to read comments")
		return
	}

	server.writeJSON(writer, http.StatusOK, responseEnvelope{
		OK: true,
		Data: commentsResponse{
			Slug:     slug,
			Comments: comments,
		},
	})
}

func (server *Server) handleCreateComment(writer http.ResponseWriter, request *http.Request) {
	clientIP := resolveClientIP(request, server.cfg.TrustProxyHeaders)
	allowed, retryAfter := server.commentLimiter.Allow(clientIP, time.Now())
	if !allowed {
		writer.Header().Set("Retry-After", fmt.Sprintf("%.0f", math.Ceil(retryAfter.Seconds())))
		server.writeError(writer, http.StatusTooManyRequests, "rate_limited", "too many comment submissions from this IP")
		return
	}

	var payload commentCreateRequest
	if !server.decodeJSONPayload(writer, request, &payload) {
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), writeOperationTimeout)
	defer cancel()

	comment, err := server.store.CreateComment(ctx, store.CreateCommentPayload{
		Slug:       payload.Slug,
		AuthorName: payload.AuthorName,
		Body:       payload.Body,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidSlug):
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		case errors.Is(err, store.ErrInvalidComment):
			server.writeError(writer, http.StatusBadRequest, "invalid_comment", "comment body must be 3-1200 characters")
		default:
			server.logger.Error("create comment failed",
				"slug", strings.TrimSpace(strings.ToLower(payload.Slug)),
				"author_present", strings.TrimSpace(payload.AuthorName) != "",
				"client_ip", clientIP,
				"error", err,
			)
			server.writeError(writer, http.StatusInternalServerError, "comment_create_failed", "unable to create comment")
		}
		return
	}

	server.dispatchCommentNotification(comment, payload.PostTitle)

	server.writeJSON(writer, http.StatusCreated, responseEnvelope{
		OK:   true,
		Data: commentCreateResponse{Comment: comment},
	})
}

func (server *Server) handleSyncPublishedPost(writer http.ResponseWriter, request *http.Request) {
	if !server.authorizePublishSyncRequest(request) {
		server.writeError(writer, http.StatusUnauthorized, "unauthorized", "publish sync request is not authorized")
		return
	}

	var payload postPublishedRequest
	if !server.decodeJSONPayload(writer, request, &payload) {
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), writeOperationTimeout)
	defer cancel()

	publishedPost, err := server.store.UpsertPublishedPost(ctx, store.PublishedPost{
		Slug:    payload.Slug,
		Title:   payload.Title,
		Summary: payload.Summary,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInvalidSlug):
			server.writeError(writer, http.StatusBadRequest, "invalid_slug", "slug must match [a-z0-9-]+")
		default:
			server.logger.Error("upsert published post failed",
				"slug", strings.TrimSpace(strings.ToLower(payload.Slug)),
				"error", err,
			)
			server.writeError(writer, http.StatusInternalServerError, "published_post_sync_failed", "unable to sync published post")
		}
		return
	}

	newlyNotified := false
	if !publishedPost.Notified && server.discordNotifier != nil && server.discordNotifier.Enabled() {
		notifyCtx, notifyCancel := context.WithTimeout(context.Background(), server.cfg.DiscordRequestTimeout)
		notifyErr := server.discordNotifier.NotifyPostPublished(notifyCtx, discord.PublishedPostNotification{
			Slug:    publishedPost.Slug,
			Title:   publishedPost.Title,
			Summary: publishedPost.Summary,
		})
		notifyCancel()
		if notifyErr != nil {
			server.logger.Error("discord published post notification failed",
				"slug", publishedPost.Slug,
				"error", notifyErr,
			)
			server.writeError(writer, http.StatusBadGateway, "publish_notification_failed", "unable to notify published post")
			return
		}

		if err := server.store.MarkPublishedPostNotified(ctx, publishedPost.Slug); err != nil {
			server.logger.Error("mark published post as notified failed",
				"slug", publishedPost.Slug,
				"error", err,
			)
			server.writeError(writer, http.StatusInternalServerError, "publish_notification_state_failed", "unable to persist notification state")
			return
		}

		newlyNotified = true
		server.logger.Info("discord_published_post_notification_sent",
			"slug", publishedPost.Slug,
			"title", publishedPost.Title,
		)
	}

	server.writeJSON(writer, http.StatusOK, responseEnvelope{
		OK: true,
		Data: postPublishedResponse{
			Slug:          publishedPost.Slug,
			NewlyNotified: newlyNotified,
		},
	})
}

func (server *Server) authorizePublishSyncRequest(request *http.Request) bool {
	requiredSecret := strings.TrimSpace(server.cfg.PublishSyncSecret)
	if requiredSecret == "" {
		return true
	}

	providedSecret := strings.TrimSpace(request.Header.Get("X-Lazzerex-Publish-Secret"))
	if providedSecret == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(requiredSecret), []byte(providedSecret)) == 1
}

func (server *Server) dispatchLikeNotification(slug, postTitle string, likeCount int64) {
	if server.discordNotifier == nil || !server.discordNotifier.Enabled() {
		return
	}

	notificationSlug := strings.TrimSpace(strings.ToLower(slug))
	notificationTitle := strings.TrimSpace(postTitle)

	go func() {
		notifyCtx, notifyCancel := context.WithTimeout(context.Background(), server.cfg.DiscordRequestTimeout)
		defer notifyCancel()

		err := server.discordNotifier.NotifyLike(notifyCtx, discord.LikeNotification{
			Slug:      notificationSlug,
			PostTitle: notificationTitle,
			LikeCount: likeCount,
		})
		if err != nil {
			server.logger.Warn("discord like notification failed",
				"slug", notificationSlug,
				"error", err,
			)
			return
		}

		server.logger.Info("discord_like_notification_sent",
			"slug", notificationSlug,
			"likes_count", likeCount,
		)
	}()
}

func (server *Server) dispatchCommentNotification(comment store.Comment, postTitle string) {
	if server.discordNotifier == nil || !server.discordNotifier.Enabled() {
		return
	}

	notificationSlug := strings.TrimSpace(strings.ToLower(comment.Slug))
	notificationTitle := strings.TrimSpace(postTitle)
	notificationAuthor := strings.TrimSpace(comment.AuthorName)
	notificationBody := strings.TrimSpace(comment.Body)

	go func() {
		notifyCtx, notifyCancel := context.WithTimeout(context.Background(), server.cfg.DiscordRequestTimeout)
		defer notifyCancel()

		err := server.discordNotifier.NotifyComment(notifyCtx, discord.CommentNotification{
			Slug:       notificationSlug,
			PostTitle:  notificationTitle,
			AuthorName: notificationAuthor,
			Body:       notificationBody,
		})
		if err != nil {
			server.logger.Warn("discord comment notification failed",
				"slug", notificationSlug,
				"error", err,
			)
			return
		}

		server.logger.Info("discord_comment_notification_sent",
			"slug", notificationSlug,
			"author", notificationAuthor,
		)
	}()
}

func (server *Server) decodeJSONPayload(writer http.ResponseWriter, request *http.Request, destination any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, server.cfg.RequestBodyLimitBytes)

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		server.writeError(writer, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return false
	}

	if decoder.More() {
		server.writeError(writer, http.StatusBadRequest, "invalid_payload", "request body contains extra JSON tokens")
		return false
	}

	return true
}

func (server *Server) writeJSON(writer http.ResponseWriter, statusCode int, payload responseEnvelope) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)

	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		server.logger.Error("failed to encode response", "error", err, "status_code", statusCode)
	}
}

func (server *Server) writeError(writer http.ResponseWriter, statusCode int, code, message string) {
	server.writeJSON(writer, statusCode, responseEnvelope{
		OK: false,
		Error: &responseError{
			Code:    code,
			Message: message,
		},
	})
}

func (server *Server) withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, request)

		server.logger.Info("http_request",
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"remote_ip", resolveClientIP(request, server.cfg.TrustProxyHeaders),
		)
	})
}

func (server *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestOrigin := normalizeOriginValue(request.Header.Get("Origin"))
		if allowedOrigin, allowed := server.resolveAllowedOrigin(requestOrigin); allowed {
			writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			writer.Header().Add("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			writer.Header().Set("Access-Control-Max-Age", "600")
		}

		if request.Method == http.MethodOptions && requestOrigin != "" {
			writer.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(writer, request)
	})
}

func (server *Server) resolveAllowedOrigin(origin string) (string, bool) {
	if origin == "" {
		return "", false
	}

	if server.allowAnyOrigin {
		return "*", true
	}

	_, found := server.allowedOrigins[origin]
	if !found {
		return "", false
	}

	return origin, true
}

func buildAllowedOrigins(origins []string) (bool, map[string]struct{}) {
	allowedOrigins := make(map[string]struct{}, len(origins))
	for _, rawOrigin := range origins {
		normalizedOrigin := normalizeOriginValue(rawOrigin)
		if normalizedOrigin == "" {
			continue
		}

		if normalizedOrigin == "*" {
			return true, map[string]struct{}{}
		}

		allowedOrigins[normalizedOrigin] = struct{}{}
	}

	return false, allowedOrigins
}

func normalizeOriginValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "*" {
		return "*"
	}

	return strings.TrimRight(trimmed, "/")
}

func resolveClientIP(request *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if forwardedFor := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwardedFor != "" {
			parts := strings.Split(forwardedFor, ",")
			if len(parts) > 0 {
				candidate := strings.TrimSpace(parts[0])
				if candidate != "" {
					return candidate
				}
			}
		}

		if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	if request.RemoteAddr == "" {
		return "unknown"
	}

	return strings.TrimSpace(request.RemoteAddr)
}

func hashIP(ip string) string {
	hashed := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(hashed[:8])
}

func hashVisitorToken(visitorToken string) string {
	hashed := sha256.Sum256([]byte(strings.TrimSpace(visitorToken)))
	return hex.EncodeToString(hashed[:])
}
