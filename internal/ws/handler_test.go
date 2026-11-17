package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cdlinkin/pingpong/internal/messages"
	"github.com/cdlinkin/pingpong/internal/ratelimit"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

type wsRepo struct{}

func (wsRepo) IsMember(context.Context, int, int) (bool, error) { return true, nil }

func (wsRepo) CreateMessage(_ context.Context, chatID, userID int, message string, replyToID *int) (messages.MessageModel, error) {
	return messages.MessageModel{
		ID:        1,
		ChatID:    chatID,
		UserID:    userID,
		Message:   message,
		ReplyToID: replyToID,
	}, nil
}

func (wsRepo) ListMessage(context.Context, int, int, int) ([]messages.MessageModel, error) {
	return nil, nil
}

func (wsRepo) SearchMessages(context.Context, int, string) ([]messages.MessageModel, error) {
	return nil, nil
}

func (wsRepo) ListMemberIDs(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

func (wsRepo) UpdateMessage(context.Context, int, int, int, string) (messages.MessageModel, error) {
	return messages.MessageModel{}, nil
}

func (wsRepo) DeleteMessage(context.Context, int, int, int) error { return nil }

func (wsRepo) MarkRead(context.Context, int, int, int) (int, error) { return 0, nil }

func (wsRepo) ListReads(context.Context, int) ([]messages.ReadModel, error) { return nil, nil }

func (wsRepo) MessageInChat(context.Context, int, int) (bool, error) { return false, nil }

func (wsRepo) RemoveReaction(context.Context, int, int, string) (bool, error) { return false, nil }

func (wsRepo) UpsertReaction(context.Context, int, int, string) error { return nil }

func (wsRepo) ListReactions(context.Context, []int) ([]messages.ReactionModel, error) {
	return nil, nil
}

func (wsRepo) CreateAttachmentMessage(context.Context, int, int, string, *int, messages.AttachmentModel) (messages.MessageModel, error) {
	return messages.MessageModel{}, nil
}

func (wsRepo) ListAttachments(context.Context, []int) ([]messages.AttachmentModel, error) {
	return nil, nil
}

func (wsRepo) GetAttachment(context.Context, int) (messages.AttachmentModel, error) {
	return messages.AttachmentModel{}, nil
}

func (wsRepo) AddMentions(context.Context, int, int, int, []string) ([]int, error) {
	return nil, nil
}

func (wsRepo) ListMentions(context.Context, []int) ([]messages.MentionModel, error) {
	return nil, nil
}

func (wsRepo) ListUnreadForSummary(context.Context, int, int, int) ([]messages.SummaryLineModel, error) {
	return nil, nil
}

type wsStorage struct{}

func (wsStorage) Save(context.Context, io.Reader) (string, int64, error) { return "", 0, nil }

func (wsStorage) Open(context.Context, string) (io.ReadSeekCloser, error) { return nil, nil }

func (wsStorage) Remove(context.Context, string) error { return nil }

func testToken(t *testing.T, secret []byte, userID string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	})
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestServeWSRejectsToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler([]byte("secret"), NewHub(), nil, ratelimit.New(rate.Inf, 1), logger)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	h.ServeWS(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", rec.Code, rec.Body)
	}
}

func TestServeWSSendMessage(t *testing.T) {
	secret := []byte("secret")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := messages.NewService(wsRepo{}, wsStorage{}, nil)
	h := NewHandler(secret, NewHub(), svc, ratelimit.New(rate.Inf, 1), logger)

	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=" + testToken(t, secret, "1")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"chat_id":10,"message":"привет"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got outgoingMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Message != "привет" || got.ChatID != 10 || got.UserID != 1 {
		t.Fatalf("unexpected message: %+v", got)
	}
}
