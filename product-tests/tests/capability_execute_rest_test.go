package tests_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/internal/runtime"
)

// ---- inline test fixture: minimal REST API ----

type fixtureItem struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

type fixtureStore struct {
	mu     sync.Mutex
	items  map[string]*fixtureItem
	nextID int
}

func newFixtureStore() *fixtureStore {
	s := &fixtureStore{items: make(map[string]*fixtureItem), nextID: 4}
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("item-%d", i)
		s.items[id] = &fixtureItem{ID: id, Name: fmt.Sprintf("Item %d", i)}
	}
	return s
}

func newRestFixtureHandler(store *fixtureStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		store.mu.Lock()
		defer store.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			items := make([]*fixtureItem, 0, len(store.items))
			for _, it := range store.items {
				items = append(items, it)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": items, "total": len(items), "page": 1, "pageSize": 20, "totalPages": 1,
			})
		case http.MethodPost:
			var inp struct {
				Name string   `json:"name"`
				Tags []string `json:"tags"`
			}
			if err := json.NewDecoder(r.Body).Decode(&inp); err != nil || inp.Name == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id := fmt.Sprintf("item-%d", store.nextID)
			store.nextID++
			it := &fixtureItem{ID: id, Name: inp.Name, Tags: inp.Tags}
			store.items[id] = it
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(it)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/items/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/items/"):]
		store.mu.Lock()
		defer store.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			it, ok := store.items[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(it)
		case http.MethodPut:
			var inp struct {
				Name string   `json:"name"`
				Tags []string `json:"tags"`
			}
			_ = json.NewDecoder(r.Body).Decode(&inp)
			it, ok := store.items[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			it.Name = inp.Name
			it.Tags = inp.Tags
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(it)
		case http.MethodDelete:
			_, ok := store.items[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(store.items, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/errors/unauthorized", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "unauthorized", "message": "authentication required"})
	})
	mux.HandleFunc("/errors/forbidden", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "forbidden", "message": "access denied"})
	})
	mux.HandleFunc("/errors/rate-limited", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "too_many_requests", "message": "rate limit exceeded"})
	})
	mux.HandleFunc("/errors/internal", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "internal_error", "message": "unexpected server error"})
	})

	var opMu sync.Mutex
	ops := map[string]map[string]any{}
	opNextID := 1
	mux.HandleFunc("/operations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		opMu.Lock()
		id := fmt.Sprintf("op-%d", opNextID)
		opNextID++
		op := map[string]any{"id": id, "status": "running", "progress": 50}
		ops[id] = op
		opMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(op)
	})
	mux.HandleFunc("/operations/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/operations/"):]
		opMu.Lock()
		op, ok := ops[id]
		opMu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(op)
	})

	return mux
}

// ---- test helpers ----

