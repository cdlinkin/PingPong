package messages

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeSummarizer struct {
	gotSystem string
	gotUser   string
	calls     int
	answer    string
	err       error
}

func (f *fakeSummarizer) Complete(ctx context.Context, system, user string) (string, error) {
	f.calls++
	f.gotSystem = system
	f.gotUser = user
	return f.answer, f.err
}

func TestSummarizeUnread(t *testing.T) {
	ctx := context.Background()
	const chatID, member, stranger = 1, 10, 99
	file := "plan.pdf"

	newRepo := func() *fakeRepo {
		repo := newFakeRepo()
		repo.members[chatID] = []int{member}
		repo.summaryLines = []SummaryLineModel{
			{Login: "anya", Message: "завтра созвон в 10?"},
			{Login: "oleg", Message: "давай в 11"},
			{Login: "anya", Message: "вот план", FileName: &file},
			{Login: "oleg", FileName: &file},
		}
		return repo
	}

	t.Run("ok", func(t *testing.T) {
		repo := newRepo()
		llm := &fakeSummarizer{answer: "- созвон в 11"}

		got, err := NewService(repo, newFakeStorage(), llm).SummarizeUnread(ctx, member, chatID)
		if err != nil {
			t.Fatalf("summarize: %v", err)
		}
		if got != "- созвон в 11" {
			t.Fatalf("want llm answer, got %q", got)
		}
		if repo.summaryLimit != summaryLimit {
			t.Fatalf("want limit %d, got %d", summaryLimit, repo.summaryLimit)
		}
		if llm.gotSystem != summaryPrompt {
			t.Fatal("system prompt must be summaryPrompt")
		}
		want := "anya: завтра созвон в 10?\noleg: давай в 11\nanya: [файл: plan.pdf] вот план\noleg: [файл: plan.pdf]\n"
		if llm.gotUser != want {
			t.Fatalf("wrong transcript:\n%q\nwant:\n%q", llm.gotUser, want)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		repo := newRepo()
		_, err := NewService(repo, newFakeStorage(), nil).SummarizeUnread(ctx, member, chatID)
		if !errors.Is(err, ErrSummaryDisabled) {
			t.Fatalf("want ErrSummaryDisabled, got %v", err)
		}
	})

	t.Run("stranger", func(t *testing.T) {
		llm := &fakeSummarizer{}
		_, err := NewService(newRepo(), newFakeStorage(), llm).SummarizeUnread(ctx, stranger, chatID)
		if !errors.Is(err, ErrChatNotFound) {
			t.Fatalf("want ErrChatNotFound, got %v", err)
		}
		if llm.calls != 0 {
			t.Fatal("llm must not be called for stranger")
		}
	})

	t.Run("nothing unread", func(t *testing.T) {
		repo := newRepo()
		repo.summaryLines = nil
		llm := &fakeSummarizer{}
		_, err := NewService(repo, newFakeStorage(), llm).SummarizeUnread(ctx, member, chatID)
		if !errors.Is(err, ErrNothingToSummarize) {
			t.Fatalf("want ErrNothingToSummarize, got %v", err)
		}
		if llm.calls != 0 {
			t.Fatal("llm must not be called without messages")
		}
	})

	t.Run("llm error is wrapped", func(t *testing.T) {
		llm := &fakeSummarizer{err: context.DeadlineExceeded}
		_, err := NewService(newRepo(), newFakeStorage(), llm).SummarizeUnread(ctx, member, chatID)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want wrapped DeadlineExceeded, got %v", err)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := newRepo()
		repo.summaryErr = errors.New("db down")
		_, err := NewService(repo, newFakeStorage(), &fakeSummarizer{}).SummarizeUnread(ctx, member, chatID)
		if err == nil || !strings.Contains(err.Error(), "db down") {
			t.Fatalf("want repo error, got %v", err)
		}
	})
}
