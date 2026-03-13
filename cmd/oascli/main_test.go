package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/pkg/catalog"
)

func TestRootCommandInvokesRuntimeToolsAndSchemas(t *testing.T) {
	tool := catalog.Tool{
		ID:        "tickets:listTickets",
		ServiceID: "tickets",
		Method:    http.MethodGet,
		Path:      "/tickets",
		Group:     "tickets",
		Command:   "list",
		Flags: []catalog.Parameter{
			{Name: "state", OriginalName: "status", Location: "query"},
		},
	}
	view := catalog.EffectiveView{Name: "discover", Mode: "discover", Tools: []catalog.Tool{tool}}
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/catalog/effective":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"catalog": map[string]any{
					"services": []map[string]any{{"id": "tickets", "alias": "tickets"}},
					"tools":    []catalog.Tool{tool},
				},
				"view": view,
			})
		case "/v1/tools/execute":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"statusCode": 200,
				"body": map[string]any{
					"items": []map[string]any{{"id": "T-1"}},
				},
			})
		case "/v1/workflows/run":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"workflowId": "nightlySync",
				"steps":      []string{"listTickets"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer runtimeServer.Close()

	var stdout bytes.Buffer
	cmd, err := NewRootCommand(CommandOptions{
		RuntimeURL: runtimeServer.URL,
		ConfigPath: "/tmp/project/.cli.json",
		Stdout:     &stdout,
		Stderr:     &stdout,
	}, []string{"tickets", "tickets", "list", "--state", "open", "--format", "json"})
	if err != nil {
		t.Fatalf("NewRootCommand returned error: %v", err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"items"`)) {
		t.Fatalf("expected runtime response in stdout, got %s", stdout.String())
	}

	stdout.Reset()
	cmd, err = NewRootCommand(CommandOptions{
		RuntimeURL: runtimeServer.URL,
		ConfigPath: "/tmp/project/.cli.json",
		Stdout:     &stdout,
		Stderr:     &stdout,
	}, []string{"tool", "schema", "tickets:listTickets"})
	if err != nil {
		t.Fatalf("NewRootCommand returned error: %v", err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema command failed: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"id":"tickets:listTickets"`)) {
		t.Fatalf("expected tool schema output, got %s", stdout.String())
	}
}
