package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"lazzerex-blog/internal/config"
	"lazzerex-blog/internal/db"
	"lazzerex-blog/internal/discord"
	"lazzerex-blog/internal/store"
)

func TestHealthEndpoint(t *testing.T) {
	server, cleanup := newTestServer(t, 5)
	defer cleanup()

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload responseEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !payload.OK {
		t.Fatalf("expected ok=true in health response")
	}
}

func TestViewsEndpointRateLimit(t *testing.T) {
	server, cleanup := newTestServer(t, 1)
	defer cleanup()

	requestOne := httptest.NewRequest(http.MethodPost, "/api/views/sample-post", nil)
	requestOne.RemoteAddr = "10.10.10.10:1234"
	responseOne := httptest.NewRecorder()

	server.Handler().ServeHTTP(responseOne, requestOne)
	if responseOne.Code != http.StatusOK {
		t.Fatalf("expected first views request status 200, got %d", responseOne.Code)
	}

	requestTwo := httptest.NewRequest(http.MethodPost, "/api/views/sample-post", nil)
	requestTwo.RemoteAddr = "10.10.10.10:4321"
	responseTwo := httptest.NewRecorder()

	server.Handler().ServeHTTP(responseTwo, requestTwo)
	if responseTwo.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second views request status 429, got %d", responseTwo.Code)
	}
}

