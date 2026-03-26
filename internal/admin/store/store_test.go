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

func TestStoreGetSource(t *testing.T) {
	store := NewTestStore(t)
	sourceID, err := store.CreateSource(context.Background(), domain.CreateSourceInput{
		Kind:        "openapi",
		DisplayName: "GitHub API",
	})
	if err != nil {
		t.Fatal(err)
	}

	source, err := store.GetSource(context.Background(), sourceID)
	if err != nil {
		t.Fatalf("expected to get source: %v", err)
	}
	if source.ID != sourceID {
		t.Errorf("expected ID %q, got %q", sourceID, source.ID)
	}
	if source.Kind != "openapi" {
		t.Errorf("expected kind %q, got %q", "openapi", source.Kind)
	}
	if source.DisplayName != "GitHub API" {
		t.Errorf("expected display name %q, got %q", "GitHub API", source.DisplayName)
	}
	if source.Status != "draft" {
		t.Errorf("expected status %q, got %q", "draft", source.Status)
	}
}

func TestStoreGetSourceNotFound(t *testing.T) {
	store := NewTestStore(t)
	_, err := store.GetSource(context.Background(), "src_nonexistent")
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestStoreListSources(t *testing.T) {
	store := NewTestStore(t)
	_, err := store.CreateSource(context.Background(), domain.CreateSourceInput{
		Kind:        "openapi",
		DisplayName: "GitHub API",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateSource(context.Background(), domain.CreateSourceInput{
		Kind:        "mcp",
		DisplayName: "Slack MCP",
	})
	if err != nil {
		t.Fatal(err)
	}

	sources, err := store.ListSources(context.Background())
	if err != nil {
		t.Fatalf("expected to list sources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
}

func TestStoreUpdateSourceStatus(t *testing.T) {
	store := NewTestStore(t)
	sourceID, err := store.CreateSource(context.Background(), domain.CreateSourceInput{
		Kind:        "openapi",
		DisplayName: "GitHub API",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = store.UpdateSourceStatus(context.Background(), sourceID, "validated")
	if err != nil {
		t.Fatalf("expected to update source status: %v", err)
	}

	source, err := store.GetSource(context.Background(), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if source.Status != "validated" {
		t.Errorf("expected status %q, got %q", "validated", source.Status)
	}
}
