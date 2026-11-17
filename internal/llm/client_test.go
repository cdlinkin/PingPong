package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestComplete(t *testing.T) {
	var got chatRequest
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  - созвон в 11  \n"}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL+"/v1/", "qwen2.5:3b", "secret", time.Second)
	answer, err := c.Complete(context.Background(), "перескажи", "anya: привет")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if answer != "- созвон в 11" {
		t.Fatalf("want trimmed answer, got %q", answer)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("wrong path %q", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("wrong auth header %q", gotAuth)
	}
	if got.Model != "qwen2.5:3b" || got.Stream {
		t.Fatalf("wrong request %+v", got)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != "system" || got.Messages[1].Content != "anya: привет" {
		t.Fatalf("wrong messages %+v", got.Messages)
	}
}

func TestCompleteNoKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := r.Header.Get("Authorization"); h != "" {
			t.Errorf("want no auth header, got %q", h)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "m", "", time.Second).Complete(context.Background(), "s", "u"); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

func TestCompleteErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
		wantMsg string
	}{
		{"server error", http.StatusInternalServerError, `model not found`, nil, "llm status 500: model not found"},
		{"no choices", http.StatusOK, `{"choices":[]}`, ErrEmptyResponse, ""},
		{"empty content", http.StatusOK, `{"choices":[{"message":{"content":"  "}}]}`, ErrEmptyResponse, ""},
		{"bad json", http.StatusOK, `{"choices":`, nil, "decode response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := New(srv.URL, "m", "", time.Second).Complete(context.Background(), "s", "u")
			if err == nil {
				t.Fatal("want error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("want %v, got %v", tt.wantErr, err)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("want %q in error, got %v", tt.wantMsg, err)
			}
		})
	}
}

func TestCompleteTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := New(srv.URL, "m", "", time.Second).Complete(ctx, "s", "u")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
}
