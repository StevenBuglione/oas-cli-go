package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"github.com/StevenBuglione/open-cli/internal/admin/domain"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

//go:embed schema.sql
var schema string

// Store provides persistence for admin control-plane state
type Store struct {
	db *sql.DB
}

// New creates a new Store with the given database connection
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// InitSchema initializes the database schema
func (s *Store) InitSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// CreateSource creates a new source and returns its ID
func (s *Store) CreateSource(ctx context.Context, input domain.CreateSourceInput) (string, error) {
	id := newID("src")
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_sources (id, kind, display_name, status, created_at, updated_at)
VALUES ($1, $2, $3, 'draft', $4, $5)
`, id, input.Kind, input.DisplayName, now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

// newID generates a new ID with the given prefix
func newID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.NewString())
}
