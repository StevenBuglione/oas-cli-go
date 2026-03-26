package service

import (
	"context"
	"fmt"

	"github.com/StevenBuglione/open-cli/internal/admin/domain"
	"github.com/StevenBuglione/open-cli/internal/admin/store"
)

// Service provides business logic for admin control-plane operations
type Service struct {
	store *store.Store
}

// New creates a new Service with the given store
func New(store *store.Store) *Service {
	return &Service{store: store}
}

// CreateBundle creates a new bundle
func (s *Service) CreateBundle(ctx context.Context, input domain.CreateBundleInput) (string, error) {
	return s.store.CreateBundle(ctx, input)
}

// GetBundle retrieves a bundle by ID
func (s *Service) GetBundle(ctx context.Context, id string) (*domain.Bundle, error) {
	return s.store.GetBundle(ctx, id)
}

// ListBundles retrieves all bundles
func (s *Service) ListBundles(ctx context.Context) ([]domain.Bundle, error) {
	return s.store.ListBundles(ctx)
}

// UpdateBundle updates a bundle
func (s *Service) UpdateBundle(ctx context.Context, id string, input domain.UpdateBundleInput) error {
	return s.store.UpdateBundle(ctx, id, input)
}

// DeleteBundle deletes a bundle
func (s *Service) DeleteBundle(ctx context.Context, id string) error {
	return s.store.DeleteBundle(ctx, id)
}

// CreateBundleAssignment creates a new bundle assignment
func (s *Service) CreateBundleAssignment(ctx context.Context, input domain.CreateBundleAssignmentInput) (string, error) {
	// Validate principal type
	if input.PrincipalType != "user" && input.PrincipalType != "group" {
		return "", fmt.Errorf("invalid principal type: %q, must be 'user' or 'group'", input.PrincipalType)
	}
	return s.store.CreateBundleAssignment(ctx, input)
}

// ListBundleAssignments retrieves all assignments for a bundle
func (s *Service) ListBundleAssignments(ctx context.Context, bundleID string) ([]domain.BundleAssignment, error) {
	return s.store.ListBundleAssignments(ctx, bundleID)
}

// DeleteBundleAssignment deletes a bundle assignment
func (s *Service) DeleteBundleAssignment(ctx context.Context, id string) error {
	return s.store.DeleteBundleAssignment(ctx, id)
}

// CreateSource creates a new source
func (s *Service) CreateSource(ctx context.Context, input domain.CreateSourceInput) (*domain.Source, error) {
	id, err := s.store.CreateSource(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.store.GetSource(ctx, id)
}

// GetSource retrieves a source by ID
func (s *Service) GetSource(ctx context.Context, id string) (*domain.Source, error) {
	return s.store.GetSource(ctx, id)
}

// ListSources retrieves all sources
func (s *Service) ListSources(ctx context.Context) ([]domain.Source, error) {
	return s.store.ListSources(ctx)
}

// ValidateSource validates a source and returns validation results
func (s *Service) ValidateSource(ctx context.Context, id string) (*domain.ValidationResult, error) {
	source, err := s.store.GetSource(ctx, id)
	if err != nil {
		return nil, err
	}

	validator := NewValidator()
	result, err := validator.Validate(ctx, source)
	if err != nil {
		return nil, err
	}

	// Update source status based on validation result
	if result.Valid {
		if err := s.store.UpdateSourceStatus(ctx, id, "validated"); err != nil {
			return nil, err
		}
	}

	return result, nil
}
