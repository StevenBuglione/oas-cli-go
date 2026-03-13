package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func testFetcher(t *testing.T, dir string, now *time.Time, client *http.Client) *Fetcher {
	t.Helper()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return NewFetcher(FetcherOptions{
		Store:  store,
		Client: client,
		Now: func() time.Time {
			return *now
		},
	})
}

func TestFetcherReturnsFreshHitWithoutNetwork(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	var hits int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte(`{"value":"cached"}`))
	}))
	t.Cleanup(server.Close)

	fetcher := testFetcher(t, t.TempDir(), &now, server.Client())

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/resource", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	first, err := fetcher.Fetch(request, FetchOptions{})
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if first.Outcome != OutcomeMiss {
		t.Fatalf("expected miss on first fetch, got %q", first.Outcome)
	}

	now = now.Add(30 * time.Minute)
	second, err := fetcher.Fetch(request, FetchOptions{})
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if second.Outcome != OutcomeFreshHit {
		t.Fatalf("expected fresh hit, got %q", second.Outcome)
	}
	if hits != 1 {
		t.Fatalf("expected exactly one network request, got %d", hits)
	}
}

func TestFetcherSendsIfNoneMatchAndUpdatesValidatedAtOn304(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	var sawConditional bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if match := r.Header.Get("If-None-Match"); match != "" {
			sawConditional = true
			if match != `"tickets-v1"` {
				t.Fatalf("expected If-None-Match \"tickets-v1\", got %q", match)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", `"tickets-v1"`)
		w.Header().Set("Cache-Control", "max-age=0")
		_, _ = w.Write([]byte(`{"tickets":[1]}`))
	}))
	t.Cleanup(server.Close)

	fetcher := testFetcher(t, t.TempDir(), &now, server.Client())

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/tickets", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	if _, err := fetcher.Fetch(request, FetchOptions{}); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}

	now = now.Add(time.Minute)
	result, err := fetcher.Fetch(request, FetchOptions{Policy: Policy{ForceRefresh: true}})
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if result.Outcome != OutcomeRevalidatedHit {
		t.Fatalf("expected revalidated hit, got %q", result.Outcome)
	}
	if !sawConditional {
		t.Fatalf("expected conditional request on revalidation")
	}
	if result.Metadata.LastValidatedAt.IsZero() {
		t.Fatalf("expected LastValidatedAt to be recorded")
	}
}

func TestFetcherFallsBackToStaleWhenOriginFailsAndPolicyAllows(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"tickets-v1"`)
		w.Header().Set("Cache-Control", "max-age=0")
		_, _ = w.Write([]byte(`{"tickets":[1]}`))
	}))

	fetcher := testFetcher(t, t.TempDir(), &now, server.Client())

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/tickets", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	if _, err := fetcher.Fetch(request, FetchOptions{}); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}

	server.Close()
	now = now.Add(time.Minute)
	result, err := fetcher.Fetch(request, FetchOptions{Policy: Policy{AllowStaleOnError: true, ForceRefresh: true}})
	if err != nil {
		t.Fatalf("Fetch after origin failure: %v", err)
	}
	if result.Outcome != OutcomeStaleHit {
		t.Fatalf("expected stale hit, got %q", result.Outcome)
	}
	if !result.Metadata.Stale {
		t.Fatalf("expected stale metadata marker")
	}
}

func TestFetcherDropsCorruptEntriesAndRefetches(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	var hits int
	dir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte(`{"tickets":[1]}`))
	}))
	t.Cleanup(server.Close)

	fetcher := testFetcher(t, dir, &now, server.Client())

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/tickets", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	options := FetchOptions{Key: "tickets"}
	if _, err := fetcher.Fetch(request, options); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}

	key := fetcher.cacheKey(request, options)
	if err := os.WriteFile(fetcher.store.metadataPath(key), []byte("{broken"), 0o644); err != nil {
		t.Fatalf("WriteFile corrupt metadata: %v", err)
	}

	now = now.Add(time.Minute)
	result, err := fetcher.Fetch(request, options)
	if err != nil {
		t.Fatalf("Fetch after corruption: %v", err)
	}
	if result.Outcome != OutcomeRefreshed {
		t.Fatalf("expected refreshed outcome after corruption, got %q", result.Outcome)
	}
	if hits != 2 {
		t.Fatalf("expected second network fetch after corruption, got %d hits", hits)
	}
}
