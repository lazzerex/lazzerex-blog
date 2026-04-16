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
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"lazzerex-blog/internal/config"
	"lazzerex-blog/internal/db"
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

func newTestServer(t *testing.T, rateLimit int) (*Server, func()) {
	server, _, cleanup := newTestServerWithDB(t, rateLimit)
	return server, cleanup
}

func newTestServerWithDB(t *testing.T, rateLimit int) (*Server, *sql.DB, func()) {
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
		ShutdownTimeout:       3 * time.Second,
		RequestBodyLimitBytes: 64 * 1024,
		TrustProxyHeaders:     false,
		AllowedOrigins:        []string{"http://localhost:4321", "http://127.0.0.1:4321"},
	}, logger, storeLayer)

	cleanup := func() {
		dbConn.Close()
	}

	return server, dbConn, cleanup
}