func TestGetViewsEndpointReturnsCount(t *testing.T) {
	server, cleanup := newTestServer(t, 5)
	defer cleanup()

	requestOne := httptest.NewRequest(http.MethodPost, "/api/views/sample-post", nil)
	requestOne.RemoteAddr = "10.10.10.10:1234"
	recorderOne := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorderOne, requestOne)
	if recorderOne.Code != http.StatusOK {
		t.Fatalf("expected first post view request status 200, got %d", recorderOne.Code)
	}

	requestTwo := httptest.NewRequest(http.MethodPost, "/api/views/sample-post", nil)
	requestTwo.RemoteAddr = "10.10.10.11:4321"
	recorderTwo := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorderTwo, requestTwo)
	if recorderTwo.Code != http.StatusOK {
		t.Fatalf("expected second post view request status 200, got %d", recorderTwo.Code)
	}

	requestRead := httptest.NewRequest(http.MethodGet, "/api/views/sample-post", nil)
	recorderRead := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorderRead, requestRead)
	if recorderRead.Code != http.StatusOK {
		t.Fatalf("expected get view count status 200, got %d", recorderRead.Code)
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Slug  string `json:"slug"`
			Count int64  `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorderRead.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode get view count payload: %v", err)
	}

	if !payload.OK || payload.Data.Count != 2 {
		t.Fatalf("expected view count to be 2, got %+v", payload.Data)
	}
}

func TestTrackEndpointPersistsEvent(t *testing.T) {
	server, dbConn, cleanup := newTestServerWithDB(t, 5)
	defer cleanup()

	body := bytes.NewBufferString(`{"slug":"sample-post","event":"open_detail","metadata":{"source":"test"}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/track", body)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.168.1.15:3333"

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected track status 202, got %d", recorder.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var count int
	if err := dbConn.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM track_events
		WHERE slug = ? AND event = ?
	`, "sample-post", "open_detail").Scan(&count); err != nil {
		t.Fatalf("query track_events: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one track_event row, got %d", count)
	}
}

func TestCORSPreflightAllowsConfiguredOrigin(t *testing.T) {
	server, cleanup := newTestServer(t, 5)
	defer cleanup()

	request := httptest.NewRequest(http.MethodOptions, "/api/track", nil)
	request.Header.Set("Origin", "http://localhost:4321")
	request.Header.Set("Access-Control-Request-Method", "POST")

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected CORS preflight status 204, got %d", recorder.Code)
	}

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4321" {
		t.Fatalf("expected Access-Control-Allow-Origin header to be set, got %q", got)
	}
}

func TestReactionToggleLifecycle(t *testing.T) {
	server, cleanup := newTestServer(t, 5)
	defer cleanup()

	body := bytes.NewBufferString(`{"slug":"sample-post","visitorToken":"visitor_token_1234567890"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/reactions/toggle", body)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.50:4500"

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected first reaction toggle to return 200, got %d", recorder.Code)
	}

	var firstPayload struct {
		OK   bool                `json:"ok"`
		Data store.ReactionState `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &firstPayload); err != nil {
		t.Fatalf("decode first reaction payload: %v", err)
	}

	if !firstPayload.OK || !firstPayload.Data.Liked || firstPayload.Data.Count != 1 {
		t.Fatalf("unexpected first reaction state: %+v", firstPayload.Data)
	}

	bodyTwo := bytes.NewBufferString(`{"slug":"sample-post","visitorToken":"visitor_token_1234567890"}`)
	requestTwo := httptest.NewRequest(http.MethodPost, "/api/reactions/toggle", bodyTwo)
	requestTwo.Header.Set("Content-Type", "application/json")
	requestTwo.RemoteAddr = "203.0.113.50:4501"

	recorderTwo := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorderTwo, requestTwo)

	if recorderTwo.Code != http.StatusOK {
		t.Fatalf("expected second reaction toggle to return 200, got %d", recorderTwo.Code)
	}

	var secondPayload struct {
		OK   bool                `json:"ok"`
		Data store.ReactionState `json:"data"`
	}
	if err := json.Unmarshal(recorderTwo.Body.Bytes(), &secondPayload); err != nil {
		t.Fatalf("decode second reaction payload: %v", err)
	}

	if !secondPayload.OK || secondPayload.Data.Liked || secondPayload.Data.Count != 0 {
		t.Fatalf("unexpected second reaction state: %+v", secondPayload.Data)
	}
}

func TestCommentsCreateAndList(t *testing.T) {
	server, cleanup := newTestServer(t, 5)
	defer cleanup()

	createBody := bytes.NewBufferString(`{"slug":"sample-post","authorName":"Reader","body":"Great write-up on this topic."}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/comments", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.RemoteAddr = "198.51.100.23:3200"

	createRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create comment to return 201, got %d", createRecorder.Code)
	}

	var createPayload struct {
		OK   bool `json:"ok"`
		Data struct {
			Comment store.Comment `json:"comment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create comment payload: %v", err)
	}

	if !createPayload.OK || createPayload.Data.Comment.Body == "" {
		t.Fatalf("unexpected create comment payload: %+v", createPayload.Data.Comment)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/comments/sample-post", nil)
	listRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list comments to return 200, got %d", listRecorder.Code)
	}

	var listPayload struct {
		OK   bool `json:"ok"`
		Data struct {
			Slug     string          `json:"slug"`
			Comments []store.Comment `json:"comments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list comment payload: %v", err)
	}

	if !listPayload.OK || len(listPayload.Data.Comments) == 0 {
		t.Fatalf("expected at least one comment in list payload")
	}
}

func TestPublishedPostSyncSendsOneDiscordNotification(t *testing.T) {
	webhookBodies := make(chan string, 4)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		webhookBodies <- string(body)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer webhookServer.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notifier := discord.NewNotifier(webhookServer.URL, 2*time.Second, logger)

	server, cleanup := newTestServerWithNotifier(t, 5, notifier, "sync-secret")
	defer cleanup()

	body := bytes.NewBufferString(`{"slug":"sample-post","title":"Sample Post","summary":"A short summary."}`)
	request := httptest.NewRequest(http.MethodPost, "/api/blogs/published", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Lazzerex-Publish-Secret", "sync-secret")
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected first publish sync to return 200, got %d", recorder.Code)
	}

	firstBody := waitForWebhookBody(t, webhookBodies, "published post notification")
	if !strings.Contains(firstBody, "Sample Post") || !strings.Contains(firstBody, "A short summary.") {
		t.Fatalf("expected publish notification to include title and summary, got %s", firstBody)
	}

	bodySecond := bytes.NewBufferString(`{"slug":"sample-post","title":"Sample Post","summary":"A short summary."}`)
	requestSecond := httptest.NewRequest(http.MethodPost, "/api/blogs/published", bodySecond)
	requestSecond.Header.Set("Content-Type", "application/json")
	requestSecond.Header.Set("X-Lazzerex-Publish-Secret", "sync-secret")
	recorderSecond := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorderSecond, requestSecond)
	if recorderSecond.Code != http.StatusOK {
		t.Fatalf("expected second publish sync to return 200, got %d", recorderSecond.Code)
	}

	ensureNoWebhookBody(t, webhookBodies, "duplicate published post notification")
}

