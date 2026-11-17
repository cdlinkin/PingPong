//go:build integration

package messages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cdlinkin/pingpong/internal/chats"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryConversation(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)
	repo := NewRepository(pool)
	chatsRepo := chats.NewRepository(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	login := "t" + suffix[len(suffix)-10:]
	var userID int
	err := pool.QueryRow(
		ctx,
		`INSERT INTO users (login, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id`,
		login,
		login+"@test.local",
		strings.Repeat("x", 60),
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var chatID int
	t.Cleanup(func() {
		cleanCtx := context.Background()
		if chatID != 0 {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM messages WHERE chat_id = $1`, chatID)
			_, _ = pool.Exec(cleanCtx, `DELETE FROM chat_members WHERE chat_id = $1`, chatID)
			_, _ = pool.Exec(cleanCtx, `DELETE FROM chats WHERE id = $1`, chatID)
		}
		_, _ = pool.Exec(cleanCtx, `DELETE FROM users WHERE id = $1`, userID)
	})

	chatID, err = chatsRepo.CreateGroupChat(ctx, "integration", userID)
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}

	created, err := repo.CreateMessage(ctx, chatID, userID, "созвон завтра", nil)
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	page, err := repo.ListMessage(ctx, chatID, int(^uint(0)>>1), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 || page[0].ID != created.ID || page[0].Message != "созвон завтра" {
		t.Fatalf("unexpected page: %+v", page)
	}

	firstRead, err := repo.MarkRead(ctx, chatID, userID, created.ID)
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if firstRead != created.ID {
		t.Fatalf("want read %d, got %d", created.ID, firstRead)
	}

	secondRead, err := repo.MarkRead(ctx, chatID, userID, 1)
	if err != nil {
		t.Fatalf("mark older: %v", err)
	}
	if secondRead != created.ID {
		t.Fatalf("read mark must not move backwards, got %d", secondRead)
	}

	hits, err := repo.SearchMessages(ctx, chatID, "созвон")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}

	if err := repo.DeleteMessage(ctx, chatID, created.ID, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	page, err = repo.ListMessage(ctx, chatID, int(^uint(0)>>1), 10)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(page) != 1 || page[0].Message != "" || page[0].DeletedAt == nil {
		t.Fatalf("deleted message must stay without text: %+v", page)
	}

	hits, err = repo.SearchMessages(ctx, chatID, "созвон")
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("deleted message must leave search, got %d", len(hits))
	}
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	var usersTable *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.users')`).Scan(&usersTable); err != nil {
		t.Fatalf("schema check: %v", err)
	}
	if usersTable == nil {
		applyMigrations(t, pool)
	}
	return pool
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}

	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		script, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		result := conn.Conn().PgConn().Exec(ctx, string(script))
		if _, err := result.ReadAll(); err != nil {
			t.Fatalf("migrate %s: %v", name, err)
		}
	}
}
