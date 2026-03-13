package discovery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/pkg/discovery"
)

func TestDiscoverAPICatalogFollowsNestedCatalogsAndReportsCycles(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/.well-known/api-catalog", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{
		  "linkset": [
		    { "href": %q, "rel": "item" },
		    { "href": %q, "rel": "api-catalog" }
		  ]
		}`, server.URL+"/services/tickets", server.URL+"/nested-catalog")
	})
	mux.HandleFunc("/nested-catalog", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{
		  "linkset": [
		    { "href": %q, "rel": "item" },
		    { "href": %q, "rel": "api-catalog" }
		  ]
		}`, server.URL+"/services/billing", server.URL+"/.well-known/api-catalog")
	})

	result, err := discovery.DiscoverAPICatalog(context.Background(), http.DefaultClient, server.URL+"/.well-known/api-catalog")
	if err != nil {
		t.Fatalf("DiscoverAPICatalog returned error: %v", err)
	}

	if len(result.Services) != 2 {
		t.Fatalf("expected 2 services, got %#v", result.Services)
	}
	if len(result.Provenance.Fetches) != 2 {
		t.Fatalf("expected 2 catalog fetches, got %#v", result.Provenance.Fetches)
	}
	if len(result.Warnings) == 0 || result.Warnings[0].Code != "api_catalog_cycle" {
		t.Fatalf("expected cycle warning, got %#v", result.Warnings)
	}
}

func TestDiscoverServiceRootUsesHeadThenFallbackToGet(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/service", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Link",
			fmt.Sprintf("<%s>; rel=\"service-desc\", <%s>; rel=\"service-meta\"",
				server.URL+"/openapi.json",
				server.URL+"/metadata.json",
			),
		)
		w.WriteHeader(http.StatusOK)
	})

	result, err := discovery.DiscoverServiceRoot(context.Background(), http.DefaultClient, server.URL+"/service")
	if err != nil {
		t.Fatalf("DiscoverServiceRoot returned error: %v", err)
	}

	if result.OpenAPIURL != server.URL+"/openapi.json" {
		t.Fatalf("expected openapi url, got %q", result.OpenAPIURL)
	}
	if result.MetadataURL != server.URL+"/metadata.json" {
		t.Fatalf("expected metadata url, got %q", result.MetadataURL)
	}
	if result.Provenance.Method != discovery.ProvenanceRFC8631 {
		t.Fatalf("expected RFC8631 provenance, got %q", result.Provenance.Method)
	}
}
