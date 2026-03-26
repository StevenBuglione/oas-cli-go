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
`, id, input.Kind, input.DisplayName, formatTime(now), formatTime(now))
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetSource retrieves a source by ID
func (s *Store) GetSource(ctx context.Context, id string) (*domain.Source, error) {
	var source domain.Source
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
SELECT id, kind, display_name, status, created_at, updated_at
FROM admin_sources
WHERE id = $1
`, id).Scan(&source.ID, &source.Kind, &source.DisplayName, &source.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	source.CreatedAt = parseTime(createdAt)
	source.UpdatedAt = parseTime(updatedAt)
	return &source, nil
}

// ListSources retrieves all sources
func (s *Store) ListSources(ctx context.Context) ([]domain.Source, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, kind, display_name, status, created_at, updated_at
FROM admin_sources
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []domain.Source
	for rows.Next() {
		var source domain.Source
		var createdAt, updatedAt string
		if err := rows.Scan(&source.ID, &source.Kind, &source.DisplayName, &source.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		source.CreatedAt = parseTime(createdAt)
		source.UpdatedAt = parseTime(updatedAt)
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

// UpdateSourceStatus updates the status of a source
func (s *Store) UpdateSourceStatus(ctx context.Context, id string, status string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE admin_sources
SET status = $1, updated_at = $2
WHERE id = $3
`, status, formatTime(time.Now()), id)
	return err
}

// CreateBundle creates a new bundle and returns its ID
func (s *Store) CreateBundle(ctx context.Context, input domain.CreateBundleInput) (string, error) {
	id := newID("bun")
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_bundles (id, name, description, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
`, id, input.Name, input.Description, formatTime(now), formatTime(now))
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetBundle retrieves a bundle by ID
func (s *Store) GetBundle(ctx context.Context, id string) (*domain.Bundle, error) {
	var bundle domain.Bundle
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, description, created_at, updated_at
FROM admin_bundles
WHERE id = $1
`, id).Scan(&bundle.ID, &bundle.Name, &bundle.Description, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	bundle.CreatedAt = parseTime(createdAt)
	bundle.UpdatedAt = parseTime(updatedAt)
	return &bundle, nil
}

// ListBundles retrieves all bundles
func (s *Store) ListBundles(ctx context.Context) ([]domain.Bundle, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, description, created_at, updated_at
FROM admin_bundles
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []domain.Bundle
	for rows.Next() {
		var bundle domain.Bundle
		var createdAt, updatedAt string
		if err := rows.Scan(&bundle.ID, &bundle.Name, &bundle.Description, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		bundle.CreatedAt = parseTime(createdAt)
		bundle.UpdatedAt = parseTime(updatedAt)
		bundles = append(bundles, bundle)
	}
	return bundles, rows.Err()
}

// UpdateBundle updates a bundle's fields
func (s *Store) UpdateBundle(ctx context.Context, id string, input domain.UpdateBundleInput) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
UPDATE admin_bundles
SET name = $1, description = $2, updated_at = $3
WHERE id = $4
`, input.Name, input.Description, formatTime(now), id)
	return err
}

// DeleteBundle deletes a bundle by ID
func (s *Store) DeleteBundle(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_bundles WHERE id = $1`, id)
	return err
}

// CreateBundleAssignment creates a new bundle assignment and returns its ID
func (s *Store) CreateBundleAssignment(ctx context.Context, input domain.CreateBundleAssignmentInput) (string, error) {
	id := newID("bas")
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO admin_bundle_assignments (id, bundle_id, principal_type, principal_id, created_at)
VALUES ($1, $2, $3, $4, $5)
`, id, input.BundleID, input.PrincipalType, input.PrincipalID, formatTime(now))
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListBundleAssignments retrieves all assignments for a bundle
func (s *Store) ListBundleAssignments(ctx context.Context, bundleID string) ([]domain.BundleAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, bundle_id, principal_type, principal_id, created_at
FROM admin_bundle_assignments
WHERE bundle_id = $1
ORDER BY created_at DESC
`, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []domain.BundleAssignment
	for rows.Next() {
		var assignment domain.BundleAssignment
		var createdAt string
		if err := rows.Scan(&assignment.ID, &assignment.BundleID, &assignment.PrincipalType, &assignment.PrincipalID, &createdAt); err != nil {
			return nil, err
		}
		assignment.CreatedAt = parseTime(createdAt)
		assignments = append(assignments, assignment)
	}
	return assignments, rows.Err()
}

// DeleteBundleAssignment deletes a bundle assignment by ID
func (s *Store) DeleteBundleAssignment(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_bundle_assignments WHERE id = $1`, id)
	return err
}

// newID generates a new ID with the given prefix
func newID(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, uuid.NewString())
}

// formatTime formats a time for storage
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// parseTime parses a stored time string
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
