package domain

import "time"

// Source represents an external API source (e.g., OpenAPI spec)
type Source struct {
	ID          string
	Kind        string
	DisplayName string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateSourceInput contains fields needed to create a source
type CreateSourceInput struct {
	Kind        string
	DisplayName string
}

// ValidationResult represents the result of validating a source
type ValidationResult struct {
	SourceID string
	Valid    bool
	Errors   []string
	Services []ServiceCandidate
	Tools    []ToolCandidate
}

// ServiceCandidate represents a service discovered from a source
type ServiceCandidate struct {
	Name        string
	Description string
	Endpoints   int
}

// ToolCandidate represents a tool discovered from a source
type ToolCandidate struct {
	Name        string
	Description string
}

// Bundle represents an access package that can be assigned to principals
type Bundle struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateBundleInput contains fields needed to create a bundle
type CreateBundleInput struct {
	Name        string
	Description string
}

// UpdateBundleInput contains fields to update a bundle
type UpdateBundleInput struct {
	Name        string
	Description string
}

// BundleAssignment represents a bundle assigned to a principal (user or group)
type BundleAssignment struct {
	ID            string
	BundleID      string
	PrincipalType string // "user" or "group"
	PrincipalID   string
	CreatedAt     time.Time
}

// CreateBundleAssignmentInput contains fields needed to create an assignment
type CreateBundleAssignmentInput struct {
	BundleID      string
	PrincipalType string
	PrincipalID   string
}
