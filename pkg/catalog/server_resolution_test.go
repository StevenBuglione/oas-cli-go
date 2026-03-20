package catalog_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StevenBuglione/open-cli/pkg/catalog"
	"github.com/StevenBuglione/open-cli/pkg/config"
)

func TestBuildAppliesOperationPathAndDocumentServerPrecedence(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Precedence API
  version: "1.0.0"
servers:
  - url: /doc
paths:
  /items:
    servers:
      - url: /path
    get:
      operationId: listItems
      servers:
        - url: https://{region}.example.com/{version}
          variables:
            region:
              default: us
            version:
              default: v2
      responses:
        "200":
          description: OK
    post:
      operationId: createItem
      responses:
        "200":
          description: OK
  /status:
    get:
      operationId: getStatus
      responses:
        "200":
          description: OK
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/specs/api.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(spec))
	}))
	defer server.Close()

	cfg := config.Config{
		CLI:  "1.0.0",
		Mode: config.ModeConfig{Default: "discover"},
		Sources: map[string]config.Source{
			"api": {Type: "openapi", URI: server.URL + "/specs/api.yaml", Enabled: true},
		},
		Services: map[string]config.Service{
			"api": {Source: "api", Alias: "api"},
		},
	}

	ntc, err := catalog.Build(context.Background(), catalog.BuildOptions{Config: cfg})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	listItems := ntc.FindTool("api:listItems")
	if listItems == nil {
		t.Fatal("expected api:listItems tool")
	}
	if len(listItems.Servers) != 1 || listItems.Servers[0] != "https://us.example.com/v2" {
		t.Fatalf("expected operation-level server precedence, got %#v", listItems.Servers)
	}

	createItem := ntc.FindTool("api:createItem")
	if createItem == nil {
		t.Fatal("expected api:createItem tool")
	}
	if len(createItem.Servers) != 1 || createItem.Servers[0] != server.URL+"/path" {
		t.Fatalf("expected path-level server precedence, got %#v", createItem.Servers)
	}

	getStatus := ntc.FindTool("api:getStatus")
	if getStatus == nil {
		t.Fatal("expected api:getStatus tool")
	}
	if len(getStatus.Servers) != 1 || getStatus.Servers[0] != server.URL+"/doc" {
		t.Fatalf("expected document-level server precedence, got %#v", getStatus.Servers)
	}
}

func TestBuildFailsWhenServerVariableDefaultMissing(t *testing.T) {
	spec := `openapi: 3.1.0
info:
  title: Broken API
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      servers:
        - url: https://{region}.example.com
          variables:
            region: {}
      responses:
        "200":
          description: OK
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/specs/api.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(spec))
	}))
	defer server.Close()

	cfg := config.Config{
		CLI:  "1.0.0",
		Mode: config.ModeConfig{Default: "discover"},
		Sources: map[string]config.Source{
			"api": {Type: "openapi", URI: server.URL + "/specs/api.yaml", Enabled: true},
		},
		Services: map[string]config.Service{
			"api": {Source: "api", Alias: "api"},
		},
	}

	if _, err := catalog.Build(context.Background(), catalog.BuildOptions{Config: cfg}); err == nil {
		t.Fatal("expected Build to fail when a server variable default is missing")
	}
}
