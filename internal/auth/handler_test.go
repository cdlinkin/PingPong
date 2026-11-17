package auth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) (*Handler, *Service) {
	t.Helper()

	svc := NewService(newFakeRepo(), testSecret)
	if _, err := svc.Register(context.Background(), "kuroame", "kuro@mail.ru", "password123"); err != nil {
		t.Fatalf("register: %v", err)
	}

	return NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))), svc
}

func doJSON(h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func decodeTokens(t *testing.T, rec *httptest.ResponseRecorder) loginResponse {
	t.Helper()

	var resp loginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatalf("want both tokens, got %+v", resp)
	}
	return resp
}

func TestHandlerLogin(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := doJSON(h.Login, "/login", `{"email":"kuro@mail.ru","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	resp := decodeTokens(t, rec)

	if _, err := ParseToken(resp.Token, testSecret); err != nil {
		t.Fatalf("access token is invalid: %v", err)
	}

	rec = doJSON(h.Login, "/login", `{"email":"kuro@mail.ru","password":"wrong-pass"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: want 401, got %d", rec.Code)
	}
}

func TestHandlerRefresh(t *testing.T) {
	h, svc := newTestHandler(t)

	tokens, err := svc.Login(context.Background(), "kuro@mail.ru", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	rec := doJSON(h.Refresh, "/refresh", `{"refresh_token":"`+tokens.Refresh+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	resp := decodeTokens(t, rec)
	if resp.RefreshToken == tokens.Refresh {
		t.Fatal("refresh token must rotate")
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"old token after rotation", `{"refresh_token":"` + tokens.Refresh + `"}`, http.StatusUnauthorized},
		{"unknown token", `{"refresh_token":"not-a-token"}`, http.StatusUnauthorized},
		{"empty token", `{}`, http.StatusUnauthorized},
		{"bad json", `{"refresh_token":`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(h.Refresh, "/refresh", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("want %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body)
			}
		})
	}

	rec = doJSON(h.Refresh, "/refresh", `{"refresh_token":"`+resp.RefreshToken+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("new token: want 200, got %d", rec.Code)
	}
}

func TestHandlerLogout(t *testing.T) {
	h, svc := newTestHandler(t)

	tokens, err := svc.Login(context.Background(), "kuro@mail.ru", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	body := `{"refresh_token":"` + tokens.Refresh + `"}`

	rec := doJSON(h.Logout, "/logout", body)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", rec.Code, rec.Body)
	}

	rec = doJSON(h.Refresh, "/refresh", body)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: want 401, got %d", rec.Code)
	}

	rec = doJSON(h.Logout, "/logout", body)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("second logout: want 204, got %d", rec.Code)
	}

	rec = doJSON(h.Logout, "/logout", `nope`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json: want 400, got %d", rec.Code)
	}
}

func TestHandlerRegister(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := doJSON(h.Register, "/register", `{"login":"sonya","email":"sonya@mail.ru","password":"password123"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
	}
	var resp registerResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID == 0 {
		t.Fatal("want user id")
	}

	rec = doJSON(h.Register, "/register", `{"login":"sonya","email":"sonya@mail.ru","password":"password123"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: want 409, got %d: %s", rec.Code, rec.Body)
	}

	rec = doJSON(h.Register, "/register", `{`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json: want 400, got %d", rec.Code)
	}

	rec = doJSON(h.Register, "/register", `{"login":"ab","email":"ab@mail.ru","password":"password123"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short login: want 400, got %d: %s", rec.Code, rec.Body)
	}

	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	got := httptest.NewRecorder()
	h.Register(got, req)
	if got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", got.Code)
	}
}

func TestHandlerMe(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = req.WithContext(ContextWithUserID(req.Context(), 1))
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var resp meResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Login != "kuroame" || resp.Email != "kuro@mail.ru" {
		t.Fatalf("unexpected profile: %+v", resp)
	}

	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	rec = httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req = req.WithContext(ContextWithUserID(req.Context(), 999))
	rec = httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
	}
}
