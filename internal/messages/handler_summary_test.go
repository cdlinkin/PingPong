package messages

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdlinkin/pingpong/internal/auth"
)

func newSummaryHandler(repo *fakeRepo, llm Summarizer) *Handler {
	svc := NewService(repo, newFakeStorage(), llm)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(svc, nil, logger)
}

func postSummary(h *Handler, path string, userID int) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /chats/{id}/summary", h.SummarizeUnread)

	req := httptest.NewRequest(http.MethodPost, path, nil)
	if userID != 0 {
		req = req.WithContext(auth.ContextWithUserID(req.Context(), userID))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandlerSummarizeUnread(t *testing.T) {
	const chatID, member = 1, 10

	newRepo := func() *fakeRepo {
		repo := newFakeRepo()
		repo.members[chatID] = []int{member}
		repo.summaryLines = []SummaryLineModel{
			{Login: "anya", Message: "завтра созвон в 10?"},
		}
		return repo
	}

	t.Run("ok", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), &fakeSummarizer{answer: "- созвон в 10"})
		rec := postSummary(h, "/chats/1/summary", member)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp summaryResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Summary != "- созвон в 10" {
			t.Fatalf("want summary, got %q", resp.Summary)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), &fakeSummarizer{answer: "x"})
		rec := postSummary(h, "/chats/1/summary", 0)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d", rec.Code)
		}
	})

	t.Run("bad id", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), &fakeSummarizer{answer: "x"})
		rec := postSummary(h, "/chats/abc/summary", member)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("stranger", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), &fakeSummarizer{answer: "x"})
		rec := postSummary(h, "/chats/1/summary", 99)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("nothing unread", func(t *testing.T) {
		repo := newRepo()
		repo.summaryLines = nil
		h := newSummaryHandler(repo, &fakeSummarizer{answer: "x"})
		rec := postSummary(h, "/chats/1/summary", member)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), nil)
		rec := postSummary(h, "/chats/1/summary", member)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("want 503, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), &fakeSummarizer{err: context.DeadlineExceeded})
		rec := postSummary(h, "/chats/1/summary", member)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("want 504, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("llm down", func(t *testing.T) {
		h := newSummaryHandler(newRepo(), &fakeSummarizer{err: errors.New("connection refused")})
		rec := postSummary(h, "/chats/1/summary", member)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("want 502, got %d: %s", rec.Code, rec.Body)
		}
	})
}