func writeFile(t *testing.T, dir, name, content string) string {
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

func restOpenAPIYAML(serverURL string) string {
	return `openapi: 3.1.0
info:
  title: Test API
  version: "1.0.0"
servers:
  - url: ` + serverURL + `
paths:
  /items:
    get:
      operationId: listItems
      tags: [items]
      parameters:
        - name: tag
          in: query
          schema: { type: string }
        - name: page
          in: query
          schema: { type: integer }
        - name: pageSize
          in: query
          schema: { type: integer }
      responses:
        "200":
          description: OK
    post:
      operationId: createItem
      tags: [items]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
                tags:
                  type: array
                  items: { type: string }
      responses:
        "201":
          description: Created
  /items/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema: { type: string }
    get:
      operationId: getItem
      tags: [items]
      responses:
        "200":
          description: OK
    put:
      operationId: updateItem
      tags: [items]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
      responses:
        "200":
          description: OK
    delete:
      operationId: deleteItem
      tags: [items]
      responses:
        "204":
          description: Deleted
  /errors/unauthorized:
    get:
      operationId: triggerUnauthorized
      tags: [errors]
      responses:
        "401":
          description: Unauthorized
  /errors/forbidden:
    get:
      operationId: triggerForbidden
      tags: [errors]
      responses:
        "403":
          description: Forbidden
  /errors/rate-limited:
    get:
      operationId: triggerRateLimited
      tags: [errors]
      responses:
        "429":
          description: Too Many Requests
  /errors/internal:
    get:
      operationId: triggerInternalError
      tags: [errors]
      responses:
        "500":
          description: Internal Error
  /operations:
    post:
      operationId: createOperation
      tags: [operations]
      responses:
        "202":
          description: Accepted
  /operations/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema: { type: string }
    get:
      operationId: getOperation
      tags: [operations]
      responses:
        "200":
          description: OK
`
}

func restCLIConfig(openapiPath string) string {
	return `{
  "cli": "1.0.0",
  "mode": { "default": "discover" },
  "sources": {
    "testapiSource": {
      "type": "openapi",
      "uri": "` + openapiPath + `",
      "enabled": true
    }
  },
  "services": {
    "testapi": {
      "source": "testapiSource",
      "alias": "testapi"
    }
  }
}`
}

func executeTool(t *testing.T, runtimeURL, configPath, toolID string, extra map[string]any) map[string]any {
	t.Helper()
	payload := map[string]any{
		"configPath": configPath,
		"toolId":     toolID,
	}
	for k, v := range extra {
		payload[k] = v
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(runtimeURL+"/v1/tools/execute", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("execute %s: %v", toolID, err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response for %s: %v", toolID, err)
	}
	return result
}

// ---- tests ----

func TestCapabilityExecuteREST(t *testing.T) {
	store := newFixtureStore()
	api := httptest.NewServer(newRestFixtureHandler(store))
	t.Cleanup(api.Close)

	dir := t.TempDir()
	openapiPath := writeFile(t, dir, "testapi.openapi.yaml", restOpenAPIYAML(api.URL))
	configPath := writeFile(t, dir, ".cli.json", restCLIConfig(openapiPath))

	srv := runtime.NewServer(runtime.Options{AuditPath: filepath.Join(dir, "audit.log")})
	runtimeSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(runtimeSrv.Close)

	t.Run("ListItems", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:listItems", nil)
		if got, ok := result["statusCode"].(float64); !ok || got != 200 {
			t.Fatalf("expected statusCode 200, got %v", result)
		}
	})

	t.Run("GetItem", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:getItem",
			map[string]any{"pathArgs": []string{"item-1"}})
		if got, ok := result["statusCode"].(float64); !ok || got != 200 {
			t.Fatalf("expected statusCode 200, got %v", result)
		}
	})

	t.Run("GetItemNotFound", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:getItem",
			map[string]any{"pathArgs": []string{"does-not-exist"}})
		if got, ok := result["statusCode"].(float64); !ok || got != 404 {
			t.Fatalf("expected statusCode 404, got %v", result)
		}
	})

	t.Run("CreateItem", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"name": "New Item", "tags": []string{"test"}})
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:createItem",
			map[string]any{"body": body})
		if got, ok := result["statusCode"].(float64); !ok || got != 201 {
			t.Fatalf("expected statusCode 201, got %v", result)
		}
	})

	t.Run("UpdateItem", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"name": "Updated Item"})
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:updateItem",
			map[string]any{"pathArgs": []string{"item-2"}, "body": body})
		if got, ok := result["statusCode"].(float64); !ok || got != 200 {
			t.Fatalf("expected statusCode 200, got %v", result)
		}
	})

	t.Run("DeleteItem", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:deleteItem",
			map[string]any{"pathArgs": []string{"item-3"}})
		if got, ok := result["statusCode"].(float64); !ok || got != 204 {
			t.Fatalf("expected statusCode 204, got %v", result)
		}
	})

	t.Run("PaginationViaFlags", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:listItems",
			map[string]any{"flags": map[string]string{"page": "1", "pageSize": "2"}})
		if got, ok := result["statusCode"].(float64); !ok || got != 200 {
			t.Fatalf("expected statusCode 200 for pagination, got %v", result)
		}
	})

	t.Run("FilterByTag", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:listItems",
			map[string]any{"flags": map[string]string{"tag": "missing-tag"}})
		if got, ok := result["statusCode"].(float64); !ok || got != 200 {
			t.Fatalf("expected statusCode 200 for tag filter, got %v", result)
		}
	})

	t.Run("ErrorUnauthorized", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:triggerUnauthorized", nil)
		if got, ok := result["statusCode"].(float64); !ok || got != 401 {
			t.Fatalf("expected statusCode 401, got %v", result)
		}
	})

	t.Run("ErrorForbidden", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:triggerForbidden", nil)
		if got, ok := result["statusCode"].(float64); !ok || got != 403 {
			t.Fatalf("expected statusCode 403, got %v", result)
		}
	})

	t.Run("ErrorRateLimited", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:triggerRateLimited", nil)
		if got, ok := result["statusCode"].(float64); !ok || got != 429 {
			t.Fatalf("expected statusCode 429, got %v", result)
		}
	})

	t.Run("ErrorInternalServer", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:triggerInternalError", nil)
		if got, ok := result["statusCode"].(float64); !ok || got != 500 {
			t.Fatalf("expected statusCode 500, got %v", result)
		}
	})

	t.Run("CreateOperation", func(t *testing.T) {
		result := executeTool(t, runtimeSrv.URL, configPath, "testapi:createOperation", nil)
		if got, ok := result["statusCode"].(float64); !ok || got != 202 {
			t.Fatalf("expected statusCode 202, got %v", result)
		}
		// Extract operation ID from body and poll it
		var body map[string]any
		if rawBody, ok := result["body"]; ok {
			bodyBytes, _ := json.Marshal(rawBody)
			_ = json.Unmarshal(bodyBytes, &body)
		}
		opID, _ := body["id"].(string)
		if opID == "" {
			t.Fatal("expected operation id in response body")
		}

		pollResult := executeTool(t, runtimeSrv.URL, configPath, "testapi:getOperation",
			map[string]any{"pathArgs": []string{opID}})
		if got, ok := pollResult["statusCode"].(float64); !ok || got != 200 {
			t.Fatalf("expected statusCode 200 for poll, got %v", pollResult)
		}
	})
}
