package messages

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestHandlerSendMessage(t *testing.T) {
	const pattern = "POST /chats/{id}/messages"

	t.Run("ok", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		rec := serve(pattern, h.SendMessage, http.MethodPost, "/chats/10/messages", `{"message":"  привет  "}`, member)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
		}
		var resp msgResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Message != "привет" || resp.ChatID != chatID || resp.UserID != member {
			t.Fatalf("unexpected message: %+v", resp)
		}
	})

	tests := []struct {
		name   string
		path   string
		userID int
		body   string
		want   int
		text   string
	}{
		{"unauthorized", "/chats/10/messages", 0, `{"message":"привет"}`, http.StatusUnauthorized, "unauthorized"},
		{"bad id", "/chats/abc/messages", member, `{"message":"привет"}`, http.StatusBadRequest, "invalid chat id"},
		{"bad json", "/chats/10/messages", member, `{`, http.StatusBadRequest, "invalid JSON"},
		{"empty text", "/chats/10/messages", member, `{"message":"  "}`, http.StatusBadRequest, "message must be 1-500 symbols"},
		{"stranger", "/chats/10/messages", stranger, `{"message":"привет"}`, http.StatusNotFound, "chat not found"},
		{"missing reply", "/chats/10/messages", member, `{"message":"привет","reply_to_id":99}`, http.StatusNotFound, "reply message not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMsgHandler(newFakeRepo(), nil, nil)
			rec := serve(pattern, h.SendMessage, http.MethodPost, tt.path, tt.body, tt.userID)
			if rec.Code != tt.want {
				t.Fatalf("want %d, got %d: %s", tt.want, rec.Code, rec.Body)
			}
			if tt.text != "" && !strings.Contains(rec.Body.String(), tt.text) {
				t.Fatalf("want body to contain %q, got %q", tt.text, rec.Body.String())
			}
		})
	}
}

func TestHandlerListMessage(t *testing.T) {
	const pattern = "GET /chats/{id}/messages"

	t.Run("ok", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listed = []MessageModel{{ID: 4, ChatID: chatID, UserID: member, Message: "раньше"}}
		h := newMsgHandler(repo, nil, nil)
		rec := serve(pattern, h.ListMessage, http.MethodGet, "/chats/10/messages?before=20&limit=5", "", member)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp []msgResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 1 || resp[0].Message != "раньше" {
			t.Fatalf("unexpected page: %+v", resp)
		}
		if repo.gotBeforeID != 20 || repo.gotLimit != 5 {
			t.Fatalf("want before 20 limit 5, got before %d limit %d", repo.gotBeforeID, repo.gotLimit)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		repo := newFakeRepo()
		h := newMsgHandler(repo, nil, nil)
		rec := serve(pattern, h.ListMessage, http.MethodGet, "/chats/10/messages", "", member)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		if repo.gotBeforeID != math.MaxInt32 || repo.gotLimit != 50 {
			t.Fatalf("want default cursor, got before %d limit %d", repo.gotBeforeID, repo.gotLimit)
		}
	})

	tests := []struct {
		name   string
		path   string
		userID int
		want   int
	}{
		{"unauthorized", "/chats/10/messages", 0, http.StatusUnauthorized},
		{"bad id", "/chats/abc/messages", member, http.StatusBadRequest},
		{"bad before", "/chats/10/messages?before=no", member, http.StatusBadRequest},
		{"bad limit", "/chats/10/messages?limit=no", member, http.StatusBadRequest},
		{"stranger", "/chats/10/messages", stranger, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMsgHandler(newFakeRepo(), nil, nil)
			rec := serve(pattern, h.ListMessage, http.MethodGet, tt.path, "", tt.userID)
			if rec.Code != tt.want {
				t.Fatalf("want %d, got %d: %s", tt.want, rec.Code, rec.Body)
			}
		})
	}
}

func TestHandlerSearchMessages(t *testing.T) {
	const pattern = "GET /chats/{id}/messages/search"

	t.Run("ok", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listed = []MessageModel{{ID: 4, ChatID: chatID, UserID: member, Message: "привет go"}}
		h := newMsgHandler(repo, nil, nil)
		rec := serve(pattern, h.SearchMessages, http.MethodGet, "/chats/10/messages/search?q=%20go%20", "", member)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp []msgResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 1 || resp[0].Message != "привет go" {
			t.Fatalf("unexpected hits: %+v", resp)
		}
		if repo.gotQuery != "go" {
			t.Fatalf("want trimmed query, got %q", repo.gotQuery)
		}
	})

	tests := []struct {
		name   string
		path   string
		userID int
		want   int
	}{
		{"unauthorized", "/chats/10/messages/search?q=go", 0, http.StatusUnauthorized},
		{"bad id", "/chats/abc/messages/search?q=go", member, http.StatusBadRequest},
		{"empty query", "/chats/10/messages/search?q=%20", member, http.StatusBadRequest},
		{"stranger", "/chats/10/messages/search?q=go", stranger, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMsgHandler(newFakeRepo(), nil, nil)
			rec := serve(pattern, h.SearchMessages, http.MethodGet, tt.path, "", tt.userID)
			if rec.Code != tt.want {
				t.Fatalf("want %d, got %d: %s", tt.want, rec.Code, rec.Body)
			}
		})
	}
}

