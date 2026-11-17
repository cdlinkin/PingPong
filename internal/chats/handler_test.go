package chats

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cdlinkin/pingpong/internal/auth"
)

func newChatsHandler(repo *fakeRepo) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(NewService(repo), logger)
}

func serveChat(pattern string, h http.HandlerFunc, method, path, body string, userID int) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, h)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != 0 {
		req = req.WithContext(auth.ContextWithUserID(req.Context(), userID))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandlerChats(t *testing.T) {
	const userID = 1

	t.Run("create group", func(t *testing.T) {
		h := newChatsHandler(newFakeRepo())
		rec := serveChat("POST /chats/group", h.CreateGroupChat, http.MethodPost, "/chats/group", `{"title":"  Go team  "}`, userID)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
		}
		var resp createChatResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID == 0 {
			t.Fatal("want chat id")
		}
	})

	t.Run("empty title", func(t *testing.T) {
		h := newChatsHandler(newFakeRepo())
		rec := serveChat("POST /chats/group", h.CreateGroupChat, http.MethodPost, "/chats/group", `{"title":"  "}`, userID)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		h := newChatsHandler(newFakeRepo())
		rec := serveChat("POST /chats/group", h.CreateGroupChat, http.MethodPost, "/chats/group", `{"title":"Go"}`, 0)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("direct chat", func(t *testing.T) {
		h := newChatsHandler(newFakeRepo())
		rec := serveChat("POST /chats/direct", h.CreateDirectChat, http.MethodPost, "/chats/direct", `{"user_id":2}`, userID)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
		}
		rec = serveChat("POST /chats/direct", h.CreateDirectChat, http.MethodPost, "/chats/direct", `{"user_id":2}`, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("second call: want 200, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("direct with yourself", func(t *testing.T) {
		h := newChatsHandler(newFakeRepo())
		rec := serveChat("POST /chats/direct", h.CreateDirectChat, http.MethodPost, "/chats/direct", `{"user_id":1}`, userID)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("direct missing user", func(t *testing.T) {
		repo := newFakeRepo()
		repo.createDirectErr = ErrUserNotFound
		h := newChatsHandler(repo)
		rec := serveChat("POST /chats/direct", h.CreateDirectChat, http.MethodPost, "/chats/direct", `{"user_id":2}`, userID)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("list", func(t *testing.T) {
		repo := newFakeRepo()
		h := newChatsHandler(repo)
		if _, err := NewService(repo).CreateGroupChat(t.Context(), "Go", userID); err != nil {
			t.Fatalf("create: %v", err)
		}
		rec := serveChat("GET /chats", h.ListUserChats, http.MethodGet, "/chats", "", userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp []chatResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("want 1 chat, got %+v", resp)
		}
	})
}

func TestHandlerMembers(t *testing.T) {
	const admin, member, other = 1, 2, 4

	newGroup := func(t *testing.T) (*Handler, *fakeRepo, int) {
		t.Helper()
		repo := newFakeRepo()
		svc := NewService(repo)
		chatID, err := svc.CreateGroupChat(t.Context(), "Go", admin)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		repo.roles[memberKey{chatID, member}] = "member"
		return newChatsHandler(repo), repo, chatID
	}

	t.Run("add", func(t *testing.T) {
		h, repo, chatID := newGroup(t)
		path := "/chats/1/members"
		if chatID != 1 {
			t.Fatalf("want chat 1, got %d", chatID)
		}
		rec := serveChat("POST /chats/{id}/members", h.AddMember, http.MethodPost, path, `{"user_id":4}`, admin)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body)
		}
		if repo.roles[memberKey{chatID, other}] != "member" {
			t.Fatal("user was not added")
		}

		rec = serveChat("POST /chats/{id}/members", h.AddMember, http.MethodPost, path, `{"user_id":5}`, member)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("member add: want 403, got %d: %s", rec.Code, rec.Body)
		}

		rec = serveChat("POST /chats/{id}/members", h.AddMember, http.MethodPost, path, `{"user_id":2}`, admin)
		if rec.Code != http.StatusConflict {
			t.Fatalf("duplicate: want 409, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("remove", func(t *testing.T) {
		h, _, chatID := newGroup(t)
		if chatID != 1 {
			t.Fatalf("want chat 1, got %d", chatID)
		}
		rec := serveChat("DELETE /chats/{id}/members/{userID}", h.RemoveMember, http.MethodDelete, "/chats/1/members/1", "", admin)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("self: want 400, got %d: %s", rec.Code, rec.Body)
		}

		rec = serveChat("DELETE /chats/{id}/members/{userID}", h.RemoveMember, http.MethodDelete, "/chats/1/members/2", "", admin)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body)
		}

		rec = serveChat("DELETE /chats/{id}/members/{userID}", h.RemoveMember, http.MethodDelete, "/chats/1/members/9", "", admin)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("missing: want 404, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("list and mute", func(t *testing.T) {
		h, repo, chatID := newGroup(t)
		rec := serveChat("GET /chats/{id}/members", h.ListMembers, http.MethodGet, "/chats/1/members", "", admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp []memberResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 2 {
			t.Fatalf("want 2 members, got %+v", resp)
		}

		rec = serveChat("PUT /chats/{id}/mute", h.SetMuted, http.MethodPut, "/chats/1/mute", `{"muted":true}`, admin)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("mute: want 204, got %d: %s", rec.Code, rec.Body)
		}
		if !repo.muted[memberKey{chatID, admin}] {
			t.Fatal("chat was not muted")
		}

		rec = serveChat("PUT /chats/{id}/mute", h.SetMuted, http.MethodPut, "/chats/1/mute", `{"muted":true}`, 9)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("stranger mute: want 404, got %d: %s", rec.Code, rec.Body)
		}
	})
}