func TestLikeAndCommentSendDiscordNotifications(t *testing.T) {
	webhookBodies := make(chan string, 8)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		webhookBodies <- string(body)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer webhookServer.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notifier := discord.NewNotifier(webhookServer.URL, 2*time.Second, logger)

	server, cleanup := newTestServerWithNotifier(t, 5, notifier, "")
	defer cleanup()

	likeBody := bytes.NewBufferString(`{"slug":"sample-post","visitorToken":"visitor_token_1234567890","postTitle":"Sample Post"}`)
	likeRequest := httptest.NewRequest(http.MethodPost, "/api/reactions/toggle", likeBody)
	likeRequest.Header.Set("Content-Type", "application/json")
	likeRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(likeRecorder, likeRequest)
	if likeRecorder.Code != http.StatusOK {
		t.Fatalf("expected like toggle to return 200, got %d", likeRecorder.Code)
	}

	likeWebhook := waitForWebhookBody(t, webhookBodies, "like notification")
	if !strings.Contains(likeWebhook, "Sample Post") || !strings.Contains(likeWebhook, "New Like") {
		t.Fatalf("expected like notification payload to include post title and event label, got %s", likeWebhook)
	}

	unlikeBody := bytes.NewBufferString(`{"slug":"sample-post","visitorToken":"visitor_token_1234567890","postTitle":"Sample Post"}`)
	unlikeRequest := httptest.NewRequest(http.MethodPost, "/api/reactions/toggle", unlikeBody)
	unlikeRequest.Header.Set("Content-Type", "application/json")
	unlikeRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(unlikeRecorder, unlikeRequest)
	if unlikeRecorder.Code != http.StatusOK {
		t.Fatalf("expected unlike toggle to return 200, got %d", unlikeRecorder.Code)
	}

	ensureNoWebhookBody(t, webhookBodies, "unlike notification")

	commentBody := bytes.NewBufferString(`{"slug":"sample-post","authorName":"Reader","body":"Great write-up on this topic.","postTitle":"Sample Post"}`)
	commentRequest := httptest.NewRequest(http.MethodPost, "/api/comments", commentBody)
	commentRequest.Header.Set("Content-Type", "application/json")
	commentRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(commentRecorder, commentRequest)
	if commentRecorder.Code != http.StatusCreated {
		t.Fatalf("expected comment create to return 201, got %d", commentRecorder.Code)
	}

	commentWebhook := waitForWebhookBody(t, webhookBodies, "comment notification")
	if !strings.Contains(commentWebhook, "Sample Post") || !strings.Contains(commentWebhook, "Reader") || !strings.Contains(commentWebhook, "Great write-up on this topic.") {
		t.Fatalf("expected comment notification payload to include post/comment context, got %s", commentWebhook)
	}
}

func waitForWebhookBody(t *testing.T, webhookBodies <-chan string, label string) string {
	t.Helper()

	select {
	case body := <-webhookBodies:
		return body
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
		return ""
	}
}

func ensureNoWebhookBody(t *testing.T, webhookBodies <-chan string, label string) {
	t.Helper()

	select {
	case body := <-webhookBodies:
		t.Fatalf("expected no webhook for %s, got payload %s", label, body)
	case <-time.After(250 * time.Millisecond):
		return
	}
}

func newTestServer(t *testing.T, rateLimit int) (*Server, func()) {
	server, _, cleanup := newTestServerWithDependencies(t, rateLimit, nil, "")
	return server, cleanup
}

func newTestServerWithDB(t *testing.T, rateLimit int) (*Server, *sql.DB, func()) {
	return newTestServerWithDependencies(t, rateLimit, nil, "")
}

func newTestServerWithNotifier(t *testing.T, rateLimit int, notifier *discord.Notifier, publishSyncSecret string) (*Server, func()) {
	server, _, cleanup := newTestServerWithDependencies(t, rateLimit, notifier, publishSyncSecret)
	return server, cleanup
}

func newTestServerWithDependencies(t *testing.T, rateLimit int, notifier *discord.Notifier, publishSyncSecret string) (*Server, *sql.DB, func()) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")

	dbConn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.ApplyMigrations(ctx, dbConn, "sqlite"); err != nil {
		dbConn.Close()
		t.Fatalf("apply migrations: %v", err)
	}

	storeLayer := store.New(dbConn, logger)
	server := NewServer(config.Config{
		Address:               ":0",
		DBPath:                dbPath,
		LogLevel:              slog.LevelInfo,
		ViewsRateLimit:        rateLimit,
		ViewsRateWindow:       time.Minute,
		DiscordRequestTimeout: time.Second,
		PublishSyncSecret:     publishSyncSecret,
		ShutdownTimeout:       3 * time.Second,
		RequestBodyLimitBytes: 64 * 1024,
		TrustProxyHeaders:     false,
		AllowedOrigins:        []string{"http://localhost:4321", "http://127.0.0.1:4321"},
	}, logger, storeLayer, notifier)

	cleanup := func() {
		dbConn.Close()
	}

	return server, dbConn, cleanup
}
