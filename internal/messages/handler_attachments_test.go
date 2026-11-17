package messages

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cdlinkin/pingpong/internal/auth"
)

func attachmentRequest(t *testing.T, path string, userID int, field string, fileName string, data []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if field != "" {
		part, err := w.CreateFormFile(field, fileName)
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if userID != 0 {
		req = req.WithContext(auth.ContextWithUserID(req.Context(), userID))
	}
	return req
}

func TestHandlerSendAttachment(t *testing.T) {
	const pattern = "POST /chats/{id}/attachments"

	t.Run("ok", func(t *testing.T) {
		note := &fakeNotifier{}
		h := newMsgHandler(newFakeRepo(), newFakeStorage(), note)
		req := attachmentRequest(t, "/chats/10/attachments", member, "file", "schema.png", pngHeader)
		rec := serveReq(pattern, h.SendAttachment, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
		}
		var resp msgResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Attachment == nil || resp.Attachment.Name != "schema.png" || resp.Attachment.ContentType != "image/png" {
			t.Fatalf("unexpected attachment: %+v", resp.Attachment)
		}
		if len(note.events) != 2 {
			t.Fatalf("want broadcast to both members, got %d", len(note.events))
		}
	})

	t.Run("missing file", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		req := attachmentRequest(t, "/chats/10/attachments", member, "", "", nil)
		rec := serveReq(pattern, h.SendAttachment, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		req := attachmentRequest(t, "/chats/10/attachments", member, "file", "empty.png", nil)
		rec := serveReq(pattern, h.SendAttachment, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		req := attachmentRequest(t, "/chats/10/attachments", member, "file", "bin.dat", []byte{0x00, 0x01, 0x02, 0x03})
		rec := serveReq(pattern, h.SendAttachment, req)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("want 415, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("stranger", func(t *testing.T) {
		h := newMsgHandler(newFakeRepo(), nil, nil)
		req := attachmentRequest(t, "/chats/10/attachments", stranger, "file", "schema.png", pngHeader)
		rec := serveReq(pattern, h.SendAttachment, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})
}

func TestHandlerGetAttachment(t *testing.T) {
	const pattern = "GET /attachments/{id}"

	store := newFakeStorage()
	store.dir = t.TempDir()
	store.files["key1"] = append([]byte(nil), pngHeader...)

	repo := newFakeRepo()
	repo.attachments = []AttachmentModel{{
		ID:          7,
		MsgID:       4,
		ChatID:      chatID,
		FileName:    "schema.png",
		ContentType: "image/png",
		Size:        int64(len(pngHeader)),
		StorageKey:  "key1",
	}}
	h := newMsgHandler(repo, store, nil)

	t.Run("ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/attachments/7", nil)
		req = req.WithContext(auth.ContextWithUserID(req.Context(), member))
		rec := serveReq(pattern, h.GetAttachment, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		if rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("want image/png, got %q", rec.Header().Get("Content-Type"))
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("want nosniff")
		}
		if got := rec.Header().Get("Content-Disposition"); !bytes.Contains([]byte(got), []byte("inline")) {
			t.Fatalf("want inline disposition, got %q", got)
		}
	})

	t.Run("stranger", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/attachments/7", nil)
		req = req.WithContext(auth.ContextWithUserID(req.Context(), stranger))
		rec := serveReq(pattern, h.GetAttachment, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("bad id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/attachments/abc", nil)
		req = req.WithContext(auth.ContextWithUserID(req.Context(), member))
		rec := serveReq(pattern, h.GetAttachment, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
	})
}
