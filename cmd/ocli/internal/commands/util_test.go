package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimepkg "github.com/StevenBuglione/open-cli/cmd/ocli/internal/runtime"
	"github.com/StevenBuglione/open-cli/pkg/catalog"
)

func TestIsTerminalReaderReturnsFalseForNonFileReader(t *testing.T) {
	if IsTerminalReader(strings.NewReader("input")) {
		t.Fatal("expected non-file reader to not be treated as a terminal")
	}
}

func TestWriteOutputUnsupportedFormatReturnsUserError(t *testing.T) {
	err := WriteOutput(ioDiscard{}, "xml", map[string]string{"ok": "true"})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	userErr, ok := err.(*UserError)
	if !ok {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Err != `unsupported format "xml"` {
		t.Fatalf("unexpected error message: %q", userErr.Err)
	}
	if userErr.Cause != "The --format flag only accepts: json, yaml, pretty, table" {
		t.Fatalf("unexpected cause: %q", userErr.Cause)
	}
	if userErr.Suggestion != "Use --format json or --format table" {
		t.Fatalf("unexpected suggestion: %q", userErr.Suggestion)
	}
}

func TestFilterTools(t *testing.T) {
	tools := []catalog.Tool{
		{
			ID:        "tickets:listTickets",
			ServiceID: "tickets",
			Group:     "tickets",
			Command:   "list",
			Safety:    catalog.Safety{ReadOnly: true, Idempotent: true},
		},
		{
			ID:        "tickets:deleteTicket",
			ServiceID: "tickets",
			Group:     "tickets",
			Command:   "delete",
			Safety:    catalog.Safety{Destructive: true, RequiresApproval: true},
		},
		{
			ID:        "users:createUser",
			ServiceID: "users",
			Group:     "admin",
			Command:   "create",
		},
	}

	tests := []struct {
		name     string
		service  string
		group    string
		safety   string
		expected []string
	}{
		{name: "service", service: "tickets", expected: []string{"tickets:listTickets", "tickets:deleteTicket"}},
		{name: "group", group: "admin", expected: []string{"users:createUser"}},
		{name: "read-only", safety: "read-only", expected: []string{"tickets:listTickets"}},
		{name: "destructive", safety: "destructive", expected: []string{"tickets:deleteTicket"}},
		{name: "requires-approval", safety: "requires-approval", expected: []string{"tickets:deleteTicket"}},
		{name: "idempotent", safety: "idempotent", expected: []string{"tickets:listTickets"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterTools(tools, tt.service, tt.group, tt.safety)
			if ids := toolIDs(got); strings.Join(ids, ",") != strings.Join(tt.expected, ",") {
				t.Fatalf("expected %v, got %v", tt.expected, ids)
			}
		})
	}
}

func TestSearchToolsMatchesCaseInsensitiveFields(t *testing.T) {
	tools := []catalog.Tool{
		{ID: "tickets:listTickets", Command: "list", Summary: "List tickets", Description: "Browse all open tickets"},
		{ID: "users:createUser", Command: "create", Summary: "Create user", Description: "Provision a teammate"},
	}

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{name: "id", pattern: "LISTTICKETS", expected: []string{"tickets:listTickets"}},
		{name: "command", pattern: "CREATE", expected: []string{"users:createUser"}},
		{name: "summary", pattern: "browse", expected: []string{"tickets:listTickets"}},
		{name: "description", pattern: "teammate", expected: []string{"users:createUser"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchTools(tools, tt.pattern)
			if ids := toolIDs(got); strings.Join(ids, ",") != strings.Join(tt.expected, ",") {
				t.Fatalf("expected %v, got %v", tt.expected, ids)
			}
		})
	}
}

func TestReadConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cli.json")
	if err := os.WriteFile(path, []byte(`{"name":"demo","enabled":true}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	raw, err := readConfigFile(path)
	if err != nil {
		t.Fatalf("readConfigFile returned error: %v", err)
	}
	if raw["name"] != "demo" {
		t.Fatalf("expected name to be demo, got %#v", raw["name"])
	}
	if raw["enabled"] != true {
		t.Fatalf("expected enabled to be true, got %#v", raw["enabled"])
	}
}

func TestLoadBody(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		body, err := LoadBody("", strings.NewReader(""))
		if err != nil {
			t.Fatalf("LoadBody returned error: %v", err)
		}
		if body != nil {
			t.Fatalf("expected nil body, got %q", string(body))
		}
	})

	t.Run("stdin", func(t *testing.T) {
		body, err := LoadBody("-", strings.NewReader(`{"source":"stdin"}`))
		if err != nil {
			t.Fatalf("LoadBody returned error: %v", err)
		}
		assertJSONBody(t, body, `{"source":"stdin"}`)
	})

	t.Run("file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "body.json")
		if err := os.WriteFile(path, []byte(`{"source":"file"}`), 0o644); err != nil {
			t.Fatalf("write body file: %v", err)
		}
		body, err := LoadBody("@"+path, strings.NewReader(""))
		if err != nil {
			t.Fatalf("LoadBody returned error: %v", err)
		}
		assertJSONBody(t, body, `{"source":"file"}`)
	})

	t.Run("literal", func(t *testing.T) {
		body, err := LoadBody(`{"source":"literal"}`, strings.NewReader(""))
		if err != nil {
			t.Fatalf("LoadBody returned error: %v", err)
		}
		assertJSONBody(t, body, `{"source":"literal"}`)
	})

	t.Run("invalid", func(t *testing.T) {
		body, err := LoadBody(`{"source":`, strings.NewReader(""))
		if err == nil {
			t.Fatal("expected invalid JSON error")
		}
		if body != nil {
			t.Fatalf("expected nil body, got %q", string(body))
		}
		userErr, ok := err.(*UserError)
		if !ok {
			t.Fatalf("expected UserError, got %T", err)
		}
		if userErr.Err != "Invalid request body" {
			t.Fatalf("unexpected error title: %q", userErr.Err)
		}
		if userErr.Cause != "The provided body is not valid JSON" {
			t.Fatalf("unexpected cause: %q", userErr.Cause)
		}
	})
}

func TestErrorHelpers(t *testing.T) {
	authErr := NewAuthError("token expired", "Run ocli auth login")
	if authErr.Err != "Authentication failed" || authErr.Cause != "token expired" || authErr.Suggestion != "Run ocli auth login" {
		t.Fatalf("unexpected auth error: %#v", authErr)
	}

	bodyErr := NewBodyError("bad body")
	if bodyErr.Err != "Invalid request body" || bodyErr.Cause != "bad body" {
		t.Fatalf("unexpected body error: %#v", bodyErr)
	}
	if !strings.Contains(bodyErr.Suggestion, "Body must be valid JSON") {
		t.Fatalf("unexpected body suggestion: %q", bodyErr.Suggestion)
	}

	mcpErr := NewMCPError("connection dropped")
	if mcpErr.Err != "MCP server error" || mcpErr.Cause != "connection dropped" {
		t.Fatalf("unexpected MCP error: %#v", mcpErr)
	}
	if !strings.Contains(mcpErr.Suggestion, "MCP server") {
		t.Fatalf("unexpected MCP suggestion: %q", mcpErr.Suggestion)
	}

	wrapped := FormatError(errors.New("boom"), "because", "try again")
	if wrapped.Err != "boom" || wrapped.Cause != "because" || wrapped.Suggestion != "try again" {
		t.Fatalf("unexpected formatted error: %#v", wrapped)
	}
}

func TestWriteTableCatalogResponse(t *testing.T) {
	resp := runtimepkg.CatalogResponse{
		Catalog: catalog.NormalizedCatalog{
			Services: []catalog.Service{
				{ID: "tickets", Alias: "helpdesk"},
			},
		},
		View: catalog.EffectiveView{
			Tools: []catalog.Tool{
				{
					ServiceID:   "tickets",
					Group:       "tickets",
					Command:     "list",
					Method:      "GET",
					Summary:     "List tickets",
					Description: "Lists tickets",
				},
			},
		},
	}

	var out bytes.Buffer
	if err := WriteTable(&out, resp); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "SERVICE") || !strings.Contains(output, "helpdesk") || !strings.Contains(output, "List tickets") {
		t.Fatalf("unexpected table output: %q", output)
	}
}

func assertJSONBody(t *testing.T, body []byte, expected string) {
	t.Helper()
	var gotJSON any
	if err := json.Unmarshal(body, &gotJSON); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	var wantJSON any
	if err := json.Unmarshal([]byte(expected), &wantJSON); err != nil {
		t.Fatalf("expected JSON is invalid: %v", err)
	}
	if !jsonEqual(gotJSON, wantJSON) {
		t.Fatalf("expected %s, got %s", expected, string(body))
	}
}

func jsonEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(left, right)
}

func toolIDs(tools []catalog.Tool) []string {
	ids := make([]string, 0, len(tools))
	for _, tool := range tools {
		ids = append(ids, tool.ID)
	}
	return ids
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
