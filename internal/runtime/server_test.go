package runtime_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/internal/runtime"
)

func writeRuntimeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestServerEnforcesCuratedViewExecutesAllowedToolsAndAuditsAttempts(t *testing.T) {
	dir := t.TempDir()
	var deleteCalls int

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": "T-1"}}})
		case http.MethodDelete:
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer api.Close()

	writeRuntimeFile(t, dir, "tickets.openapi.yaml", `
openapi: 3.1.0
info:
  title: Tickets API
  version: "1.0.0"
servers:
  - url: `+api.URL+`
paths:
  /tickets:
    get:
      operationId: listTickets
      tags: [tickets]
      parameters:
        - name: status
          in: query
          schema: { type: string }
      responses:
        "200":
          description: OK
  /tickets/{id}:
    delete:
      operationId: deleteTicket
      tags: [tickets]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      responses:
        "204":
          description: Deleted
`)
	configPath := writeRuntimeFile(t, dir, ".cli.json", `{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "ticketsSource": {
	      "type": "openapi",
	      "uri": "./tickets.openapi.yaml",
	      "enabled": true
	    }
	  },
	  "services": {
	    "tickets": {
	      "source": "ticketsSource",
	      "alias": "tickets"
	    }
	  },
	  "curation": {
	    "toolSets": {
	      "sandbox": {
	        "allow": ["tickets:listTickets"],
	        "deny": ["**"]
	      }
	    }
	  },
	  "agents": {
	    "profiles": {
	      "sandbox": {
	        "mode": "curated",
	        "toolSet": "sandbox"
	      }
	    },
	    "defaultProfile": "sandbox"
	  }
	}`)

	server := runtime.NewServer(runtime.Options{AuditPath: filepath.Join(dir, "audit.log")})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/v1/catalog/effective?config=" + configPath + "&agentProfile=sandbox")
	if err != nil {
		t.Fatalf("get effective catalog: %v", err)
	}
	defer resp.Body.Close()
	var effective map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&effective); err != nil {
		t.Fatalf("decode effective catalog: %v", err)
	}
	view := effective["view"].(map[string]any)
	tools := view["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected one curated tool, got %#v", tools)
	}

	denyBody := bytes.NewBufferString(`{
	  "configPath": "` + configPath + `",
	  "agentProfile": "sandbox",
	  "toolId": "tickets:deleteTicket",
	  "pathArgs": ["T-1"]
	}`)
	denyResp, err := http.Post(httpServer.URL+"/v1/tools/execute", "application/json", denyBody)
	if err != nil {
		t.Fatalf("deny execute request: %v", err)
	}
	if denyResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for denied tool, got %d", denyResp.StatusCode)
	}
	if deleteCalls != 0 {
		t.Fatalf("expected denied tool not to hit upstream, got %d delete calls", deleteCalls)
	}

	allowBody := bytes.NewBufferString(`{
	  "configPath": "` + configPath + `",
	  "agentProfile": "sandbox",
	  "toolId": "tickets:listTickets",
	  "flags": { "status": "open" }
	}`)
	allowResp, err := http.Post(httpServer.URL+"/v1/tools/execute", "application/json", allowBody)
	if err != nil {
		t.Fatalf("allow execute request: %v", err)
	}
	defer allowResp.Body.Close()
	if allowResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for allowed tool, got %d", allowResp.StatusCode)
	}

	auditResp, err := http.Get(httpServer.URL + "/v1/audit/events")
	if err != nil {
		t.Fatalf("get audit events: %v", err)
	}
	defer auditResp.Body.Close()
	var events []map[string]any
	if err := json.NewDecoder(auditResp.Body).Decode(&events); err != nil {
		t.Fatalf("decode audit events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %#v", events)
	}
}

func TestServerResolvesBearerAuthFromSecretReferences(t *testing.T) {
	dir := t.TempDir()
	if err := os.Setenv("TICKETS_TOKEN", "token-abc"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TICKETS_TOKEN") })

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token-abc" {
			t.Fatalf("expected bearer auth header, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer api.Close()

	writeRuntimeFile(t, dir, "tickets.openapi.yaml", `
openapi: 3.1.0
info:
  title: Tickets API
  version: "1.0.0"
servers:
  - url: `+api.URL+`
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
paths:
  /tickets:
    get:
      operationId: listTickets
      tags: [tickets]
      responses:
        "200":
          description: OK
`)
	configPath := writeRuntimeFile(t, dir, ".cli.json", `{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "ticketsSource": {
	      "type": "openapi",
	      "uri": "./tickets.openapi.yaml",
	      "enabled": true
	    }
	  },
	  "services": {
	    "tickets": {
	      "source": "ticketsSource",
	      "alias": "tickets"
	    }
	  },
	  "secrets": {
	    "bearerAuth": {
	      "type": "env",
	      "value": "TICKETS_TOKEN"
	    }
	  }
	}`)

	server := runtime.NewServer(runtime.Options{AuditPath: filepath.Join(dir, "audit.log")})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	allowBody := bytes.NewBufferString(`{
	  "configPath": "` + configPath + `",
	  "toolId": "tickets:listTickets"
	}`)
	allowResp, err := http.Post(httpServer.URL+"/v1/tools/execute", "application/json", allowBody)
	if err != nil {
		t.Fatalf("allow execute request: %v", err)
	}
	defer allowResp.Body.Close()
	if allowResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for allowed tool, got %d", allowResp.StatusCode)
	}
}

func TestServerResolvesExecSecretReferencesWhenAllowed(t *testing.T) {
	dir := t.TempDir()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token-from-exec" {
			t.Fatalf("expected bearer auth header from exec secret, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer api.Close()

	writeRuntimeFile(t, dir, "tickets.openapi.yaml", `
openapi: 3.1.0
info:
  title: Tickets API
  version: "1.0.0"
servers:
  - url: `+api.URL+`
security:
  - bearerAuth: []
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
paths:
  /tickets:
    get:
      operationId: listTickets
      tags: [tickets]
      responses:
        "200":
          description: OK
`)
	configPath := writeRuntimeFile(t, dir, ".cli.json", `{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "ticketsSource": {
	      "type": "openapi",
	      "uri": "./tickets.openapi.yaml",
	      "enabled": true
	    }
	  },
	  "services": {
	    "tickets": {
	      "source": "ticketsSource",
	      "alias": "tickets"
	    }
	  },
	  "policy": {
	    "allowExecSecrets": true
	  },
	  "secrets": {
	    "bearerAuth": {
	      "type": "exec",
	      "command": ["sh", "-lc", "printf token-from-exec"]
	    }
	  }
	}`)

	server := runtime.NewServer(runtime.Options{AuditPath: filepath.Join(dir, "audit.log")})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	allowBody := bytes.NewBufferString(`{
	  "configPath": "` + configPath + `",
	  "toolId": "tickets:listTickets"
	}`)
	allowResp, err := http.Post(httpServer.URL+"/v1/tools/execute", "application/json", allowBody)
	if err != nil {
		t.Fatalf("allow execute request: %v", err)
	}
	defer allowResp.Body.Close()
	if allowResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for allowed tool, got %d", allowResp.StatusCode)
	}
}
