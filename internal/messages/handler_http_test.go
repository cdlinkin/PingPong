package messages

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cdlinkin/pingpong/internal/auth"
)

type fakeNotifier struct {
	userIDs []int
	events  [][]byte
}

func (n *fakeNotifier) SendToUser(userID int, data []byte) {
	n.userIDs = append(n.userIDs, userID)
	n.events = append(n.events, append([]byte(nil), data...))
}

func (n *fakeNotifier) lastType() string {
	if len(n.events) == 0 {
		return ""
	}
	var event map[string]any
	if err := json.Unmarshal(n.events[len(n.events)-1], &event); err != nil {
		return ""
	}
	kind, _ := event["type"].(string)
	return kind
}

func newMsgHandler(repo *fakeRepo, store FileStorage, n Notifier) *Handler {
	if store == nil {
		store = newFakeStorage()
	}
	if n == nil {
		n = &fakeNotifier{}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(NewService(repo, store, nil), n, logger)
}

func serve(
	pattern string,
	h http.HandlerFunc,
	method, path, body string,
	userID int,
) *httptest.ResponseRecorder {
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

func serveReq(pattern string, h http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, h)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
