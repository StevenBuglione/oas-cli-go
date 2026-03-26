package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/StevenBuglione/open-cli/internal/admin/domain"
	_ "modernc.org/sqlite"
)

// NewTestStore creates an in-memory SQLite store for testing
func NewTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := New(db)
	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return store
}

func TestStoreCreatesSource(t *testing.T) {
	store := NewTestStore(t)
	sourceID, err := store.CreateSource(context.Background(), domain.CreateSourceInput{
		Kind:        "openapi",
		DisplayName: "GitHub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sourceID == "" {
		t.Fatal("expected source id")
	}

	var (
		id          string
		kind        string
		displayName string
		status      string
	)
	err = store.db.QueryRowContext(
		context.Background(),
		`SELECT id, kind, display_name, status FROM admin_sources WHERE id = $1`,
		sourceID,
	).Scan(&id, &kind, &displayName, &status)
	if err != nil {
		t.Fatalf("expected stored source row: %v", err)
	}
	if id != sourceID || kind != "openapi" || displayName != "GitHub" || status != "draft" {
		t.Fatalf("unexpected stored source row: id=%q kind=%q displayName=%q status=%q", id, kind, displayName, status)
	}
}

func TestStoreCreateSourceReturnsEmptyIDOnError(t *testing.T) {
	store := NewTestStore(t)
	if err := store.db.Close(); err != nil {
		t.Fatalf("close test db: %v", err)
	}

	sourceID, err := store.CreateSource(context.Background(), domain.CreateSourceInput{
		Kind:        "openapi",
		DisplayName: "GitHub",
	})
	if err == nil {
		t.Fatal("expected create source error")
	}
	if sourceID != "" {
		t.Fatalf("expected empty source id on error, got %q", sourceID)
	}
}
