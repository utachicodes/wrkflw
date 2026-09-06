package gateway

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/utachicodes/wrkflw/server/internal/database"
	"github.com/utachicodes/wrkflw/server/internal/migrations"
)

func openTestDB(t *testing.T) *database.Pool {
	t.Helper()
	url := os.Getenv("WRKFLW_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set WRKFLW_TEST_DATABASE_URL to run gateway store tests")
	}
	db, err := database.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if _, err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func createTestUser(t *testing.T, ctx context.Context, db *database.Pool) string {
	t.Helper()
	email := fmt.Sprintf("gateway-%s-%d@wrkflw.test", strings.ToLower(t.Name()), time.Now().UnixNano())
	var id string
	if err := db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, 'test')
		RETURNING id::text
	`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestUpsertAndGetRoundTrip(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	store := NewStore(db)
	userID := createTestUser(t, ctx, db)

	input := Config{
		Channel: "telegram",
		Agent:   "codex",
		Telegram: TelegramConfig{
			BotToken:     "token-from-BotFather",
			AllowUserIDs: []int64{123},
		},
		Routes: []Route{{Thread: "telegram:dm:123", Agent: "claude"}},
	}
	stored, err := store.Upsert(ctx, userID, input)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Channel != "telegram" || stored.Telegram.BotToken != "token-from-BotFather" {
		t.Fatalf("stored = %#v", stored)
	}

	got, err := store.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Channel != "telegram" || len(got.Telegram.AllowUserIDs) != 1 || got.Telegram.AllowUserIDs[0] != 123 {
		t.Fatalf("got = %#v", got)
	}
	if len(got.Routes) != 1 || got.Routes[0].Agent != "claude" {
		t.Fatalf("routes = %#v", got.Routes)
	}
	if got.LastPulledAt != nil {
		t.Fatalf("lastPulledAt = %v, want nil before any pull", got.LastPulledAt)
	}
}

func TestGetReturnsEmptyConfigWhenUnset(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	store := NewStore(db)
	userID := createTestUser(t, ctx, db)

	got, err := store.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Channel != "" || got.Routes == nil {
		t.Fatalf("got = %#v, want zero config", got)
	}
}

func TestUpsertRejectsInvalidConfig(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	store := NewStore(db)
	userID := createTestUser(t, ctx, db)

	for _, input := range []Config{
		{Channel: "carrier-pigeon"},
		{Agent: "hal-9000"},
		{Telegram: TelegramConfig{BotToken: strings.Repeat("x", maxTokenChars+1)}},
	} {
		if _, err := store.Upsert(ctx, userID, input); err == nil {
			t.Fatalf("Upsert(%#v) succeeded, want error", input)
		}
	}
}

func TestOutboxEnqueuePollAndIsolation(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	store := NewStore(db)
	first := createTestUser(t, ctx, db)
	second := createTestUser(t, ctx, db)

	if _, err := store.Enqueue(ctx, first, "", "hello"); err == nil {
		t.Fatal("empty thread accepted")
	}
	if _, err := store.Enqueue(ctx, first, "telegram:dm:1", ""); err == nil {
		t.Fatal("empty body accepted")
	}
	firstMessage, err := store.Enqueue(ctx, first, "telegram:dm:1", "hello from the board")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enqueue(ctx, second, "telegram:dm:9", "other account"); err != nil {
		t.Fatal(err)
	}

	messages, err := store.Poll(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].ID != firstMessage.ID || messages[0].Body != "hello from the board" {
		t.Fatalf("poll = %#v", messages)
	}
	again, err := store.Poll(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second poll = %#v, want empty", again)
	}
	other, err := store.Poll(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 {
		t.Fatalf("other account poll = %#v, want its own message", other)
	}
}

func TestMarkPulledStampsAndIsolatesAccounts(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	store := NewStore(db)
	first := createTestUser(t, ctx, db)
	second := createTestUser(t, ctx, db)

	if _, err := store.Upsert(ctx, first, Config{Channel: "slack"}); err != nil {
		t.Fatal(err)
	}
	pulled, err := store.MarkPulled(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if pulled.Channel != "slack" || pulled.LastPulledAt == nil {
		t.Fatalf("pulled = %#v", pulled)
	}

	other, err := store.Get(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if other.Channel != "" || other.LastPulledAt != nil {
		t.Fatalf("other account config = %#v, want empty", other)
	}
}
