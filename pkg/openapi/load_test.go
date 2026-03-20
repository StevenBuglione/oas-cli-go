package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenBuglione/open-cli/pkg/cache"
)

func TestLoadDocumentResolvesRemoteRelativeServersAgainstSpecURL(t *testing.T) {
	spec := []byte(`openapi: 3.1.0
info:
  title: Relative API
  version: "1.0.0"
servers:
  - url: /v1
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/specs/api.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(spec)
	}))
	defer server.Close()

	doc, err := LoadDocument(context.Background(), "", server.URL+"/specs/api.yaml", nil, nil, cache.Policy{})
	if err != nil {
		t.Fatalf("LoadDocument returned error: %v", err)
	}
	if len(doc.Document.Servers) != 1 {
		t.Fatalf("expected one normalized server, got %#v", doc.Document.Servers)
	}
	if got := doc.Document.Servers[0].URL; got != server.URL+"/v1" {
		t.Fatalf("expected normalized server %q, got %q", server.URL+"/v1", got)
	}
}

func TestLoadDocumentFallsBackToRemoteSpecOriginWhenServersMissing(t *testing.T) {
	spec := []byte(`openapi: 3.1.0
info:
  title: Origin API
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nested/api.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(spec)
	}))
	defer server.Close()

	doc, err := LoadDocument(context.Background(), "", server.URL+"/nested/api.yaml", nil, nil, cache.Policy{})
	if err != nil {
		t.Fatalf("LoadDocument returned error: %v", err)
	}
	if len(doc.Document.Servers) != 1 {
		t.Fatalf("expected one fallback server, got %#v", doc.Document.Servers)
	}
	if got := doc.Document.Servers[0].URL; got != server.URL {
		t.Fatalf("expected fallback origin %q, got %q", server.URL, got)
	}
}

func TestLoadDocumentPreservesMissingServersForFileSpecs(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "api.yaml")
	if err := os.WriteFile(specPath, []byte(`openapi: 3.1.0
info:
  title: Local API
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
`), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	doc, err := LoadDocument(context.Background(), dir, "./api.yaml", nil, nil, cache.Policy{})
	if err != nil {
		t.Fatalf("LoadDocument returned error: %v", err)
	}
	if len(doc.Document.Servers) != 0 {
		t.Fatalf("expected file-based spec to preserve missing servers, got %#v", doc.Document.Servers)
	}
}