func TestHandlerEditMessage(t *testing.T) {
	const pattern = "PATCH /chats/{id}/messages/{msgID}"

	t.Run("ok", func(t *testing.T) {
		note := &fakeNotifier{}
		h := newMsgHandler(newFakeRepo(), nil, note)
		rec := serve(pattern, h.EditMessage, http.MethodPatch, "/chats/10/messages/4", `{"message":"  новый  "}`, member)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp msgResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID != 4 || resp.Message != "новый" {
			t.Fatalf("unexpected message: %+v", resp)
		}
		if note.lastType() != "message.edited" || len(note.events) != 2 {
			t.Fatalf("want edit event for both members, got %d events, last %q", len(note.events), note.lastType())
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := newFakeRepo()
		repo.updateErr = ErrMessageNotFound
		h := newMsgHandler(repo, nil, nil)
		rec := serve(pattern, h.EditMessage, http.MethodPatch, "/chats/10/messages/4", `{"message":"новый"}`, member)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})

	tests := []struct {
		name   string
		path   string
		userID int
		body   string
		want   int
	}{
		{"unauthorized", "/chats/10/messages/4", 0, `{"message":"новый"}`, http.StatusUnauthorized},
		{"bad message id", "/chats/10/messages/abc", member, `{"message":"новый"}`, http.StatusBadRequest},
		{"empty text", "/chats/10/messages/4", member, `{"message":" "}`, http.StatusBadRequest},
		{"stranger", "/chats/10/messages/4", stranger, `{"message":"новый"}`, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMsgHandler(newFakeRepo(), nil, nil)
			rec := serve(pattern, h.EditMessage, http.MethodPatch, tt.path, tt.body, tt.userID)
			if rec.Code != tt.want {
				t.Fatalf("want %d, got %d: %s", tt.want, rec.Code, rec.Body)
			}
		})
	}
}

func TestHandlerDeleteMessage(t *testing.T) {
	const pattern = "DELETE /chats/{id}/messages/{msgID}"

	t.Run("ok", func(t *testing.T) {
		note := &fakeNotifier{}
		h := newMsgHandler(newFakeRepo(), nil, note)
		rec := serve(pattern, h.DeleteMessage, http.MethodDelete, "/chats/10/messages/4", "", member)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body)
		}
		if note.lastType() != "message.deleted" {
			t.Fatalf("want delete event, got %q", note.lastType())
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := newFakeRepo()
		repo.deleteErr = ErrMessageNotFound
		h := newMsgHandler(repo, nil, nil)
		rec := serve(pattern, h.DeleteMessage, http.MethodDelete, "/chats/10/messages/4", "", member)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})

	tests := []struct {
		name   string
		path   string
		userID int
		want   int
	}{
		{"unauthorized", "/chats/10/messages/4", 0, http.StatusUnauthorized},
		{"bad message id", "/chats/10/messages/abc", member, http.StatusBadRequest},
		{"stranger", "/chats/10/messages/4", stranger, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMsgHandler(newFakeRepo(), nil, nil)
			rec := serve(pattern, h.DeleteMessage, http.MethodDelete, tt.path, "", tt.userID)
			if rec.Code != tt.want {
				t.Fatalf("want %d, got %d: %s", tt.want, rec.Code, rec.Body)
			}
		})
	}
}

func TestHandlerReads(t *testing.T) {
	t.Run("mark", func(t *testing.T) {
		repo := newFakeRepo()
		repo.lastReadID = 8
		note := &fakeNotifier{}
		h := newMsgHandler(repo, nil, note)
		rec := serve("POST /chats/{id}/read", h.MarkRead, http.MethodPost, "/chats/10/read", `{"message_id":8}`, member)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body)
		}
		if repo.gotMarkID != 8 {
			t.Fatalf("want mark id 8, got %d", repo.gotMarkID)
		}
		if note.lastType() != "message.read" {
			t.Fatalf("want read event, got %q", note.lastType())
		}
	})

	t.Run("mark invalid", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		rec := serve("POST /chats/{id}/read", h.MarkRead, http.MethodPost, "/chats/10/read", `{"message_id":0}`, member)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("list", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		rec := serve("GET /chats/{id}/reads", h.ListReads, http.MethodGet, "/chats/10/reads", "", member)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		var resp []readResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 1 || resp[0].UserID != member || resp[0].LastReadID != 7 {
			t.Fatalf("unexpected reads: %+v", resp)
		}
	})

	t.Run("stranger", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		rec := serve("GET /chats/{id}/reads", h.ListReads, http.MethodGet, "/chats/10/reads", "", stranger)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})
}

func TestHandlerToggleReaction(t *testing.T) {
	const pattern = "POST /chats/{id}/messages/{msgID}/reactions"

	repo := newFakeRepo()
	repo.chatMessages = map[int][]int{chatID: {4}}
	note := &fakeNotifier{}
	h := newMsgHandler(repo, nil, note)

	rec := serve(pattern, h.ToggleReaction, http.MethodPost, "/chats/10/messages/4/reactions", `{"emoji":"👍"}`, member)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var resp []reactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 || resp[0].Emoji != "👍" || len(resp[0].UserIDs) != 1 {
		t.Fatalf("unexpected reactions: %+v", resp)
	}
	if note.lastType() != "message.reactions" {
		t.Fatalf("want reaction event, got %q", note.lastType())
	}

	rec = serve(pattern, h.ToggleReaction, http.MethodPost, "/chats/10/messages/4/reactions", `{"emoji":"nope"}`, member)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad emoji: want 400, got %d: %s", rec.Code, rec.Body)
	}

	rec = serve(pattern, h.ToggleReaction, http.MethodPost, "/chats/10/messages/9/reactions", `{"emoji":"👍"}`, member)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing message: want 404, got %d: %s", rec.Code, rec.Body)
	}
}
