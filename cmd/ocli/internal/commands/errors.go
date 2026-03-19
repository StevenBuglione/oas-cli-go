package commands

import "fmt"

// UserError holds a structured error with cause and suggestion for display.
type UserError struct {
	Err        string
	Cause      string
	Suggestion string
}

func (e *UserError) Error() string {
	return fmt.Sprintf("Error: %s\n\nCause: %s\n\nSuggestion: %s", e.Err, e.Cause, e.Suggestion)
}

// FormatError wraps an error with structured cause and suggestion text.
func FormatError(err error, cause, suggestion string) *UserError {
	return &UserError{Err: err.Error(), Cause: cause, Suggestion: suggestion}
}

// NewUserError creates a structured error from string components.
func NewUserError(msg, cause, suggestion string) *UserError {
	return &UserError{Err: msg, Cause: cause, Suggestion: suggestion}
}

// NewAuthError creates a user error for authentication failures.
func NewAuthError(cause, suggestion string) *UserError {
	return NewUserError("Authentication failed", cause, suggestion)
}

// NewBodyError creates a user error for invalid JSON body input.
func NewBodyError(cause string) *UserError {
	return NewUserError("Invalid request body",
		cause,
		"Body must be valid JSON. Use --body '{\"key\":\"value\"}' or --body @file.json")
}

// NewMCPError creates a user error for MCP transport failures.
func NewMCPError(cause string) *UserError {
	return NewUserError("MCP server error",
		cause,
		"Check that the MCP server is running and the transport config is correct")
}
